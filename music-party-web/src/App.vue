<!-- src/App.vue -->
<template>
  <!-- 全局 Toast 挂载点 -->
  <ToastNotification ref="toastInstance" />

  <div class="app-viewport w-full overflow-hidden font-sans">
    <AudioEngine />
    <!-- 1. 认证遮罩 -->
    <AuthOverlay @unlocked="userStore.isAuthPassed = true" v-if="!userStore.isAuthPassed" />

    <!-- 2. 启动页 (Start Screen) -->
    <!-- 注意：点击 Connect 后，我们先不销毁它，直到 socket 连接成功，或者直接切换布局 -->
    <div v-if="userStore.isAuthPassed && !hasStarted" class="absolute inset-0 z-[var(--z-overlay)] bg-[radial-gradient(circle_at_top,rgba(211,194,243,0.08),transparent_42%),var(--surface-0)] flex flex-col items-center justify-center space-y-8">
      <div class="text-4xl md:text-5xl font-bold tracking-tight text-[var(--text-primary)]">MUSIC PARTY</div>
      <div class="font-mono text-xs text-[var(--text-tertiary)] tracking-[0.3em]">准备就绪</div>
      <div class="w-full max-w-md rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)]/80 p-4 shadow-lg backdrop-blur">
        <div class="mb-3 flex items-center justify-between">
          <span class="text-xs font-semibold tracking-[0.18em] text-[var(--text-tertiary)]">LISTENING ROOMS</span>
          <button @click="roomStore.fetchRooms" class="text-xs text-[var(--accent)]">Refresh</button>
        </div>
        <div class="grid max-h-52 gap-2 overflow-y-auto">
          <div
              v-for="room in roomStore.rooms"
              :key="room.roomId"
              class="flex items-center justify-between gap-2 rounded-xl border px-3 py-3 text-left transition-colors"
              :class="roomStore.currentRoomId === room.roomId ? 'border-[var(--accent)] bg-[var(--accent-subtle)]' : 'border-[var(--border-default)] bg-[var(--surface-2)] hover:bg-[var(--surface-3)]'"
          >
            <button class="min-w-0 flex-1 text-left" @click="selectRoomBeforeStart(room)">
              <span class="block truncate font-semibold text-[var(--text-primary)]">{{ room.name }}</span>
              <span class="text-xs text-[var(--text-tertiary)]">{{ room.onlineCount || 0 }} active</span>
            </button>
            <div v-if="canManageRoom(room)" class="flex flex-shrink-0 items-center gap-1">
              <button
                class="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--text-tertiary)] transition-colors hover:bg-[var(--surface-3)] hover:text-[var(--text-primary)]"
                title="Edit room"
                @click="openEditRoom(room)"
              >
                <span class="material-symbols-outlined text-[18px]">edit</span>
              </button>
              <button
                class="flex h-8 w-8 items-center justify-center rounded-lg text-[var(--text-tertiary)] transition-colors hover:bg-[var(--error-soft-bg)] hover:text-[var(--error-soft-text)]"
                title="Delete room"
                @click="openDeleteRoom(room)"
              >
                <span class="material-symbols-outlined text-[18px]">delete</span>
              </button>
            </div>
          </div>
        </div>
        <div class="mt-3 flex gap-2">
          <input
              v-model="newRoomName"
              class="min-w-0 flex-1 rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] px-3 py-2 text-sm outline-none focus:border-[var(--accent)]"
              placeholder="Create a room..."
              @keyup.enter="createRoom"
          />
          <button @click="createRoom" class="rounded-xl bg-[var(--accent)] px-4 text-sm font-semibold text-[var(--text-inverse)]">Create</button>
        </div>
      </div>
      <button
          @click="startGame"
          class="min-h-[44px] px-12 py-4 bg-[var(--accent)] text-[var(--text-inverse)] font-semibold text-lg hover:bg-[var(--accent-hover)] active:scale-[0.98] transition-colors rounded-xl shadow-lg"
      >
        进入 {{ roomStore.currentRoom?.name || 'Lounge' }}
      </button>
    </div>

    <div v-if="editingRoom" class="fixed inset-0 z-[var(--z-modal)] flex items-center justify-center bg-[var(--surface-0)]/70 p-4 backdrop-blur-xl">
      <div class="w-full max-w-md rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] p-6 shadow-2xl">
        <div class="mb-5 flex items-center justify-between">
          <h2 class="text-lg font-bold text-[var(--text-primary)]">Edit Lounge</h2>
          <button class="text-[var(--text-tertiary)] hover:text-[var(--text-primary)]" @click="closeRoomDialog">
            <span class="material-symbols-outlined text-[20px]">close</span>
          </button>
        </div>
        <div class="space-y-4">
          <label class="block">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.16em] text-[var(--text-tertiary)]">Name</span>
            <input
              v-model="roomForm.name"
              class="w-full rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-[var(--accent)]"
            />
          </label>
          <label class="flex items-center justify-between rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] px-3 py-2">
            <span class="text-sm font-semibold text-[var(--text-primary)]">Private Lounge</span>
            <input v-model="roomForm.isPrivate" type="checkbox" class="h-4 w-4 accent-[var(--accent)]" />
          </label>
          <label v-if="roomForm.isPrivate" class="block">
            <span class="mb-1 block text-xs font-semibold uppercase tracking-[0.16em] text-[var(--text-tertiary)]">Password</span>
            <input
              v-model="roomForm.password"
              type="password"
              class="w-full rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-[var(--accent)]"
              :placeholder="roomForm.keepExistingPassword ? 'Leave blank to keep current password' : 'Required for private Lounge'"
            />
          </label>
          <label v-if="roomForm.isPrivate && editingRoom.privateRoom" class="flex items-center gap-2 text-sm text-[var(--text-secondary)]">
            <input v-model="roomForm.keepExistingPassword" type="checkbox" class="h-4 w-4 accent-[var(--accent)]" />
            Keep existing password
          </label>
          <div v-if="roomDialogError" class="text-sm text-[var(--error-soft-text)]">{{ roomDialogError }}</div>
        </div>
        <div class="mt-6 flex justify-end gap-2">
          <button class="rounded-xl border border-[var(--border-default)] px-4 py-2 text-sm text-[var(--text-secondary)] hover:bg-[var(--surface-3)]" @click="closeRoomDialog">Cancel</button>
          <button class="rounded-xl bg-[var(--accent)] px-4 py-2 text-sm font-semibold text-[var(--text-inverse)] hover:bg-[var(--accent-hover)]" @click="saveRoomEdit">Save</button>
        </div>
      </div>
    </div>

    <div v-if="deletingRoom" class="fixed inset-0 z-[var(--z-modal)] flex items-center justify-center bg-[var(--surface-0)]/70 p-4 backdrop-blur-xl">
      <div class="w-full max-w-sm rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] p-6 shadow-2xl">
        <h2 class="text-lg font-bold text-[var(--text-primary)]">Delete Lounge</h2>
        <p class="mt-2 text-sm text-[var(--text-secondary)]">Delete {{ deletingRoom.name }} and move listeners back to Lounge?</p>
        <div v-if="roomDialogError" class="mt-3 text-sm text-[var(--error-soft-text)]">{{ roomDialogError }}</div>
        <div class="mt-6 flex justify-end gap-2">
          <button class="rounded-xl border border-[var(--border-default)] px-4 py-2 text-sm text-[var(--text-secondary)] hover:bg-[var(--surface-3)]" @click="closeRoomDialog">Cancel</button>
          <button class="rounded-xl bg-[var(--error-soft-bg)] px-4 py-2 text-sm font-semibold text-[var(--error-soft-text)] hover:brightness-110" @click="confirmDeleteRoom">Delete</button>
        </div>
      </div>
    </div>

    <PrivateRoomAccessDialog
      :open="privateRoomDialogOpen"
      :room="privateRoomDialogRoom"
      :loading="privateRoomDialogLoading"
      :error="privateRoomDialogError"
      @submit="submitPrivateRoomPassword"
      @cancel="closePrivateRoomDialog"
    />

    <!-- 3. 主界面 (当 hasStarted 为 true 时显示) -->
    <MobilePreviewShell v-if="hasStarted && isMobileLayout && usePreviewShell">
      <MobileLayout />
    </MobilePreviewShell>

    <MobileLayout v-else-if="hasStarted && isMobileLayout" />

    <MainLayout
      v-else-if="hasStarted"
      @search="handleSearchClick"
      @toggle-mobile-chat="handleMobileChat"
    >
      <template #default>
        <LayoutRenderer />
      </template>
    </MainLayout>

    <!-- 4. 全局弹窗 -->
    <SearchModal v-if="!isMobileLayout" :isOpen="showSearch" @close="showSearch = false" />
    <NamePromptModal />
    <ChatOverlay v-if="showChatOverlay" ref="chatOverlayRef" />
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue';
import { useEventListener, useWindowSize } from '@vueuse/core';
import { usePlayerStore } from './stores/player';
import { useUserStore } from './stores/user';
import { useUiStore } from './stores/ui';
import { useRoomStore } from './stores/room';
import { useLayoutStore } from './stores/layout';
import { useToast } from './composables/useToast';
import { useShortcuts } from './composables/useShortcuts';

// Components
import MainLayout from './components/layout/MainLayout.vue';
import LayoutRenderer from './layouts/LayoutRenderer.vue';
import AudioEngine from './components/AudioEngine.vue';
import AuthOverlay from './components/AuthOverlay.vue';
import SearchModal from './components/SearchModal.vue';
import NamePromptModal from './components/NamePromptModal.vue';
import ChatOverlay from './components/ChatOverlay.vue';
import ToastNotification from './components/ToastNotification.vue';
import PrivateRoomAccessDialog from './components/PrivateRoomAccessDialog.vue';
import MobileLayout from './components/mobile/MobileLayout.vue';
import MobilePreviewShell from './components/mobile/MobilePreviewShell.vue';

const player = usePlayerStore();
const userStore = useUserStore();
const uiStore = useUiStore();
const roomStore = useRoomStore();
const layoutStore = useLayoutStore();
const hasStarted = ref(false);
const showSearch = ref(false);
const newRoomName = ref('');
const editingRoom = ref(null);
const deletingRoom = ref(null);
const roomDialogError = ref('');
const privateRoomDialogOpen = ref(false);
const privateRoomDialogRoom = ref(null);
const privateRoomDialogAction = ref('select');
const privateRoomDialogLoading = ref(false);
const privateRoomDialogError = ref('');
const roomForm = ref({
  name: '',
  isPrivate: false,
  password: '',
  keepExistingPassword: true
});
const toastInstance = ref(null);
const chatOverlayRef = ref(null);
const { register } = useToast();

useShortcuts({
  onSearch: () => handleSearchClick(),
  onCloseModals: () => {
    showSearch.value = false;
  }
});

const { width } = useWindowSize();
const isMobileLayout = computed(() => uiStore.forceMobileLayout || width.value < 768);
const usePreviewShell = computed(() => uiStore.forceMobileLayout && width.value >= 768);

const showChatOverlay = computed(() =>
  hasStarted.value &&
  !uiStore.isLiteMode &&
  !isMobileLayout.value &&
  !layoutStore.placedModuleIds.includes('chat')
);

let autoLiteTimer = null;
let lastInteractionAt = Date.now();
const AUTO_LITE_DELAY_MS = 180000;
const ACTIVITY_EVENTS = ['pointerdown', 'pointermove', 'keydown', 'touchstart', 'wheel'];

const setAppViewportHeight = () => {
  const vh = window.visualViewport ? window.visualViewport.height : window.innerHeight;
  document.documentElement.style.setProperty('--app-height', `${vh}px`);
};

const openPrivateRoomDialog = (room, action = 'select') => {
  privateRoomDialogRoom.value = room;
  privateRoomDialogAction.value = action;
  privateRoomDialogError.value = '';
  privateRoomDialogOpen.value = true;
};

const closePrivateRoomDialog = () => {
  privateRoomDialogOpen.value = false;
  privateRoomDialogRoom.value = null;
  privateRoomDialogAction.value = 'select';
  privateRoomDialogError.value = '';
  privateRoomDialogLoading.value = false;
};

const selectRoomBeforeStart = (room) => {
  if (!room?.roomId) return;
  if (room.privateRoom && !roomStore.hasValidRoomAccess(room.roomId)) {
    openPrivateRoomDialog(room, 'select');
    return;
  }
  roomStore.setCurrentRoom(room.roomId);
};

const startGame = () => {
  const room = roomStore.currentRoom;
  if (room?.privateRoom && !roomStore.hasValidRoomAccess(room.roomId)) {
    openPrivateRoomDialog(room, 'start');
    return;
  }
  hasStarted.value = true;
  player.connect();
};

const submitPrivateRoomPassword = async (password) => {
  const room = privateRoomDialogRoom.value;
  if (!room?.roomId) return;
  privateRoomDialogLoading.value = true;
  privateRoomDialogError.value = '';
  try {
    await roomStore.verifyRoomAccess(room.roomId, password);
    roomStore.setCurrentRoom(room.roomId);
    const shouldStart = privateRoomDialogAction.value === 'start';
    closePrivateRoomDialog();
    if (shouldStart) {
      hasStarted.value = true;
      player.connect();
    }
  } catch (error) {
    privateRoomDialogError.value = error?.response?.data?.message || 'Room password could not be verified';
  } finally {
    privateRoomDialogLoading.value = false;
  }
};

const createRoom = () => {
  const name = newRoomName.value.trim();
  if (!name) return;

  const submitCreate = () => {
    roomStore.createRoom(name);
    newRoomName.value = '';
  };

  if (!hasStarted.value) {
    hasStarted.value = true;
    player.connect();
  }

  if (userStore.isGuest) {
    userStore.setPostNameAction(() => submitCreate());
    userStore.showNameModal = true;
    return;
  }

  submitCreate();
};

const canManageRoom = (room) => !room?.system && (userStore.isAdmin || room.creatorPublicId === userStore.publicId);

const closeRoomDialog = () => {
  editingRoom.value = null;
  deletingRoom.value = null;
  roomDialogError.value = '';
};

const openEditRoom = (room) => {
  if (!canManageRoom(room)) return;
  editingRoom.value = room;
  deletingRoom.value = null;
  roomDialogError.value = '';
  roomForm.value = {
    name: room.name || '',
    isPrivate: Boolean(room.privateRoom),
    password: '',
    keepExistingPassword: Boolean(room.privateRoom)
  };
};

const saveRoomEdit = async () => {
  if (!editingRoom.value) return;
  roomDialogError.value = '';
  const name = roomForm.value.name.trim();
  if (!name) {
    roomDialogError.value = 'Room name cannot be empty';
    return;
  }
  if (roomForm.value.isPrivate && !roomForm.value.keepExistingPassword && !roomForm.value.password.trim()) {
    roomDialogError.value = 'Private Lounge password is required';
    return;
  }
  try {
    await roomStore.updateRoom(editingRoom.value.roomId, {
      name,
      isPrivate: roomForm.value.isPrivate,
      password: roomForm.value.password,
      keepExistingPassword: roomForm.value.keepExistingPassword
    });
    closeRoomDialog();
  } catch (error) {
    roomDialogError.value = error?.response?.data?.message || 'Failed to update Lounge';
  }
};

const openDeleteRoom = (room) => {
  if (!canManageRoom(room)) return;
  deletingRoom.value = room;
  editingRoom.value = null;
  roomDialogError.value = '';
};

const confirmDeleteRoom = async () => {
  if (!deletingRoom.value) return;
  roomDialogError.value = '';
  try {
    await roomStore.deleteRoom(deletingRoom.value.roomId);
    closeRoomDialog();
  } catch (error) {
    roomDialogError.value = error?.response?.data?.message || 'Failed to delete Lounge';
  }
};

const clearAutoLiteTimer = () => {
  if (autoLiteTimer) {
    clearTimeout(autoLiteTimer);
    autoLiteTimer = null;
  }
};

const recordInteraction = () => {
  lastInteractionAt = Date.now();
};

// 自动性能优化：后台停留较久后才进入精简模式，避免短暂切换应用时频繁触发。
useEventListener(document, 'visibilitychange', () => {
  clearAutoLiteTimer();

  if (document.visibilityState === 'hidden') {
    if (
      hasStarted.value &&
      !player.isPaused &&
      uiStore.autoLiteMode &&
      !uiStore.isLiteMode
    ) {
      const hiddenAt = Date.now();
      autoLiteTimer = setTimeout(() => {
        if (
          document.visibilityState === 'hidden' &&
          hasStarted.value &&
          !player.isPaused &&
          uiStore.autoLiteMode &&
          lastInteractionAt <= hiddenAt
        ) {
          uiStore.isLiteMode = true;
        }
      }, AUTO_LITE_DELAY_MS);
    }
    return;
  }
});

ACTIVITY_EVENTS.forEach(eventName => {
  useEventListener(window, eventName, recordInteraction, { passive: true });
});

const handleSearchClick = () => {
  // 简单的搜索逻辑代理
  if (userStore.isGuest) {
    userStore.setPostNameAction(() => { showSearch.value = true; });
    userStore.showNameModal = true;
  } else {
    showSearch.value = true;
  }
};

const handleMobileChat = () => {
  if (userStore.isGuest) {
    userStore.setPostNameAction(() => {
      chatOverlayRef.value?.toggleChat?.();
    });
    userStore.showNameModal = true;
    return;
  }

  chatOverlayRef.value?.toggleChat?.();
};

onMounted(() => {

  roomStore.fetchRooms();
  setAppViewportHeight();
  window.addEventListener('resize', setAppViewportHeight);
  window.addEventListener('orientationchange', setAppViewportHeight);
  window.visualViewport?.addEventListener('resize', setAppViewportHeight);

  const params = new URLSearchParams(window.location.search);
  const mobilePreview = params.get('mobilePreview');
  if (mobilePreview === '1') uiStore.setForceMobileLayout(true);
  if (mobilePreview === '0') uiStore.setForceMobileLayout(false);

  if (toastInstance.value) register(toastInstance.value);
});

onBeforeUnmount(() => {
  clearAutoLiteTimer();
  window.removeEventListener('resize', setAppViewportHeight);
  window.removeEventListener('orientationchange', setAppViewportHeight);
  window.visualViewport?.removeEventListener('resize', setAppViewportHeight);
});
</script>
