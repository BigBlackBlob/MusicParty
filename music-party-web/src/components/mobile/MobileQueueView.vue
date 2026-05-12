<template>
  <section class="flex h-full flex-col overflow-hidden">
    <div class="border-b border-[var(--border-default)] px-4 py-3">
      <h2 class="text-lg font-bold">播放队列</h2>
      <p class="text-xs text-[var(--text-tertiary)]">{{ player.queue.length }} 首待播</p>
    </div>

    <div class="min-h-0 flex-1 overflow-y-auto px-4 py-3">
      <div v-if="player.queue.length === 0" class="flex h-full items-center justify-center text-sm text-[var(--text-tertiary)]">
        队列为空
      </div>

      <div v-for="(item, index) in player.queue" :key="item.queueId" class="mb-2 flex min-h-[72px] items-center gap-3 rounded-3xl border border-[var(--border-default)] bg-[var(--surface-4)] px-3 py-2">
        <div class="w-7 text-center font-mono text-xs text-[var(--text-tertiary)]">{{ index + 1 }}</div>
        <div class="h-12 w-12 flex-shrink-0 overflow-hidden rounded-2xl bg-[var(--surface-3)]">
          <CoverImage :src="item.music.coverUrl" :alt="`${item.music.name} 封面`" loading="lazy" class="h-full w-full" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-semibold text-[var(--text-primary)]">{{ item.music.name }}</div>
          <div class="truncate text-xs text-[var(--text-secondary)]">{{ item.music.artists.join(' / ') }}</div>
          <div class="mt-1 truncate text-[11px] text-[var(--text-tertiary)]">{{ item.enqueuedBy.name }}</div>
        </div>
        <div v-if="!user.isGuest" class="flex flex-col gap-1">
          <button class="queue-action" @click="player.topSong(item.queueId)" aria-label="置顶"><ArrowUpToLine class="h-4 w-4" /></button>
          <button class="queue-action text-[var(--error)]" @click="player.removeSong(item.queueId)" aria-label="移除"><Trash2 class="h-4 w-4" /></button>
        </div>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ArrowUpToLine, Trash2 } from 'lucide-vue-next';
import { usePlayerStore } from '../../stores/player';
import { useUserStore } from '../../stores/user';
import CoverImage from '../CoverImage.vue';

const player = usePlayerStore();
const user = useUserStore();
</script>

<style scoped>
.queue-action {
  display: inline-flex;
  min-width: 2.75rem;
  min-height: 2.75rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: var(--surface-3);
  color: var(--text-secondary);
}
</style>
