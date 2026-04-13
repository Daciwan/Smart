package handlers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"smart-community/pkg/db"
	"smart-community/pkg/models"
)

type CreateProposalRequest struct {
	PropTitle  string `json:"propTitle" binding:"required,min=4,max=50"`
	PropDesc   string `json:"propDesc" binding:"required,max=2000"`
	PropType   int    `json:"propType" binding:"oneof=0 1"` // 0:一人一票 1:面积加权
	Deadline   string `json:"deadline" binding:"required"` // YYYY-MM-DD HH:mm:ss
	CreatorAddr string `json:"creatorAddr" binding:"required"`
	PropID     uint   `json:"propId"`  // 链上生成的 ProposalID，创建合约成功后由前端回填
	TxHash     string `json:"txHash"`  // 提案创建交易哈希
}

type ProposalResponse struct {
	models.Proposal
	ImagePaths []string `json:"imagePaths"`
}

// CreateProposal 发起社区提案 (链下部分：存储文本并生成 ContentHash)
func CreateProposal(c *gin.Context) {
	req, files, err := bindCreateProposalRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 前端传入的是不带时区的本地时间字符串，需要按本地时区解析
	deadline, err := time.ParseInLocation("2006-01-02 15:04:05", req.Deadline, time.Local)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid deadline format"})
		return
	}
	if deadline.Before(time.Now()) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "deadline must be in the future"})
		return
	}

	// 计算内容哈希（链下文本 -> keccak256）
	hash := crypto.Keccak256Hash([]byte(req.PropDesc))
	contentHash := hash.Hex() // 0x 开头 66 字符

	prop := models.Proposal{
		PropTitle:   strings.TrimSpace(req.PropTitle),
		PropDesc:    strings.TrimSpace(req.PropDesc),
		ContentHash: contentHash,
		PropType:    req.PropType,
		CreatorAddr: strings.ToLower(strings.TrimSpace(req.CreatorAddr)),
		Deadline:    deadline,
		PropStatus:  0,
		PropID:      req.PropID,
		TxHash:      strings.TrimSpace(req.TxHash),
	}

	if err := db.DB.Create(&prop).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proposal: " + err.Error()})
		return
	}

	imagePaths, err := saveProposalImages(prop.PropID, files)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save proposal images: " + err.Error()})
		return
	}

	if len(imagePaths) > 0 {
		pathsJSON, _ := json.Marshal(imagePaths)
		img := models.ProposalImage{
			PropID:     prop.PropID,
			ImagePaths: string(pathsJSON),
		}
		if err := db.DB.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "prop_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"image_paths", "update_time"}),
		}).Create(&img).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save proposal image paths: " + err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"message":      "提案已创建（链下部分），请在前端调用合约完成上链",
		"proposal":     ProposalResponse{Proposal: prop, ImagePaths: imagePaths},
		"contentHash":  contentHash,
	})
}

// ListProposals 提案列表查询与公示 (链下部分)
func ListProposals(c *gin.Context) {
	statusStr := c.Query("status")

	var props []models.Proposal
	query := db.DB.Model(&models.Proposal{})
	if statusStr != "" {
		status, err := strconv.Atoi(statusStr)
		if err == nil {
			query = query.Where("prop_status = ?", status)
		}
	}

	if err := query.Order("create_time desc").Find(&props).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query proposals: " + err.Error()})
		return
	}

	resp, err := buildProposalResponses(props)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build proposal response: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetProposalDetail 提案详情浏览（链下部分）
func GetProposalDetail(c *gin.Context) {
	id := c.Param("id")

	var prop models.Proposal
	if err := db.DB.First(&prop, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proposal not found"})
		return
	}

	resp, err := buildProposalResponses([]models.Proposal{prop})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build proposal detail: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp[0])
}

type RecordVoteRequest struct {
	PropID     uint    `json:"propId" binding:"required"`
	VoterAddr  string  `json:"voterAddr" binding:"required"`
	VoteChoice int     `json:"voteChoice" binding:"required"` // 1,2,3
	VoteWeight float64 `json:"voteWeight" binding:"required,gt=0"`
	VoteTxHash string  `json:"voteTxHash" binding:"required"`
}

// RecordVote 记录投票链下缓存，链上实际投票由前端调用合约完成。
func RecordVote(c *gin.Context) {
	_ = c.Param("id") // 兼容 RESTful 路径，但实际以 body 中的 propId 为准

	var req RecordVoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	record := models.VoteRecord{
		VoteTxHash: strings.TrimSpace(req.VoteTxHash),
		PropID:     req.PropID,
		VoterAddr:  strings.ToLower(strings.TrimSpace(req.VoterAddr)),
		VoteChoice: req.VoteChoice,
		VoteWeight: req.VoteWeight,
	}

	if err := db.DB.Create(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save vote record: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "投票记录已缓存（链下）",
		"record":  record,
	})
}

// ListVotesByProposal 查询某个提案的所有链下投票记录。
func ListVotesByProposal(c *gin.Context) {
	id := c.Param("id")
	var records []models.VoteRecord

	if err := db.DB.Where("prop_id = ?", id).Order("vote_time asc").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query votes: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, records)
}

func bindCreateProposalRequest(c *gin.Context) (CreateProposalRequest, []*multipart.FileHeader, error) {
	contentType := c.ContentType()
	if strings.Contains(contentType, "multipart/form-data") {
		req := CreateProposalRequest{
			PropTitle:   strings.TrimSpace(c.PostForm("propTitle")),
			PropDesc:    strings.TrimSpace(c.PostForm("propDesc")),
			Deadline:    strings.TrimSpace(c.PostForm("deadline")),
			CreatorAddr: strings.TrimSpace(c.PostForm("creatorAddr")),
			TxHash:      strings.TrimSpace(c.PostForm("txHash")),
		}
		propType, _ := strconv.Atoi(strings.TrimSpace(c.PostForm("propType")))
		req.PropType = propType
		propID, _ := strconv.Atoi(strings.TrimSpace(c.PostForm("propId")))
		if propID > 0 {
			req.PropID = uint(propID)
		}

		if req.PropTitle == "" || req.PropDesc == "" || req.Deadline == "" || req.CreatorAddr == "" {
			return CreateProposalRequest{}, nil, fmt.Errorf("missing required fields")
		}
		if len(req.PropTitle) < 4 || len(req.PropTitle) > 50 {
			return CreateProposalRequest{}, nil, fmt.Errorf("propTitle length must be 4-50")
		}
		if req.PropType != 0 && req.PropType != 1 {
			return CreateProposalRequest{}, nil, fmt.Errorf("propType must be 0 or 1")
		}

		var files []*multipart.FileHeader
		form, err := c.MultipartForm()
		if err == nil && form != nil {
			files = form.File["images"]
		}
		return req, files, nil
	}

	var req CreateProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return CreateProposalRequest{}, nil, err
	}
	return req, nil, nil
}

func saveProposalImages(propID uint, files []*multipart.FileHeader) ([]string, error) {
	if len(files) == 0 {
		return []string{}, nil
	}
	dir := filepath.Join("backend", "resources", "proposals", strconv.Itoa(int(propID)))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	rand.Seed(time.Now().UnixNano())
	out := make([]string, 0, len(files))
	for _, f := range files {
		if f == nil {
			continue
		}
		ext := strings.ToLower(filepath.Ext(f.Filename))
		if ext == "" {
			ext = ".jpg"
		}
		filename := fmt.Sprintf("%d_%06d%s", time.Now().UnixNano(), rand.Intn(1000000), ext)
		dst := filepath.Join(dir, filename)
		if err := saveMultipartFile(f, dst); err != nil {
			return nil, err
		}
		out = append(out, "/resources/proposals/"+strconv.Itoa(int(propID))+"/"+filename)
	}
	return out, nil
}

func saveMultipartFile(file *multipart.FileHeader, dst string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = out.ReadFrom(src)
	return err
}

func buildProposalResponses(props []models.Proposal) ([]ProposalResponse, error) {
	if len(props) == 0 {
		return []ProposalResponse{}, nil
	}
	ids := make([]uint, 0, len(props))
	for _, p := range props {
		ids = append(ids, p.PropID)
	}

	var imgs []models.ProposalImage
	if err := db.DB.Where("prop_id IN ?", ids).Find(&imgs).Error; err != nil {
		return nil, err
	}
	pathMap := map[uint][]string{}
	for _, it := range imgs {
		var arr []string
		_ = json.Unmarshal([]byte(it.ImagePaths), &arr)
		pathMap[it.PropID] = arr
	}

	out := make([]ProposalResponse, 0, len(props))
	for _, p := range props {
		out = append(out, ProposalResponse{
			Proposal:   p,
			ImagePaths: pathMap[p.PropID],
		})
	}
	return out, nil
}

// DeleteProposal 管理员删除链下提案记录
func DeleteProposal(c *gin.Context) {
	id := c.Param("id")
	adminAddr := strings.ToLower(strings.TrimSpace(c.GetHeader("X-Admin-Addr")))
	
	if adminAddr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing admin address"})
		return
	}

	// 1. 确认提案存在
	var prop models.Proposal
	if err := db.DB.First(&prop, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proposal not found"})
		return
	}

	// 2. 删除提案记录 (硬删除)
	if err := db.DB.Delete(&models.Proposal{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete proposal: " + err.Error()})
		return
	}

	// 3. 将删除操作写入 sys_configs 表
	logEntry := models.SysConfig{
		ParamName:  "DELETE_PROPOSAL",
		ParamValue: fmt.Sprintf("Deleted PropID: %d, Title: %s", prop.PropID, prop.PropTitle),
		AdminAddr:  adminAddr,
	}
	if err := db.DB.Create(&logEntry).Error; err != nil {
		// 日志写入失败不影响主流程，但打印记录
		fmt.Printf("failed to write sys config log: %v\n", err)
	}

	c.JSON(http.StatusOK, gin.H{"message": "proposal deleted successfully"})
}

// GetUserVotes 获取特定用户的投票历史
func GetUserVotes(c *gin.Context) {
	addr := strings.ToLower(c.GetHeader("X-Wallet-Addr"))
	if addr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing wallet address"})
		return
	}

	type VoteResult struct {
		models.VoteRecord
		PropTitle string `json:"propTitle"`
	}

	var results []VoteResult
	// 关联查询提案标题
	err := db.DB.Table("vote_records").
		Select("vote_records.*, proposals.prop_title").
		Joins("left join proposals on vote_records.prop_id = proposals.prop_id").
		Where("vote_records.voter_addr = ?", addr).
		Order("vote_records.vote_time desc").
		Scan(&results).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch votes"})
		return
	}

	c.JSON(http.StatusOK, results)
}