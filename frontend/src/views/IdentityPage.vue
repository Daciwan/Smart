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
  <div class="page-wrapper">
    <div class="auth-card">
      <div class="auth-header">
        <div class="icon-wrapper">
          <svg viewBox="0 0 24 24" width="28" height="28" stroke="currentColor" stroke-width="2" fill="none" stroke-linecap="round" stroke-linejoin="round"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"></path><circle cx="12" cy="7" r="4"></circle></svg>
        </div>
        <h2>居民身份认证</h2>
        <p class="tip">绑定钱包地址与链下真实身份，获取社区治理投票权</p>
      </div>

      <div v-if="statusText" class="status-banner" :class="{
        'pending': statusText === '审核中',
        'success': statusText === '已认证',
        'error': statusText.includes('被驳回') || statusText === '查询失败',
        'default': statusText === '未提交认证信息'
      }">
        <span class="status-indicator"></span>
        <span class="status-msg">当前状态：<strong>{{ statusText }}</strong></span>
      </div>

      <form class="auth-form" @submit.prevent="submit">
        <div class="form-grid">
          <div class="field">
            <label>真实姓名</label>
            <input v-model="form.realName" placeholder="请输入您的真实姓名" required />
          </div>
          
          <div class="field">
            <label>身份证后四位</label>
            <input v-model="form.idCard4" maxlength="4" placeholder="例如: 123X" required />
          </div>
        </div>

        <div class="field-group-title">房屋资产信息 (用于计算投票权重)</div>
        <div class="form-grid three-cols">
          <div class="field">
            <label>楼栋号</label>
            <input v-model="form.buildNo" placeholder="如: 1" required />
          </div>
          <div class="field">
            <label>单元号</label>
            <input v-model="form.unitNo" placeholder="如: 2" required />
          </div>
          <div class="field">
            <label>房号</label>
            <input v-model="form.roomNo" placeholder="如: 301" required />
          </div>
        </div>

        <div class="form-grid">
          <div class="field">
            <label>房屋面积 (㎡)</label>
            <div class="input-with-suffix">
              <input v-model="form.houseArea" type="number" min="0" step="0.01" placeholder="0.00" required />
              <span class="suffix">㎡</span>
            </div>
          </div>
          <div class="field">
            <label>联系电话</label>
            <input v-model="form.phoneNo" maxlength="11" placeholder="请输入手机号" required />
          </div>
        </div>

        <div class="action-area">
          <button class="btn-submit" type="submit" :disabled="loading || statusText === '审核中' || statusText === '已认证'">
            <span v-if="loading" class="spinner"></span>
            {{ loading ? '数据加密提交中...' : (statusText === '已认证' ? '已完成认证' : '提交认证申请') }}
          </button>
          <p class="privacy-notice">
            <svg viewBox="0 0 24 24" width="14" height="14" stroke="currentColor" stroke-width="2" fill="none"><rect x="3" y="11" width="18" height="11" rx="2" ry="2"></rect><path d="M7 11V7a5 5 0 0 1 10 0v4"></path></svg>
            您的数据将被加密存储，仅用于社区白名单审核验证
          </p>
        </div>
      </form>
    </div>
  </div>
</template>

<style scoped>
.page-wrapper {
  display: flex;
  justify-content: center;
  padding: 40px 20px;
  animation: fadeIn 0.4s ease-out;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10px); }
  to { opacity: 1; transform: translateY(0); }
}

.auth-card {
  width: 100%;
  max-width: 640px;
  background: #ffffff;
  padding: 40px;
  border-radius: 24px;
  border: 1px solid #e2e8f0;
  box-shadow: 0 10px 25px -5px rgba(15, 23, 42, 0.05), 0 8px 10px -6px rgba(15, 23, 42, 0.01);
}

.auth-header {
  text-align: center;
  margin-bottom: 32px;
}

.icon-wrapper {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  background: #eff6ff;
  color: #2563eb;
  border-radius: 16px;
  margin-bottom: 16px;
}

.auth-header h2 {
  font-size: 24px;
  font-weight: 800;
  color: #0f172a;
  margin: 0 0 8px 0;
}

.tip {
  font-size: 14px;
  color: #64748b;
  margin: 0;
}

/* 状态横幅 */
.status-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  border-radius: 12px;
  margin-bottom: 32px;
  font-size: 14px;
}

.status-indicator {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}

.status-banner.default { background: #f1f5f9; color: #475569; }
.status-banner.default .status-indicator { background: #94a3b8; }

.status-banner.pending { background: #fffbeb; color: #b45309; border: 1px solid #fef3c7; }
.status-banner.pending .status-indicator { background: #f59e0b; box-shadow: 0 0 0 3px #fef3c7; }

.status-banner.success { background: #ecfdf5; color: #047857; border: 1px solid #d1fae5; }
.status-banner.success .status-indicator { background: #10b981; box-shadow: 0 0 0 3px #d1fae5; }

.status-banner.error { background: #fef2f2; color: #b91c1c; border: 1px solid #fee2e2; }
.status-banner.error .status-indicator { background: #ef4444; box-shadow: 0 0 0 3px #fee2e2; }

/* 表单布局 */
.auth-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.form-grid.three-cols {
  grid-template-columns: 1fr 1fr 1fr;
}

@media (max-width: 500px) {
  .form-grid, .form-grid.three-cols {
    grid-template-columns: 1fr;
  }
}

.field-group-title {
  font-size: 13px;
  font-weight: 700;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-top: 8px;
  margin-bottom: -8px;
  border-bottom: 1px solid #e2e8f0;
  padding-bottom: 8px;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

label {
  font-size: 13px;
  font-weight: 600;
  color: #334155;
}

input {
  width: 100%;
  border-radius: 10px;
  border: 1px solid #cbd5e1;
  padding: 12px 14px;
  font-size: 14px;
  color: #0f172a;
  background: #f8fafc;
  transition: all 0.2s;
}

input:focus {
  outline: none;
  border-color: #2563eb;
  background: #ffffff;
  box-shadow: 0 0 0 3px rgba(37, 99, 235, 0.1);
}

.input-with-suffix {
  position: relative;
  display: flex;
  align-items: center;
}
.input-with-suffix input {
  padding-right: 36px;
}
.suffix {
  position: absolute;
  right: 14px;
  color: #64748b;
  font-size: 14px;
  pointer-events: none;
}

/* 底部提交区 */
.action-area {
  margin-top: 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.btn-submit {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border-radius: 12px;
  padding: 14px 24px;
  background-color: #2563eb;
  color: #ffffff;
  font-size: 15px;
  font-weight: 600;
  border: none;
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: 0 4px 6px -1px rgba(37, 99, 235, 0.2);
}

.btn-submit:hover:not(:disabled) {
  background-color: #1d4ed8;
  transform: translateY(-1px);
}

.btn-submit:disabled {
  background-color: #94a3b8;
  cursor: not-allowed;
  box-shadow: none;
}

.spinner {
  width: 16px;
  height: 16px;
  border: 2px solid #ffffff;
  border-top-color: transparent;
  border-radius: 50%;
  animation: spin 1s linear infinite;
}

.privacy-notice {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: #94a3b8;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>