export function proposalStatusLabel(status: number): string {
  if (status === 0) return '进行中';
  if (status === 1) return '已截至但未结算';
  if (status === 2) return '已通过';
  if (status === 3) return '已驳回';
  return '未知';
}

export function formatProposalDeadline(deadline: string): string {
  return new Date(deadline).toLocaleString();
}

export function parseDateTimeLocalToPayload(localValue: string): { text: string; unixSeconds: number } {
  const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(localValue);
  if (!m) {
    return { text: '', unixSeconds: 0 };
  }
  const [, yy, mm, dd, hh, mi] = m;
  const y = Number(yy);
  const mon = Number(mm);
  const day = Number(dd);
  const hour = Number(hh);
  const minute = Number(mi);
  const d = new Date(y, mon - 1, day, hour, minute, 0);
  return {
    text: `${yy}-${mm}-${dd} ${hh}:${mi}:00`,
    unixSeconds: Math.floor(d.getTime() / 1000),
  };
}

export function explainChainError(err: any, fallback: string): string {
  const msg = String(err?.shortMessage || err?.message || '').toLowerCase();

  if (msg.includes('could not coalesce error') || msg.includes('failed to fetch')) {
    return '链上请求失败：请确认 Ganache 正在运行，且 MetaMask 已切换到本地链（127.0.0.1:8545）。';
  }
  if (msg.includes('not in whitelist')) {
    return '当前钱包不在白名单，无法投票。';
  }
  if (msg.includes('user rejected') || msg.includes('rejected')) {
    return '你已在钱包中取消本次交易。';
  }
  if (msg.includes('cannot resolve yet') || msg.includes('voting closed')) {
    return '当前提案暂不满足结算条件，或已被其他人结算，请刷新后重试。';
  }
  if (msg.includes('missing revert data')) {
    return fallback;
  }
  if (msg.includes('insufficient funds')) {
    return '钱包余额不足，无法支付 Gas。';
  }
  return fallback;
}
