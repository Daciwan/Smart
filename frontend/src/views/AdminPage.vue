<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { ethers } from 'ethers';
import { useWalletStore } from '../stores/wallet';
import { GOVERNOR_CONTRACT_ADDRESS } from '../config';

const wallet = useWalletStore();

interface UserItem {
  id: number;
  walletAddr: string;
  realName: string;
  buildNo: string;
  unitNo: string;
  roomNo: string;
  houseArea: number;
}

interface Proposal {
  id: number;
  propId: number;
  propTitle: string;
  propDesc: string;
  creatorAddr: string;
  propStatus: number;
}

const pendingUsers = ref<UserItem[]>([]);
const approvedUsers = ref<UserItem[]>([]);
const allProposals = ref<Proposal[]>([]);
const loadingPending = ref(false);
const loadingApproved = ref(false);
const loadingProposals = ref(false);

// 与合约交互的配置
const CONTRACT_ADDRESS = GOVERNOR_CONTRACT_ADDRESS;
const CONTRACT_ABI = [
  'function setVoter(address voter, uint256 weight, bool auth) external',
  'function removeVoter(address voter) external',
  'function paused() view returns (bool)',
  'function setPaused(bool value) external',
];

const paused = ref<boolean | null>(null);
const togglingPause = ref(false);

async function loadPending() {
  loadingPending.value = true;
  try {
    const resp = await fetch('http://127.0.0.1:8080/api/admin/identity/pending');
    if (resp.ok) pendingUsers.value = await resp.json();
  } catch (err) {
    console.error(err);
  } finally {
    loadingPending.value = false;
  }
}

async function loadApproved() {
  loadingApproved.value = true;
  try {
    const resp = await fetch('http://127.0.0.1:8080/api/admin/identity/approved');
    if (resp.ok) approvedUsers.value = await resp.json();
  } catch (err) {
    console.error(err);
  } finally {
    loadingApproved.value = false;
  }
}

// 加载所有提案用于管理员管理
async function loadAllProposals() {
  loadingProposals.value = true;
  try {
    const resp = await fetch('http://127.0.0.1:8080/api/proposals');
    if (resp.ok) allProposals.value = await resp.json();
  } catch (err) {
    console.error(err);
  } finally {
    loadingProposals.value = false;
  }
}

async function loadPaused() {
  try {
    if (!window.ethereum || !CONTRACT_ADDRESS) return;
    const provider = new ethers.BrowserProvider(window.ethereum);
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, provider);
    const p = await contract.paused();
    paused.value = Boolean(p);
  } catch (err) {
    console.error('load paused state failed', err);
  }
}

async function approve(u: UserItem) {
  if (!wallet.address) { alert('请先在右上角使用管理员钱包连接'); return; }
  if (!window.ethereum) { alert('未检测到 MetaMask'); return; }
  
  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);
    const tx = await contract.setVoter(u.walletAddr, Math.round(u.houseArea * 100), true);
    alert('链上白名单交易已发送，等待确认：' + tx.hash);
    await tx.wait();

    await fetch(`http://127.0.0.1:8080/api/admin/identity/${u.id}/approve`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Admin-Addr': wallet.address || '' },
      body: JSON.stringify({ voteWeight: u.houseArea, txHash: tx.hash }),
    });
    await Promise.all([loadPending(), loadApproved()]);
  } catch (err: any) {
    alert(err?.shortMessage || err?.message || '审核失败');
  }
}

async function reject(u: UserItem) {
  const reason = window.prompt('请输入驳回原因：', '');
  if (reason === null) return;
  try {
    await fetch(`http://127.0.0.1:8080/api/admin/identity/${u.id}/reject`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Admin-Addr': wallet.address || '' },
      body: JSON.stringify({ remark: reason }),
    });
    await loadPending();
  } catch (err) {
    alert('驳回失败');
  }
}

async function removeFromWhitelist(u: UserItem) {
  if (!wallet.address) { alert('请先使用管理员钱包连接'); return; }
  if (!window.confirm(`确定将 ${u.realName} (${u.walletAddr}) 从白名单移除吗？`)) return;

  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);
    const tx = await contract.removeVoter(u.walletAddr);
    alert('链上移除白名单交易已发送，等待确认：' + tx.hash);
    await tx.wait();

    await fetch(`http://127.0.0.1:8080/api/admin/identity/${u.id}/remove`, {
      method: 'POST',
      headers: { 'X-Admin-Addr': wallet.address || '' },
    });
    await Promise.all([loadPending(), loadApproved()]);
  } catch (err: any) {
    alert(err?.shortMessage || err?.message || '移除白名单失败');
  }
}

// 删除提案操作
async function deleteProposal(p: Proposal) {
  if (!wallet.address) { alert('请先使用管理员钱包连接'); return; }
  
  const confirmDel = window.confirm(`警告：确定要删除提案 [${p.propTitle}] 吗？\n注意：此操作将删除数据库记录并计入审计日志，但无法撤销链上已有的哈希记录。`);
  if (!confirmDel) return;

  try {
    const resp = await fetch(`http://127.0.0.1:8080/api/admin/proposals/${p.id}`, {
      method: 'DELETE',
      headers: {
        'X-Admin-Addr': wallet.address || '',
      },
    });
    const data = await resp.json();
    if (!resp.ok) throw new Error(data.error || '删除失败');
    
    alert('提案删除成功');
    await loadAllProposals();
  } catch (err: any) {
    console.error(err);
    alert('删除提案失败: ' + err.message);
  }
}

async function togglePause() {
  if (!wallet.address || paused.value === null) return;
  const target = !paused.value;
  if (!window.confirm(target ? '确定要暂停所有提案发起和投票吗？' : '确定要恢复提案发起和投票吗？')) return;

  togglingPause.value = true;
  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);
    const tx = await contract.setPaused(target);
    alert('全局暂停状态交易已发送，等待确认：' + tx.hash);
    await tx.wait();

    await fetch('http://127.0.0.1:8080/api/admin/contract/pause-log', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json', 'X-Admin-Addr': wallet.address || '' },
      body: JSON.stringify({ paused: target, txHash: tx.hash }),
    });
    await loadPaused();
  } catch (err: any) {
    alert(err?.shortMessage || err?.message || '切换暂停状态失败');
  } finally {
    togglingPause.value = false;
  }
}

onMounted(() => {
  loadPending();
  loadApproved();
  loadAllProposals();
  loadPaused();
});
</script>

<template>
  <div class="admin-dashboard">
    <header class="dashboard-header">
      <div class="header-text">
        <h2>系统管理控制台</h2>
        <p class="tip">控制网络全局状态，审核居民身份，管理系统提案数据。</p>
      </div>
      
      <div class="status-controller" :class="{ 'is-paused': paused, 'is-active': paused === false }">
        <div class="status-info">
          <div class="status-dot"></div>
          <span class="status-text">系统状态：{{ paused === null ? '获取中...' : paused ? '已暂停 (不可投票)' : '运行中 (正常)' }}</span>
        </div>
        <button
          type="button"
          class="btn-toggle-pause"
          :class="paused ? 'resume-mode' : 'pause-mode'"
          :disabled="paused === null || togglingPause"
          @click="togglePause"
        >
          <span v-if="togglingPause" class="spinner-small"></span>
          {{ togglingPause ? '上链中...' : (paused ? '恢复系统运行' : '紧急暂停系统') }}
        </button>
      </div>
    </header>

    <div class="dashboard-grid top-row">
      <section class="admin-section">
        <div class="section-header">
          <h3>
            <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" stroke-width="2" fill="none"><path d="M16 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"></path><circle cx="8.5" cy="7" r="4"></circle><polyline points="17 11 19 13 23 9"></polyline></svg>
            待审核用户申请
          </h3>
          <span class="badge" v-if="pendingUsers.length">{{ pendingUsers.length }}</span>
        </div>
        <div class="list-container">
          <div v-if="loadingPending" class="state-msg"><span class="spinner-small blue"></span> 正在同步数据...</div>
          <div v-else-if="!pendingUsers.length" class="state-msg empty"><p>目前没有待审核的申请</p></div>
          <div v-else class="user-list">
            <div v-for="u in pendingUsers" :key="u.id" class="data-card">
              <div class="info-body">
                <div class="name-row">
                  <span class="name">{{ u.realName }}</span>
                  <span class="tag">{{ u.buildNo }}栋{{ u.unitNo }}单元{{ u.roomNo }}室 · {{ u.houseArea }}㎡</span>
                </div>
                <div class="addr-box">{{ u.walletAddr }}</div>
              </div>
              <div class="action-buttons">
                <button class="btn-action approve" @click="approve(u)">授权上链</button>
                <button class="btn-action reject" @click="reject(u)">驳回</button>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="admin-section">
        <div class="section-header">
          <h3>
            <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" stroke-width="2" fill="none"><path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"></path></svg>
            链上白名单列表
          </h3>
          <span class="badge gray" v-if="approvedUsers.length">{{ approvedUsers.length }}</span>
        </div>
        <div class="list-container">
          <div v-if="loadingApproved" class="state-msg"><span class="spinner-small blue"></span> 正在同步链上数据...</div>
          <div v-else-if="!approvedUsers.length" class="state-msg empty"><p>当前白名单为空</p></div>
          <div v-else class="user-list">
            <div v-for="u in approvedUsers" :key="u.id" class="data-card">
              <div class="info-body">
                <div class="name-row">
                  <span class="name">{{ u.realName }}</span>
                  <span class="tag bg-blue">{{ u.houseArea }}㎡ 权重</span>
                </div>
                <div class="addr-box">{{ u.walletAddr }}</div>
              </div>
              <div class="action-buttons">
                <button class="btn-action remove" @click="removeFromWhitelist(u)">移除权限</button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>

    <div class="dashboard-grid bottom-row">
      <section class="admin-section full-width">
        <div class="section-header">
          <h3>
            <svg viewBox="0 0 24 24" width="20" height="20" stroke="currentColor" stroke-width="2" fill="none"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"></path><polyline points="14 2 14 8 20 8"></polyline><line x1="16" y1="13" x2="8" y2="13"></line><line x1="16" y1="17" x2="8" y2="17"></line><polyline points="10 9 9 9 8 9"></polyline></svg>
            社区提案管理
          </h3>
        </div>
        <div class="list-container">
          <div v-if="loadingProposals" class="state-msg"><span class="spinner-small blue"></span> 加载提案数据...</div>
          <div v-else-if="!allProposals.length" class="state-msg empty"><p>当前系统内暂无提案</p></div>
          <div v-else class="proposal-list">
            <div v-for="p in allProposals" :key="p.id" class="data-card prop-card">
              <div class="prop-id-box">
                <span class="prop-id-label">链上ID</span>
                <span class="prop-id-val">#{{ p.propId || p.id }}</span>
              </div>
              <div class="info-body prop-body">
                <div class="prop-title">{{ p.propTitle }}</div>
                <div class="prop-desc">{{ p.propDesc.length > 60 ? p.propDesc.substring(0, 60) + '...' : p.propDesc }}</div>
                <div class="addr-box">发起人: {{ p.creatorAddr }}</div>
              </div>
              <div class="action-buttons">
                <button class="btn-action remove" @click="deleteProposal(p)">删除提案</button>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.admin-dashboard { max-width: 1200px; margin: 0 auto; animation: fadeIn 0.4s ease-out; }
@keyframes fadeIn { from { opacity: 0; transform: translateY(10px); } to { opacity: 1; transform: translateY(0); } }

/* 头部样式 */
.dashboard-header { display: flex; justify-content: space-between; align-items: flex-end; margin-bottom: 32px; padding-bottom: 24px; border-bottom: 1px solid #e2e8f0; flex-wrap: wrap; gap: 20px; }
.header-text h2 { font-size: 28px; font-weight: 800; color: #0f172a; margin: 0 0 8px 0; letter-spacing: -0.5px; }
.tip { font-size: 14px; color: #64748b; margin: 0; }

/* 状态控制 */
.status-controller { display: flex; align-items: center; gap: 20px; padding: 12px 20px; background: #ffffff; border-radius: 16px; border: 1px solid #e2e8f0; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05); transition: all 0.3s; }
.status-controller.is-active { border-color: #a7f3d0; background: #ecfdf5; }
.status-controller.is-active .status-dot { background: #10b981; box-shadow: 0 0 0 3px #d1fae5; }
.status-controller.is-active .status-text { color: #047857; }
.status-controller.is-paused { border-color: #fecaca; background: #fef2f2; }
.status-controller.is-paused .status-dot { background: #ef4444; box-shadow: 0 0 0 3px #fee2e2; }
.status-controller.is-paused .status-text { color: #b91c1c; }
.status-info { display: flex; align-items: center; gap: 12px; }
.status-dot { width: 10px; height: 10px; border-radius: 50%; background: #94a3b8; }
.status-text { font-size: 14px; font-weight: 600; }

.btn-toggle-pause { display: flex; align-items: center; gap: 8px; border-radius: 8px; padding: 8px 16px; border: none; font-size: 13px; font-weight: 600; cursor: pointer; color: #fff; }
.btn-toggle-pause:disabled { opacity: 0.6; cursor: not-allowed; }
.pause-mode { background-color: #ef4444; } .pause-mode:hover:not(:disabled) { background-color: #dc2626; }
.resume-mode { background-color: #10b981; } .resume-mode:hover:not(:disabled) { background-color: #059669; }

/* 网格布局 */
.dashboard-grid { display: grid; gap: 32px; }
.top-row { grid-template-columns: 1fr 1fr; margin-bottom: 32px; }
.bottom-row { grid-template-columns: 1fr; }
@media (max-width: 960px) { .top-row { grid-template-columns: 1fr; } }

.admin-section { display: flex; flex-direction: column; gap: 16px; }
.section-header { display: flex; align-items: center; gap: 12px; }
.section-header h3 { font-size: 18px; font-weight: 700; color: #1e293b; margin: 0; display: flex; align-items: center; gap: 8px; }
.badge { background: #ef4444; color: white; font-size: 12px; font-weight: 700; padding: 2px 8px; border-radius: 999px; }
.badge.gray { background: #e2e8f0; color: #475569; }

.list-container { background: #ffffff; border-radius: 16px; border: 1px solid #e2e8f0; min-height: 200px; padding: 16px; max-height: 500px; overflow-y: auto;}
.state-msg { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 160px; color: #64748b; font-size: 14px; gap: 12px; }

/* 卡片通用样式 */
.user-list, .proposal-list { display: flex; flex-direction: column; gap: 12px; }
.data-card { display: flex; justify-content: space-between; align-items: center; padding: 16px; border-radius: 12px; border: 1px solid #f1f5f9; background: #f8fafc; gap: 16px; }
.data-card:hover { background: #ffffff; border-color: #cbd5e1; box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05); }

.info-body { display: flex; flex-direction: column; gap: 6px; flex: 1; }
.name-row { display: flex; align-items: center; gap: 10px; }
.name { font-weight: 700; color: #0f172a; font-size: 15px; }
.tag { font-size: 12px; background: #e2e8f0; color: #475569; padding: 2px 8px; border-radius: 6px; font-weight: 500; }
.tag.bg-blue { background: #dbeafe; color: #1e40af; }
.addr-box { font-family: ui-monospace, SFMono-Regular, monospace; font-size: 13px; color: #64748b; background: #ffffff; border: 1px solid #e2e8f0; padding: 4px 8px; border-radius: 6px; word-break: break-all; width: fit-content;}

/* 提案特殊样式 */
.prop-card { align-items: flex-start; }
.prop-id-box { display: flex; flex-direction: column; align-items: center; background: #eff6ff; border: 1px solid #bfdbfe; border-radius: 8px; padding: 8px 12px; min-width: 60px; }
.prop-id-label { font-size: 11px; color: #2563eb; font-weight: 700; text-transform: uppercase;}
.prop-id-val { font-size: 18px; font-weight: 800; color: #1e3a8a; }
.prop-title { font-size: 16px; font-weight: 700; color: #0f172a; }
.prop-desc { font-size: 13px; color: #475569; line-height: 1.5; margin-bottom: 4px;}

/* 操作按钮 */
.action-buttons { display: flex; gap: 8px; align-items: center;}
.btn-action { border-radius: 8px; padding: 8px 16px; border: 1px solid transparent; font-size: 13px; font-weight: 600; cursor: pointer; transition: all 0.2s; white-space: nowrap;}
.btn-action.approve { background-color: #2563eb; color: #fff; } .btn-action.approve:hover { background-color: #1d4ed8; }
.btn-action.reject { background-color: transparent; border-color: #fecaca; color: #ef4444; } .btn-action.reject:hover { background-color: #fef2f2; }
.btn-action.remove { background-color: #fef2f2; border-color: #fecaca; color: #dc2626; } .btn-action.remove:hover { background-color: #fee2e2; }

.spinner-small { width: 14px; height: 14px; border: 2px solid #ffffff; border-top-color: transparent; border-radius: 50%; animation: spin 1s linear infinite; }
.spinner-small.blue { border-color: #e2e8f0; border-top-color: #2563eb; width: 20px; height: 20px; }
@keyframes spin { to { transform: rotate(360deg); } }
</style>