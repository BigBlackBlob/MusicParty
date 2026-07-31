<template>
  <div v-if="!passed" class="fixed inset-0 z-[var(--z-modal)] flex items-center justify-center bg-[var(--surface-0)]/90 p-4 backdrop-blur-xl">
    <main class="w-full max-w-md rounded-xl bg-[var(--surface-4)] p-8 shadow-xl">
      <h1 class="text-xl font-semibold text-[var(--text-primary)]">{{ title }}</h1>
      <p class="mt-2 text-sm text-[var(--text-secondary)]">{{ description }}</p>
      <form v-if="inviteSecret" class="mt-6 space-y-3" @submit.prevent="redeem">
        <label class="block text-sm text-[var(--text-secondary)]" for="invite-display-name">显示名</label>
        <input id="invite-display-name" v-model.trim="displayName" autofocus maxlength="32" autocomplete="nickname"
          class="w-full rounded-md bg-[var(--surface-2)] px-3 py-2 text-[var(--text-primary)] outline-none focus:ring-2 focus:ring-[var(--accent-muted)]" placeholder="你想被怎样称呼" />
        <button class="min-h-[40px] w-full rounded-md bg-[var(--accent)] font-semibold text-[var(--text-inverse)] disabled:opacity-50" :disabled="loading || !displayName">
          {{ loading ? '正在加入…' : '加入房间' }}
        </button>
      </form>
      <form v-else class="mt-6 space-y-3" @submit.prevent="loginAdmin">
        <label class="block text-sm text-[var(--text-secondary)]" for="admin-username">管理员账号</label>
        <input id="admin-username" v-model.trim="adminUsername" autocomplete="username"
          class="w-full rounded-md bg-[var(--surface-2)] px-3 py-2 text-[var(--text-primary)] outline-none focus:ring-2 focus:ring-[var(--accent-muted)]" />
        <label class="block text-sm text-[var(--text-secondary)]" for="admin-password">管理员密码</label>
        <input id="admin-password" v-model="adminPassword" type="password" autocomplete="current-password"
          class="w-full rounded-md bg-[var(--surface-2)] px-3 py-2 text-[var(--text-primary)] outline-none focus:ring-2 focus:ring-[var(--accent-muted)]" />
        <button class="min-h-[40px] w-full rounded-md bg-[var(--surface-control-active)] font-semibold text-[var(--text-primary)] disabled:opacity-50"
          :disabled="loading || !adminUsername || !adminPassword">
          {{ loading ? '正在登录…' : '平台管理员登录' }}
        </button>
      </form>
      <p v-if="errorMessage" class="mt-4 text-sm text-[var(--error-soft-text)]">{{ errorMessage }}</p>
    </main>
  </div>
</template>

<script setup>
import { computed, onMounted, ref } from 'vue';
import { authApi } from '../api/auth';
import { useUserStore } from '../stores/user';

const emit = defineEmits(['unlocked']);
const userStore = useUserStore();
const passed = ref(false);
const inviteSecret = ref('');
const roomName = ref('');
const displayName = ref('');
const adminUsername = ref('');
const adminPassword = ref('');
const loading = ref(false);
const errorMessage = ref('');
const title = computed(() => inviteSecret.value ? `加入 ${roomName.value || 'MusicParty'}` : '需要邀请才能加入');
const description = computed(() => inviteSecret.value ? '填写显示名即可在这台设备上加入。' : '请向房主索取一次性邀请链接。');

const finish = (session) => { userStore.initAccount(session); passed.value = true; emit('unlocked'); };
const secretFromLocation = () => {
  const match = window.location.pathname.match(/^\/join\/([^/]+)$/);
  return match ? decodeURIComponent(match[1]) : '';
};
const load = async () => {
  try { const existing = await authApi.getAccountMe(); finish(existing); return; } catch { /* invitation required */ }
  inviteSecret.value = secretFromLocation();
  if (!inviteSecret.value) return;
  try { const metadata = await authApi.inviteMetadata(inviteSecret.value); roomName.value = metadata.roomName || ''; }
  catch { errorMessage.value = '这个邀请无效或已失效。'; inviteSecret.value = ''; }
};
const redeem = async () => {
  if (!displayName.value || loading.value) return;
  loading.value = true; errorMessage.value = '';
  try {
    const session = await authApi.redeemInvite(inviteSecret.value, displayName.value);
    window.history.replaceState({}, '', '/');
    finish(session);
  } catch { errorMessage.value = '这个邀请无效、已失效，或已被使用。'; }
  finally { loading.value = false; }
};
const loginAdmin = async () => {
  if (!adminUsername.value || !adminPassword.value || loading.value) return;
  loading.value = true; errorMessage.value = '';
  try { finish(await authApi.loginAccount(adminUsername.value, adminPassword.value)); }
  catch { errorMessage.value = '管理员账号或密码无效。'; }
  finally { loading.value = false; }
};
onMounted(load);
</script>
