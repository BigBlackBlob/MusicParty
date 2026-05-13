<template>
  <div class="h-[var(--app-height)] w-full flex flex-col overflow-hidden bg-[var(--surface-0)] text-[var(--text-primary)]">
    <!-- Header -->
    <header class="h-[var(--mobile-top-bar-height)] flex-shrink-0 flex items-center justify-between border-b border-[var(--border-default)] bg-[var(--surface-1)] px-4 pt-safe">
      <div class="flex items-center gap-2">
        <div class="w-2 h-2 rounded-full bg-[var(--accent)] shadow-[0_0_8px_var(--accent-muted)]"></div>
        <h1 class="text-sm font-black tracking-tighter uppercase">Music Party</h1>
      </div>

      <div class="flex items-center gap-1">
        <IconButton size="sm" @click="showMembers = true" title="在线成员">
          <Users class="h-4 w-4" />
        </IconButton>
        <IconButton size="sm" @click="ui.toggleDarkMode" title="切换主题">
          <Sun v-if="ui.isDarkMode" class="h-4 w-4" />
          <Moon v-else class="h-4 w-4" />
        </IconButton>
      </div>
    </header>

    <!-- Main Content -->
    <main class="flex-1 min-h-0 relative overflow-hidden bg-[var(--surface-0)]">
      <Transition name="fade-slide" mode="out-in">
        <MobileNowPlaying v-if="activeTab === 'now'" :key="'now'" @open-search="activeTab = 'search'" />
        <MobileQueueView v-else-if="activeTab === 'queue'" :key="'queue'" />
        <MobileSearchView v-else-if="activeTab === 'search'" :key="'search'" />
        <MobileChatView v-else-if="activeTab === 'chat'" :key="'chat'" />
      </Transition>
    </main>

    <!-- Navigation -->
    <MobileBottomNav v-model:active="activeTab" />

    <!-- Members Overlay (using reka-ui Dialog) -->
    <DialogRoot :open="showMembers" @update:open="val => showMembers = val">
      <DialogPortal>
        <DialogOverlay class="fixed inset-0 z-[var(--z-modal)] bg-black/60 backdrop-blur-sm transition-all duration-300" />
        <DialogContent class="fixed inset-x-0 bottom-0 z-[var(--z-modal)] flex justify-center outline-none">
          <div class="w-full max-w-lg bg-[var(--surface-1)] rounded-t-[var(--radius-lg)] border-t border-[var(--border-default)] flex flex-col max-h-[80dvh] shadow-2xl animate-in slide-in-from-bottom duration-300">
            <div class="flex items-center justify-between px-4 py-4 border-b border-[var(--border-default)]">
              <div>
                <DialogTitle class="text-sm font-bold text-[var(--text-primary)]">在线成员</DialogTitle>
                <DialogDescription class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-widest">{{ user.onlineUsers.length }} Active</DialogDescription>
              </div>
              <IconButton variant="secondary" size="sm" @click="showMembers = false">
                <X class="h-4 w-4" />
              </IconButton>
            </div>

            <div class="flex-1 overflow-y-auto p-3">
              <div v-for="member in user.onlineUsers" :key="member.token" class="flex items-center gap-3 px-3 py-2.5 rounded-[var(--radius-md)] hover:bg-[var(--surface-2)] transition-colors mb-1">
                <div class="h-9 w-9 rounded-[var(--radius-sm)] bg-[var(--surface-3)] flex items-center justify-center text-[var(--text-secondary)] font-bold text-xs uppercase">
                  {{ member.name.charAt(0) }}
                </div>
                <div class="min-w-0 flex-1">
                  <div class="truncate text-sm font-bold text-[var(--text-primary)]">{{ member.name }}</div>
                  <div class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-wider">{{ member.isGuest ? 'Guest' : 'Member' }}</div>
                </div>
                <div class="h-1.5 w-1.5 rounded-full bg-[var(--success)] shadow-[0_0_8px_var(--success)]"></div>
              </div>
            </div>
            <div class="h-safe-bottom" />
          </div>
        </DialogContent>
      </DialogPortal>
    </DialogRoot>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue';
import { Moon, Sun, Users, X } from 'lucide-vue-next';
import { DialogRoot, DialogPortal, DialogOverlay, DialogContent, DialogTitle, DialogDescription } from 'reka-ui';
import { useChatStore } from '../../stores/chat';
import { usePlayerStore } from '../../stores/player';
import { useUiStore } from '../../stores/ui';
import { useUserStore } from '../../stores/user';
import MobileBottomNav from './MobileBottomNav.vue';
import MobileChatView from './MobileChatView.vue';
import MobileNowPlaying from './MobileNowPlaying.vue';
import MobileQueueView from './MobileQueueView.vue';
import MobileSearchView from './MobileSearchView.vue';

// UI Primitives
import IconButton from '../ui/IconButton.vue';

const activeTab = ref('now');
const showMembers = ref(false);
const chat = useChatStore();
const player = usePlayerStore();
const ui = useUiStore();
const user = useUserStore();

watch(activeTab, (tab) => {
  chat.isOpen = tab === 'chat';
  if (tab === 'chat') chat.unreadCount = 0;
});
</script>

<style scoped>
.pt-safe {
  padding-top: env(safe-area-inset-top);
  height: calc(var(--mobile-top-bar-height) + env(safe-area-inset-top));
}

.h-safe-bottom {
  height: env(safe-area-inset-bottom);
}

.fade-slide-enter-active, .fade-slide-leave-active {
  transition: all 0.2s ease;
}
.fade-slide-enter-from { opacity: 0; transform: translateX(8px); }
.fade-slide-leave-to { opacity: 0; transform: translateX(-8px); }

.slide-up-enter-active, .slide-up-leave-active {
  transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.2s ease;
}
.slide-up-enter-from { transform: translateY(100%); opacity: 0; }
.slide-up-leave-to { transform: translateY(100%); opacity: 0; }
</style>
