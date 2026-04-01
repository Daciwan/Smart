<script setup lang="ts">
import { onMounted, onUnmounted, ref, shallowRef, computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useWalletStore } from '../stores/wallet';
import { ethers } from 'ethers';
import { GOVERNOR_CONTRACT_ADDRESS } from '../config';
import { explainChainError, formatProposalDeadline, proposalStatusLabel } from '../utils/proposal';

const route = useRoute();
const router = useRouter();
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

// 使用 shallowRef 避免 Vue 深度代理破坏 Ethers 合约实例
const contractRef = shallowRef<any | null>(null); 
let pollTimer: ReturnType<typeof setInterval> | null = null;

const CONTRACT_ADDRESS = GOVERNOR_CONTRACT_ADDRESS;
const CONTRACT_ABI = [
  'function getProposal(uint256 proposalId) view returns (tuple(uint256 id, bytes32 contentHash, address creator, uint8 propType, uint64 startTime, uint64 deadline, uint256 yesVotes, uint256 noVotes, uint256 abstainVotes, uint8 status, bool tallied))',
  'function whitelist(address voter) view returns (bool isAuth, uint256 weight)',
  'function vote(uint256 proposalId, uint8 choice)',
  'function resolve(uint256 proposalId)',
  'event ProposalResolved(uint256 indexed proposalId, uint8 status, uint256 yesVotes, uint256 noVotes, uint256 abstainVotes)',
];

const id = computed(() => Number(route.params.id));

async function loadProposal(isSilent = false) {
  if (!isSilent) loading.value = true;
  try {
    const resp = await fetch(`http://127.0.0.1:8080/api/proposals/${id.value}`);
    if (!resp.ok) throw new Error('加载失败');
    proposal.value = await resp.json();
  } catch (err) {
    console.error(err);
    if (!isSilent) alert('加载提案失败，请确认后端已启动');
  } finally {
    if (!isSilent) loading.value = false;
  }
}

async function loadOnchain() {
  if (!window.ethereum || !CONTRACT_ADDRESS || !proposal.value) return;
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
      if (Number(proposalId) !== currentId) return;
      onchainInfo.value = { ...(onchainInfo.value || {}), yesVotes, noVotes, abstainVotes, status, tallied: true };
      loadProposal(true);
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
  return { background: `conic-gradient(#10b981 0 ${yesDeg}deg, #ef4444 ${yesDeg}deg ${noDeg}deg, #9ca3af ${noDeg}deg 360deg)` };
});

const canResolve = computed(() => {
  if (proposal.value?.propStatus === 1) return true;
  if (!onchainInfo.value) return false;
  return !onchainInfo.value.tallied && Number(onchainInfo.value.deadline) > 0 && Math.floor(Date.now() / 1000) >= Number(onchainInfo.value.deadline);
});

const showSettlementSection = computed(() => {
  if (proposal.value) return proposal.value.propStatus === 0 || proposal.value.propStatus === 1;
  return false;
});

const resolving = ref(false);

async function resolveProposal() {
  if (!wallet.address || !window.ethereum || !proposal.value) return;
  resolving.value = true;
  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);
    const tx = await contract.resolve(proposal.value.propId || proposal.value.id);
    await tx.wait();
    await loadOnchain();
    await loadProposal(true);
  } catch (err: any) {
    alert(explainChainError(err, '结算失败'));
  } finally {
    resolving.value = false;
  }
}

async function doVote(choice: number) {
  if (!wallet.address || !window.ethereum) return;
  voting.value = true;
  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);
    const tx = await contract.vote(proposal.value.propId || proposal.value.id, choice);
    await tx.wait();
    await fetch(`http://127.0.0.1:8080/api/proposals/${proposal.value.id}/votes`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ propId: proposal.value.propId || proposal.value.id, voterAddr: wallet.address, voteChoice: choice, voteWeight: 1, voteTxHash: tx.hash }),
    });
  } catch (err: any) {
    alert(explainChainError(err, '投票失败'));
  } finally {
    voting.value = false;
  }
}

function goBack() {
  router.back();
}

onMounted(async () => {
  await loadProposal();
  await loadOnchain();
  await setupEventListener();
  pollTimer = setInterval(() => { loadProposal(true); loadOnchain(); }, 5000);
});

onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
  if (contractRef.value) contractRef.value.removeAllListeners('ProposalResolved');
});
</script>

<template>
  <div class="page-container" v-if="proposal">
    <button class="back-btn" @click="goBack">
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
            <div class="meta-item"><span class="label">提案发起人</span><span class="value addr">{{ proposal.creatorAddr }}</span></div>
            <div class="meta-item">
              <span class="label">计票模式</span>
              <span class="value mode-tag">
                <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"></path><polyline points="3.27 6.96 12 12.01 20.73 6.96"></polyline><line x1="12" y1="22.08" x2="12" y2="12"></line></svg>
                {{ proposal.propType === 1 ? '面积加权' : '一人一票' }}
              </span>
            </div>
            <div class="meta-item"><span class="label">截止时间</span><span class="value">{{ formatProposalDeadline(proposal.deadline) }}</span></div>
          </div>
        </header>
        <section class="proposal-body">
          <h3 class="section-title">提案详情</h3>
          <p class="desc-text">{{ proposal.propDesc }}</p>
          <div v-if="proposalImageUrls.length" class="image-gallery">
            <div v-for="(src, idx) in proposalImageUrls" :key="idx" class="img-wrapper">
              <img :src="src" class="proposal-image" loading="lazy" />
            </div>
          </div>
        </section>
      </article>

      <aside class="sidebar">
        <div class="action-card">
          <h3 class="card-title">治理参与</h3>
          <div v-if="proposal.propStatus === 0" class="vote-buttons">
            <p class="hint-text">请连接钱包参与实时投票。</p>
            <button class="vote-btn btn-yes" :disabled="voting" @click="doVote(1)">赞成</button>
            <button class="vote-btn btn-no" :disabled="voting" @click="doVote(2)">反对</button>
            <button class="vote-btn btn-abstain" :disabled="voting" @click="doVote(3)">弃权</button>
          </div>
          <div v-else class="ended-notice-box">
            <div class="ended-status-msg">
              <svg v-if="proposal.propStatus === 1" viewBox="0 0 24 24" width="20" height="20" stroke="#ea580c" fill="none"><circle cx="12" cy="12" r="10"></circle><polyline points="12 6 12 12 16 14"></polyline></svg>
              <svg v-else viewBox="0 0 24 24" width="20" height="20" stroke="#2563eb" fill="none"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>
              <span>{{ proposal.propStatus === 1 ? '投票已截止，等待结算' : '治理流程已完成' }}</span>
            </div>
          </div>
        </div>

        <div class="action-card data-card">
          <div class="card-header-flex">
            <h3 class="card-title">当前投票统计</h3>
            <span class="on-chain-label">区块链实时数据</span>
          </div>
          
          <div v-if="onchainInfo" class="results-container">
            <div class="chart-container">
              <div class="donut-chart" :style="chartStyle">
                <div class="donut-hole"><span class="total-number">{{ (Number(onchainInfo.yesVotes) + Number(onchainInfo.noVotes) + Number(onchainInfo.abstainVotes)) }}</span><span class="total-label">总计</span></div>
              </div>
              <div class="legend-list">
                <div class="legend-item"><span class="dot yes"></span><span class="label">赞成</span><span class="count">{{ onchainInfo.yesVotes.toString() }}</span></div>
                <div class="legend-item"><span class="dot no"></span><span class="label">反对</span><span class="count">{{ onchainInfo.noVotes.toString() }}</span></div>
                <div class="legend-item"><span class="dot abstain"></span><span class="label">弃权</span><span class="count">{{ onchainInfo.abstainVotes.toString() }}</span></div>
              </div>
            </div>
          </div>
          <div v-else class="results-loading-state">
            <span class="spinner-small blue"></span>
            <span>正在同步链上票数...</span>
          </div>
        </div>

        <div v-if="showSettlementSection" class="action-card settlement-card">
          <h3 class="card-title">上链结算</h3>
          <p class="hint-text">截止后需手动触发，将共识结果永久记录在区块中。</p>
          <button class="resolve-btn" :class="{ active: canResolve }" :disabled="!wallet.isConnected || !canResolve || resolving" @click="resolveProposal()">
            {{ resolving ? '上链确认中...' : (canResolve ? '执行结算' : '尚未到结算时间') }}
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
.page-container { max-width: 1200px; margin: 0 auto; animation: fadeIn 0.4s ease-out; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }
.back-btn { background: transparent; border: none; color: #64748b; font-size: 14px; cursor: pointer; padding: 0; margin-bottom: 24px; display: flex; align-items: center; gap: 8px; }
.content-wrapper { display: grid; grid-template-columns: 1fr 340px; gap: 24px; align-items: start; }
@media (max-width: 960px) { .content-wrapper { grid-template-columns: 1fr; } }

.proposal-card { background: #fff; border-radius: 16px; border: 1px solid #e2e8f0; overflow: hidden; }
.proposal-header { padding: 32px; border-bottom: 1px solid #e2e8f0; background: #f8fafc; }
.title { font-size: 26px; font-weight: 800; color: #0f172a; margin-top: 12px; }
.status-badge { display: inline-flex; align-items: center; gap: 6px; padding: 6px 12px; border-radius: 999px; font-size: 13px; font-weight: 600; }
.status-dot { width: 6px; height: 6px; border-radius: 50%; }
.status-badge[data-status='0'] { background: #ecfdf5; color: #059669; } .status-badge[data-status='0'] .status-dot { background: #059669; }
.status-badge[data-status='1'] { background: #fff7ed; color: #ea580c; } .status-badge[data-status='1'] .status-dot { background: #ea580c; }
.status-badge[data-status='2'] { background: #eff6ff; color: #2563eb; } .status-badge[data-status='2'] .status-dot { background: #2563eb; }
.status-badge[data-status='3'] { background: #fef2f2; color: #dc2626; } .status-badge[data-status='3'] .status-dot { background: #dc2626; }

.meta-banner { display: flex; flex-direction: column; gap: 12px; background: #fff; padding: 16px; border-radius: 12px; border: 1px solid #e2e8f0; margin-top: 20px;}
.meta-item { display: flex; justify-content: space-between; align-items: center; }
.label { font-size: 13px; color: #64748b; }
.value { font-size: 14px; font-weight: 600; }
.mode-tag { background: #e0e7ff; color: #4338ca; padding: 4px 10px; border-radius: 6px; display: flex; align-items: center; gap: 4px; }

.proposal-body { padding: 32px; }
.desc-text { line-height: 1.8; color: #334155; white-space: pre-wrap; margin-bottom: 24px; }
.image-gallery { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 16px; }
.img-wrapper { border: 1px solid #e2e8f0; border-radius: 8px; overflow: hidden; height: 180px; }
.proposal-image { width: 100%; height: 100%; object-fit: contain; }

.sidebar { display: flex; flex-direction: column; gap: 24px; }
.action-card { background: #fff; border-radius: 16px; border: 1px solid #e2e8f0; padding: 24px; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.05); }
.card-title { font-size: 16px; font-weight: 700; margin-bottom: 12px; }
.hint-text { font-size: 13px; color: #64748b; margin-bottom: 16px; }

.vote-buttons { display: flex; flex-direction: column; gap: 10px; }
.vote-btn { padding: 12px; border-radius: 12px; border: none; font-weight: 700; cursor: pointer; color: #fff; }
.btn-yes { background: #10b981; } .btn-no { background: #ef4444; } .btn-abstain { background: #94a3b8; }

.ended-notice-box { background-color: #f8fafc; border: 1px solid #e2e8f0; padding: 12px 16px; border-radius: 10px; margin-top: 10px; }
.ended-status-msg { display: flex; align-items: center; gap: 8px; color: #334155; font-weight: 600; font-size: 14px; }

.card-header-flex { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.on-chain-label { font-size: 11px; font-weight: 700; background: #f1f5f9; padding: 4px 8px; border-radius: 4px; color: #475569; }

.chart-container { display: flex; flex-direction: column; align-items: center; gap: 24px; }
.donut-chart { width: 150px; height: 150px; border-radius: 50%; position: relative; }
.donut-hole { position: absolute; inset: 25px; background: #fff; border-radius: 50%; display: flex; flex-direction: column; align-items: center; justify-content: center; box-shadow: inset 0 2px 4px rgba(0,0,0,0.05); }
.total-number { font-size: 22px; font-weight: 800; }
.total-label { font-size: 11px; color: #64748b; }

.legend-list { width: 100%; display: flex; flex-direction: column; gap: 8px; }
.legend-item { display: flex; align-items: center; justify-content: space-between; background: #f8fafc; padding: 8px 12px; border-radius: 8px; }
.dot { width: 8px; height: 8px; border-radius: 50%; }
.dot.yes { background: #10b981; } .dot.no { background: #ef4444; } .dot.abstain { background: #94a3b8; }
.count { font-weight: 700; }

.results-loading-state { height: 150px; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 12px; color: #64748b; font-size: 13px; }

.resolve-btn { width: 100%; padding: 12px; border-radius: 12px; border: 1px solid #e2e8f0; background: #f8fafc; color: #94a3b8; font-weight: 700; cursor: not-allowed; }
.resolve-btn.active { background: #2563eb; color: #fff; border: none; cursor: pointer; }

.spinner-small { width: 18px; height: 18px; border: 2px solid #e2e8f0; border-top-color: #2563eb; border-radius: 50%; animation: spin 1s linear infinite; }
@keyframes spin { to { transform: rotate(360deg); } }

.state-container { display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 80px 20px; color: #64748b; font-size: 15px; background: #ffffff; border-radius: 16px; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.05); }
.spinner { width: 32px; height: 32px; border: 3px solid #e2e8f0; border-top-color: #2563eb; border-radius: 50%; animation: spin 1s linear infinite; margin-bottom: 16px; }
</style>