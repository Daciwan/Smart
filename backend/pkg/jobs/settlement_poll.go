package jobs

import (
	"context"
	"log"
	"time"

	"smart-community/pkg/chain"
	"smart-community/pkg/db"
	"smart-community/pkg/models"
)

// StartSettlementStatusPoll 每隔一段时间把链上 resolve 结果回写到链下 prop_status。
//
// 场景：
// 1) prop_status=1（已截至但未结算）时，前端触发 resolve 后可能尚未回写链下；
// 2) 本任务周期性查询链上 getProposal，如果 tallied=true，则把 prop_status 更新为：
//    - 链上 status=1 => 链下 prop_status=2（已通过）
//    - 链上 status=2 => 链下 prop_status=3（已驳回）
func StartSettlementStatusPoll(ctx context.Context, interval time.Duration) {
	gov, err := chain.NewGovernorClientFromEnv()
	if err != nil {
		log.Printf("[settlement-poll] disabled: %v", err)
		return
	}

	log.Printf("[settlement-poll] started. from=%s interval=%s", gov.FromAddress(), interval)

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Printf("[settlement-poll] stopped: %v", ctx.Err())
				return
			case <-ticker.C:
				runSettlementPollOnce(ctx, gov)
			}
		}
	}()

	// 立即执行一次，减少等待 interval 的时间
	runSettlementPollOnce(ctx, gov)
}

func runSettlementPollOnce(ctx context.Context, gov *chain.GovernorClient) {
	now := time.Now()
	var props []models.Proposal

	// 覆盖 prop_status=1（已截至但未结算）以及 prop_status=0（可能刚过期、尚未被 0->1 同步任务更新）
	if err := db.DB.
		Where("prop_status IN ? AND deadline <= ?", []int{0, 1}, now).
		Order("deadline asc").
		Limit(80).
		Find(&props).Error; err != nil {
		log.Printf("[settlement-poll] query proposals failed: %v", err)
		return
	}

	if len(props) == 0 {
		// 不要刷屏：仅在分钟级别仍有空跑时才有必要
		log.Printf("[settlement-poll] no candidates. now=%s", now.Format(time.RFC3339))
		return
	}

	// 为了可观测性，打印前 5 个候选提案 id
	log.Printf("[settlement-poll] candidates count=%d firstPropIds=%v", len(props), func() []uint {
		ids := make([]uint, 0, 5)
		for i, p := range props {
			if i >= 5 {
				break
			}
			ids = append(ids, p.PropID)
		}
		return ids
	}())

	for _, p := range props {
		select {
		case <-ctx.Done():
			return
		default:
		}

		proposalID := uint64(p.PropID)
		opCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
		final, err := gov.GetProposal(opCtx, proposalID)
		cancel()
		if err != nil {
			log.Printf("[settlement-poll] getProposal failed: proposalID=%d err=%v", proposalID, err)
			continue
		}

		if !final.Tallied {
			continue
		}

		// 合约 status: 0/1/2 => 进行中/已通过/已驳回
		// 链下 prop_status: 0/1/2/3 => 进行中/已截至但未结算/已通过/已驳回
		var next int
		switch final.Status {
		case 1:
			next = 2
		case 2:
			next = 3
		default:
			// tallied=true 时理论上不会是 0，这里保守忽略
			continue
		}

		if p.PropStatus == next {
			continue
		}

		p.PropStatus = next
		if err := db.DB.Save(&p).Error; err != nil {
			log.Printf("[settlement-poll] update proposal status failed: propId=%d next=%d err=%v", p.PropID, next, err)
			continue
		}

		log.Printf("[settlement-poll] updated proposal: propId=%d final.status=%d tallied=%v from->to=%d->%d tx-not-required", p.PropID, final.Status, final.Tallied, p.PropStatus, next)
	}
}

