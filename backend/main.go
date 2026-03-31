package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"smart-community/pkg/db"
	"smart-community/pkg/handlers"
	"smart-community/pkg/models"
	"smart-community/pkg/jobs"
)

func main() {
	// 初始化数据库（自动建库和迁移表）
	if err := db.Init(); err != nil {
		log.Fatalf("failed to init database: %v", err)
	}

	// 定时更新：到达截止时间但尚未结算的提案，将 prop_status 从 0(进行中) 更新为 1(已截至但未结算)
	// 该逻辑不调用合约 resolve，只更新链下状态，供前端展示。
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			now := time.Now()
			if err := db.DB.Model(&models.Proposal{}).
				Where("prop_status = ? AND deadline <= ?", 0, now).
				Update("prop_status", 1).Error; err != nil {
				log.Printf("[deadline-status-sync] update failed: %v", err)
			}
		}
	}()

	// 定时回写：前端触发链上 resolve 后，可能尚未更新链下 prop_status
	// 因此每 30 秒查询一次链上 getProposal，若 tallied=true 则把链下 prop_status 更新为 2/3
	jobs.StartSettlementStatusPoll(context.Background(), 30*time.Second)

	engine := gin.Default()

	// 提案图片静态资源目录：backend/resources
	_ = os.MkdirAll("backend/resources", 0755)
	engine.Static("/resources", "backend/resources")

	// CORS：允许前端本地开发访问，并正确处理预检 OPTIONS（避免 404）
	engine.Use(cors.New(cors.Config{
		AllowOrigins: []string{
			"http://localhost:5173",
			"http://127.0.0.1:5173",
		},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "X-Wallet-Addr", "X-Admin-Addr"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// 基础健康检查
	engine.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "smart community backend running",
		})
	})

	api := engine.Group("/api")
	{
		// 身份认证相关
		api.POST("/identity/register", handlers.RegisterIdentity)
		api.GET("/identity/me", handlers.GetCurrentIdentity)

		// 管理员审核相关
		admin := api.Group("/admin")
		{
			admin.GET("/identity/pending", handlers.ListPendingIdentities)
			admin.GET("/identity/approved", handlers.ListApprovedIdentities)
			admin.POST("/identity/:id/approve", handlers.ApproveIdentity)
			admin.POST("/identity/:id/reject", handlers.RejectIdentity)
			admin.POST("/identity/:id/remove", handlers.RemoveIdentityFromWhitelist)
			admin.POST("/contract/pause-log", handlers.LogPauseToggle)
			admin.DELETE("/proposals/:id", handlers.DeleteProposal)
		}

		// 提案与投票
		api.POST("/proposals", handlers.CreateProposal)
		api.GET("/proposals", handlers.ListProposals)
		api.GET("/proposals/:id", handlers.GetProposalDetail)
		api.POST("/proposals/:id/votes", handlers.RecordVote)
		api.GET("/proposals/:id/votes", handlers.ListVotesByProposal)
	}

	if err := engine.Run(":8080"); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

