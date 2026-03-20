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
    background: `conic-gradient(#16a34a 0 ${yesDeg}deg, #dc2626 ${yesDeg}deg ${noDeg}deg, #9ca3af ${noDeg}deg 360deg)`,
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
  <div class="page" v-if="proposal">
    <div class="header">
      <h2>{{ proposal.propTitle }}</h2>
      <span class="status" :data-status="proposal.propStatus">{{ proposalStatusLabel(proposal.propStatus) }}</span>
    </div>
    <div class="meta">
      <span>发起人：{{ proposal.creatorAddr?.slice(0, 6) }}...{{ proposal.creatorAddr?.slice(-4) }}</span>
      <span>截止时间：{{ formatProposalDeadline(proposal.deadline) }}</span>
    </div>

    <section class="section">
      <h3>提案详情</h3>
      <p class="desc">{{ proposal.propDesc }}</p>
      <div v-if="proposalImageUrls.length" class="images">
        <img v-for="(src, idx) in proposalImageUrls" :key="idx" :src="src" alt="提案图片" class="proposal-image" />
      </div>
    </section>

    <section class="section">
      <h3>参与投票</h3>
      <p class="hint">请使用 MetaMask 对当前提案投票，所有投票结果将记录在区块链上。</p>
      <div class="btn-group">
        <button type="button" class="btn-yes" :disabled="voting" @click="doVote(1)">赞成</button>
        <button type="button" class="btn-no" :disabled="voting" @click="doVote(2)">反对</button>
        <button type="button" class="btn-abstain" :disabled="voting" @click="doVote(3)">弃权</button>
      </div>
    </section>

    <section v-if="showSettlementSection" class="section">
      <h3>结算与裁决</h3>
      <p class="hint">
      </p>
      <button
        type="button"
        class="btn-secondary"
        :disabled="!wallet.isConnected || !canResolve || resolving"
        @click="resolveProposal()"
      >
        {{
          !wallet.isConnected
            ? '请先连接钱包'
            : resolving
              ? '结算中...'
              : canResolve
                ? '查看结果'
                : '未到截止时间/已结算'
        }}
      </button>
    </section>

    <section v-if="onchainInfo" class="section">
      <h3>链上票数概览</h3>
      <div class="chart-row">
        <div class="pie" :style="chartStyle"></div>
        <div class="legend">
          <div class="legend-item">
            <span class="dot yes"></span>
            <span>赞成票：{{ onchainInfo.yesVotes?.toString?.() }}</span>
          </div>
          <div class="legend-item">
            <span class="dot no"></span>
            <span>反对票：{{ onchainInfo.noVotes?.toString?.() }}</span>
          </div>
          <div class="legend-item">
            <span class="dot abstain"></span>
            <span>弃权票：{{ onchainInfo.abstainVotes?.toString?.() }}</span>
          </div>
        </div>
      </div>
    </section>
  </div>

  <div v-else-if="loading">加载中...</div>
  <div v-else>未找到该提案</div>
</template>

<style scoped>
.page {
  max-width: 800px;
  margin: 0 auto;
  background: #fff;
  padding: 24px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(148, 163, 184, 0.25);
}

.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.status {
  padding: 4px 10px;
  border-radius: 999px;
  font-size: 12px;
}

.status[data-status='0'] {
  background-color: #ecfdf5;
  color: #15803d;
}

.status[data-status='1'] {
  background-color: #fef9c3;
  color: #a16207;
}

.status[data-status='2'] {
  background-color: #eff6ff;
  color: #1d4ed8;
}

.status[data-status='3'] {
  background-color: #fef2f2;
  color: #b91c1c;
}

.meta {
  margin-top: 8px;
  font-size: 12px;
  color: #6b7280;
  display: flex;
  justify-content: space-between;
}

.section {
  margin-top: 20px;
}

.desc {
  font-size: 14px;
  color: #374151;
  white-space: pre-wrap;
}

.images {
  margin-top: 10px;
  width: 100%;
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.proposal-image {
  width: 100%;
  height: 240px;
  object-fit: contain;
  background: transparent;
  border-radius: 8px;
  border: 1px solid #e5e7eb;
}

@media (max-width: 768px) {
  .images {
    grid-template-columns: 1fr;
  }
}

.btn-group {
  display: flex;
  gap: 10px;
  margin-top: 10px;
}

.btn-yes,
.btn-no,
.btn-abstain {
  border-radius: 999px;
  padding: 8px 16px;
  border: none;
  cursor: pointer;
  font-size: 14px;
}

.btn-yes {
  background-color: #16a34a;
  color: #fff;
}

.btn-no {
  background-color: #dc2626;
  color: #fff;
}

.btn-abstain {
  background-color: #e5e7eb;
  color: #111827;
}

.hint {
  font-size: 13px;
  color: #4b5563;
}

.chart-row {
  margin-top: 10px;
  display: flex;
  gap: 16px;
  align-items: center;
}

.pie {
  width: 140px;
  height: 140px;
  border-radius: 50%;
  background: #e5e7eb;
  box-shadow: inset 0 0 0 10px #fff, 0 6px 16px rgba(15, 23, 42, 0.12);
}

.legend {
  display: flex;
  flex-direction: column;
  gap: 10px;
  font-size: 13px;
  color: #374151;
}

.legend-item {
  display: flex;
  align-items: center;
  gap: 10px;
}

.dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  display: inline-block;
}

.dot.yes {
  background-color: #16a34a;
}

.dot.no {
  background-color: #dc2626;
}

.dot.abstain {
  background-color: #9ca3af;
}
</style>

