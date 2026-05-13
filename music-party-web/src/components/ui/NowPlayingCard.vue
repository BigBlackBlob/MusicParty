<template>
  <div class="flex items-center gap-4 min-w-0 group/card">
    <div
      class="relative h-14 w-14 sm:h-16 sm:w-16 flex-shrink-0 overflow-hidden rounded-[var(--radius-md)] bg-[var(--surface-3)] shadow-lg border border-[var(--border-default)] cursor-pointer"
      @click="$emit('open-source')"
      title="打开来源页面"
    >
      <CoverImage
        :src="coverUrl"
        :alt="title"
        class="h-full w-full object-cover transition-transform duration-500 group-hover/card:scale-110"
      />
      <div class="absolute inset-0 bg-black/40 flex items-center justify-center opacity-0 group-hover/card:opacity-100 transition-opacity">
        <ExternalLink class="w-5 h-5 text-white" />
      </div>
    </div>

    <div class="min-w-0 flex-1">
      <div class="flex flex-col">
        <h2
          class="truncate text-base sm:text-lg font-bold transition-colors duration-300"
          :class="!connected ? 'text-[var(--error-soft-text)]' : 'text-[var(--text-primary)]'"
        >
          {{ !connected ? '连接已断开' : (title || '等待播放') }}
        </h2>
        <p class="truncate text-xs font-medium text-[var(--text-secondary)]">
          {{ !connected ? '正在重连...' : (artist || 'Music Party') }}
        </p>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ExternalLink } from 'lucide-vue-next';
import CoverImage from '../CoverImage.vue';

defineProps({
  title: String,
  artist: String,
  coverUrl: String,
  connected: Boolean
});

defineEmits(['open-source']);
</script>
