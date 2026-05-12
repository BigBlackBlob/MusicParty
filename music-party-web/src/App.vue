<!-- src/App.vue -->
<template>
  <!-- 全局 Toast 挂载点 -->
  <ToastNotification ref="toastInstance" />

  <div class="h-screen w-screen overflow-hidden font-sans">
    <AudioEngine />
    <!-- 1. 认证遮罩 -->
    <AuthOverlay @unlocked="userStore.isAuthPassed = true" v-if="!userStore.isAuthPassed" />

    <!-- 2. 启动页 (Start Screen) -->
    <!-- 注意：点击 Connect 后，我们先不销毁它，直到 socket 连接成功，或者直接切换布局 -->
    <div v-if="userStore.isAuthPassed && !hasStarted" class="absolute inset-0 z-[var(--z-overlay)] bg-[radial-gradient(circle_at_top,rgba(211,194,243,0.08),transparent_42%),var(--surface-0)] flex flex-col items-center justify-center space-y-8">
      <div class="text-4xl md:text-5xl font-bold tracking-tight text-[var(--text-primary)]">MUSIC PARTY</div>
      <div class="font-mono text-xs text-[var(--text-tertiary)] tracking-[0.3em]">准备就绪</div>
      <button
          @click="startGame"
          class="min-h-[44px] px-12 py-4 bg-[var(--accent)] text-[var(--text-inverse)] font-semibold text-lg hover:bg-[var(--accent-hover)] active:scale-[0.98] transition-colors rounded-xl shadow-lg"
      >
        进入房间
      </button>
    </div>

    <!-- 3. 主界面 (当 hasStarted 为 true 时显示) -->
    <MobilePreviewShell v-if="hasStarted && isMobileLayout && usePreviewShell">
      <MobileLayout />
    </MobilePreviewShell>

    <MobileLayout v-else-if="hasStarted && isMobileLayout" />

    <MainLayout v-else-if="hasStarted" @search="handleSearchClick" @toggle-mobile-chat="handleMobileChat">
      <!-- 中间插槽: 视觉控制台 -->
      <CenterConsole />

      <!-- 底部插槽: 播放器 -->
      <!-- 注意：这里不使用 v-if，而是 v-show，或者因为在 MainLayout 里是 slot，
           只有 MainLayout 渲染了，它才会渲染。
           关键是 useAudio 里的逻辑已经修好了，会自动处理播放。
      -->
      <template #player>
        <PlayerControl />
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
import { useToast } from './composables/useToast';

// Components
import MainLayout from './components/layout/MainLayout.vue';
import CenterConsole from './components/CenterConsole.vue';
import PlayerControl from './components/PlayerControl.vue';
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
const hasStarted = ref(false);
const showSearch = ref(false);
const toastInstance = ref(null);
const chatOverlayRef = ref(null);
const { register } = useToast();
const { width } = useWindowSize();
const isMobileLayout = computed(() => uiStore.forceMobileLayout || width.value < 768);
const usePreviewShell = computed(() => uiStore.forceMobileLayout && width.value >= 768);
let autoLiteTimer = null;
let autoLiteSuppressedUntil = 0;
const AUTO_LITE_DELAY_MS = 180000;
const AUTO_LITE_SUPPRESS_MS = 600000;

const startGame = () => {
  hasStarted.value = true;
  player.connect();
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

onMounted(() => {
  const params = new URLSearchParams(window.location.search);
  const mobilePreview = params.get('mobilePreview');
  if (mobilePreview === '1') uiStore.setForceMobileLayout(true);
  if (mobilePreview === '0') uiStore.setForceMobileLayout(false);

  if (toastInstance.value) register(toastInstance.value);
});

onBeforeUnmount(() => {
  clearAutoLiteTimer();
});
</script>
