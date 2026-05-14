<template>
  <div class="glass-panel flex h-full min-h-0 w-full flex-col overflow-hidden rounded-lg p-4">
    <!-- Header -->
    <div class="mb-3 flex flex-shrink-0 items-center justify-between gap-3 px-1">
      <div class="flex items-center rounded-md border border-border-subtle bg-[var(--surface-control)] p-1">
        <button
          class="px-3 py-1 rounded font-section-label text-section-label uppercase tracking-widest transition-colors shadow-sm"
          :class="activeView === 'queue' ? 'bg-[var(--surface-control-active)] text-primary' : 'text-text-muted hover:text-text-primary'"
          @click="activeView = 'queue'"
        >
          {{ t('queue.upNext') }}
        </button>
        <button
          class="px-3 py-1 rounded font-section-label text-section-label uppercase tracking-widest transition-colors shadow-sm"
          :class="activeView === 'liked' ? 'bg-[var(--surface-control-active)] text-primary' : 'text-text-muted hover:text-text-primary'"
          @click="activeView = 'liked'"
        >
          {{ t('queue.liked') }}
        </button>
      </div>

      <!-- Actions / Metadata -->
      <div class="flex min-w-0 items-center gap-2">
        <span class="font-micro text-micro text-text-muted uppercase">{{ activeView === 'queue' ? queue.length : player.likedSongs.length }} {{ t('queue.tracks') }}</span>
        <button v-if="activeView === 'queue' && queue.length > 0" @click="toggleSelectionMode" class="flex h-8 w-8 items-center justify-center rounded-md text-text-muted hover:bg-[var(--surface-control-hover)] hover:text-text-primary" :title="t('queue.selectTracks')">
          <span class="material-symbols-outlined text-[16px]">{{ selectionMode ? 'close' : 'checklist' }}</span>
        </button>

        <button v-if="activeView === 'liked' && player.likedSongs.length > 0" @click="exportLikedSongs" class="text-text-muted hover:text-text-primary" :title="t('common.export')">
          <span class="material-symbols-outlined text-[16px]">download</span>
        </button>
      </div>
    </div>

    <div v-if="selectionMode && activeView === 'queue'" class="mb-3 flex flex-shrink-0 items-center gap-2 rounded-md border border-border-subtle bg-[var(--surface-control)] px-2 py-2">
      <span class="min-w-0 flex-1 truncate text-xs text-text-muted">{{ selectedCount }} {{ t('queue.selected') }}</span>
      <button class="h-8 rounded-md px-2 text-xs text-text-secondary hover:bg-[var(--surface-control-hover)] hover:text-text-primary" @click="selectAll">{{ t('queue.all') }}</button>
      <button class="h-8 rounded-md px-2 text-xs text-text-secondary hover:bg-[var(--surface-control-hover)] hover:text-text-primary disabled:opacity-40" :disabled="!hasSelection" @click="batchTop">{{ t('queue.top') }}</button>
      <button class="h-8 rounded-md px-2 text-xs text-error hover:bg-[var(--surface-control-hover)] disabled:opacity-40" :disabled="!hasSelection" @click="batchRemove">{{ t('queue.remove') }}</button>
    </div>

    <div class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto pr-1">

      <!-- Liked View -->
      <div v-if="activeView === 'liked'" class="flex flex-col gap-2">
        <div v-if="player.likedSongs.length === 0" class="py-16 text-center">
          <div class="text-sm font-bold text-text-primary">{{ t('queue.noLiked') }}</div>
          <div class="mt-1 text-xs text-text-muted">{{ t('queue.likedDesc') }}</div>
        </div>

        <TrackListItem
          v-for="song in player.likedSongs"
          :key="song.key"
          :title="song.name"
            :artist="formatArtists(song.artists)"
          :cover-url="song.coverUrl"
        >
          <template #meta>
            {{ song.platform }}
          </template>
          <template #suffix>
            <button @click="player.removeLikedSong(song.key)" class="text-error hover:text-red-400" :title="t('queue.remove')">
              <span class="material-symbols-outlined text-[18px]">delete</span>
            </button>
          </template>
        </TrackListItem>
      </div>

      <!-- Queue View -->
      <div v-else class="flex min-h-0 flex-1 flex-col gap-2">
        <div v-if="queue.length === 0" class="py-16 text-center">
          <div class="text-sm font-bold text-text-primary">{{ t('queue.empty') }}</div>
          <div class="mt-1 text-xs text-text-muted">{{ t('queue.emptyDesc') }}</div>
        </div>

        <div v-else class="flex min-h-0 flex-1 flex-col gap-2">
          <QueueItem
            v-for="(item, index) in queue"
            :key="item.queueId || `${item.music?.platform || 'track'}:${item.music?.id || index}`"
            :item="item"
            :index="index"
            :selection-mode="selectionMode"
            :selected="isSelected(item.queueId)"
            @toggle-select="toggleSelected(item.queueId)"
          />
        </div>
      </div>

    </div>
  </div>
</template>

<script setup>
import { computed, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { usePlayerStore } from '../stores/player';
import QueueItem from './QueueItem.vue';
import TrackListItem from './ui/TrackListItem.vue';
import { createLikedSongsFilename, createLikedSongsText } from '../utils/likedSongs';
import { useQueueSelection } from '../composables/useQueueSelection';

const player = usePlayerStore();
const { t } = useI18n();
const queue = computed(() => player.queue);
const activeView = ref('queue');
const {
  selectionMode,
  selectedCount,
  selectedIds,
  hasSelection,
  toggleSelectionMode,
  exitSelectionMode,
  toggleSelected,
  isSelected,
  selectAll
} = useQueueSelection(queue);

const batchTop = () => {
  const ids = selectedIds.value;
  if (!ids.length) return;
  if (!player.topSongs(ids)) player.topSongsCompat(ids);
  exitSelectionMode();
};

const batchRemove = () => {
  const ids = selectedIds.value;
  if (!ids.length) return;
  if (!player.removeSongs(ids)) player.removeSongsCompat(ids);
  exitSelectionMode();
};

const exportLikedSongs = () => {
  if (!player.likedSongs.length) return;
  const blob = new Blob([createLikedSongsText(player.likedSongs)], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = createLikedSongsFilename();
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};

const formatArtists = (artists) => Array.isArray(artists) && artists.length ? artists.join(' / ') : t('common.unknownArtist');

</script>
