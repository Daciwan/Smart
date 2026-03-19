import { defineStore } from 'pinia';
import { ref, computed } from 'vue';
import { ethers } from 'ethers';

declare global {
  interface Window {
    ethereum?: any;
  }
}

export const useWalletStore = defineStore('wallet', () => {
  const MANUAL_DISCONNECT_KEY = 'wallet_manual_disconnect';
  const address = ref<string | null>(null);
  const isConnected = computed(() => !!address.value);
  const networkName = ref<string | null>(null);
  let accountListenerBound = false;

  function bindAccountListener() {
    if (!window.ethereum || typeof window.ethereum.on !== 'function' || accountListenerBound) {
      return;
    }
    window.ethereum.on('accountsChanged', (accs: string[]) => {
      if (!accs || accs.length === 0) {
        address.value = null;
        networkName.value = null;
      } else {
        address.value = accs[0] ?? null;
      }
    });
    accountListenerBound = true;
  }

  async function connect() {
    if (!window.ethereum) {
      throw new Error('未检测到 MetaMask，请先安装钱包插件');
    }

    const provider = new ethers.BrowserProvider(window.ethereum);
    const accounts = await provider.send('eth_requestAccounts', []);
    if (!accounts || accounts.length === 0) {
      throw new Error('未选择账户');
    }
    address.value = accounts[0];
    sessionStorage.removeItem(MANUAL_DISCONNECT_KEY);

    const network = await provider.getNetwork();
    networkName.value = network.name ?? `chainId-${network.chainId.toString()}`;

    bindAccountListener();
  }

  // 刷新后恢复已授权账号（不主动弹钱包）
  async function restoreSession() {
    if (!window.ethereum) return;
    if (sessionStorage.getItem(MANUAL_DISCONNECT_KEY) === '1') {
      address.value = null;
      networkName.value = null;
      return;
    }
    try {
      const provider = new ethers.BrowserProvider(window.ethereum);
      const accounts = await provider.send('eth_accounts', []);
      if (accounts && accounts.length > 0) {
        address.value = accounts[0] ?? null;
        const network = await provider.getNetwork();
        networkName.value = network.name ?? `chainId-${network.chainId.toString()}`;
      } else {
        address.value = null;
        networkName.value = null;
      }
      bindAccountListener();
    } catch {
      address.value = null;
      networkName.value = null;
    }
  }

  function disconnect() {
    address.value = null;
    networkName.value = null;
    sessionStorage.setItem(MANUAL_DISCONNECT_KEY, '1');
  }

  return {
    address,
    isConnected,
    networkName,
    connect,
    restoreSession,
    disconnect,
  };
});

