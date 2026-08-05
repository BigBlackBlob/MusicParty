<template>
  <div class="fixed inset-0 z-[var(--z-modal)] flex items-center justify-center bg-[var(--surface-0)]/90 p-4 backdrop-blur-xl">
    <main class="w-full max-w-md rounded-xl bg-[var(--surface-4)] p-8 shadow-xl">
      <template v-if="mode === 'loading'">
        <h1 class="text-xl font-semibold text-[var(--text-primary)]">MusicParty</h1>
        <p class="mt-2 text-sm text-[var(--text-secondary)]">正在恢复会话…</p>
      </template>

      <template v-else-if="mode === 'entry'">
        <h1 class="text-xl font-semibold text-[var(--text-primary)]">欢迎来到 MusicParty</h1>
        <p class="mt-2 text-sm text-[var(--text-secondary)]">选择进入方式，房间将在下一步选择。</p>
        <div class="mt-6 space-y-3">
          <button type="button" class="min-h-[40px] w-full rounded-md bg-[var(--accent)] font-semibold text-[var(--text-inverse)]" @click="mode = 'guest'">
            以访客身份进入
          </button>
          <button type="button" class="min-h-[40px] w-full rounded-md bg-[var(--surface-control-active)] font-semibold text-[var(--text-primary)]" @click="mode = 'invite'">
            使用邀请码加入
          </button>
          <button type="button" class="min-h-[40px] w-full rounded-md border border-[var(--border-default)] text-sm font-semibold text-[var(--text-secondary)] hover:bg-[var(--surface-3)] hover:text-[var(--text-primary)]" @click="mode = 'admin'">
            平台管理员登录
          </button>
        </div>
      </template>

      <template v-else-if="mode === 'error'">
        <h1 class="text-xl font-semibold text-[var(--text-primary)]">暂时无法进入</h1>
        <p class="mt-2 text-sm text-[var(--text-secondary)]">MusicParty 没有完成会话检查。</p>
      </template>

      <!-- 访客模式 -->
      <template v-else-if="mode === 'guest'">
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
            @click="mode = 'entry'">
            返回进入方式
          </button>
        </form>
      </template>

      <!-- 邀请码模式 -->
      <template v-else-if="mode === 'invite'">
        <h1 class="text-xl font-semibold text-[var(--text-primary)]">加入 {{ roomName || 'MusicParty' }}</h1>
        <p class="mt-2 text-sm text-[var(--text-secondary)]">填写显示名即可在这台设备上加入。</p>
        <form class="mt-6 space-y-3" @submit.prevent="redeem">
          <template v-if="!inviteFromUrl">
            <label class="block text-sm text-[var(--text-secondary)]" for="invite-secret">邀请码</label>
            <input id="invite-secret" v-model.trim="inviteSecret" autocomplete="one-time-code"
              class="w-full rounded-md bg-[var(--surface-2)] px-3 py-2 text-[var(--text-primary)] outline-none focus:ring-2 focus:ring-[var(--accent-muted)]"
              placeholder="输入邀请链接中的代码" />
          </template>
          <label class="block text-sm text-[var(--text-secondary)]" for="invite-display-name">显示名</label>
          <input id="invite-display-name" v-model.trim="displayName" autofocus maxlength="32" autocomplete="nickname"
            class="w-full rounded-md bg-[var(--surface-2)] px-3 py-2 text-[var(--text-primary)] outline-none focus:ring-2 focus:ring-[var(--accent-muted)]"
            placeholder="你想被怎样称呼" />
          <button class="min-h-[40px] w-full rounded-md bg-[var(--accent)] font-semibold text-[var(--text-inverse)] disabled:opacity-50"
            :disabled="loading || !displayName || !inviteSecret">
            {{ loading ? '正在加入…' : '使用邀请码加入' }}
          </button>
          <button v-if="!inviteFromUrl" type="button" class="w-full text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)]" @click="mode = 'entry'">
            返回进入方式
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
            @click="mode = 'entry'">
            返回进入方式
          </button>
        </form>
      </template>

      <p v-if="errorMessage" class="mt-4 text-sm text-[var(--error-soft-text)]" role="alert">{{ errorMessage }}</p>
      <button v-if="mode === 'error'" type="button" class="mt-4 min-h-[40px] w-full rounded-md bg-[var(--surface-control-active)] font-semibold text-[var(--text-primary)]" @click="load">
        重试
      </button>
    </main>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { authApi } from '../api/auth';
import { useUserStore } from '../stores/user';
import type { Session } from '../contracts/generated/models';
import { isAPIError } from '../transport/errors';

const userStore = useUserStore();
type EntryMode = 'loading' | 'entry' | 'guest' | 'invite' | 'admin' | 'error';
const mode = ref<EntryMode>('loading');
const inviteSecret = ref('');
const inviteFromUrl = ref(false);
const roomName = ref('');
const guestName = ref('');
const displayName = ref('');
const adminUsername = ref('');
const adminPassword = ref('');
const loading = ref(false);
const errorMessage = ref('');

const finish = (session: Session) => {
  userStore.initAccount(session);
};

const secretFromLocation = () => {
  const match = window.location.pathname.match(/^\/join\/([^/]+)$/);
  return match?.[1] ? decodeURIComponent(match[1]) : '';
};

const load = async () => {
  mode.value = 'loading';
  errorMessage.value = '';
  // 先尝试恢复已有会话
  try {
    const existing = await authApi.getAccountMe();
    finish(existing);
    return;
  } catch (error) {
    if (!isAPIError(error) || (error.status !== 401 && error.status !== 403)) {
      userStore.status = 'error';
      mode.value = 'error';
      errorMessage.value = '无法连接服务器，请检查网络后重试。';
      return;
    }
    userStore.status = 'anonymous';
  }

  // 检查是否有邀请码
  inviteSecret.value = secretFromLocation();
  inviteFromUrl.value = Boolean(inviteSecret.value);
  if (inviteSecret.value) {
    mode.value = 'invite';
    try {
      const metadata = await authApi.inviteMetadata(inviteSecret.value);
      roomName.value = metadata.roomName || '';
    } catch {
      errorMessage.value = '这个邀请无效或已失效。';
      inviteSecret.value = '';
      inviteFromUrl.value = false;
      mode.value = 'entry';
    }
  } else {
    // 默认访客模式
    mode.value = 'entry';
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
  if (!displayName.value || !inviteSecret.value || loading.value) return;
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
