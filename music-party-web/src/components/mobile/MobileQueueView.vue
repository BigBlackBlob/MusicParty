<template>
  <section class="flex h-full flex-col overflow-hidden">
    <div class="border-b border-[var(--border-default)] px-4 py-3">
      <div class="flex items-start justify-between gap-3">
        <div>
          <h2 class="text-lg font-bold">{{ activeView === 'queue' ? '播放队列' : '喜欢的歌' }}</h2>
          <p class="text-xs text-[var(--text-tertiary)]">
            {{ activeView === 'queue' ? `${player.queue.length} 首待播` : `${player.likedSongs.length} 首已保存` }}
          </p>
        </div>
        <button
          v-if="activeView === 'liked'"
          class="export-action"
          :disabled="player.likedSongs.length === 0"
          @click="exportLikedSongs"
          aria-label="导出喜欢的歌"
        >
          <Download class="h-4 w-4" />
          导出
        </button>
      </div>
      <div class="mt-3 grid grid-cols-2 rounded-2xl bg-[var(--surface-3)] p-1">
        <button
          class="view-toggle"
          :class="activeView === 'queue' ? 'view-toggle--active' : ''"
          @click="activeView = 'queue'"
        >
          队列
        </button>
        <button
          class="view-toggle"
          :class="activeView === 'liked' ? 'view-toggle--active' : ''"
          @click="activeView = 'liked'"
        >
          喜欢
        </button>
      </div>
    </div>

    <div v-if="activeView === 'queue'" class="min-h-0 flex-1 overflow-y-auto px-4 py-3">
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

    <div v-else class="min-h-0 flex-1 overflow-y-auto px-4 py-3">
      <div v-if="player.likedSongs.length === 0" class="flex h-full items-center justify-center px-8 text-center text-sm leading-relaxed text-[var(--text-tertiary)]">
        还没有喜欢的歌。播放页右侧红心会把当前歌曲保存到这里。
      </div>

      <div v-for="song in player.likedSongs" :key="song.key" class="mb-2 flex min-h-[72px] items-center gap-3 rounded-3xl border border-[var(--border-default)] bg-[var(--surface-4)] px-3 py-2">
        <div class="h-12 w-12 flex-shrink-0 overflow-hidden rounded-2xl bg-[var(--surface-3)]">
          <CoverImage :src="song.coverUrl" :alt="`${song.name} 封面`" loading="lazy" class="h-full w-full" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="truncate text-sm font-semibold text-[var(--text-primary)]">{{ song.name }}</div>
          <div class="truncate text-xs text-[var(--text-secondary)]">{{ song.artists.join(' / ') }}</div>
          <div class="mt-1 truncate text-[11px] text-[var(--text-tertiary)]">{{ song.platform }}</div>
        </div>
        <button class="queue-action text-[var(--error)]" @click="player.removeLikedSong(song.key)" aria-label="从喜欢的歌移除">
          <Trash2 class="h-4 w-4" />
        </button>
      </div>
    </div>
  </section>
</template>

<script setup>
import { ref } from 'vue';
import { ArrowUpToLine, Download, Trash2 } from 'lucide-vue-next';
import { usePlayerStore } from '../../stores/player';
import { useUserStore } from '../../stores/user';
import { createLikedSongsFilename, createLikedSongsText } from '../../utils/likedSongs';
import CoverImage from '../CoverImage.vue';

const player = usePlayerStore();
const user = useUserStore();
const activeView = ref('queue');

const exportLikedSongs = () => {
  if (!player.likedSongs.length) return;
  const content = createLikedSongsText(player.likedSongs);
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = createLikedSongsFilename();
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};
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

.view-toggle {
  min-height: 2.35rem;
  border-radius: 0.85rem;
  color: var(--text-secondary);
  font-size: 0.8rem;
  font-weight: 700;
  transition: background-color 0.18s ease, color 0.18s ease;
}

.view-toggle--active {
  background: var(--surface-1);
  color: var(--text-primary);
  box-shadow: 0 8px 22px rgba(0, 0, 0, 0.12);
}

.export-action {
  display: inline-flex;
  min-height: 2.5rem;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  border-radius: 9999px;
  background: var(--accent);
  padding: 0 0.85rem;
  color: var(--text-inverse);
  font-size: 0.75rem;
  font-weight: 800;
}

.export-action:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}
</style>
