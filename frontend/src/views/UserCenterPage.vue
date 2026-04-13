<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue';
import { useWalletStore } from '../stores/wallet';

const wallet = useWalletStore();
const identity = ref<any>(null);
const votes = ref<any[]>([]);
const loading = ref(true);
const showFullInfo = ref(false);

// 筛选与分页
const filterDate = ref('');
const currentPage = ref(1);
const pageSize = 5;

async function loadData() {
  if (!wallet.address) return;
  loading.value = true;
  try {
    const idResp = await fetch('http://127.0.0.1:8080/api/identity/me', {
      headers: { 'X-Wallet-Addr': wallet.address }
    });
    if (idResp.ok) identity.value = await idResp.json();

    const voteResp = await fetch('http://127.0.0.1:8080/api/user/votes', {
      headers: { 'X-Wallet-Addr': wallet.address }
    });
    if (voteResp.ok) votes.value = await voteResp.json();
  } catch (err) {
    console.error('加载个人中心数据失败:', err);
  } finally {
    loading.value = false;
  }
}

// 1. 基础过滤逻辑
const filteredVotes = computed(() => {
  if (!filterDate.value) return votes.value;
  return votes.value.filter(v => v.voteTime && v.voteTime.startsWith(filterDate.value));
});

// 2. 分页切片逻辑
const totalPages = computed(() => Math.ceil(filteredVotes.value.length / pageSize) || 1);

const pagedVotes = computed(() => {
  const start = (currentPage.value - 1) * pageSize;
  const end = start + pageSize;
  return filteredVotes.value.slice(start, end);
});

// 统计信息
const totalWeight = computed(() => votes.value.reduce((acc, v) => acc + Number(v.voteWeight), 0));

// 监听筛选条件变化，重置页码
watch([filterDate, votes], () => {
  currentPage.value = 1;
});

function getChoiceLabel(c: number) {
  return c === 1 ? '赞成' : c === 2 ? '反对' : '弃权';
}

function changePage(page: number) {
  if (page >= 1 && page <= totalPages.value) {
    currentPage.value = page;
    window.scrollTo({ top: 0, behavior: 'smooth' });
  }
}

onMounted(loadData);
</script>

<template>
  <div class="user-center">
    <section v-if="identity" class="hero-card">
      <div class="hero-bg"></div>
      <div class="hero-content">
        <div class="user-main">
          <div class="avatar-circle">{{ identity.realName?.[0] }}</div>
          <div class="user-text">
            <h2>{{ identity.realName }}</h2>
            <p class="addr-tag">{{ wallet.address?.slice(0, 6) }}...{{ wallet.address?.slice(-4) }}</p>
          </div>
          <button class="detail-trigger" @click="showFullInfo = true">
            查看完整档案
          </button>
        </div>
        
        <div class="stats-row">
          <div class="stat-item">
            <span class="stat-val">{{ votes.length }}</span>
            <span class="stat-label">参与提案</span>
          </div>
          <div class="stat-sep"></div>
          <div class="stat-item">
            <span class="stat-val">{{ totalWeight }} <small>㎡</small></span>
            <span class="stat-label">总计票权重</span>
          </div>
          <div class="stat-sep"></div>
          <div class="stat-item">
            <span class="stat-val">{{ identity.houseArea }} <small>㎡</small></span>
            <span class="stat-label">登记房产面积</span>
          </div>
        </div>
      </div>
    </section>

    <section class="records-section">
      <div class="section-header">
        <div class="title-group">
          <h3>我的治理足迹</h3>
          <span class="count-badge">{{ filteredVotes.length }} 条记录</span>
        </div>
        <div class="filter-box">
          <input type="date" v-model="filterDate" class="date-input" />
        </div>
      </div>

      <div v-if="loading" class="loading-state">
        <div class="pulse-loader"></div>
        <p>正在同步链上治理记录...</p>
      </div>

      <div v-else-if="pagedVotes.length === 0" class="empty-state">
        <div class="empty-icon">📂</div>
        <p>在该筛选条件下暂无投票记录</p>
      </div>
      
      <div v-else class="vote-container">
        <div class="vote-grid">
          <div v-for="v in pagedVotes" :key="v.id" class="modern-vote-card">
            <div class="card-top">
              <h4 class="prop-title">{{ v.propTitle || '载入中...' }}</h4>
              <div class="choice-indicator" :class="'c-' + v.voteChoice">
                {{ getChoiceLabel(v.voteChoice) }}
              </div>
            </div>
            
            <div class="card-mid">
              <div class="info-tag">
                <span class="label">投入权重</span>
                <span class="val">{{ v.voteWeight }} ㎡</span>
              </div>
              <div class="info-tag">
                <span class="label">投票时间</span>
                <span class="val">{{ new Date(v.voteTime).toLocaleString() }}</span>
              </div>
            </div>

            <div class="card-bottom">
              <a :href="'https://sepolia.etherscan.io/tx/' + v.voteTxHash" target="_blank" class="hash-link">
                <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" stroke-width="2"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path></svg>
                链上哈希验证
              </a>
            </div>
          </div>
        </div>

        <div v-if="totalPages > 1" class="pagination-wrapper">
          <button 
            class="p-btn" 
            :disabled="currentPage === 1" 
            @click="changePage(currentPage - 1)"
          >上一页</button>
          
          <div class="page-numbers">
            <button 
              v-for="p in totalPages" 
              :key="p" 
              class="n-btn" 
              :class="{ active: p === currentPage }"
              @click="changePage(p)"
            >{{ p }}</button>
          </div>

          <button 
            class="p-btn" 
            :disabled="currentPage === totalPages" 
            @click="changePage(currentPage + 1)"
          >下一页</button>
        </div>
      </div>
    </section>

    <div v-if="showFullInfo" class="glass-modal" @click.self="showFullInfo = false">
      <div class="modal-box">
        <header class="modal-header">
          <h3>业主身份数字凭证</h3>
          <button class="close-x" @click="showFullInfo = false">&times;</button>
        </header>
        <div class="detail-list">
          <div class="detail-row"><label>姓名</label><span>{{ identity.realName }}</span></div>
          <div class="detail-row"><label>身份核验</label><span>********{{ identity.idCard4 }}</span></div>
          <div class="detail-row"><label>所属社区</label><span>智慧家园 {{ identity.buildNo }}栋{{ identity.roomNo }}室</span></div>
          <div class="detail-row"><label>房产面积</label><span>{{ identity.houseArea }} ㎡</span></div>
          <div class="detail-row"><label>钱包地址</label><code class="addr-code">{{ identity.walletAddr }}</code></div>
        </div>
        <div class="modal-footer">
          <p class="security-hint">该信息已由居委会人工审核并上链加密</p>
          <button class="btn-done" @click="showFullInfo = false">返回个人中心</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.user-center { max-width: 900px; margin: 0 auto; padding: 20px; font-family: system-ui, -apple-system, sans-serif; }

/* 英雄卡片设计 */
.hero-card { 
  position: relative; border-radius: 24px; overflow: hidden; color: white;
  background: linear-gradient(135deg, #2563eb 0%, #1e40af 100%);
  box-shadow: 0 20px 25px -5px rgba(37, 99, 235, 0.2); margin-bottom: 40px;
}
.hero-content { position: relative; padding: 40px; z-index: 2; }
.user-main { display: flex; align-items: center; gap: 20px; margin-bottom: 40px; }
.avatar-circle { 
  width: 64px; height: 64px; background: rgba(255,255,255,0.2); 
  border-radius: 50%; display: flex; align-items: center; justify-content: center;
  font-size: 24px; font-weight: 800; border: 2px solid rgba(255,255,255,0.4);
}
.user-text h2 { margin: 0; font-size: 24px; }
.addr-tag { margin: 4px 0 0 0; font-family: monospace; opacity: 0.8; font-size: 14px; }
.detail-trigger { 
  margin-left: auto; background: rgba(255,255,255,0.15); border: 1px solid rgba(255,255,255,0.3);
  color: white; padding: 8px 18px; border-radius: 12px; cursor: pointer; backdrop-filter: blur(8px);
  transition: all 0.2s;
}
.detail-trigger:hover { background: rgba(255,255,255,0.25); }

.stats-row { display: flex; align-items: center; justify-content: space-around; background: rgba(0,0,0,0.1); border-radius: 16px; padding: 20px; }
.stat-item { text-align: center; }
.stat-val { display: block; font-size: 22px; font-weight: 800; }
.stat-val small { font-size: 14px; font-weight: 400; opacity: 0.8; }
.stat-label { font-size: 12px; opacity: 0.7; font-weight: 500; text-transform: uppercase; letter-spacing: 0.5px; }
.stat-sep { width: 1px; height: 30px; background: rgba(255,255,255,0.2); }

/* 列表区样式 */
.records-section { background: white; border-radius: 24px; padding: 32px; border: 1px solid #e2e8f0; }
.section-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 30px; }
.title-group h3 { margin: 0; font-size: 18px; color: #0f172a; }
.count-badge { background: #eff6ff; color: #2563eb; font-size: 12px; font-weight: 600; padding: 2px 10px; border-radius: 999px; margin-top: 4px; display: inline-block; }
.date-input { border: 1px solid #e2e8f0; padding: 10px 16px; border-radius: 12px; outline: none; transition: border-color 0.2s; }
.date-input:focus { border-color: #2563eb; }

/* 现代卡片样式 */
.vote-grid { display: flex; flex-direction: column; gap: 16px; }
.modern-vote-card { 
  border: 1px solid #f1f5f9; background: #f8fafc; border-radius: 16px; padding: 20px;
  transition: all 0.2s; cursor: default;
}
.modern-vote-card:hover { transform: scale(1.01); border-color: #cbd5e1; background: white; box-shadow: 0 10px 15px -3px rgba(0,0,0,0.05); }

.card-top { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 16px; }
.prop-title { margin: 0; font-size: 16px; color: #0f172a; font-weight: 700; flex: 1; padding-right: 20px; }
.choice-indicator { padding: 4px 14px; border-radius: 999px; font-size: 12px; font-weight: 700; }
.c-1 { background: #dcfce7; color: #166534; }
.c-2 { background: #fee2e2; color: #991b1b; }
.c-3 { background: #f1f5f9; color: #475569; }

.card-mid { display: flex; gap: 32px; margin-bottom: 16px; }
.info-tag { display: flex; flex-direction: column; gap: 4px; }
.info-tag .label { font-size: 11px; color: #64748b; font-weight: 600; text-transform: uppercase; }
.info-tag .val { font-size: 13px; color: #334155; font-weight: 600; }

.card-bottom { border-top: 1px solid #edf2f7; padding-top: 12px; }
.hash-link { font-size: 12px; color: #2563eb; text-decoration: none; display: flex; align-items: center; gap: 6px; font-weight: 600; }
.hash-link:hover { color: #1d4ed8; }

/* 分页逻辑样式 */
.pagination-wrapper { display: flex; justify-content: center; align-items: center; gap: 20px; margin-top: 40px; padding-top: 20px; border-top: 1px solid #f1f5f9; }
.p-btn { 
  background: white; border: 1px solid #e2e8f0; padding: 8px 16px; border-radius: 10px;
  font-size: 13px; font-weight: 600; cursor: pointer; transition: all 0.2s;
}
.p-btn:disabled { opacity: 0.4; cursor: not-allowed; }
.p-btn:hover:not(:disabled) { border-color: #2563eb; color: #2563eb; }

.page-numbers { display: flex; gap: 8px; }
.n-btn { 
  width: 36px; height: 36px; border: 1px solid #e2e8f0; background: white; 
  border-radius: 10px; font-size: 13px; cursor: pointer; transition: all 0.2s;
}
.n-btn.active { background: #2563eb; border-color: #2563eb; color: white; font-weight: 700; }
.n-btn:hover:not(.active) { border-color: #2563eb; color: #2563eb; }

/* 模态框样式 */
.glass-modal { position: fixed; inset: 0; background: rgba(15, 23, 42, 0.4); backdrop-filter: blur(4px); z-index: 1000; display: flex; align-items: center; justify-content: center; }
.modal-box { background: white; border-radius: 24px; width: 480px; padding: 32px; box-shadow: 0 25px 50px -12px rgba(0,0,0,0.25); }
.modal-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 24px; }
.close-x { background: #f1f5f9; border: none; width: 32px; height: 32px; border-radius: 50%; cursor: pointer; font-size: 20px; }

.detail-list { display: flex; flex-direction: column; gap: 16px; margin-bottom: 32px; }
.detail-row { display: flex; justify-content: space-between; padding-bottom: 12px; border-bottom: 1px solid #f1f5f9; }
.detail-row label { color: #64748b; font-size: 14px; }
.detail-row span { font-weight: 600; color: #0f172a; }
.addr-code { background: #f1f5f9; padding: 4px 8px; border-radius: 6px; font-family: monospace; font-size: 12px; color: #2563eb; }

.modal-footer { text-align: center; }
.security-hint { font-size: 11px; color: #94a3b8; margin-bottom: 16px; }
.btn-done { width: 100%; background: #0f172a; color: white; border: none; padding: 14px; border-radius: 14px; font-weight: 700; cursor: pointer; }

/* 状态加载 */
.loading-state { text-align: center; padding: 60px 0; color: #64748b; }
.pulse-loader { width: 40px; height: 40px; background: #2563eb; border-radius: 50%; margin: 0 auto 16px; animation: pulse 1.5s infinite; opacity: 0.6; }
@keyframes pulse { 0% { transform: scale(0.8); opacity: 0.6; } 50% { transform: scale(1.1); opacity: 0.3; } 100% { transform: scale(0.8); opacity: 0.6; } }
</style>