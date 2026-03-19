package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"smart-community/pkg/db"
	"smart-community/pkg/models"
)

type RegisterIdentityRequest struct {
	WalletAddr string  `json:"walletAddr" binding:"required"`
	RealName   string  `json:"realName" binding:"required"`
	IDCard4    string  `json:"idCard4" binding:"required,len=4"`
	BuildNo    string  `json:"buildNo" binding:"required"`
	UnitNo     string  `json:"unitNo" binding:"required"`
	RoomNo     string  `json:"roomNo" binding:"required"`
	HouseArea  float64 `json:"houseArea" binding:"required,gt=0"`
	PhoneNo    string  `json:"phoneNo" binding:"required,len=11"`
}

// RegisterIdentity 身份信息提交与注册 (SRS_USRM01.02)
func RegisterIdentity(c *gin.Context) {
	var req RegisterIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	addr := strings.TrimSpace(req.WalletAddr)
	if !strings.HasPrefix(addr, "0x") || len(addr) != 42 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid wallet address"})
		return
	}
	addr = strings.ToLower(addr)

	var existing models.User
	err := db.DB.Where("wallet_addr = ?", addr).First(&existing).Error
	if err == nil {
		// 该钱包已存在
		switch existing.AuthStatus {
		case 1:
			c.JSON(http.StatusBadRequest, gin.H{"error": "该钱包已认证，无需重复提交"})
			return
		case 0:
			c.JSON(http.StatusBadRequest, gin.H{"error": "您已提交过，请等待管理员审核"})
			return
		case 2:
			// 被驳回用户允许重新提交：更新原记录为审核中并刷新信息
			existing.RealName = strings.TrimSpace(req.RealName)
			existing.IDCard4 = strings.TrimSpace(req.IDCard4)
			existing.BuildNo = strings.TrimSpace(req.BuildNo)
			existing.UnitNo = strings.TrimSpace(req.UnitNo)
			existing.RoomNo = strings.TrimSpace(req.RoomNo)
			existing.HouseArea = req.HouseArea
			existing.PhoneNo = strings.TrimSpace(req.PhoneNo)
			existing.AuthStatus = 0
			existing.Remark = ""
			if err := db.DB.Save(&existing).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "更新申请失败: " + err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"message": "已重新提交，请等待管理员审核",
				"user":    existing,
			})
			return
		}
	}

	user := models.User{
		WalletAddr: addr,
		RealName:   strings.TrimSpace(req.RealName),
		IDCard4:    strings.TrimSpace(req.IDCard4),
		BuildNo:    strings.TrimSpace(req.BuildNo),
		UnitNo:     strings.TrimSpace(req.UnitNo),
		RoomNo:     strings.TrimSpace(req.RoomNo),
		HouseArea:  req.HouseArea,
		PhoneNo:    strings.TrimSpace(req.PhoneNo),
		AuthStatus: 0, // 审核中
	}

	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "提交成功，请等待管理员审核",
		"user":    user,
	})
}

// GetCurrentIdentity 查询当前钱包地址对应的身份信息。
// 前端通过请求头 X-Wallet-Addr 传入当前连接的钱包地址。
func GetCurrentIdentity(c *gin.Context) {
	addr := strings.TrimSpace(c.GetHeader("X-Wallet-Addr"))
	if addr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "X-Wallet-Addr header required"})
		return
	}
	addr = strings.ToLower(addr)

	var user models.User
	if err := db.DB.Where("wallet_addr = ?", addr).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "identity not found"})
		return
	}

	c.JSON(http.StatusOK, user)
}

// ListPendingIdentities 管理员查看待审核列表。
func ListPendingIdentities(c *gin.Context) {
	var users []models.User
	if err := db.DB.Where("auth_status = ?", 0).Order("reg_time asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query users: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

// ListApprovedIdentities 管理员查看已认证（白名单）用户列表（链下视角）。
func ListApprovedIdentities(c *gin.Context) {
	var users []models.User
	if err := db.DB.Where("auth_status = ?", 1).Order("reg_time asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query users: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, users)
}

type reviewRequest struct {
	VoteWeight float64 `json:"voteWeight" binding:"omitempty,gt=0"`
	Remark     string  `json:"remark"`
	TxHash     string  `json:"txHash"`
}

// ApproveIdentity 管理员审核通过（链下），并设置投票权重。
// 实际白名单上链与权重写入由前端通过 MetaMask 调用合约完成。
func ApproveIdentity(c *gin.Context) {
	id := c.Param("id")

	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil && !strings.Contains(err.Error(), "EOF") {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if req.VoteWeight > 0 {
		user.VoteWeight = req.VoteWeight
	} else {
		user.VoteWeight = user.HouseArea
	}
	user.AuthStatus = 1
	user.Remark = strings.TrimSpace(req.Remark)

	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user: " + err.Error()})
		return
	}

	// 记录白名单添加日志
	adminAddr := c.GetHeader("X-Admin-Addr")
	logSysEvent("WHITELIST_ADD", "wallet="+user.WalletAddr+" tx="+strings.TrimSpace(req.TxHash), adminAddr)

	c.JSON(http.StatusOK, gin.H{
		"message": "审核通过（链下），请在前端调用合约完成白名单上链",
		"user":    user,
	})
}

// RejectIdentity 管理员审核驳回。
func RejectIdentity(c *gin.Context) {
	id := c.Param("id")

	var req reviewRequest
	if err := c.ShouldBindJSON(&req); err != nil && !strings.Contains(err.Error(), "EOF") {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	user.AuthStatus = 2
	user.Remark = strings.TrimSpace(req.Remark)

	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user: " + err.Error()})
		return
	}

	// 记录驳回日志
	adminAddr := c.GetHeader("X-Admin-Addr")
	logSysEvent("WHITELIST_REJECT", "wallet="+user.WalletAddr+" reason="+strings.TrimSpace(req.Remark), adminAddr)

	c.JSON(http.StatusOK, gin.H{
		"message": "审核已驳回",
		"user":    user,
	})
}

// RemoveIdentityFromWhitelist 移除白名单用户（链下）。
// 通常与前端调用合约 setVoter(address, weight, false) 同时使用。
func RemoveIdentityFromWhitelist(c *gin.Context) {
	id := c.Param("id")

	var user models.User
	if err := db.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	user.AuthStatus = 2
	user.VoteWeight = 0
	if user.Remark == "" {
		user.Remark = "已从白名单移除"
	} else {
		user.Remark = strings.TrimSpace(user.Remark) + "（已从白名单移除）"
	}

	if err := db.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update user: " + err.Error()})
		return
	}

	// 记录白名单移除日志
	adminAddr := c.GetHeader("X-Admin-Addr")
	logSysEvent("WHITELIST_REMOVE", user.WalletAddr, adminAddr)

	c.JSON(http.StatusOK, gin.H{
		"message": "用户已从白名单移除（链下）",
		"user":    user,
	})
}


