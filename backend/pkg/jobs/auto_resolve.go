package jobs

import (
	"context"
	"fmt"
	"log"
	"time"

	"smart-community/pkg/chain"
	"smart-community/pkg/db"
	"smart-community/pkg/models"
)

// StartAutoResolver 启动自动结算任务：
// - 周期性扫描数据库中已到期且未裁决的提案
// - 使用管理员私钥自动调用合约 resolve(proposalId)
// - 交易确认后读取链上状态回写数据库，并写 sys_configs 日志
func StartAutoResolver(ctx context.Context, interval time.Duration) {
	gov, err := chain.NewGovernorClientFromEnv()
	if err != nil {
		log.Printf("[auto-resolve] disabled: %v", err)
		return
	}

	log.Printf("[auto-resolve] started. from=%s interval=%s", gov.FromAddress(), interval)

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Printf("[auto-resolve] stopped: %v", ctx.Err())
				return
			case <-ticker.C:
				runOnce(ctx, gov)
			}
		}
	}()
}

func runOnce(ctx context.Context, gov *chain.GovernorClient) {
	now := time.Now()
	var props []models.Proposal

	// 仅处理：链上 prop_id > 0 且链下标记仍为进行中(0)，并且已到期
	if err := db.DB.Where("prop_status = ? AND prop_id > ? AND deadline <= ?", 0, 0, now).
		Order("deadline asc").Limit(50).Find(&props).Error; err != nil {
		log.Printf("[auto-resolve] query proposals failed: %v", err)
		return
	}

	if len(props) == 0 {
		return
	}

	for _, p := range props {
		select {
		case <-ctx.Done():
			return
		default:
		}

		proposalID := uint64(p.PropID)
		opCtx, cancel := context.WithTimeout(ctx, 90*time.Second)
		txHash, final, err := gov.Resolve(opCtx, proposalID)
		cancel()
		if err != nil {
			log.Printf("[auto-resolve] resolve proposalId=%d failed: %v", proposalID, err)
			continue
		}

		if final.Tallied {
			// 回写链下状态（0/1/2）
			p.PropStatus = int(final.Status)
			if err := db.DB.Save(&p).Error; err != nil {
				log.Printf("[auto-resolve] update proposal status failed: %v", err)
			}

			// 写 sys_configs 日志
			adminAddr := gov.FromAddress()
			val := fmt.Sprintf("propId=%d status=%d tx=%s", proposalID, final.Status, txHash)
			_ = db.DB.Create(&models.SysConfig{
				ParamName:  "PROPOSAL_RESOLVE",
				ParamValue: val,
				AdminAddr:  adminAddr,
			}).Error
		}
	}
}

