package handlers

import (
	"fmt"
	"log"
	"strings"

	"github.com/gin-gonic/gin"

	"smart-community/pkg/db"
	"smart-community/pkg/models"
)

// logSysEvent 将管理操作写入 SysConfig 表，作为简单的系统日志。
func logSysEvent(paramName, paramValue, adminAddr string) {
	cfg := models.SysConfig{
		ParamName:  paramName,
		ParamValue: paramValue,
		AdminAddr:  strings.ToLower(strings.TrimSpace(adminAddr)),
	}
	if err := db.DB.Create(&cfg).Error; err != nil {
		log.Printf("failed to log sys event %s: %v", paramName, err)
	}
}

type PauseLogRequest struct {
	Paused bool   `json:"paused"`
	TxHash string `json:"txHash"`
}

// LogPauseToggle 记录合约暂停/恢复操作到 SysConfig。
func LogPauseToggle(c *gin.Context) {
	var req PauseLogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	adminAddr := c.GetHeader("X-Admin-Addr")
	val := fmt.Sprintf("paused=%t tx=%s", req.Paused, strings.TrimSpace(req.TxHash))
	logSysEvent("PAUSE_TOGGLE", val, adminAddr)

	c.JSON(200, gin.H{"message": "pause state logged"})
}

