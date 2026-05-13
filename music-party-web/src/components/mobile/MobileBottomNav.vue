<template>
  <nav class="h-[var(--mobile-bottom-nav-height)] md:hidden relative z-[var(--z-header)] flex-shrink-0 border-t border-[var(--border-default)] bg-[var(--surface-1)]/95 backdrop-blur-xl px-4 pt-1 safe-area-bottom">
    <div class="grid h-full grid-cols-4 items-center gap-1">
      <button
        v-for="item in items"
        :key="item.key"
        type="button"
        class="relative flex flex-col items-center justify-center gap-1 transition-all active:scale-[0.9]"
        :class="active === item.key ? 'text-[var(--accent)]' : 'text-[var(--text-tertiary)]'"
        @click="$emit('update:active', item.key)"
      >
        <div
          class="p-2 rounded-[var(--radius-sm)] transition-colors"
          :class="active === item.key ? 'bg-[var(--accent-subtle)]' : 'hover:bg-[var(--surface-3)]'"
        >
          <component :is="item.icon" class="h-5 w-5" />
        </div>
        <span class="text-[9px] font-black uppercase tracking-widest leading-none">{{ item.label }}</span>

        <span v-if="item.badge" class="absolute top-1.5 right-[calc(50%-18px)] flex h-4 min-w-[16px] items-center justify-center rounded-full bg-[var(--accent)] px-1 text-[8px] font-bold text-[var(--text-inverse)] ring-2 ring-[var(--surface-1)]">
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
  { key: 'now', label: 'NOW', icon: Music2 },
  { key: 'queue', label: 'QUEUE', icon: ListMusic },
  { key: 'search', label: 'SEARCH', icon: Search },
  { key: 'chat', label: 'CHAT', icon: MessageSquare, badge: chat.unreadCount > 0 ? (chat.unreadCount > 99 ? '99+' : chat.unreadCount) : '' }
];
</script>

<style scoped>
.safe-area-bottom {
  height: calc(var(--mobile-bottom-nav-height) + env(safe-area-inset-bottom));
  padding-bottom: env(safe-area-inset-bottom);
}
</style>
