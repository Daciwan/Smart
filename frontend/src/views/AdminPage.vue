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

const pendingUsers = ref<UserItem[]>([]);
const approvedUsers = ref<UserItem[]>([]);
const loadingPending = ref(false);
const loadingApproved = ref(false);

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
    if (!resp.ok) {
      throw new Error('加载失败');
    }
    pendingUsers.value = await resp.json();
  } catch (err) {
    console.error(err);
    alert('加载待审核用户失败，请确认后端已启动');
  } finally {
    loadingPending.value = false;
  }
}

async function loadApproved() {
  loadingApproved.value = true;
  try {
    const resp = await fetch('http://127.0.0.1:8080/api/admin/identity/approved');
    if (!resp.ok) {
      throw new Error('加载失败');
    }
    approvedUsers.value = await resp.json();
  } catch (err) {
    console.error(err);
    alert('加载白名单用户失败，请确认后端已启动');
  } finally {
    loadingApproved.value = false;
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
  if (!wallet.address) {
    alert('请先在右上角使用管理员钱包连接');
    return;
  }
  if (!window.ethereum) {
    alert('未检测到 MetaMask');
    return;
  }
  if (!CONTRACT_ADDRESS) {
    alert('请在前端 Admin 页面中配置实际的合约地址');
    return;
  }

  try {
    // 1. 调用合约，将地址加入白名单
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);
    const tx = await contract.setVoter(u.walletAddr, Math.round(u.houseArea * 100), true);
    alert('链上白名单交易已发送，等待确认：' + tx.hash);
    await tx.wait();

    // 2. 更新后端链下状态
    await fetch(`http://127.0.0.1:8080/api/admin/identity/${u.id}/approve`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Admin-Addr': wallet.address || '',
      },
      body: JSON.stringify({
        voteWeight: u.houseArea,
        txHash: tx.hash,
      }),
    });

    await Promise.all([loadPending(), loadApproved()]);
  } catch (err: any) {
    console.error(err);
    alert(err?.shortMessage || err?.message || '审核失败');
  }
}

async function reject(u: UserItem) {
  const reason = window.prompt('请输入驳回原因：', '');
  if (reason === null) return;
  try {
    await fetch(`http://127.0.0.1:8080/api/admin/identity/${u.id}/reject`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Admin-Addr': wallet.address || '',
      },
      body: JSON.stringify({ remark: reason }),
    });
    await loadPending();
  } catch (err) {
    console.error(err);
    alert('驳回失败');
  }
}

async function removeFromWhitelist(u: UserItem) {
  if (!wallet.address) {
    alert('请先在右上角使用管理员钱包连接');
    return;
  }
  if (!window.ethereum) {
    alert('未检测到 MetaMask');
    return;
  }
  if (!CONTRACT_ADDRESS) {
    alert('请在前端 Admin 页面中配置实际的合约地址');
    return;
  }

  const confirmRemove = window.confirm(`确定将 ${u.realName} (${u.walletAddr}) 从白名单移除吗？`);
  if (!confirmRemove) return;

  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);
    const tx = await contract.removeVoter(u.walletAddr);
    alert('链上移除白名单交易已发送，等待确认：' + tx.hash);
    await tx.wait();

    await fetch(`http://127.0.0.1:8080/api/admin/identity/${u.id}/remove`, {
      method: 'POST',
      headers: {
        'X-Admin-Addr': wallet.address || '',
      },
    });

    await Promise.all([loadPending(), loadApproved()]);
  } catch (err: any) {
    console.error(err);
    alert(err?.shortMessage || err?.message || '移除白名单失败');
  }
}

async function togglePause() {
  if (!wallet.address) {
    alert('请先在右上角使用管理员钱包连接');
    return;
  }
  if (!window.ethereum) {
    alert('未检测到 MetaMask');
    return;
  }
  if (!CONTRACT_ADDRESS || paused.value === null) {
    return;
  }

  const target = !paused.value;
  const msg = target ? '确定要暂停所有提案发起和投票吗？' : '确定要恢复提案发起和投票吗？';
  if (!window.confirm(msg)) return;

  togglingPause.value = true;
  try {
    const provider = new ethers.BrowserProvider(window.ethereum);
    const signer = await provider.getSigner();
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, signer);
    const tx = await contract.setPaused(target);
    alert('全局暂停状态交易已发送，等待确认：' + tx.hash);
    await tx.wait();

    // 记录暂停/恢复操作到后端 SysConfig 日志
    await fetch('http://127.0.0.1:8080/api/admin/contract/pause-log', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Admin-Addr': wallet.address || '',
      },
      body: JSON.stringify({ paused: target, txHash: tx.hash }),
    });

    await loadPaused();
  } catch (err: any) {
    console.error(err);
    alert(err?.shortMessage || err?.message || '切换暂停状态失败');
  } finally {
    togglingPause.value = false;
  }
}

onMounted(() => {
  loadPending();
  loadApproved();
  loadPaused();
});
</script>

<template>
  <div class="page">
    <h2>用户与白名单管理（管理员）</h2>
    <p class="tip">
      使用管理员钱包连接后，可对待审核用户进行授权、对已有白名单用户进行移除，并控制系统是否暂停投票。
    </p>

    <section class="section">
      <h3>系统投票开关</h3>
      <div class="pause-row">
        <span>当前状态：{{ paused === null ? '未知' : paused ? '已暂停' : '正常' }}</span>
        <button
          type="button"
          class="btn-pause"
          :disabled="paused === null || togglingPause"
          @click="togglePause"
        >
          {{ paused ? '恢复提案与投票' : '紧急暂停投票' }}
        </button>
      </div>
    </section>

    <section class="section">
      <h3>待审核用户</h3>
      <div v-if="loadingPending">加载中...</div>
      <div v-else-if="!pendingUsers.length" class="empty">当前没有待审核用户</div>
      <div v-else class="list">
        <div v-for="u in pendingUsers" :key="u.id" class="card">
          <div class="top">
            <div>
              <div class="name">{{ u.realName }}</div>
              <div class="addr">{{ u.walletAddr }}</div>
            </div>
            <div class="house">
              {{ u.buildNo }}-{{ u.unitNo }}-{{ u.roomNo }} · {{ u.houseArea }}㎡
            </div>
          </div>
          <div class="actions">
            <button type="button" class="btn-approve" @click="approve(u)">通过并上链</button>
            <button type="button" class="btn-reject" @click="reject(u)">驳回</button>
          </div>
        </div>
      </div>
    </section>

    <section class="section">
      <h3>已在白名单中的用户</h3>
      <div v-if="loadingApproved">加载中...</div>
      <div v-else-if="!approvedUsers.length" class="empty">当前没有白名单用户</div>
      <div v-else class="list">
        <div v-for="u in approvedUsers" :key="u.id" class="card">
          <div class="top">
            <div>
              <div class="name">{{ u.realName }}</div>
              <div class="addr">{{ u.walletAddr }}</div>
            </div>
            <div class="house">
              {{ u.buildNo }}-{{ u.unitNo }}-{{ u.roomNo }} · {{ u.houseArea }}㎡
            </div>
          </div>
          <div class="actions">
            <button type="button" class="btn-reject" @click="removeFromWhitelist(u)">移除白名单</button>
          </div>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.page {
  max-width: 900px;
  margin: 0 auto;
}

.tip {
  font-size: 13px;
  color: #4b5563;
}

.section {
  margin-top: 16px;
}

.pause-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  font-size: 13px;
}

.btn-pause {
  border-radius: 999px;
  padding: 6px 14px;
  border: none;
  font-size: 13px;
  cursor: pointer;
  background-color: #f97316;
  color: #fff;
}

.list {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.card {
  background: #fff;
  border-radius: 12px;
  padding: 12px 14px;
  box-shadow: 0 1px 4px rgba(148, 163, 184, 0.3);
}

.top {
  display: flex;
  justify-content: space-between;
}

.name {
  font-weight: 600;
}

.addr {
  font-size: 12px;
  color: #6b7280;
}

.house {
  font-size: 13px;
  color: #374151;
}

.actions {
  margin-top: 10px;
  display: flex;
  gap: 10px;
}

.btn-approve,
.btn-reject {
  border-radius: 999px;
  padding: 6px 14px;
  border: none;
  font-size: 13px;
  cursor: pointer;
}

.btn-approve {
  background-color: #16a34a;
  color: #fff;
}

.btn-reject {
  background-color: #fee2e2;
  color: #b91c1c;
}

.empty {
  margin-top: 12px;
  font-size: 14px;
  color: #6b7280;
}
</style>

