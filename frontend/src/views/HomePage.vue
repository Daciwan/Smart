<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useRouter } from 'vue-router';

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

async function loadProposals() {
  loading.value = true;
  try {
    const resp = await fetch('http://127.0.0.1:8080/api/proposals');
    if (!resp.ok) {
      throw new Error('加载失败');
    }
    proposals.value = await resp.json();
  } catch (err) {
    console.error(err);
  } finally {
    loading.value = false;
  }
}

function goDetail(p: Proposal) {
  router.push(`/proposals/${p.id}`);
}

function statusLabel(s: number) {
  if (s === 0) return '进行中';
  if (s === 1) return '已截至但未结算';
  if (s === 2) return '已通过';
  if (s === 3) return '已驳回';
  return '未知';
}

onMounted(() => {
  loadProposals();
});
</script>

<template>
  <div class="home">
    <section class="hero">
      <div class="hero-text">
        <h1>社区协同决策一览</h1>
        <p>下方展示的是历史提案与当前正在进行的提案，你可以点击查看详情与投票结果。</p>
      </div>
    </section>

    <section class="list-section">
      <h2 class="section-title">历史提案</h2>
      <div v-if="loading">加载中...</div>
      <div v-else-if="!proposals.length" class="empty">当前暂无提案</div>
      <div v-else class="list">
        <div
          v-for="p in proposals"
          :key="p.id"
          class="card"
          role="button"
          tabindex="0"
          @click="goDetail(p)"
        >
          <div class="title-row">
            <h3>{{ p.propTitle }}</h3>
            <span class="status" :data-status="p.propStatus">{{ statusLabel(p.propStatus) }}</span>
          </div>
          <div class="meta">
            <span>发起人：{{ p.creatorAddr?.slice(0, 6) }}...{{ p.creatorAddr?.slice(-4) }}</span>
            <span>截止时间：{{ new Date(p.deadline).toLocaleString() }}</span>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.home {
  max-width: 1120px;
  margin: 0 auto;
}

.hero {
  padding: 24px 24px 12px;
}

.hero-text h1 {
  font-size: 24px;
  margin-bottom: 8px;
}

.hero-text p {
  font-size: 14px;
  color: #4b5563;
}

.list-section {
  margin-top: 8px;
}

.section-title {
  font-size: 16px;
  margin-bottom: 8px;
}

.list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.card {
  background: #fff;
  border-radius: 12px;
  padding: 12px 14px;
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
</style>

