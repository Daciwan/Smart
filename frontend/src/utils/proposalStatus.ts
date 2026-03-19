export interface ProposalLike {
  /** 链下状态（后端 prop_status）：0 进行中，1 已截至但未结算，2 已通过，3 已驳回 */
  propStatus: number;
  /** 链下截止时间字符串，例如 "2026-03-02 18:30:00" */
  deadline: string;
  /** 可选：链上状态（若已结算），0 进行中，1 已通过，2 已驳回 */
  onchainStatus?: number;
  /** 可选：链上是否已经结算（tallied = true） */
  tallied?: boolean;
}

/** 判断是否已超过截止时间 */
export function isExpired(deadline: string): boolean {
  if (!deadline) return false;
  const end = new Date(deadline).getTime();
  if (Number.isNaN(end)) return false;
  return Date.now() > end;
}

/** 统一计算提案在前端展示的状态文案 */
export function computeDisplayStatus(p: ProposalLike): string {
  // 1. 优先使用链上结果（如果有）
  if (p.tallied || (p.onchainStatus ?? 0) !== 0) {
    const s = p.onchainStatus ?? 0;
    if (s === 1) return '已通过';
    if (s === 2) return '已驳回';
    return '已结束';
  }

  // 2. 再看链下状态
  if (p.propStatus === 1) return '已截至但未结算';
  if (p.propStatus === 2) return '已通过';
  if (p.propStatus === 3) return '已驳回';

  // 3. 进行中 + 截止时间
  if (isExpired(p.deadline)) return '已截至但未结算';

  return '进行中';
}

