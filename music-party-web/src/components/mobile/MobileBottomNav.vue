<template>
  <nav class="relative w-full bg-surface-overlay/80 backdrop-blur-lg border-t border-border-default min-h-[72px] flex-shrink-0 flex items-stretch z-30 pb-[env(safe-area-inset-bottom)]">
    <button
      v-for="item in items"
      :key="item.key"
      type="button"
      role="tab"
      :aria-selected="active === item.key"
      :aria-label="item.ariaLabel"
      class="relative flex-1 flex flex-col items-center justify-center gap-1 transition-all duration-300 active:scale-95"
      :class="active === item.key ? 'text-primary' : 'text-text-muted hover:text-primary'"
      @click="$emit('update:active', item.key)"
    >
      <span class="material-symbols-outlined" :style="active === item.key ? 'font-variation-settings: \'FILL\' 1;' : ''">
        {{ item.iconName }}
      </span>
      <span class="text-micro font-micro uppercase tracking-wider">{{ item.label }}</span>

      <span v-if="item.badge" class="absolute top-2 right-[25%] flex h-4 min-w-[16px] items-center justify-center rounded-full bg-primary px-1 text-[9px] font-bold text-on-primary shadow-lg z-10">
        {{ item.badge }}
      </span>
    </button>
  </nav>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
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
  { key: 'now', label: t('nav.nowPlaying'), ariaLabel: t('nav.nowPlaying'), iconName: 'play_circle' },
  { key: 'queue', label: t('nav.queue'), ariaLabel: t('nav.queue'), iconName: 'queue_music' },
  { key: 'search', label: t('nav.search'), ariaLabel: t('nav.search'), iconName: 'search' },
  { key: 'chat', label: t('nav.chat'), ariaLabel: t('nav.chat'), iconName: 'chat', badge: chat.unreadCount > 0 ? (chat.unreadCount > 99 ? '99+' : chat.unreadCount) : '' }
]);
</script>

<style scoped>
/* Removed specific safe-area-bottom height as it is handled by padding and box-content */
</style>
