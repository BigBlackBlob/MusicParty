<template>
  <div v-if="!passed" class="fixed inset-0 z-[var(--z-modal)] flex items-center justify-center bg-[var(--surface-0)]/90 p-4 backdrop-blur-xl">
    <main class="w-full max-w-md rounded-xl bg-[var(--surface-4)] p-8 shadow-xl">
      <!-- 访客模式 -->
      <template v-if="mode === 'guest'">
        <h1 class="text-xl font-semibold text-[var(--text-primary)]">欢迎来到 MusicParty</h1>
        <p class="mt-2 text-sm text-[var(--text-secondary)]">输入昵称即可开始听歌</p>
        <form class="mt-6 space-y-3" @submit.prevent="enterAsGuest">
          <label class="block text-sm text-[var(--text-secondary)]" for="guest-name">昵称</label>
          <input id="guest-name" v-model.trim="guestName" autofocus maxlength="32" autocomplete="nickname"
            class="w-full rounded-md bg-[var(--surface-2)] px-3 py-2 text-[var(--text-primary)] outline-none focus:ring-2 focus:ring-[var(--accent-muted)]"
            placeholder="你想被怎样称呼" />
          <button class="min-h-[40px] w-full rounded-md bg-[var(--accent)] font-semibold text-[var(--text-inverse)] disabled:opacity-50"
            :disabled="loading || !guestName">
            {{ loading ? '正在进入…' : '立即进入' }}
          </button>
          <button type="button" class="w-full text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            @click="mode = 'admin'">
            管理员登录
          </button>
        </form>
      </template>

      <!-- 邀请码模式 -->
      <template v-else-if="mode === 'invite'">
        <h1 class="text-xl font-semibold text-[var(--text-primary)]">加入 {{ roomName || 'MusicParty' }}</h1>
        <p class="mt-2 text-sm text-[var(--text-secondary)]">填写显示名即可在这台设备上加入。</p>
        <form class="mt-6 space-y-3" @submit.prevent="redeem">
          <label class="block text-sm text-[var(--text-secondary)]" for="invite-display-name">显示名</label>
          <input id="invite-display-name" v-model.trim="displayName" autofocus maxlength="32" autocomplete="nickname"
            class="w-full rounded-md bg-[var(--surface-2)] px-3 py-2 text-[var(--text-primary)] outline-none focus:ring-2 focus:ring-[var(--accent-muted)]"
            placeholder="你想被怎样称呼" />
          <button class="min-h-[40px] w-full rounded-md bg-[var(--accent)] font-semibold text-[var(--text-inverse)] disabled:opacity-50"
            :disabled="loading || !displayName">
            {{ loading ? '正在加入…' : '使用邀请码加入' }}
          </button>
        </form>
      </template>

      <!-- 管理员登录模式 -->
      <template v-else-if="mode === 'admin'">
        <h1 class="text-xl font-semibold text-[var(--text-primary)]">管理员登录</h1>
        <p class="mt-2 text-sm text-[var(--text-secondary)]">需要管理权限才能登录</p>
        <form class="mt-6 space-y-3" @submit.prevent="loginAdmin">
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
          <button type="button" class="w-full text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
            @click="mode = 'guest'">
            返回访客模式
          </button>
        </form>
      </template>

      <p v-if="errorMessage" class="mt-4 text-sm text-[var(--error-soft-text)]">{{ errorMessage }}</p>
    </main>
  </div>
</template>

<script setup>
import { onMounted, ref } from 'vue';
import { authApi } from '../api/auth';
import { useUserStore } from '../stores/user';

const emit = defineEmits(['unlocked']);
const userStore = useUserStore();
const passed = ref(false);
const mode = ref('guest'); // 'guest' | 'invite' | 'admin'
const inviteSecret = ref('');
const roomName = ref('');
const guestName = ref('');
const displayName = ref('');
const adminUsername = ref('');
const adminPassword = ref('');
const loading = ref(false);
const errorMessage = ref('');

const finish = (session) => {
  userStore.initAccount(session);
  passed.value = true;
  emit('unlocked');
};

const secretFromLocation = () => {
  const match = window.location.pathname.match(/^\/join\/([^/]+)$/);
  return match ? decodeURIComponent(match[1]) : '';
};

const load = async () => {
  // 先尝试恢复已有会话
  try {
    const existing = await authApi.getAccountMe();
    finish(existing);
    return;
  } catch {
    /* no existing session */
  }

  // 检查是否有邀请码
  inviteSecret.value = secretFromLocation();
  if (inviteSecret.value) {
    mode.value = 'invite';
    try {
      const metadata = await authApi.inviteMetadata(inviteSecret.value);
      roomName.value = metadata.roomName || '';
    } catch {
      errorMessage.value = '这个邀请无效或已失效。';
      inviteSecret.value = '';
      mode.value = 'guest';
    }
  } else {
    // 默认访客模式
    mode.value = 'guest';
  }
};

const enterAsGuest = async () => {
  if (!guestName.value || loading.value) return;
  loading.value = true;
  errorMessage.value = '';
  try {
    const session = await authApi.createGuestSession(guestName.value);
    finish(session);
  } catch (err) {
    errorMessage.value = '创建访客会话失败，请重试。';
    console.error('Guest session error:', err);
  } finally {
    loading.value = false;
  }
};

const redeem = async () => {
  if (!displayName.value || loading.value) return;
  loading.value = true;
  errorMessage.value = '';
  try {
    const session = await authApi.redeemInvite(inviteSecret.value, displayName.value);
    window.history.replaceState({}, '', '/');
    finish(session);
  } catch {
    errorMessage.value = '这个邀请无效、已失效，或已被使用。';
  } finally {
    loading.value = false;
  }
};

const loginAdmin = async () => {
  if (!adminUsername.value || !adminPassword.value || loading.value) return;
  loading.value = true;
  errorMessage.value = '';
  try {
    finish(await authApi.loginAccount(adminUsername.value, adminPassword.value));
  } catch {
    errorMessage.value = '管理员账号或密码无效。';
  } finally {
    loading.value = false;
  }
};

onMounted(load);
</script>
