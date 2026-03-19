<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { ethers } from 'ethers';
import { useWalletStore } from '../stores/wallet';
import { GOVERNOR_CONTRACT_ADDRESS } from '../config';

interface Proposal {
  id: number;
  propTitle: string;
  creatorAddr: string;
  createTime: string;
  deadline: string;
  propStatus: number;
}

// 与详情页保持一致的合约地址和 ABI
const CONTRACT_ADDRESS = GOVERNOR_CONTRACT_ADDRESS;
const CONTRACT_ABI = [
  'function createProposal(bytes32 contentHash, uint8 propType, uint64 deadline) returns (uint256)',
  'function proposalCount() view returns (uint256)',
];

const router = useRouter();
const wallet = useWalletStore();

const proposals = ref<Proposal[]>([]);
const loading = ref(false);
const keyword = ref('');
type CategoryKey = 'all' | 'ongoing' | 'settled' | 'passed' | 'rejected' | 'unsettled';
const activeCategory = ref<CategoryKey>('all');
const currentPage = ref(1);
const PAGE_SIZE = 5;

const showCreate = ref(false);
const creating = ref(false);
const createError = ref<string | null>(null);
const form = ref({
  title: '',
  desc: '',
  mode: '0', // 0: 一人一票, 1: 面积加权
  deadline: '',
});

async function loadProposals() {
  if (loading.value) return;
  loading.value = true;
  try {
    const resp = await fetch('http://127.0.0.1:8080/api/proposals');
    if (!resp.ok) {
      throw new Error('加载失败');
    }
    proposals.value = await resp.json();
  } catch (err) {
    console.error(err);
    alert('加载提案失败，请确认后端已启动');
  } finally {
    loading.value = false;
  }
}

function statusLabelFor(p: Proposal) {
  // propStatus 语义（与后端保持一致）：
  // 0：进行中
  // 1：已截至但未结算
  // 2：已通过
  // 3：已驳回
  if (p.propStatus === 0) return '进行中';
  if (p.propStatus === 1) return '已截至但未结算';
  if (p.propStatus === 2) return '已通过';
  if (p.propStatus === 3) return '已驳回';
  return '未知';
}

const categoryOptions: Array<{ key: CategoryKey; label: string }> = [
  { key: 'all', label: '全部' },
  { key: 'ongoing', label: '进行中' },
  { key: 'settled', label: '已结算' },
  { key: 'passed', label: '已通过' },
  { key: 'rejected', label: '已驳回' },
  { key: 'unsettled', label: '未结算' },
];

const categoryProposals = computed(() => {
  return proposals.value.filter((p) => {
    if (activeCategory.value === 'all') return true;
    if (activeCategory.value === 'ongoing') return p.propStatus === 0;
    if (activeCategory.value === 'settled') return p.propStatus === 2 || p.propStatus === 3;
    if (activeCategory.value === 'passed') return p.propStatus === 2;
    if (activeCategory.value === 'rejected') return p.propStatus === 3;
    return p.propStatus === 1; // unsettled
  });
});

const filteredProposals = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  return categoryProposals.value.filter((p) => p.propTitle.toLowerCase().includes(kw));
});

const totalPages = computed(() => Math.max(1, Math.ceil(filteredProposals.value.length / PAGE_SIZE)));

const pagedProposals = computed(() => {
  const start = (currentPage.value - 1) * PAGE_SIZE;
  return filteredProposals.value.slice(start, start + PAGE_SIZE);
});

function prevPage() {
  if (currentPage.value > 1) currentPage.value -= 1;
}

function nextPage() {
  if (currentPage.value < totalPages.value) currentPage.value += 1;
}

watch([keyword, proposals, activeCategory], () => {
  currentPage.value = 1;
});

function goDetail(p: Proposal) {
  router.push(`/proposals/${p.id}`);
}

function openCreate() {
  if (!wallet.address) {
    alert('请先在右上角连接钱包');
    return;
  }
  showCreate.value = true;
  createError.value = null;
}

function closeCreate() {
  showCreate.value = false;
  createError.value = null;
}

function parseDateTimeLocal(localValue: string): {
  text: string; // YYYY-MM-DD HH:mm:ss（用于后端存 MySQL）
  unixSeconds: number; // 用于合约
} {
  // datetime-local 的值形如 "2026-03-02T18:30"（不带时区）
  // 不能直接 new Date(string)（不同浏览器/环境可能按 UTC 解析导致日期偏移）
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
  const d = new Date(y, mon - 1, day, hour, minute, 0); // 按本地时区构造
  return {
    text: `${yy}-${mm}-${dd} ${hh}:${mi}:00`,
    unixSeconds: Math.floor(d.getTime() / 1000),
  };
}

async function submitCreate() {
  if (!wallet.address) {
    alert('请先在右上角连接钱包');
    return;
  }
  const title = form.value.title.trim();
  const desc = form.value.desc.trim();
  if (!title || !desc || !form.value.deadline) {
    createError.value = '请完整填写提案标题、内容和截止时间';
    return;
  }
  if (title.length < 4 || title.length > 50) {
    createError.value = '提案标题长度需为 4-50 个字符';
    return;
  }
  if (!window.ethereum) {
    createError.value = '未检测到 MetaMask';
    return;
  }

  creating.value = true;
  createError.value = null;
  try {
    // 1. 先在链上创建提案（生成 ProposalId）
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);

    const contentHash = ethers.keccak256(ethers.toUtf8Bytes(desc));
    const propType = parseInt(form.value.mode, 10);
    const parsed = parseDateTimeLocal(form.value.deadline);
    if (!parsed.text || !parsed.unixSeconds) {
      createError.value = '投票截止时间格式不正确';
      return;
    }
    const deadlineSec = parsed.unixSeconds;

    const tx = await contract.createProposal(contentHash, propType, deadlineSec);
    alert('链上创建提案交易已发送：' + tx.hash);
    await tx.wait();

    // 取当前提案总数，作为新的 ProposalId（合约中从 1 开始递增）
    const count = await contract.proposalCount();
    const onchainId = Number(count);

    // 2. 再把提案的链下内容写入后端，并带上 propId 与 txHash
    const resp = await fetch('http://127.0.0.1:8080/api/proposals', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        propTitle: title,
        propDesc: desc,
        propType,
        deadline: parsed.text,
        creatorAddr: wallet.address,
        propId: onchainId,
        txHash: tx.hash,
      }),
    });
    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(data.error || '创建失败（链下部分）');
    }

    alert('提案已成功创建：链上 + 链下均已记录');
    showCreate.value = false;
    form.value = { title: '', desc: '', mode: '0', deadline: '' };
    await loadProposals();
  } catch (err: any) {
    console.error(err);
    createError.value = err?.message || '创建失败';
  } finally {
    creating.value = false;
  }
}

onMounted(() => {
  loadProposals();
});
</script>

<template>
  <div class="page">
    <div class="header">
      <div>
        <h2>社区提案与投票</h2>
        <span class="sub">查看所有已公示的提案及其状态</span>
      </div>
      <button class="btn-primary" type="button" @click="openCreate">发起提案</button>
    </div>

    <div class="toolbar">
      <div class="category-tabs">
        <button
          v-for="c in categoryOptions"
          :key="c.key"
          type="button"
          class="tab-btn"
          :class="{ active: activeCategory === c.key }"
          @click="activeCategory = c.key"
        >
          {{ c.label }}
        </button>
      </div>
      <input
        v-model.trim="keyword"
        class="search-input"
        type="text"
        placeholder="按标题搜索（仅当前分类）"
      />
    </div>

    <div v-if="loading">加载中...</div>
    <div v-else-if="!filteredProposals.length" class="empty">暂无匹配提案</div>
    <div v-else class="list">
      <div
        v-for="p in pagedProposals"
        :key="p.id"
        class="card"
        role="button"
        tabindex="0"
        @click="goDetail(p)"
      >
        <div class="title-row">
          <h3>{{ p.propTitle }}</h3>
          <span class="status" :data-status="p.propStatus">{{ statusLabelFor(p) }}</span>
        </div>
        <div class="meta">
          <span>发起人：{{ p.creatorAddr?.slice(0, 6) }}...{{ p.creatorAddr?.slice(-4) }}</span>
          <span>截止时间：{{ new Date(p.deadline).toLocaleString() }}</span>
        </div>
      </div>
    </div>
    <div v-if="!loading && filteredProposals.length" class="pagination">
      <button class="btn-secondary" type="button" :disabled="currentPage <= 1" @click="prevPage">
        上一页
      </button>
      <span>第 {{ currentPage }} / {{ totalPages }} 页</span>
      <button class="btn-secondary" type="button" :disabled="currentPage >= totalPages" @click="nextPage">
        下一页
      </button>
    </div>

    <!-- 发起提案表单，入口放在提案与投票页 -->
    <div v-if="showCreate" class="modal-backdrop">
      <div class="modal">
        <h3>发起新提案</h3>
        <p class="hint">该表单负责链下提案内容记录，上链与投票由合约控制。</p>

        <div class="field">
          <label>提案标题</label>
          <input v-model="form.title" maxlength="50" />
        </div>
        <div class="field">
          <label>提案详情</label>
          <textarea v-model="form.desc" rows="4" maxlength="2000" />
        </div>
        <div class="field-row">
          <div class="field">
            <label>计票模式</label>
            <select v-model="form.mode">
              <option value="0">一人一票</option>
              <option value="1">面积加权</option>
            </select>
          </div>
          <div class="field">
            <label>投票截止时间</label>
            <input v-model="form.deadline" type="datetime-local" />
          </div>
        </div>

        <div v-if="createError" class="error">{{ createError }}</div>

        <div class="actions">
          <button type="button" class="btn-secondary" @click="closeCreate">取消</button>
          <button type="button" class="btn-primary" :disabled="creating" @click="submitCreate">
            {{ creating ? '创建中...' : '提交提案' }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.page {
  max-width: 960px;
  margin: 0 auto;
}

.header {
  margin-bottom: 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.sub {
  font-size: 13px;
  color: #4b5563;
}

.toolbar {
  margin-bottom: 12px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.search-input {
  width: 360px;
  max-width: 100%;
  height: 34px;
  border-radius: 8px;
  border: 1px solid #d1d5db;
  padding: 0 10px;
  font-size: 13px;
}

.category-tabs {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.tab-btn {
  border: 1px solid #d1d5db;
  background: #fff;
  color: #374151;
  border-radius: 999px;
  padding: 4px 12px;
  font-size: 13px;
  cursor: pointer;
}

.tab-btn.active {
  border-color: #1f6feb;
  background: #1f6feb;
  color: #fff;
}

.btn-primary,
.btn-secondary {
  border-radius: 999px;
  padding: 6px 14px;
  font-size: 13px;
  cursor: pointer;
  border: none;
}

.btn-primary {
  background-color: #1f6feb;
  color: #fff;
}

.btn-secondary {
  background-color: #e5ecff;
  color: #1f3b8a;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.card {
  background: #fff;
  border-radius: 12px;
  padding: 14px 16px;
  box-shadow: 0 1px 4px rgba(148, 163, 184, 0.3);
  cursor: pointer;
}

.card:hover {
  box-shadow: 0 4px 12px rgba(148, 163, 184, 0.45);
}

.title-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.meta {
  margin-top: 6px;
  font-size: 12px;
  color: #6b7280;
  display: flex;
  justify-content: space-between;
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

.empty {
  font-size: 14px;
  color: #6b7280;
}

.pagination {
  margin-top: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  font-size: 13px;
  color: #374151;
}

.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.35);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 50;
}

.modal {
  width: 520px;
  max-width: 95%;
  background: #fff;
  border-radius: 16px;
  padding: 20px 22px;
  box-shadow: 0 10px 30px rgba(15, 23, 42, 0.5);
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
}

.field-row {
  display: flex;
  gap: 10px;
  margin-top: 8px;
}

label {
  font-size: 13px;
  color: #374151;
}

input,
textarea,
select {
  border-radius: 8px;
  border: 1px solid #d1d5db;
  padding: 6px 10px;
  font-size: 14px;
}

.actions {
  margin-top: 14px;
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.error {
  margin-top: 6px;
  font-size: 12px;
  color: #b91c1c;
}

.hint {
  font-size: 12px;
  color: #6b7280;
}
</style>

