<template>
  <nav class="mobile-bottom-nav flex-shrink-0 border-t border-[var(--border-default)] bg-[var(--surface-4)]/95 px-2 backdrop-blur-xl">
    <div class="grid h-full grid-cols-4 gap-1">
      <button
        v-for="item in items"
        :key="item.key"
        type="button"
        class="relative flex min-h-0 min-w-0 flex-col items-center justify-center gap-0.5 rounded-2xl text-[10px] font-semibold transition-colors active:scale-[0.98]"
        :class="active === item.key ? 'bg-[var(--accent-subtle)] text-[var(--accent)]' : 'text-[var(--text-secondary)]'"
        :aria-label="item.label"
        @click="$emit('update:active', item.key)"
      >
        <component :is="item.icon" class="h-5 w-5 flex-shrink-0" />
        <span class="max-w-full truncate leading-tight">{{ item.label }}</span>
        <span v-if="item.badge" class="absolute right-3 top-2 min-w-5 rounded-full bg-[var(--accent)] px-1.5 text-[10px] leading-5 text-[var(--text-inverse)]">
          {{ item.badge }}
        </span>
      </button>
    </div>
  </nav>
</template>

<script setup>
import { MessageSquare, Music2, Search, ListMusic } from 'lucide-vue-next';
import { useChatStore } from '../../stores/chat';

defineProps({
  active: {
    type: String,
    required: true
  }
});
defineEmits(['update:active']);

const chat = useChatStore();
const items = [
  { key: 'now', label: '播放', icon: Music2 },
  { key: 'queue', label: '队列', icon: ListMusic },
  { key: 'search', label: '搜索', icon: Search },
  { key: 'chat', label: '聊天', icon: MessageSquare, badge: chat.unreadCount > 0 ? (chat.unreadCount > 99 ? '99+' : chat.unreadCount) : '' }
];
</script>

<style scoped>
.mobile-bottom-nav {
  height: calc(4.75rem + env(safe-area-inset-bottom));
  padding-top: 0.5rem;
  padding-bottom: calc(0.5rem + env(safe-area-inset-bottom));
}
</style>
