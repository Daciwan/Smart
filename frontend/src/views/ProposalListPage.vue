<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue';
import { useRouter } from 'vue-router';
import { ethers } from 'ethers';
import { useWalletStore } from '../stores/wallet';
import { GOVERNOR_CONTRACT_ADDRESS } from '../config';
import { formatProposalDeadline, parseDateTimeLocalToPayload, proposalStatusLabel } from '../utils/proposal';

interface Proposal {
  id: number;
  propTitle: string;
  creatorAddr: string;
  createTime: string;
  deadline: string;
  propStatus: number;
  imagePaths?: string[];
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
const imageFiles = ref<File[]>([]);

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
  imageFiles.value = [];
}

function onSelectImages(e: Event) {
  const target = e.target as HTMLInputElement;
  const files = target.files ? Array.from(target.files) : [];
  imageFiles.value = files;
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
    const parsed = parseDateTimeLocalToPayload(form.value.deadline);
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
    const body = new FormData();
    body.append('propTitle', title);
    body.append('propDesc', desc);
    body.append('propType', String(propType));
    body.append('deadline', parsed.text);
    body.append('creatorAddr', wallet.address);
    body.append('propId', String(onchainId));
    body.append('txHash', tx.hash);
    for (const f of imageFiles.value) {
      body.append('images', f);
    }

    const resp = await fetch('http://127.0.0.1:8080/api/proposals', {
      method: 'POST',
      body,
    });
    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(data.error || '创建失败（链下部分）');
    }

    alert('提案已成功创建：链上 + 链下均已记录');
    showCreate.value = false;
    form.value = { title: '', desc: '', mode: '0', deadline: '' };
    imageFiles.value = [];
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
      <div class="header-text">
        <h2>社区提案与投票</h2>
        <span class="sub">浏览、搜索并参与社区治理的各项决策</span>
      </div>
      <button class="btn-primary" type="button" @click="openCreate">
        <svg viewBox="0 0 24 24" width="16" height="16" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><line x1="12" y1="5" x2="12" y2="19"></line><line x1="5" y1="12" x2="19" y2="12"></line></svg>
        发起提案
      </button>
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
      <div class="search-wrapper">
        <svg class="search-icon" viewBox="0 0 24 24" width="16" height="16" stroke="#94a3b8" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><line x1="21" y1="21" x2="16.65" y2="16.65"></line></svg>
        <input
          v-model.trim="keyword"
          class="search-input"
          type="text"
          placeholder="搜索提案标题..."
        />
      </div>
    </div>

    <div v-if="loading" class="state-container">
      <div class="spinner"></div>
      <span>加载提案数据中...</span>
    </div>
    
    <div v-else-if="!filteredProposals.length" class="state-container empty">
      <svg viewBox="0 0 24 24" width="48" height="48" stroke="#cbd5e1" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><line x1="3" y1="9" x2="21" y2="9"></line><line x1="9" y1="21" x2="9" y2="9"></line></svg>
      <p>暂无匹配的提案，换个关键词或分类试试吧</p>
    </div>

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
          <span class="status" :data-status="p.propStatus">
            <span class="status-dot"></span>
            {{ proposalStatusLabel(p.propStatus) }}
          </span>
        </div>
        <div class="meta-data">
          <div class="meta-item">
            <span class="meta-label">发起人</span>
            <span class="meta-value">{{ p.creatorAddr?.slice(0, 6) }}...{{ p.creatorAddr?.slice(-4) }}</span>
          </div>
          <div class="meta-item">
            <span class="meta-label">截止时间</span>
            <span class="meta-value time">{{ formatProposalDeadline(p.deadline) }}</span>
          </div>
        </div>
      </div>
    </div>

    <div v-if="!loading && filteredProposals.length" class="pagination">
      <button class="page-btn" type="button" :disabled="currentPage <= 1" @click="prevPage">上一页</button>
      <span class="page-info">第 <strong>{{ currentPage }}</strong> / {{ totalPages }} 页</span>
      <button class="page-btn" type="button" :disabled="currentPage >= totalPages" @click="nextPage">下一页</button>
    </div>

    <transition name="fade">
      <div v-if="showCreate" class="modal-backdrop">
        <div class="modal">
          <div class="modal-header">
            <h3>发起新提案</h3>
            <button class="close-btn" @click="closeCreate">&times;</button>
          </div>
          <p class="hint">该表单负责链下详情记录，核心逻辑上链并由智能合约控制。</p>

          <div class="form-content">
            <div class="field">
              <label>提案标题 <span class="required">*</span></label>
              <input v-model="form.title" maxlength="50" placeholder="请输入简明扼要的标题 (4-50字符)" />
            </div>
            <div class="field">
              <label>提案详情 <span class="required">*</span></label>
              <textarea v-model="form.desc" rows="4" maxlength="2000" placeholder="详细描述该提案的背景、目的及实施方案..." />
            </div>
            <div class="field-row">
              <div class="field">
                <label>计票模式 <span class="required">*</span></label>
                <div class="select-wrapper">
                  <select v-model="form.mode">
                    <option value="0">一人一票</option>
                    <option value="1">面积加权</option>
                  </select>
                </div>
              </div>
              <div class="field">
                <label>投票截止时间 <span class="required">*</span></label>
                <input v-model="form.deadline" type="datetime-local" />
              </div>
            </div>
            <div class="field">
              <label>附加图片 (可选)</label>
              <div class="file-upload">
                <input type="file" accept="image/*" multiple @change="onSelectImages" class="file-input" />
                <div class="file-placeholder">
                  <span v-if="imageFiles.length === 0">点击选择或拖拽图片到此处</span>
                  <span v-else class="file-selected">已选择 {{ imageFiles.length }} 张图片</span>
                </div>
              </div>
            </div>

            <div v-if="createError" class="error-msg">
              <svg viewBox="0 0 24 24" width="16" height="16" stroke="currentColor" stroke-width="2" fill="none"><circle cx="12" cy="12" r="10"></circle><line x1="12" y1="8" x2="12" y2="12"></line><line x1="12" y1="16" x2="12.01" y2="16"></line></svg>
              {{ createError }}
            </div>
          </div>

          <div class="actions">
            <button type="button" class="btn-ghost" @click="closeCreate">取消</button>
            <button type="button" class="btn-primary submit-btn" :disabled="creating" @click="submitCreate">
              <span v-if="creating" class="spinner-small"></span>
              {{ creating ? '上链确认中...' : '提交上链' }}
            </button>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<style scoped>
.page {
  animation: fadeIn 0.4s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.header {
  margin-bottom: 24px;
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  padding-bottom: 16px;
  border-bottom: 1px solid #e2e8f0;
}

.header-text h2 {
  font-size: 28px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 8px 0;
  letter-spacing: -0.5px;
}

.sub {
  font-size: 14px;
  color: #64748b;
}

.toolbar {
  margin-bottom: 24px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}

.category-tabs {
  display: inline-flex;
  background: #f1f5f9;
  padding: 4px;
  border-radius: 999px;
  gap: 4px;
}

.tab-btn {
  border: none;
  background: transparent;
  color: #64748b;
  border-radius: 999px;
  padding: 6px 16px;
  font-size: 14px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s;
}

.tab-btn:hover {
  color: #0f172a;
}

.tab-btn.active {
  background: #ffffff;
  color: #2563eb;
  box-shadow: 0 1px 3px rgba(0,0,0,0.1);
  font-weight: 600;
}

.search-wrapper {
  position: relative;
  flex: 1;
  max-width: 360px;
}

.search-icon {
  position: absolute;
  left: 12px;
  top: 50%;
  transform: translateY(-50%);
}

.search-input {
  width: 100%;
  height: 40px;
  border-radius: 999px;
  border: 1px solid #e2e8f0;
  padding: 0 16px 0 36px;
  font-size: 14px;
  transition: all 0.2s;
  background: #ffffff;
}

.search-input:focus {
  outline: none;
  border-color: #2563eb;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
}

.btn-primary {
  display: flex;
  align-items: center;
  gap: 6px;
  background-color: #2563eb;
  color: #fff;
  border: none;
  border-radius: 999px;
  padding: 10px 20px;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: 0 4px 6px -1px rgba(37, 99, 235, 0.2);
}

.btn-primary:hover:not(:disabled) {
  background-color: #1d4ed8;
  transform: translateY(-1px);
}

.btn-primary:disabled {
  opacity: 0.7;
  cursor: not-allowed;
}

/* 列表卡片样式 */
.list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
  gap: 20px;
}

.card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 24px;
  transition: all 0.3s ease;
  cursor: pointer;
}

.card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 24px -8px rgba(15, 23, 42, 0.1);
  border-color: #cbd5e1;
}

.title-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 20px;
}

.title-row h3 {
  font-size: 17px;
  font-weight: 700;
  color: #1e293b;
  margin: 0;
  line-height: 1.4;
  flex: 1;
}

.status {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  border-radius: 999px;
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
}

.status-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
}

.status[data-status='0'] { background-color: #ecfdf5; color: #059669; }
.status[data-status='0'] .status-dot { background-color: #059669; }
.status[data-status='1'] { background-color: #fefce8; color: #ca8a04; }
.status[data-status='1'] .status-dot { background-color: #ca8a04; }
.status[data-status='2'] { background-color: #eff6ff; color: #2563eb; }
.status[data-status='2'] .status-dot { background-color: #2563eb; }
.status[data-status='3'] { background-color: #fef2f2; color: #dc2626; }
.status[data-status='3'] .status-dot { background-color: #dc2626; }

.meta-data {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px dashed #e2e8f0;
}

.meta-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
}

.meta-label {
  color: #64748b;
  font-weight: 500;
}

.meta-value {
  color: #334155;
  font-family: ui-monospace, SFMono-Regular, monospace;
  background: #f1f5f9;
  padding: 4px 8px;
  border-radius: 6px;
  font-weight: 500;
}
.meta-value.time { font-family: inherit; }

/* 状态容器 (加载中/空数据) */
.state-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 64px 20px;
  background: #ffffff;
  border-radius: 16px;
  border: 1px dashed #cbd5e1;
  color: #64748b;
  gap: 16px;
  font-size: 15px;
}

.spinner {
  width: 28px;
  height: 28px;
  border: 3px solid #e2e8f0;
  border-top-color: #2563eb;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

/* 分页 */
.pagination {
  margin-top: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
}

.page-btn {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 8px;
  padding: 8px 16px;
  font-size: 14px;
  font-weight: 500;
  color: #334155;
  cursor: pointer;
  transition: all 0.2s;
}

.page-btn:hover:not(:disabled) {
  border-color: #2563eb;
  color: #2563eb;
}

.page-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  background: #f8fafc;
}

.page-info {
  font-size: 14px;
  color: #64748b;
}
.page-info strong { color: #0f172a; }

/* Modal 弹窗 */
.modal-backdrop {
  position: fixed;
  inset: 0;
  background: rgba(15, 23, 42, 0.4);
  backdrop-filter: blur(4px);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}

.modal {
  width: 560px;
  max-width: 90vw;
  background: #ffffff;
  border-radius: 20px;
  padding: 28px;
  box-shadow: 0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1);
  animation: modalSlideUp 0.3s cubic-bezier(0.16, 1, 0.3, 1);
}

@keyframes modalSlideUp {
  from { opacity: 0; transform: translateY(20px) scale(0.95); }
  to { opacity: 1; transform: translateY(0) scale(1); }
}

.modal-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.modal-header h3 {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
  margin: 0;
}

.close-btn {
  background: transparent;
  border: none;
  font-size: 24px;
  color: #94a3b8;
  cursor: pointer;
  transition: color 0.2s;
}
.close-btn:hover { color: #0f172a; }

.hint {
  font-size: 13px;
  color: #64748b;
  margin: 8px 0 20px 0;
}

.form-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.field-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.field label {
  font-size: 14px;
  font-weight: 600;
  color: #334155;
}

.required { color: #ef4444; }

input, textarea, select {
  width: 100%;
  border-radius: 8px;
  border: 1px solid #cbd5e1;
  padding: 10px 12px;
  font-size: 14px;
  color: #0f172a;
  transition: all 0.2s;
  background: #f8fafc;
  font-family: inherit;
}

input:focus, textarea:focus, select:focus {
  outline: none;
  border-color: #2563eb;
  background: #ffffff;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
}

.file-upload {
  position: relative;
  border: 1px dashed #cbd5e1;
  border-radius: 8px;
  background: #f8fafc;
  padding: 16px;
  text-align: center;
  transition: all 0.2s;
}
.file-upload:hover { border-color: #2563eb; background: #eff6ff; }

.file-input {
  position: absolute;
  inset: 0;
  opacity: 0;
  cursor: pointer;
  width: 100%;
}

.file-placeholder {
  font-size: 13px;
  color: #64748b;
  pointer-events: none;
}
.file-selected { color: #2563eb; font-weight: 500; }

.error-msg {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  background: #fef2f2;
  border: 1px solid #fecaca;
  color: #dc2626;
  border-radius: 8px;
  font-size: 13px;
  font-weight: 500;
}

.actions {
  margin-top: 24px;
  display: flex;
  justify-content: flex-end;
  gap: 12px;
}

.btn-ghost {
  padding: 10px 20px;
  border-radius: 999px;
  border: 1px solid transparent;
  background: transparent;
  color: #64748b;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s;
}
.btn-ghost:hover { background: #f1f5f9; color: #0f172a; }

.spinner-small {
  width: 14px; height: 14px;
  border: 2px solid #ffffff;
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }

/* Vue 动画过渡 */
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>