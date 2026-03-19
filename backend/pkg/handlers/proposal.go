package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"

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

// CreateProposal 发起社区提案 (链下部分：存储文本并生成 ContentHash)
func CreateProposal(c *gin.Context) {
	var req CreateProposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
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

	c.JSON(http.StatusOK, gin.H{
		"message":      "提案已创建（链下部分），请在前端调用合约完成上链",
		"proposal":     prop,
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

	c.JSON(http.StatusOK, props)
}

// GetProposalDetail 提案详情浏览（链下部分）
func GetProposalDetail(c *gin.Context) {
	id := c.Param("id")

	var prop models.Proposal
	if err := db.DB.First(&prop, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "proposal not found"})
		return
	}

	c.JSON(http.StatusOK, prop)
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

