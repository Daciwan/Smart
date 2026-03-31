<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { useRouter } from 'vue-router';
import { formatProposalDeadline, proposalStatusLabel } from '../utils/proposal';

interface Proposal {
  id: number;
  propTitle: string;
  creatorAddr: string;
  createTime: string;
  deadline: string;
  propStatus: number;
}

const router = useRouter();
const proposals = ref<Proposal[]>([]);
const loading = ref(false);
let pollTimer: ReturnType<typeof setInterval> | null = null;

// 增加 isSilent 参数，静默刷新时不会显示加载动画
async function loadProposals(isSilent = false) {
  if (!isSilent) loading.value = true;
  try {
    const resp = await fetch('http://127.0.0.1:8080/api/proposals');
    if (!resp.ok) {
      throw new Error('加载失败');
    }
    proposals.value = await resp.json();
  } catch (err) {
    console.error(err);
  } finally {
    if (!isSilent) loading.value = false;
  }
}

function goDetail(p: Proposal) {
  router.push(`/proposals/${p.id}`);
}

const latestThreeProposals = computed(() => {
  return [...proposals.value]
    .sort((a, b) => new Date(b.createTime).getTime() - new Date(a.createTime).getTime())
    .slice(0, 3);
});

const passedProposals = computed(() => {
  return [...proposals.value]
    .filter(p => p.propStatus === 2)
    .sort((a, b) => new Date(b.createTime).getTime() - new Date(a.createTime).getTime())
    .slice(0, 3);
});

onMounted(() => {
  loadProposals(); // 首次正常加载
  
  // 每 5 秒后台静默刷新一次数据，对齐后端更新频率
  pollTimer = setInterval(() => {
    loadProposals(true); 
  }, 5000);
});

// 组件销毁时清除定时器，防止内存泄漏
onUnmounted(() => {
  if (pollTimer) clearInterval(pollTimer);
});
</script>

<template>
  <div class="home">
    <section class="hero">
      <div class="hero-text">
        <h1>社区协同决策看板</h1>
        <p>透明、公开、不可篡改。在这里浏览社区的核心提案，你的每一次投票都将记录在链，共同决定社区的未来。</p>
      </div>
    </section>

    <section class="list-section">
      <h2 class="section-title">最新提案</h2>
      
      <div v-if="loading" class="loading-state">
        <div class="spinner"></div>
        <span>同步链上数据中...</span>
      </div>
      
      <div v-else-if="!latestThreeProposals.length" class="empty-state">
        <svg viewBox="0 0 24 24" width="48" height="48" stroke="#cbd5e1" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><line x1="3" y1="9" x2="21" y2="9"></line><line x1="9" y1="21" x2="9" y2="9"></line></svg>
        <p>当前暂无提案，前往发起你的第一个社区提议吧。</p>
      </div>
      
      <div v-else class="list">
        <div
          v-for="p in latestThreeProposals"
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
    </section>

    <section class="list-section passed-section">
      <h2 class="section-title success-title">
        <svg viewBox="0 0 24 24" width="22" height="22" stroke="currentColor" stroke-width="2.5" fill="none" stroke-linecap="round" stroke-linejoin="round" class="success-icon"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"></path><polyline points="22 4 12 14.01 9 11.01"></polyline></svg>
        社区已达成共识
      </h2>
      <p class="section-subtitle">以下提案已在区块链上完成结算，并获得社区投票通过，即将进入执行阶段。</p>
      
      <div v-if="loading" class="loading-state">
        <div class="spinner"></div>
        <span>同步链上数据中...</span>
      </div>
      
      <div v-else-if="!passedProposals.length" class="empty-state">
        <svg viewBox="0 0 24 24" width="48" height="48" stroke="#cbd5e1" stroke-width="1.5" fill="none" stroke-linecap="round" stroke-linejoin="round"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path></svg>
        <p>暂无已通过的提案记录。</p>
      </div>
      
      <div v-else class="list">
        <div
          v-for="p in passedProposals"
          :key="p.id"
          class="card success-card"
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
              <span class="meta-label">截票时间</span>
              <span class="meta-value time">{{ formatProposalDeadline(p.deadline) }}</span>
            </div>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.home {
  animation: fadeIn 0.4s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

/* 头部 Banner 区域 */
.hero {
  margin-bottom: 40px;
  background: linear-gradient(135deg, #ffffff 0%, #f1f5f9 100%);
  padding: 40px;
  border-radius: 20px;
  border: 1px solid #e2e8f0;
  box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.02);
}

.hero-text h1 {
  font-size: 32px;
  font-weight: 800;
  color: #0f172a;
  margin-bottom: 12px;
  letter-spacing: -0.5px;
}

.hero-text p {
  font-size: 16px;
  color: #64748b;
  line-height: 1.6;
  max-width: 600px;
}

/* 列表区块及标题 */
.list-section {
  margin-bottom: 48px;
}

.section-title {
  font-size: 20px;
  font-weight: 700;
  color: #0f172a;
  margin-bottom: 24px;
  display: flex;
  align-items: center;
  gap: 10px;
}

.section-title::before {
  content: '';
  display: block;
  width: 4px;
  height: 20px;
  background: #2563eb;
  border-radius: 4px;
}

/* 已通过区块特殊样式 */
.passed-section {
  padding-top: 32px;
  border-top: 1px dashed #cbd5e1;
}

.success-title {
  color: #065f46;
  margin-bottom: 8px;
}

.success-title::before {
  display: none; /* 隐藏默认蓝条 */
}

.success-icon {
  color: #10b981;
}

.section-subtitle {
  font-size: 14px;
  color: #64748b;
  margin-bottom: 24px;
}

/* Grid 卡片布局 */
.list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 24px;
}

.card {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 16px;
  padding: 24px;
  transition: all 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  cursor: pointer;
  position: relative;
  overflow: hidden;
}

.card:hover {
  transform: translateY(-4px);
  box-shadow: 0 12px 24px -8px rgba(15, 23, 42, 0.1);
  border-color: #cbd5e1;
}

/* 已通过卡片的微弱绿底渐变，增加荣誉感 */
.success-card {
  background: linear-gradient(180deg, #ffffff 0%, #f0fdf4 100%);
  border-color: #d1fae5;
}

.success-card:hover {
  border-color: #6ee7b7;
  box-shadow: 0 12px 24px -8px rgba(16, 185, 129, 0.15);
}

.title-row {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  margin-bottom: 24px;
}

.title-row h3 {
  font-size: 18px;
  font-weight: 700;
  color: #1e293b;
  margin: 0;
  line-height: 1.4;
  flex: 1;
}

/* 状态标签样式设计 */
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

/* 0: 进行中 */
.status[data-status='0'] { background-color: #ecfdf5; color: #059669; }
.status[data-status='0'] .status-dot { background-color: #059669; }

/* 1: 待执行 (未结算) */
.status[data-status='1'] { background-color: #fefce8; color: #ca8a04; }
.status[data-status='1'] .status-dot { background-color: #ca8a04; }

/* 2: 已完成 (已通过) */
.status[data-status='2'] { background-color: #eff6ff; color: #2563eb; }
.status[data-status='2'] .status-dot { background-color: #2563eb; }

/* 3: 已拒绝/失败 */
.status[data-status='3'] { background-color: #fef2f2; color: #dc2626; }
.status[data-status='3'] .status-dot { background-color: #dc2626; }

/* 提案元数据 */
.meta-data {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px dashed #e2e8f0;
}

.success-card .meta-data {
  border-top-color: #a7f3d0;
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

.success-card .meta-value {
  background: #ffffff;
  border: 1px solid #d1fae5;
}

.meta-value.time {
  font-family: 'Inter', sans-serif;
}

/* 空状态与加载状态 */
.empty-state, .loading-state {
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
}

.empty-state p {
  font-size: 15px;
}

.spinner {
  width: 24px;
  height: 24px;
  border: 3px solid #e2e8f0;
  border-top-color: #2563eb;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>