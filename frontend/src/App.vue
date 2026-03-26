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
      <div class="header-container">
        <div class="brand">
          <span class="brand-title">智慧社区协同决策平台</span>
          <span class="brand-sub">Web3.0 治理网络</span>
        </div>
        <nav class="nav-links">
          <RouterLink to="/">首页</RouterLink>
          <RouterLink to="/identity">身份认证</RouterLink>
          <RouterLink to="/proposals">提案与投票</RouterLink>
          <RouterLink v-if="isAdmin" to="/admin">管理后台</RouterLink>
        </nav>
        <div class="wallet-area">
          <button v-if="!wallet.isConnected" class="btn-primary" type="button" @click="handleConnect">
            <svg viewBox="0 0 24 24" width="18" height="18" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round" class="wallet-icon"><path d="M21 12V7H5a2 2 0 0 1 0-4h14v4"></path><path d="M3 5v14a2 2 0 0 0 2 2h16v-5"></path><path d="M18 12h.01"></path></svg>
            连接钱包
          </button>
          <div v-else class="wallet-info">
            <span class="addr">{{ shortAddress }}</span>
            <button class="btn-ghost" type="button" @click="handleDisconnect">退出</button>
          </div>
        </div>
      </div>
    </header>

    <main class="app-main">
      <RouterView />
    </main>
  </div>
</template>

<style scoped>
/* 全局变量 */
.app-root {
  --primary-color: #2563eb;
  --primary-hover: #1d4ed8;
  --bg-color: #f8fafc;
  --text-main: #0f172a;
  --text-muted: #64748b;
  --header-bg: rgba(255, 255, 255, 0.85);
  --border-color: #e2e8f0;
  
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg-color);
  font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}

/* 导航栏毛玻璃效果与布局 */
.app-header {
  position: sticky;
  top: 0;
  z-index: 50;
  background: var(--header-bg);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-bottom: 1px solid var(--border-color);
  box-shadow: 0 1px 2px 0 rgba(0, 0, 0, 0.03);
}

.header-container {
  max-width: 1200px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 24px;
}

.brand {
  display: flex;
  flex-direction: column;
}

.brand-title {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-main);
  letter-spacing: -0.5px;
}

.brand-sub {
  font-size: 12px;
  color: var(--primary-color);
  font-weight: 600;
  margin-top: 2px;
}

/* 导航链接 */
.nav-links {
  display: flex;
  gap: 32px;
}

.nav-links a {
  color: var(--text-muted);
  text-decoration: none;
  font-size: 15px;
  font-weight: 500;
  transition: all 0.2s ease;
  position: relative;
}

.nav-links a:hover {
  color: var(--text-main);
}

.nav-links a.router-link-active {
  color: var(--primary-color);
  font-weight: 600;
}

.nav-links a.router-link-active::after {
  content: '';
  position: absolute;
  bottom: -6px;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--primary-color);
  border-radius: 2px;
}

/* 钱包按钮区域 */
.wallet-area {
  display: flex;
  align-items: center;
}

.btn-primary {
  display: flex;
  align-items: center;
  gap: 8px;
  border: none;
  padding: 10px 20px;
  border-radius: 999px;
  background-color: var(--primary-color);
  color: #ffffff;
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  box-shadow: 0 4px 6px -1px rgba(37, 99, 235, 0.2), 0 2px 4px -1px rgba(37, 99, 235, 0.1);
}

.btn-primary:hover {
  background-color: var(--primary-hover);
  transform: translateY(-1px);
  box-shadow: 0 6px 8px -1px rgba(37, 99, 235, 0.3), 0 4px 6px -1px rgba(37, 99, 235, 0.2);
}

.wallet-info {
  display: flex;
  align-items: center;
  padding: 6px 6px 6px 16px;
  border-radius: 999px;
  background: #ffffff;
  border: 1px solid var(--border-color);
  box-shadow: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
}

.addr {
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace;
  font-size: 14px;
  font-weight: 600;
  color: var(--text-main);
  margin-right: 12px;
}

.btn-ghost {
  border: 1px solid var(--border-color);
  padding: 6px 16px;
  border-radius: 999px;
  background-color: transparent;
  color: var(--text-muted);
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.2s ease;
}

.btn-ghost:hover {
  background-color: #fee2e2;
  color: #ef4444;
  border-color: #f87171;
}

/* 主内容区域布局 */
.app-main {
  flex: 1;
  width: 100%;
  max-width: 1200px;
  margin: 0 auto;
  padding: 40px 24px;
}
</style>