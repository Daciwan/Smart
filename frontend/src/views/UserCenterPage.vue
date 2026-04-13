<script setup lang="ts">
import { ref, onMounted, computed } from 'vue';
import { useWalletStore } from '../stores/wallet';

const wallet = useWalletStore();
const identity = ref<any>(null);
const votes = ref<any[]>([]);
const loading = ref(true);
const showFullInfo = ref(false);

// 筛选方案
const filterDate = ref('');

async function loadData() {
  if (!wallet.address) return;
  loading.value = true;
  try {
    // 1. 加载身份信息
    const idResp = await fetch('http://127.0.0.1:8080/api/identity/me', {
      headers: { 'X-Wallet-Addr': wallet.address }
    });
    if (idResp.ok) identity.value = await idResp.json();

    // 2. 加载投票记录
    const voteResp = await fetch('http://127.0.0.1:8080/api/user/votes', {
      headers: { 'X-Wallet-Addr': wallet.address }
    });
    if (voteResp.ok) votes.value = await voteResp.json();
  } finally {
    loading.value = false;
  }
}

const filteredVotes = computed(() => {
  if (!filterDate.value) return votes.value;
  return votes.value.filter(v => v.voteTime.startsWith(filterDate.value));
});

function getChoiceLabel(c: number) {
  return c === 1 ? '赞成' : c === 2 ? '反对' : '弃权';
}

onMounted(loadData);
</script>

<template>
  <div class="user-center">
    <section v-if="identity" class="info-card">
      <div class="card-header">
        <h3>已认证身份信息</h3>
        <button class="btn-ghost" @click="showFullInfo = true">详情</button>
      </div>
      <div class="summary-info">
        <p><strong>姓名：</strong>{{ identity.realName }}</p>
        <p><strong>住址：</strong>{{ identity.buildNo }}栋{{ identity.roomNo }}室</p>
      </div>
    </section>

    <section class="vote-history">
      <div class="history-header">
        <h3>我的投票记录</h3>
        <input type="date" v-model="filterDate" class="date-picker" />
      </div>

      <div v-if="loading" class="state-msg">加载中...</div>
      <div v-else-if="filteredVotes.length === 0" class="state-msg">暂无投票记录</div>
      
      <div v-else class="vote-list">
        <div v-for="v in filteredVotes" :key="v.id" class="vote-item">
          <div class="vote-main">
            <h4>{{ v.propTitle || '未知提案' }}</h4>
            <span class="choice-tag" :class="'choice-' + v.voteChoice">
              {{ getChoiceLabel(v.voteChoice) }}
            </span>
          </div>
          <div class="vote-meta">
            <span>权重: {{ v.voteWeight }}㎡</span>
            <span>时间: {{ new Date(v.voteTime).toLocaleDateString() }}</span>
            <a :href="'https://sepolia.etherscan.io/tx/' + v.voteTxHash" target="_blank" class="tx-link">查看交易</a>
          </div>
        </div>
      </div>
    </section>

    <div v-if="showFullInfo" class="modal-overlay" @click.self="showFullInfo = false">
      <div class="modal-content">
        <h3>认证详情</h3>
        <div class="detail-grid">
          <div class="item"><label>姓名</label><span>{{ identity.realName }}</span></div>
          <div class="item"><label>身份证号</label><span>********{{ identity.idCard4 }}</span></div>
          <div class="item"><label>联系电话</label><span>{{ identity.phoneNo }}</span></div>
          <div class="item"><label>房产面积</label><span>{{ identity.houseArea }} ㎡</span></div>
          <div class="item"><label>钱包地址</label><span class="addr">{{ identity.walletAddr }}</span></div>
        </div>
        <button class="btn-primary" @click="showFullInfo = false">关闭</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.info-card { background: white; padding: 24px; border-radius: 16px; border: 1px solid #e2e8f0; margin-bottom: 24px; }
.card-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.history-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px; }
.date-picker { padding: 8px; border-radius: 8px; border: 1px solid #cbd5e1; }
.vote-item { background: #f8fafc; padding: 16px; border-radius: 12px; margin-bottom: 12px; border: 1px solid #f1f5f9; }
.vote-main { display: flex; justify-content: space-between; margin-bottom: 8px; }
.choice-tag { padding: 4px 12px; border-radius: 999px; font-size: 12px; font-weight: 600; }
.choice-1 { background: #ecfdf5; color: #059669; }
.choice-2 { background: #fef2f2; color: #dc2626; }
.choice-3 { background: #f1f5f9; color: #475569; }
.vote-meta { display: flex; gap: 20px; font-size: 13px; color: #64748b; }
.modal-overlay { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 1000; }
.modal-content { background: white; padding: 32px; border-radius: 20px; width: 480px; }
.detail-grid { display: flex; flex-direction: column; gap: 16px; margin: 24px 0; }
.detail-grid .item { display: flex; justify-content: space-between; }
.detail-grid label { color: #64748b; }
.addr { font-family: monospace; font-size: 12px; }
</style>