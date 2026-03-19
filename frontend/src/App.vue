<script setup lang="ts">
import { RouterView, RouterLink } from 'vue-router';
import { useWalletStore } from './stores/wallet';
import { computed, onMounted, ref } from 'vue';
import { ethers } from 'ethers';
import { GOVERNOR_CONTRACT_ADDRESS, RPC_HTTP_URL } from './config';

const wallet = useWalletStore();

// 合约地址与 ABI（仅需 owner 查询）
const CONTRACT_ADDRESS = GOVERNOR_CONTRACT_ADDRESS;
const CONTRACT_ABI = ['function owner() view returns (address)'];

const contractOwner = ref<string | null>(null);

const shortAddress = computed(() => {
  if (!wallet.address) return '';
  return wallet.address.slice(0, 6) + '...' + wallet.address.slice(-4);
});

const isAdmin = computed(() => {
  if (!wallet.address || !contractOwner.value) return false;
  return wallet.address.toLowerCase() === contractOwner.value.toLowerCase();
});

async function loadContractOwner() {
  try {
    const provider = new ethers.JsonRpcProvider(RPC_HTTP_URL);
    const contract: any = new ethers.Contract(CONTRACT_ADDRESS, CONTRACT_ABI, provider);
    const ownerAddr = await contract.owner();
    contractOwner.value = String(ownerAddr);
  } catch (err) {
    console.error('加载合约 owner 失败', err);
  }
}

async function handleConnect() {
  try {
    await wallet.connect();
    if (!contractOwner.value) {
      await loadContractOwner();
    }
  } catch (err: any) {
    alert(err?.message || '连接钱包失败');
  }
}

function handleDisconnect() {
  wallet.disconnect();
}

onMounted(async () => {
  // 刷新后自动恢复钱包登录态（不会触发钱包弹窗）
  await wallet.restoreSession();
  // 提前从本地节点读取合约 owner，避免每次连接都额外等待
  loadContractOwner();
});
</script>

<template>
  <div class="app-root">
    <header class="app-header">
      <div class="brand">
        <span class="brand-title">智慧社区协同决策平台</span>
        <span class="brand-sub">基于区块链的透明治理</span>
      </div>
      <nav class="nav-links">
        <RouterLink to="/">首页</RouterLink>
        <RouterLink to="/identity">身份认证</RouterLink>
        <RouterLink to="/proposals">提案与投票</RouterLink>
        <RouterLink v-if="isAdmin" to="/admin">管理后台</RouterLink>
      </nav>
      <div class="wallet-area">
        <button v-if="!wallet.isConnected" class="btn-primary" type="button" @click="handleConnect">
          连接钱包
        </button>
        <div v-else class="wallet-info">
          <span class="addr">{{ shortAddress }}</span>
          <button class="btn-ghost" type="button" @click="handleDisconnect">退出登录</button>
        </div>
      </div>
    </header>

    <main class="app-main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
.app-root {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: #f5f7fb;
}

.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 32px;
  background: linear-gradient(90deg, #1f6feb, #27a87a);
  color: #fff;
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.12);
}

.brand-title {
  font-size: 18px;
  font-weight: 600;
}

.brand-sub {
  font-size: 12px;
  opacity: 0.85;
  margin-left: 4px;
}

.nav-links {
  display: flex;
  gap: 16px;
}

.nav-links a {
  color: #e2e8ff;
  text-decoration: none;
  font-size: 14px;
}

.nav-links a.router-link-active {
  font-weight: 600;
  border-bottom: 2px solid #fff;
}

.wallet-area {
  display: flex;
  align-items: center;
}

.btn-primary {
  border: none;
  padding: 8px 16px;
  border-radius: 999px;
  background-color: #ffffff;
  color: #1f6feb;
  font-size: 14px;
  cursor: pointer;
}

.btn-primary:hover {
  background-color: #e2ecff;
}

.wallet-info {
  padding: 6px 12px;
  border-radius: 999px;
  background: rgba(0, 0, 0, 0.18);
  font-size: 13px;
}

.app-main {
  flex: 1;
  padding: 24px 32px 32px;
}
</style>
