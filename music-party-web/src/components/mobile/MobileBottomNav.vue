<template>
  <nav class="h-[var(--mobile-bottom-nav-height)] relative z-[var(--z-header)] flex-shrink-0 border-t border-[var(--border-default)] bg-[var(--surface-1)]/95 px-4 pt-1 safe-area-bottom">
    <div class="grid h-full grid-cols-4 items-center gap-1">
      <button
        v-for="item in items"
        :key="item.key"
        type="button"
        role="tab"
        :aria-selected="active === item.key"
        :aria-label="item.ariaLabel"
        class="relative flex flex-col items-center justify-center gap-1 transition-all duration-300 active:scale-[0.9]"
        :class="active === item.key ? 'text-[var(--accent)]' : 'text-[var(--text-tertiary)] hover:text-[var(--text-primary)]'"
        @click="$emit('update:active', item.key)"
      >
        <div
          class="p-2 rounded-[var(--radius-sm)] transition-colors duration-300"
        >
          <component :is="item.icon" class="h-6 w-6" :class="active === item.key ? 'drop-shadow-[0_0_8px_var(--accent)]' : ''" />
        </div>
        <span class="text-[9px] font-black uppercase tracking-widest leading-none mt-0.5 opacity-80">{{ item.label }}</span>

        <span v-if="item.badge" class="absolute top-1 right-[calc(50%-20px)] flex h-4 min-w-[16px] items-center justify-center rounded-full bg-[var(--accent)] px-1 text-[9px] font-bold text-white shadow-lg">
          {{ item.badge }}
        </span>
      </button>
    </div>
  </nav>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { MessageSquare, Music2, Search, ListMusic } from 'lucide-vue-next';
import { useChatStore } from '../../stores/chat';

defineProps({
  active: {
    type: String,
    required: true
  }
});
defineEmits(['update:active']);

const { t } = useI18n();
const chat = useChatStore();
const items = computed(() => [
  { key: 'now', label: t('nav.nowPlaying'), ariaLabel: t('nav.nowPlaying'), icon: Music2 },
  { key: 'queue', label: t('nav.queue'), ariaLabel: t('nav.queue'), icon: ListMusic },
  { key: 'search', label: t('nav.search'), ariaLabel: t('nav.search'), icon: Search },
  { key: 'chat', label: t('nav.chat'), ariaLabel: t('nav.chat'), icon: MessageSquare, badge: chat.unreadCount > 0 ? (chat.unreadCount > 99 ? '99+' : chat.unreadCount) : '' }
]);
</script>

<style scoped>
.safe-area-bottom {
  height: calc(var(--mobile-bottom-nav-height) + env(safe-area-inset-bottom));
  padding-bottom: env(safe-area-inset-bottom);
}
</style>
