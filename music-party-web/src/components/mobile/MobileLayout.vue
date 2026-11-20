<template>
  <div class="mobile-shell bg-bg-base overflow-hidden">
    <StageBackground />

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

    <Transition name="mp-modal-fade">
      <SettingsCenter v-if="showSettings" mobile @close="showSettings = false" />
    </Transition>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import SettingsCenter from '../SettingsCenter.vue';
import StageBackground from '../ui/StageBackground.vue';
import { useChatStore } from '../../stores/chat';
import { useUiStore } from '../../stores/ui';
import MobileBottomNav from './MobileBottomNav.vue';
import MobileChatView from './MobileChatView.vue';
import MobileNowPlaying from './MobileNowPlaying.vue';
import MobileQueueView from './MobileQueueView.vue';
import MobileSearchView from './MobileSearchView.vue';

const showSettings = ref(false);
const chat = useChatStore();
const ui = useUiStore();
const activeTab = computed({
  get: () => ui.mobileActiveTab,
  set: (tab) => ui.setMobileActiveTab(tab)
});

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

</style>
