<!-- src/App.vue -->
<template>
  <!-- 全局 Toast 挂载点 -->
  <ToastNotification ref="toastInstance" />

  <div class="app-viewport w-screen overflow-hidden font-sans">
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
          <button
              v-for="room in roomStore.rooms"
              :key="room.roomId"
              @click="roomStore.setCurrentRoom(room.roomId)"
              class="flex items-center justify-between rounded-xl border px-3 py-3 text-left transition-colors"
              :class="roomStore.currentRoomId === room.roomId ? 'border-[var(--accent)] bg-[var(--accent-subtle)]' : 'border-[var(--border-default)] bg-[var(--surface-2)] hover:bg-[var(--surface-3)]'"
          >
            <span class="font-semibold text-[var(--text-primary)]">{{ room.name }}</span>
            <span class="text-xs text-[var(--text-tertiary)]">{{ room.onlineCount || 0 }} active</span>
          </button>
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

    <!-- 3. 主界面 (当 hasStarted 为 true 时显示) -->
    <MobilePreviewShell v-if="hasStarted && isMobileLayout && usePreviewShell">
      <MobileLayout />
    </MobilePreviewShell>

    <MobileLayout v-else-if="hasStarted && isMobileLayout" />

    <MainLayout
      v-else-if="hasStarted"
      :is-queue-visible="effectiveQueueVisible"
      @search="handleSearchClick"
      @toggle-mobile-chat="handleMobileChat"
    >
      <template #default>
        <!-- Center Stage & Lyrics -->
        <CenterConsole
          :is-queue-visible="effectiveQueueVisible"
          :is-queue-auto-suppressed="autoQueueSuppressed"
          @toggle-queue="handleQueueToggle"
        />

        <!-- Right Panel: Queue -->
        <div v-show="effectiveQueueVisible" class="flex h-full min-h-0 min-w-0 flex-col overflow-hidden" id="right-queue-panel">
          <QueueList />
        </div>
      </template>
    </MainLayout>

    <!-- 4. 全局弹窗 -->
    <SearchModal v-if="!isMobileLayout" :isOpen="showSearch" @close="showSearch = false" />
    <NamePromptModal />
    <ChatOverlay v-if="hasStarted && !uiStore.isLiteMode && !isMobileLayout" ref="chatOverlayRef" />
    <TutorialOverlay v-if="hasStarted && !uiStore.isLiteMode && !isMobileLayout" />
  </div>
</template>

<script setup>
import { computed, ref, onMounted, onBeforeUnmount } from 'vue';
import { useEventListener, useWindowSize } from '@vueuse/core';
import { usePlayerStore } from './stores/player';
import { useUserStore } from './stores/user';
import { useUiStore } from './stores/ui';
import { useRoomStore } from './stores/room';
import { useToast } from './composables/useToast';
import { useShortcuts } from './composables/useShortcuts';

// Components
import MainLayout from './components/layout/MainLayout.vue';
import CenterConsole from './components/CenterConsole.vue';
import QueueList from './components/QueueList.vue';
import AudioEngine from './components/AudioEngine.vue';
import AuthOverlay from './components/AuthOverlay.vue';
import SearchModal from './components/SearchModal.vue';
import NamePromptModal from './components/NamePromptModal.vue';
import ChatOverlay from './components/ChatOverlay.vue';
import ToastNotification from './components/ToastNotification.vue';
import TutorialOverlay from './components/TutorialOverlay.vue';
import MobileLayout from './components/mobile/MobileLayout.vue';
import MobilePreviewShell from './components/mobile/MobilePreviewShell.vue';

const player = usePlayerStore();
const userStore = useUserStore();
const uiStore = useUiStore();
const roomStore = useRoomStore();
const hasStarted = ref(false);
const showSearch = ref(false);
const userQueueVisible = ref(true);
const newRoomName = ref('');
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
const MIN_READABLE_LYRICS_WIDTH = 360;
const DESKTOP_STAGE_HORIZONTAL_PADDING = 40;
const DESKTOP_STAGE_MAX_WIDTH = 1520;
const DESKTOP_PANEL_WIDE_WIDTH = 420;
const DESKTOP_PANEL_NARROW_WIDTH = 360;
const DESKTOP_STAGE_WIDE_GAP = 24;
const DESKTOP_STAGE_NARROW_GAP = 18;

const desktopPanelWidth = computed(() => (
  width.value <= 1180 ? DESKTOP_PANEL_NARROW_WIDTH : DESKTOP_PANEL_WIDE_WIDTH
));

const desktopStageGap = computed(() => (
  width.value <= 1180 ? DESKTOP_STAGE_NARROW_GAP : DESKTOP_STAGE_WIDE_GAP
));

const desktopStageWidth = computed(() => {
  const zoom = Math.max(1, Number(uiStore.globalZoomLevel) || 1);
  const viewportWidth = Math.max(0, width.value / zoom - DESKTOP_STAGE_HORIZONTAL_PADDING);
  const scaledMaxWidth = DESKTOP_STAGE_MAX_WIDTH * uiStore.mainStageScale;
  return Math.max(0, Math.min(scaledMaxWidth, viewportWidth));
});

const autoQueueSuppressed = computed(() => {
  if (isMobileLayout.value || !userQueueVisible.value) return false;

  const lyricsWidthWithQueue = desktopStageWidth.value
    - (desktopPanelWidth.value * 2)
    - (desktopStageGap.value * 2);

  return lyricsWidthWithQueue < MIN_READABLE_LYRICS_WIDTH;
});

const effectiveQueueVisible = computed(() => userQueueVisible.value && !autoQueueSuppressed.value);
let autoLiteTimer = null;
let autoLiteSuppressedUntil = 0;
const AUTO_LITE_DELAY_MS = 180000;
const AUTO_LITE_SUPPRESS_MS = 600000;

const setAppViewportHeight = () => {
  document.documentElement.style.setProperty('--app-height', `${window.innerHeight}px`);
};

const startGame = () => {
  hasStarted.value = true;
  player.connect();
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

const clearAutoLiteTimer = () => {
  if (autoLiteTimer) {
    clearTimeout(autoLiteTimer);
    autoLiteTimer = null;
  }
};

// 自动性能优化：后台停留较久后才进入精简模式，避免短暂切换应用时频繁触发。
useEventListener(document, 'visibilitychange', () => {
  clearAutoLiteTimer();

  if (document.visibilityState === 'hidden') {
    if (
      hasStarted.value &&
      !player.isPaused &&
      uiStore.autoLiteMode &&
      !uiStore.isLiteMode &&
      Date.now() > autoLiteSuppressedUntil
    ) {
      autoLiteTimer = setTimeout(() => {
        if (document.visibilityState === 'hidden' && hasStarted.value && !player.isPaused && uiStore.autoLiteMode) {
          uiStore.isLiteMode = true;
        }
      }, AUTO_LITE_DELAY_MS);
    }
    return;
  }

  if (uiStore.isLiteMode) {
    autoLiteSuppressedUntil = Date.now() + AUTO_LITE_SUPPRESS_MS;
  }
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

const handleQueueToggle = () => {
  userQueueVisible.value = !userQueueVisible.value;
};

onMounted(() => {
  roomStore.fetchRooms();
  setAppViewportHeight();
  window.addEventListener('resize', setAppViewportHeight);
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
  window.visualViewport?.removeEventListener('resize', setAppViewportHeight);
});
</script>
