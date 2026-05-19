<template>
  <div class="mobile-shell bg-bg-base overflow-hidden">
    <!-- Ambient Background -->
    <Transition name="mobile-ambient-fade">
      <div
        v-if="currentCover && activeTab === 'now'"
        :key="currentCover"
        class="mobile-ambient"
        :style="{ backgroundImage: `url(${currentCover})` }"
      />
    </Transition>

    <main class="mobile-shell-main">
      <Transition name="fade-slide" mode="out-in">
        <MobileNowPlaying 
          v-if="activeTab === 'now'" 
          :key="'now'" 
          @open-settings="showSettings = true"
          @close="activeTab = 'queue'"
        />
        <MobileQueueView v-else-if="activeTab === 'queue'" :key="'queue'" />
        <MobileSearchView v-else-if="activeTab === 'search'" :key="'search'" />
        <MobileChatView v-else-if="activeTab === 'chat'" :key="'chat'" />
      </Transition>
    </main>

    <MobileBottomNav v-model:active="activeTab" />

    <SettingsCenter v-if="showSettings" mobile @close="showSettings = false" />
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import SettingsCenter from '../SettingsCenter.vue';
import { useChatStore } from '../../stores/chat';
import { usePlayerStore } from '../../stores/player';
import { useUiStore } from '../../stores/ui';
import MobileBottomNav from './MobileBottomNav.vue';
import MobileChatView from './MobileChatView.vue';
import MobileNowPlaying from './MobileNowPlaying.vue';
import MobileQueueView from './MobileQueueView.vue';
import MobileSearchView from './MobileSearchView.vue';

const showSettings = ref(false);
const chat = useChatStore();
const player = usePlayerStore();
const ui = useUiStore();
const activeTab = computed({
  get: () => ui.mobileActiveTab,
  set: (tab) => ui.setMobileActiveTab(tab)
});

const currentCover = computed(() => player.nowPlaying?.music?.coverUrl || '');
watch(activeTab, (tab) => {
  chat.isOpen = tab === 'chat';
  if (tab === 'chat') chat.unreadCount = 0;
});
</script>

<style scoped>
.mobile-shell {
  position: relative;
  display: flex;
  width: 100%;
  height: 100%;
  min-height: 0;
  flex-direction: column;
}

.mobile-ambient {
  pointer-events: none;
  position: absolute;
  inset: -10%;
  z-index: 0;
  background-position: center;
  background-size: cover;
  filter: blur(60px) saturate(1.2) brightness(0.4);
  opacity: 0.3;
  transform: scale(1.1);
}

.mobile-shell-main {
  position: relative;
  z-index: 1;
  min-height: 0;
  flex: 1;
  overflow: hidden;
}

.fade-slide-enter-active,
.fade-slide-leave-active {
  transition: opacity 300ms cubic-bezier(0.16, 1, 0.3, 1), transform 300ms cubic-bezier(0.16, 1, 0.3, 1);
}

.fade-slide-enter-from {
  opacity: 0;
  transform: translateX(10px);
}

.fade-slide-leave-to {
  opacity: 0;
  transform: translateX(-10px);
}

.mobile-ambient-fade-enter-active,
.mobile-ambient-fade-leave-active {
  transition: opacity 1000ms ease-out;
}

.mobile-ambient-fade-enter-from,
.mobile-ambient-fade-leave-to {
  opacity: 0;
}
</style>
