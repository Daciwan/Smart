<script setup lang="ts">
import { onMounted, onUnmounted, ref, computed } from 'vue';
import { useRoute } from 'vue-router';
import { useWalletStore } from '../stores/wallet';
import { ethers } from 'ethers';
import { GOVERNOR_CONTRACT_ADDRESS } from '../config';
import { explainChainError, formatProposalDeadline, proposalStatusLabel } from '../utils/proposal';

const route = useRoute();
const wallet = useWalletStore();
const BACKEND_BASE = 'http://127.0.0.1:8080';

const proposal = ref<any | null>(null);
const proposalImageUrls = computed(() => {
  const arr: string[] = Array.isArray(proposal.value?.imagePaths) ? proposal.value.imagePaths : [];
  return arr.map((p) => (p.startsWith('http') ? p : `${BACKEND_BASE}${p}`));
});
const loading = ref(false);
const voting = ref(false);
const onchainInfo = ref<any | null>(null);
const contractRef = ref<any | null>(null);

// TODO: 部署合约后将此地址替换为实际部署地址
const CONTRACT_ADDRESS = GOVERNOR_CONTRACT_ADDRESS;
const CONTRACT_ABI = [
  'function getProposal(uint256 proposalId) view returns (tuple(uint256 id, bytes32 contentHash, address creator, uint8 propType, uint64 startTime, uint64 deadline, uint256 yesVotes, uint256 noVotes, uint256 abstainVotes, uint8 status, bool tallied))',
  'function whitelist(address voter) view returns (bool isAuth, uint256 weight)',
  'function vote(uint256 proposalId, uint8 choice)',
  'function resolve(uint256 proposalId)',
  'event ProposalResolved(uint256 indexed proposalId, uint8 status, uint256 yesVotes, uint256 noVotes, uint256 abstainVotes)',
];

const id = computed(() => Number(route.params.id));

async function loadProposal() {
  loading.value = true;
  try {
    const resp = await fetch(`http://127.0.0.1:8080/api/proposals/${id.value}`);
    if (!resp.ok) {
      throw new Error('加载失败');
    }
    proposal.value = await resp.json();
  } catch (err) {
    console.error(err);
    alert('加载提案失败，请确认后端已启动');
  } finally {
    loading.value = false;
  }
}

async function loadOnchain() {
  if (!window.ethereum || !CONTRACT_ADDRESS) {
    return;
  }
  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, provider);
    const p = await contract.getProposal(proposal.value.propId || proposal.value.id);
    onchainInfo.value = p;
    contractRef.value = contract;
  } catch (err) {
    console.error('load onchain proposal failed', err);
  }
}

async function setupEventListener() {
  if (!contractRef.value || !proposal.value) return;
  const currentId = Number(proposal.value.propId || proposal.value.id);

  contractRef.value.on(
    'ProposalResolved',
    (proposalId: bigint, status: number, yesVotes: bigint, noVotes: bigint, abstainVotes: bigint) => {
      const resolvedId = Number(proposalId);
      if (resolvedId !== currentId) return;

      onchainInfo.value = {
        ...(onchainInfo.value || {}),
        yesVotes,
        noVotes,
        abstainVotes,
        status,
        tallied: true,
      };
    }
  );
}

const chartStyle = computed(() => {
  if (!onchainInfo.value) return {};

  const yes = Number(onchainInfo.value.yesVotes ?? 0n);
  const no = Number(onchainInfo.value.noVotes ?? 0n);
  const abstain = Number(onchainInfo.value.abstainVotes ?? 0n);
  const total = yes + no + abstain || 1;

  const yesDeg = (yes / total) * 360;
  const noDeg = yesDeg + (no / total) * 360;

  return {
    background: `conic-gradient(#10b981 0 ${yesDeg}deg, #ef4444 ${yesDeg}deg ${noDeg}deg, #9ca3af ${noDeg}deg 360deg)`,
  };
});

const canResolve = computed(() => {
  if (!onchainInfo.value) return false;
  const tallied = Boolean(onchainInfo.value.tallied);
  const deadlineSec = Number(onchainInfo.value.deadline ?? 0);
  const nowSec = Math.floor(Date.now() / 1000);
  return !tallied && deadlineSec > 0 && nowSec >= deadlineSec;
});

const showSettlementSection = computed(() => {
  if (!onchainInfo.value) return false;
  return !Boolean(onchainInfo.value.tallied);
});

const resolving = ref(false);

// 仅在用户点击“查看结果”按钮时触发结算
async function resolveProposal() {
  if (!wallet.address) {
    alert('请先在右上角连接钱包');
    return;
  }
  if (!window.ethereum) {
    alert('未检测到 MetaMask');
    return;
  }
  if (!proposal.value || !CONTRACT_ADDRESS) return;

  resolving.value = true;
  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);

    const pid = Number(proposal.value.propId || proposal.value.id);
    // 先做静态调用，提前拿到更清晰的 revert 提示
    await contract.resolve.staticCall(pid);
    const tx = await contract.resolve(pid);
    alert('结算交易已发送，等待确认：' + tx.hash);
    await tx.wait();

    // 结算后主动刷新一次链上数据，避免事件监听未命中时 UI 不更新
    await loadOnchain();
  } catch (err: any) {
    console.error(err);
    alert(explainChainError(err, '结算失败'));
  } finally {
    resolving.value = false;
  }
}

async function doVote(choice: number) {
  if (!wallet.address) {
    alert('请先在右上角连接钱包');
    return;
  }
  if (!window.ethereum) {
    alert('未检测到 MetaMask');
    return;
  }
  if (!CONTRACT_ADDRESS) {
    alert('请先在前端代码中配置合约地址');
    return;
  }
  voting.value = true;
  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);
    const signerAddr = await signer.getAddress();
    const voter = await contract.whitelist(signerAddr);
    const isAuth = Boolean(voter?.isAuth ?? voter?.[0]);
    if (!isAuth) {
      alert('当前钱包不在白名单，无法投票。');
      return;
    }
    const tx = await contract.vote(proposal.value.propId || proposal.value.id, choice);
    alert('交易已发送，等待确认：' + tx.hash);

    // 链上成功后，将投票记录写入后端缓存（简化实现，这里没有等待确认事件）
    await fetch(`http://127.0.0.1:8080/api/proposals/${proposal.value.id}/votes`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        propId: proposal.value.propId || proposal.value.id,
        voterAddr: wallet.address,
        voteChoice: choice,
        voteWeight: 1,
        voteTxHash: tx.hash,
      }),
    });
  } catch (err: any) {
    console.error(err);
    alert(explainChainError(err, '投票失败'));
  } finally {
    voting.value = false;
  }
}

onMounted(async () => {
  await loadProposal();
  await loadOnchain();
  await setupEventListener();
});

onUnmounted(() => {
  if (contractRef.value) {
    contractRef.value.removeAllListeners('ProposalResolved');
  }
});
</script>

<template>
  <div class="page-container" v-if="proposal">
    <button class="back-btn" @click="$router.back()">
      <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" stroke-width="2" fill="none"><line x1="19" y1="12" x2="5" y2="12"></line><polyline points="12 19 5 12 12 5"></polyline></svg>
      返回提案列表
    </button>

    <main class="content-wrapper">
      <article class="proposal-card">
        <header class="proposal-header">
          <div class="title-wrap">
            <span class="status-badge" :data-status="proposal.propStatus">
              <span class="status-dot"></span>
              {{ proposalStatusLabel(proposal.propStatus) }}
            </span>
            <h1 class="title">{{ proposal.propTitle }}</h1>
          </div>
          
          <div class="meta-banner">
            <div class="meta-item">
              <span class="label">提案发起人</span>
              <span class="value addr">{{ proposal.creatorAddr }}</span>
            </div>
            <div class="meta-item">
              <span class="label">截止时间</span>
              <span class="value">{{ formatProposalDeadline(proposal.deadline) }}</span>
            </div>
          </div>
        </header>

        <section class="proposal-body">
          <h3 class="section-title">提案详情</h3>
          <p class="desc-text">{{ proposal.propDesc }}</p>
          
          <div v-if="proposalImageUrls.length" class="image-gallery">
            <div v-for="(src, idx) in proposalImageUrls" :key="idx" class="img-wrapper">
              <img :src="src" alt="提案相关资料" class="proposal-image" loading="lazy" />
            </div>
          </div>
        </section>
      </article>

      <aside class="sidebar">
        <div class="action-card">
          <h3 class="card-title">链上投票</h3>
          <p class="hint-text">请连接 MetaMask 钱包进行投票，记录不可篡改。</p>
          
          <div class="vote-buttons">
            <button class="vote-btn btn-yes" :disabled="voting" @click="doVote(1)">
              <span class="icon">👍</span> 投赞成票
            </button>
            <button class="vote-btn btn-no" :disabled="voting" @click="doVote(2)">
              <span class="icon">👎</span> 投反对票
            </button>
            <button class="vote-btn btn-abstain" :disabled="voting" @click="doVote(3)">
              <span class="icon">✋</span> 弃权
            </button>
          </div>
        </div>

        <div v-if="onchainInfo" class="action-card data-card">
          <h3 class="card-title">实时票数分布</h3>
          
          <div class="chart-container">
            <div class="donut-chart" :style="chartStyle">
              <div class="donut-hole">
                <span class="total-label">总票数</span>
                <span class="total-number">
                  {{ (Number(onchainInfo.yesVotes) + Number(onchainInfo.noVotes) + Number(onchainInfo.abstainVotes)) }}
                </span>
              </div>
            </div>
            
            <div class="legend-list">
              <div class="legend-item">
                <span class="dot yes"></span>
                <span class="label">赞成</span>
                <span class="count">{{ onchainInfo.yesVotes?.toString?.() }}</span>
              </div>
              <div class="legend-item">
                <span class="dot no"></span>
                <span class="label">反对</span>
                <span class="count">{{ onchainInfo.noVotes?.toString?.() }}</span>
              </div>
              <div class="legend-item">
                <span class="dot abstain"></span>
                <span class="label">弃权</span>
                <span class="count">{{ onchainInfo.abstainVotes?.toString?.() }}</span>
              </div>
            </div>
          </div>
        </div>

        <div v-if="showSettlementSection" class="action-card settlement-card">
          <h3 class="card-title">结算与裁决</h3>
          <p class="hint-text">投票时间截止后，任何人均可触发合约结算流程，将最终结果记录在链上。</p>
          <button
            type="button"
            class="resolve-btn"
            :class="{ active: canResolve }"
            :disabled="!wallet.isConnected || !canResolve || resolving"
            @click="resolveProposal()"
          >
            {{
              !wallet.isConnected ? '请先连接钱包'
                : resolving ? '结算执行中...'
                : canResolve ? '执行结算 (执行上链)'
                : '未到截止时间，无法结算'
            }}
          </button>
        </div>
      </aside>
    </main>
  </div>

  <div v-else-if="loading" class="state-container">
    <div class="spinner"></div>
    <span>正在从链上和数据库同步提案数据...</span>
  </div>
  <div v-else class="state-container empty">
    <span>未找到该提案或数据加载失败</span>
  </div>
</template>

<style scoped>
.page-container {
  max-width: 1200px;
  margin: 0 auto;
  animation: fadeIn 0.4s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.back-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  background: transparent;
  border: none;
  color: #64748b;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  padding: 0;
  margin-bottom: 24px;
  transition: color 0.2s;
}
.back-btn:hover { color: #0f172a; }

/* 响应式两栏布局 */
.content-wrapper {
  display: grid;
  grid-template-columns: 1fr 340px;
  gap: 24px;
  align-items: start;
}

@media (max-width: 960px) {
  .content-wrapper {
    grid-template-columns: 1fr;
  }
}

/* 左侧主卡片 */
.proposal-card {
  background: #ffffff;
  border-radius: 16px;
  border: 1px solid #e2e8f0;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05);
  overflow: hidden;
}

.proposal-header {
  padding: 32px 32px 24px;
  border-bottom: 1px solid #e2e8f0;
  background: #f8fafc;
}

.title-wrap { margin-bottom: 20px; }

.title {
  font-size: 26px;
  font-weight: 800;
  color: #0f172a;
  margin: 12px 0 0 0;
  line-height: 1.4;
  letter-spacing: -0.5px;
}

/* 状态徽章 */
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 600;
}
.status-dot { width: 6px; height: 6px; border-radius: 50%; }
.status-badge[data-status='0'] { background-color: #ecfdf5; color: #059669; }
.status-badge[data-status='0'] .status-dot { background-color: #059669; }
.status-badge[data-status='1'] { background-color: #fefce8; color: #ca8a04; }
.status-badge[data-status='1'] .status-dot { background-color: #ca8a04; }
.status-badge[data-status='2'] { background-color: #eff6ff; color: #2563eb; }
.status-badge[data-status='2'] .status-dot { background-color: #2563eb; }
.status-badge[data-status='3'] { background-color: #fef2f2; color: #dc2626; }
.status-badge[data-status='3'] .status-dot { background-color: #dc2626; }

/* 提案元数据 Banner */
.meta-banner {
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: #ffffff;
  padding: 16px;
  border-radius: 12px;
  border: 1px solid #e2e8f0;
}
.meta-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.meta-item .label { font-size: 13px; color: #64748b; font-weight: 500; }
.meta-item .value { font-size: 14px; font-weight: 600; color: #0f172a; }
.meta-item .value.addr { font-family: ui-monospace, monospace; color: #2563eb; }

/* 提案正文 */
.proposal-body {
  padding: 32px;
}
.section-title {
  font-size: 18px;
  font-weight: 700;
  color: #0f172a;
  margin-bottom: 16px;
}
.desc-text {
  font-size: 15px;
  color: #334155;
  line-height: 1.8;
  white-space: pre-wrap;
  margin-bottom: 32px;
}

/* 图片展示区 */
.image-gallery {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}
.img-wrapper {
  border-radius: 12px;
  overflow: hidden;
  border: 1px solid #e2e8f0;
  background: #f8fafc;
}
.proposal-image {
  width: 100%;
  height: 240px;
  object-fit: contain;
  transition: transform 0.3s;
  cursor: pointer;
  background: #f8fafc;
  display: flex;             /* 新增：使用 flex 布局 */
  align-items: center;       /* 新增：垂直居中 */
  justify-content: center;
}
.proposal-image:hover { transform: scale(1.02); }

/* 右侧边栏卡片通用样式 */
.sidebar {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.action-card {
  background: #ffffff;
  border-radius: 16px;
  border: 1px solid #e2e8f0;
  padding: 24px;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05);
}

.card-title {
  font-size: 16px;
  font-weight: 700;
  color: #0f172a;
  margin-bottom: 8px;
}

.hint-text {
  font-size: 13px;
  color: #64748b;
  margin-bottom: 20px;
  line-height: 1.5;
}

/* 投票按钮 */
.vote-buttons {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.vote-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  padding: 12px;
  border-radius: 12px;
  border: none;
  font-size: 15px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  color: #ffffff;
}

.vote-btn:disabled { opacity: 0.6; cursor: not-allowed; }

.btn-yes { background-color: #10b981; box-shadow: 0 4px 12px rgba(16, 185, 129, 0.2); }
.btn-yes:hover:not(:disabled) { background-color: #059669; transform: translateY(-1px); }

.btn-no { background-color: #ef4444; box-shadow: 0 4px 12px rgba(239, 68, 68, 0.2); }
.btn-no:hover:not(:disabled) { background-color: #dc2626; transform: translateY(-1px); }

.btn-abstain { background-color: #94a3b8; box-shadow: 0 4px 12px rgba(148, 163, 184, 0.2); }
.btn-abstain:hover:not(:disabled) { background-color: #64748b; transform: translateY(-1px); }

/* 环形图设计 (Donut Chart) */
.chart-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 24px;
  margin-top: 20px;
}

.donut-chart {
  width: 160px;
  height: 160px;
  border-radius: 50%;
  position: relative;
  /* 背景渐变由 Vue script 控制 */
}

/* 核心技巧：挖空中心形成环形图 */
.donut-hole {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 110px;
  height: 110px;
  background-color: #ffffff;
  border-radius: 50%;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  box-shadow: inset 0 2px 4px rgba(0,0,0,0.05);
}

.total-label { font-size: 12px; color: #64748b; }
.total-number { font-size: 24px; font-weight: 800; color: #0f172a; }

/* 图例 */
.legend-list {
  width: 100%;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.legend-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 12px;
  background: #f8fafc;
  border-radius: 8px;
}

.legend-item .label {
  flex: 1;
  font-size: 14px;
  font-weight: 500;
  color: #334155;
  margin-left: 8px;
}

.legend-item .count {
  font-size: 15px;
  font-weight: 700;
  color: #0f172a;
}

.dot { width: 10px; height: 10px; border-radius: 50%; }
.dot.yes { background-color: #10b981; }
.dot.no { background-color: #ef4444; }
.dot.abstain { background-color: #94a3b8; }

/* 结算按钮 */
.settlement-card {
  background: linear-gradient(to bottom right, #f8fafc, #f1f5f9);
}

.resolve-btn {
  width: 100%;
  padding: 12px;
  border-radius: 12px;
  border: 1px solid #cbd5e1;
  background: #ffffff;
  color: #64748b;
  font-size: 14px;
  font-weight: 600;
  cursor: not-allowed;
  transition: all 0.2s;
}

.resolve-btn.active {
  border-color: #2563eb;
  background: #2563eb;
  color: #ffffff;
  cursor: pointer;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.2);
}

.resolve-btn.active:hover {
  background: #1d4ed8;
  transform: translateY(-1px);
}

/* 加载空状态 */
.state-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 80px 20px;
  color: #64748b;
  font-size: 15px;
  background: #ffffff;
  border-radius: 16px;
  box-shadow: 0 4px 6px -1px rgba(0,0,0,0.05);
}

.spinner {
  width: 32px; height: 32px;
  border: 3px solid #e2e8f0;
  border-top-color: #2563eb;
  border-radius: 50%;
  animation: spin 1s linear infinite;
  margin-bottom: 16px;
}
</style>