<template>
  <div v-if="!passed" class="fixed inset-0 z-[var(--z-modal)] flex items-center justify-center bg-[var(--surface-0)]/90 p-4 backdrop-blur-xl">
    <div class="relative w-full max-w-md overflow-hidden rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] p-8 shadow-2xl">
      <div class="absolute inset-x-0 top-0 h-1 bg-[var(--accent)]"></div>

      <div class="mb-6">
        <h2 class="text-2xl font-bold tracking-tight text-[var(--text-primary)]">
          {{ requiresSetup ? t('auth.initializeTitle') : t('auth.accessTitle') }}
        </h2>
        <p class="mt-1 font-mono text-xs tracking-[0.2em] text-[var(--text-tertiary)]">
          {{ requiresSetup ? 'CREATE ADMIN ACCOUNT' : 'ACCOUNT LOGIN' }}
        </p>
      </div>

      <div class="space-y-4">
        <input
          v-model="username"
          type="text"
          autocomplete="username"
          placeholder="username"
          class="w-full rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] p-3 text-center font-mono text-base tracking-widest text-[var(--text-primary)] outline-none placeholder:text-[var(--text-tertiary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
          autofocus
          @keyup.enter="handleAction"
        />
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          :placeholder="requiresSetup ? 'set account password' : 'password'"
          class="w-full rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] p-3 text-center font-mono text-base tracking-widest text-[var(--text-primary)] outline-none placeholder:text-[var(--text-tertiary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
          @keyup.enter="handleAction"
        />

        <button
          class="min-h-[44px] w-full rounded-xl bg-[var(--accent)] py-3 font-semibold text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)] active:scale-[0.98] disabled:opacity-50"
          :disabled="loading"
          @click="handleAction"
        >
          {{ loading ? t('auth.verifying') : (requiresSetup ? 'Create admin' : t('auth.unlock')) }}
        </button>
      </div>

      <div v-if="errorMessage" class="mt-4 animate-pulse text-center font-mono text-xs text-[var(--error-soft-text)]">
        > {{ t('common.error') }}: {{ errorMessage }}
      </div>
    </div>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { authApi } from '../api/auth';
import { STORAGE_KEYS } from '../constants/keys';
import { useUserStore } from '../stores/user';

const emit = defineEmits(['unlocked']);
const { t } = useI18n();
const userStore = useUserStore();

const passed = ref(false);
const requiresSetup = ref(false);
const username = ref(localStorage.getItem(STORAGE_KEYS.ACCOUNT_USERNAME) || '');
const password = ref('');
const errorMessage = ref('');
const loading = ref(false);

const finish = (session) => {
  userStore.initAccount(session);
  passed.value = true;
  emit('unlocked');
};

const checkStatus = async () => {
  loading.value = true;
  try {
    const status = await authApi.getAccountStatus();
    requiresSetup.value = Boolean(status.requiresSetup);
    const cachedToken = localStorage.getItem(STORAGE_KEYS.SESSION_TOKEN);
    if (cachedToken && !requiresSetup.value) {
      const session = await authApi.getAccountMe(cachedToken);
      finish(session);
    }
  } catch {
    localStorage.removeItem(STORAGE_KEYS.SESSION_TOKEN);
  } finally {
    loading.value = false;
  }
};

const handleAction = async () => {
  if (loading.value) return;
  errorMessage.value = '';
  if (!username.value.trim() || password.value.length < 8) {
    errorMessage.value = '用户名不能为空，密码至少 8 位';
    return;
  }
  loading.value = true;
  try {
    const session = requiresSetup.value
      ? await authApi.registerAccount(username.value.trim(), password.value)
      : await authApi.loginAccount(username.value.trim(), password.value);
    finish(session);
  } catch (error) {
    errorMessage.value = error?.code === 'ECONNABORTED'
      ? '请求超时，服务器可能仍在处理，请稍后尝试登录'
      : (error?.response?.data?.message || '登录失败');
  } finally {
    loading.value = false;
    password.value = '';
  }
};

onMounted(checkStatus);
</script>
