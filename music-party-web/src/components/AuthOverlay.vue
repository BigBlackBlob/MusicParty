<template>
  <div v-if="!passed" class="fixed inset-0 z-[var(--z-modal)] flex items-center justify-center bg-[var(--surface-0)]/90 p-4 backdrop-blur-xl">
    <div class="relative w-full max-w-md overflow-hidden rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] p-8 shadow-2xl">
      <div class="absolute inset-x-0 top-0 h-1 bg-[var(--accent)]"></div>

      <div class="mb-6">
        <h2 class="text-2xl font-bold tracking-tight text-[var(--text-primary)]">
          {{ authTitle }}
        </h2>
        <p class="mt-1 font-mono text-xs tracking-[0.2em] text-[var(--text-tertiary)]">
          {{ authKicker }}
        </p>
      </div>

      <div
        v-if="!requiresSetup"
        data-testid="auth-mode-toggle"
        class="mb-5 grid grid-cols-2 rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] p-1"
      >
        <button
          type="button"
          class="min-h-[36px] rounded-lg text-sm font-semibold transition-colors"
          :class="authMode === 'login' ? 'bg-[var(--accent)] text-[var(--text-inverse)] shadow-sm' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'"
          @click="setAuthMode('login')"
        >
          {{ t('auth.loginAccount') }}
        </button>
        <button
          type="button"
          class="min-h-[36px] rounded-lg text-sm font-semibold transition-colors"
          :class="authMode === 'register' ? 'bg-[var(--accent)] text-[var(--text-inverse)] shadow-sm' : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)]'"
          @click="setAuthMode('register')"
        >
          {{ t('auth.createAccount') }}
        </button>
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
          :autocomplete="isRegisterMode ? 'new-password' : 'current-password'"
          :placeholder="isRegisterMode ? 'set account password' : 'password'"
          class="w-full rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] p-3 text-center font-mono text-base tracking-widest text-[var(--text-primary)] outline-none placeholder:text-[var(--text-tertiary)] focus:border-[var(--accent)] focus:ring-2 focus:ring-[var(--accent-muted)]"
          @keyup.enter="handleAction"
        />

        <button
          class="min-h-[44px] w-full rounded-xl bg-[var(--accent)] py-3 font-semibold text-[var(--text-inverse)] transition-colors hover:bg-[var(--accent-hover)] active:scale-[0.98] disabled:opacity-50"
          :disabled="loading"
          @click="handleAction"
        >
          {{ actionLabel }}
        </button>
      </div>

      <div v-if="errorMessage" class="mt-4 animate-pulse text-center font-mono text-xs text-[var(--error-soft-text)]">
        > {{ t('common.error') }}: {{ errorMessage }}
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { authApi } from '../api/auth';
import { STORAGE_KEYS } from '../constants/keys';
import { useUserStore } from '../stores/user';

const emit = defineEmits(['unlocked']);
const { t } = useI18n();
const userStore = useUserStore();

const passed = ref(false);
const requiresSetup = ref(false);
const authMode = ref('login');
const username = ref(localStorage.getItem(STORAGE_KEYS.ACCOUNT_USERNAME) || '');
const password = ref('');
const errorMessage = ref('');
const loading = ref(false);

const isRegisterMode = computed(() => requiresSetup.value || authMode.value === 'register');
const authTitle = computed(() => {
  if (requiresSetup.value) return t('auth.initializeTitle');
  return authMode.value === 'register' ? t('auth.registerTitle') : t('auth.accessTitle');
});
const authKicker = computed(() => {
  if (requiresSetup.value) return 'CREATE ADMIN ACCOUNT';
  return authMode.value === 'register' ? 'CREATE ACCOUNT' : 'ACCOUNT LOGIN';
});
const actionLabel = computed(() => {
  if (loading.value) return t('auth.verifying');
  if (requiresSetup.value) return t('auth.createAdmin');
  return authMode.value === 'register' ? t('auth.createAccount') : t('auth.unlock');
});

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
    authMode.value = requiresSetup.value ? 'register' : 'login';
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

const setAuthMode = (mode) => {
  authMode.value = mode;
  password.value = '';
  errorMessage.value = '';
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
    const session = isRegisterMode.value
      ? await authApi.registerAccount(username.value.trim(), password.value)
      : await authApi.loginAccount(username.value.trim(), password.value);
    finish(session);
  } catch (error) {
    errorMessage.value = error?.code === 'ECONNABORTED'
      ? '请求超时，服务器可能仍在处理，请稍后尝试登录'
      : (error?.response?.data?.message || (isRegisterMode.value ? '注册失败' : '登录失败'));
  } finally {
    loading.value = false;
    password.value = '';
  }
};

onMounted(checkStatus);
</script>
