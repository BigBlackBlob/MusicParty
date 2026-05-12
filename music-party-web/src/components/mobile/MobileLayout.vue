<template>
  <div class="mobile-layout flex h-full min-h-0 w-full flex-col overflow-hidden bg-[var(--surface-0)] text-[var(--text-primary)]">
    <header class="flex min-h-[56px] flex-shrink-0 items-center justify-between border-b border-[var(--border-default)] bg-[var(--surface-1)] px-4 pt-[env(safe-area-inset-top)]">
      <div class="min-w-0">
        <div class="text-sm font-bold tracking-tight">MUSIC PARTY</div>
        <div class="text-[11px] text-[var(--text-tertiary)]">
          {{ player.connected ? `${user.onlineUsers.length} 人在线` : '正在重连' }}
        </div>
      </div>
      <div class="flex items-center gap-2">
        <button
          type="button"
          class="flex min-h-[44px] min-w-[44px] items-center justify-center rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-secondary)] active:scale-[0.96]"
          aria-label="在线成员"
          @click="showMembers = true"
        >
          <Users class="h-4 w-4" />
        </button>
        <button
          type="button"
          class="flex min-h-[44px] min-w-[44px] items-center justify-center rounded-xl border border-[var(--border-default)] bg-[var(--surface-2)] text-[var(--text-secondary)] active:scale-[0.96]"
          :aria-label="ui.isDarkMode ? '切换到浅色模式' : '切换到深色模式'"
          @click="ui.toggleDarkMode"
        >
          <Sun v-if="ui.isDarkMode" class="h-4 w-4" />
          <Moon v-else class="h-4 w-4" />
        </button>
      </div>
    </header>

    <main class="min-h-0 flex-1 overflow-hidden">
      <MobileNowPlaying v-if="activeTab === 'now'" @open-search="activeTab = 'search'" />
      <MobileQueueView v-else-if="activeTab === 'queue'" />
      <MobileSearchView v-else-if="activeTab === 'search'" />
      <MobileChatView v-else-if="activeTab === 'chat'" />
    </main>

    <MobileBottomNav v-model:active="activeTab" />

    <div v-if="showMembers" class="fixed inset-0 z-[var(--z-modal)] bg-black/55 p-4 backdrop-blur-sm" @click.self="showMembers = false">
      <div class="mx-auto mt-[calc(env(safe-area-inset-top)+1rem)] max-h-[75dvh] max-w-sm overflow-hidden rounded-3xl border border-[var(--border-default)] bg-[var(--surface-4)] shadow-2xl">
        <div class="flex items-center justify-between border-b border-[var(--border-default)] px-4 py-3">
          <div>
            <div class="text-sm font-bold">在线成员</div>
            <div class="text-xs text-[var(--text-tertiary)]">{{ user.onlineUsers.length }} 人</div>
          </div>
          <button class="min-h-[44px] min-w-[44px] rounded-xl text-[var(--text-secondary)]" @click="showMembers = false" aria-label="关闭成员列表">
            <X class="mx-auto h-5 w-5" />
          </button>
        </div>
        <div class="max-h-[60dvh] overflow-y-auto p-3">
          <div v-for="member in user.onlineUsers" :key="member.token" class="mb-2 rounded-2xl bg-[var(--surface-2)] px-3 py-2">
            <div class="truncate text-sm font-semibold">{{ member.name }}</div>
            <div class="text-[11px] text-[var(--text-tertiary)]">{{ member.isGuest ? '访客' : '成员' }}</div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, watch } from 'vue';
import { Moon, Sun, Users, X } from 'lucide-vue-next';
import { useChatStore } from '../../stores/chat';
import { usePlayerStore } from '../../stores/player';
import { useUiStore } from '../../stores/ui';
import { useUserStore } from '../../stores/user';
import MobileBottomNav from './MobileBottomNav.vue';
import MobileChatView from './MobileChatView.vue';
import MobileNowPlaying from './MobileNowPlaying.vue';
import MobileQueueView from './MobileQueueView.vue';
import MobileSearchView from './MobileSearchView.vue';

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
.mobile-layout {
  height: 100%;
  min-height: 0;
}
</style>
