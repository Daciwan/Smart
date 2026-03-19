<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { useWalletStore } from '../stores/wallet';

const wallet = useWalletStore();

const form = ref({
  realName: '',
  idCard4: '',
  buildNo: '',
  unitNo: '',
  roomNo: '',
  houseArea: '',
  phoneNo: '',
});

const loading = ref(false);
const statusText = ref<string | null>(null);

async function loadStatus() {
  if (!wallet.address) return;
  loading.value = true;
  try {
    const resp = await fetch('http://127.0.0.1:8080/api/identity/me', {
      headers: {
        'X-Wallet-Addr': wallet.address,
      },
    });
    if (resp.ok) {
      const data = await resp.json();
      if (data.authStatus === 0) statusText.value = '审核中';
      else if (data.authStatus === 1) statusText.value = '已认证';
      else if (data.authStatus === 2) statusText.value = '被驳回：' + (data.remark || '');
    } else {
      statusText.value = '未提交认证信息';
    }
  } catch (err) {
    console.error(err);
    statusText.value = '查询失败';
  } finally {
    loading.value = false;
  }
}

async function submit() {
  if (!wallet.address) {
    alert('请先在右上角连接钱包');
    return;
  }
  loading.value = true;
  try {
    const resp = await fetch('http://127.0.0.1:8080/api/identity/register', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({
        walletAddr: wallet.address,
        realName: form.value.realName,
        idCard4: form.value.idCard4,
        buildNo: form.value.buildNo,
        unitNo: form.value.unitNo,
        roomNo: form.value.roomNo,
        houseArea: parseFloat(form.value.houseArea),
        phoneNo: form.value.phoneNo,
      }),
    });
    const data = await resp.json();
    if (!resp.ok) {
      throw new Error(data.error || '提交失败');
    }
    alert('提交成功，请等待管理员审核');
    statusText.value = '审核中';
  } catch (err: any) {
    alert(err?.message || '提交失败');
  } finally {
    loading.value = false;
  }
}

onMounted(() => {
  loadStatus();
});
</script>

<template>
  <div class="page">
    <h2>身份认证</h2>
    <p class="tip">请先连接 MetaMask 钱包，再填写线下身份信息，完成实名认证。</p>

    <div v-if="statusText" class="status">当前状态：{{ statusText }}</div>

    <form class="form" @submit.prevent="submit">
      <div class="field">
        <label>真实姓名</label>
        <input v-model="form.realName" required />
      </div>
      <div class="field">
        <label>身份证后四位</label>
        <input v-model="form.idCard4" maxlength="4" required />
      </div>
      <div class="field-row">
        <div class="field">
          <label>楼栋号</label>
          <input v-model="form.buildNo" required />
        </div>
        <div class="field">
          <label>单元号</label>
          <input v-model="form.unitNo" required />
        </div>
        <div class="field">
          <label>房号</label>
          <input v-model="form.roomNo" required />
        </div>
      </div>
      <div class="field">
        <label>房屋面积（㎡）</label>
        <input v-model="form.houseArea" type="number" min="0" step="0.01" required />
      </div>
      <div class="field">
        <label>联系电话</label>
        <input v-model="form.phoneNo" maxlength="11" required />
      </div>

      <button class="btn-primary" type="submit" :disabled="loading">
        {{ loading ? '提交中...' : '提交认证信息' }}
      </button>
    </form>
  </div>
</template>

<style scoped>
.page {
  max-width: 720px;
  margin: 0 auto;
  background: #fff;
  padding: 24px;
  border-radius: 16px;
  box-shadow: 0 4px 20px rgba(148, 163, 184, 0.25);
}

.tip {
  font-size: 13px;
  color: #4b5563;
}

.status {
  margin-top: 12px;
  padding: 8px 12px;
  background-color: #eff6ff;
  border-radius: 8px;
  font-size: 13px;
}

.form {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.field-row {
  display: flex;
  gap: 10px;
}

label {
  font-size: 13px;
  color: #374151;
}

input {
  border-radius: 8px;
  border: 1px solid #d1d5db;
  padding: 6px 10px;
  font-size: 14px;
}

.btn-primary {
  align-self: flex-start;
  margin-top: 8px;
  border-radius: 999px;
  padding: 8px 18px;
  background-color: #1f6feb;
  color: #fff;
  border: none;
  cursor: pointer;
  font-size: 14px;
}
</style>

