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

      <div v-if="activeView === 'queue' && selectionMode" class="mt-3 flex items-center justify-between rounded-2xl bg-[var(--accent-subtle)] px-3 py-2">
        <span class="text-sm font-bold text-[var(--accent)]">已选 {{ selectedCount }}</span>
        <span class="text-xs text-[var(--text-tertiary)]">点击条目调整选择</span>
      </div>
    </div>

    <div v-if="activeView === 'queue'" class="relative min-h-0 flex-1 overflow-hidden">
      <div class="h-full overflow-y-auto px-3 py-2" :class="selectionMode ? 'pr-[4.75rem]' : ''">
      <div v-if="player.queue.length === 0" class="flex h-full items-center justify-center text-sm text-[var(--text-tertiary)]">
        队列为空
      </div>

      <div
        v-for="(item, index) in player.queue"
        :key="item.queueId"
        class="mb-1.5 grid min-h-[52px] grid-cols-[1.25rem_2.25rem_minmax(0,1fr)_2.35rem] items-center gap-2 rounded-2xl border bg-[var(--surface-4)] py-1 pl-2.5 pr-1.5"
        :class="[
          selectionMode ? 'cursor-pointer' : '',
          isSelected(item.queueId) ? 'border-[var(--border-accent)] bg-[var(--accent-subtle)]' : 'border-[var(--border-default)]'
        ]"
        @click="handleQueueItemClick(item.queueId)"
        @pointerdown="startLongPress(item.queueId)"
        @pointerup="clearLongPress"
        @pointercancel="clearLongPress"
        @pointerleave="clearLongPress"
      >
        <div class="text-center font-mono text-[11px] text-[var(--text-tertiary)]">
          <span
            v-if="selectionMode"
            class="inline-flex h-6 w-6 items-center justify-center rounded-full border"
            :class="isSelected(item.queueId) ? 'border-[var(--accent)] bg-[var(--accent)] text-[var(--text-inverse)]' : 'border-[var(--border-default)] text-transparent'"
          >
            <Check class="h-3.5 w-3.5" />
          </span>
          <span v-else>{{ index + 1 }}</span>
        </div>
        <div class="h-9 w-9 overflow-hidden rounded-xl bg-[var(--surface-3)]">
          <CoverImage :src="item.music.coverUrl" :alt="`${item.music.name} 封面`" loading="lazy" class="h-full w-full" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="truncate text-[13px] font-semibold leading-tight text-[var(--text-primary)]">{{ item.music.name }}</div>
          <div class="truncate text-[11px] leading-tight text-[var(--text-secondary)]">{{ item.music.artists.join(' / ') }}</div>
          <div class="truncate text-[10px] leading-tight text-[var(--text-tertiary)]">{{ item.enqueuedBy.name }}</div>
        </div>
        <div v-if="!user.isGuest && !selectionMode" class="flex flex-col items-center gap-1">
          <button class="queue-action" @click.stop="player.topSong(item.queueId)" aria-label="置顶"><ArrowUpToLine class="h-4 w-4" /></button>
          <button class="queue-action text-[var(--error)]" @click.stop="player.removeSong(item.queueId)" aria-label="移除"><Trash2 class="h-4 w-4" /></button>
        </div>
      </div>
      </div>

      <Transition name="selection-rail">
        <div v-if="selectionMode" class="selection-rail" aria-label="多选操作">
          <button class="selection-rail-button" @click="selectAll" :disabled="player.queue.length === 0" aria-label="全选">
            <CheckSquare class="h-5 w-5" />
          </button>
          <button class="selection-rail-button" @click="topSelected" :disabled="!hasSelection" aria-label="置顶已选">
            <ArrowUpToLine class="h-5 w-5" />
          </button>
          <button class="selection-rail-button selection-rail-button--danger" @click="requestDeleteSelected" :disabled="!hasSelection" aria-label="删除已选">
            <Trash2 class="h-5 w-5" />
          </button>
          <button class="selection-rail-button selection-rail-button--close" @click="cancelSelection" aria-label="取消选择">
            <X class="h-6 w-6" />
          </button>
        </div>
      </Transition>

      <Transition name="delete-confirm-float">
        <div v-if="selectionMode && pendingDelete" class="delete-confirm-float">
          <div class="text-sm font-semibold text-[var(--text-primary)]">删除已选 {{ selectedCount }} 首？</div>
          <div class="mt-2 grid grid-cols-2 gap-2">
            <button class="delete-confirm-action" @click="confirmDeleteSelected">确认删除</button>
            <button class="delete-cancel-action" @click="pendingDelete = false">取消</button>
          </div>
        </div>
      </Transition>
    </div>

    <div v-else class="min-h-0 flex-1 overflow-y-auto px-4 py-3">
      <div v-if="player.likedSongs.length === 0" class="flex h-full items-center justify-center px-8 text-center text-sm leading-relaxed text-[var(--text-tertiary)]">
        还没有喜欢的歌。播放页右侧红心会把当前歌曲保存到这里。
      </div>

      <div v-for="song in player.likedSongs" :key="song.key" class="mb-1.5 grid min-h-[52px] grid-cols-[2.25rem_minmax(0,1fr)_2.35rem] items-center gap-2 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] py-1 pl-2.5 pr-1.5">
        <div class="h-9 w-9 overflow-hidden rounded-xl bg-[var(--surface-3)]">
          <CoverImage :src="song.coverUrl" :alt="`${song.name} 封面`" loading="lazy" class="h-full w-full" />
        </div>
        <div class="min-w-0 flex-1">
          <div class="truncate text-[13px] font-semibold leading-tight text-[var(--text-primary)]">{{ song.name }}</div>
          <div class="truncate text-[11px] leading-tight text-[var(--text-secondary)]">{{ song.artists.join(' / ') }}</div>
          <div class="truncate text-[10px] leading-tight text-[var(--text-tertiary)]">{{ song.platform }}</div>
        </div>
        <button class="queue-action text-[var(--error)]" @click="player.removeLikedSong(song.key)" aria-label="从喜欢的歌移除">
          <Trash2 class="h-4 w-4" />
        </button>
      </div>
    </div>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue';
import { ArrowUpToLine, Check, CheckSquare, Download, Trash2, X } from 'lucide-vue-next';
import { usePlayerStore } from '../../stores/player';
import { useUserStore } from '../../stores/user';
import { createLikedSongsFilename, createLikedSongsText } from '../../utils/likedSongs';
import { useQueueSelection } from '../../composables/useQueueSelection';
import CoverImage from '../CoverImage.vue';

const player = usePlayerStore();
const user = useUserStore();
const activeView = ref('queue');
const queue = computed(() => player.queue);
const pendingDelete = ref(false);
const longPressTimer = ref(null);
const longPressTriggered = ref(false);
const LONG_PRESS_MS = 450;
const {
  selectionMode,
  selectedCount,
  selectedIds,
  hasSelection,
  enterSelectionMode,
  exitSelectionMode,
  toggleSelected,
  isSelected,
  selectAll
} = useQueueSelection(queue);

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

const startLongPress = (queueId) => {
  if (user.isGuest || selectionMode.value) return;
  clearLongPress();
  longPressTriggered.value = false;
  longPressTimer.value = setTimeout(() => {
    longPressTriggered.value = true;
    enterSelectionMode(queueId);
  }, LONG_PRESS_MS);
};

const clearLongPress = () => {
  if (longPressTimer.value) {
    clearTimeout(longPressTimer.value);
    longPressTimer.value = null;
  }
};

const handleQueueItemClick = (queueId) => {
  if (longPressTriggered.value) {
    longPressTriggered.value = false;
    return;
  }
  if (selectionMode.value) {
    toggleSelected(queueId);
  }
};

const cancelSelection = () => {
  pendingDelete.value = false;
  exitSelectionMode();
};

const topSelected = () => {
  if (!hasSelection.value) return;
  player.topSongs(selectedIds.value);
  cancelSelection();
};

const requestDeleteSelected = () => {
  if (!hasSelection.value) return;
  pendingDelete.value = true;
};

const confirmDeleteSelected = () => {
  if (!hasSelection.value) return;
  player.removeSongs(selectedIds.value);
  cancelSelection();
};
</script>

<style scoped>
.queue-action {
  display: inline-flex;
  min-width: 2.2rem;
  min-height: 2.2rem;
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

.delete-confirm-action,
.delete-cancel-action {
  display: inline-flex;
  min-height: 2.75rem;
  align-items: center;
  justify-content: center;
  gap: 0.35rem;
  border-radius: 9999px;
  font-size: 0.8rem;
  font-weight: 800;
}

.delete-cancel-action {
  background: var(--surface-3);
  color: var(--text-secondary);
}

.selection-rail {
  position: absolute;
  right: 0.65rem;
  top: 50%;
  z-index: 30;
  display: flex;
  transform: translateY(-50%);
  flex-direction: column;
  align-items: center;
  gap: 0.65rem;
  pointer-events: auto;
}

.selection-rail-button {
  display: inline-flex;
  width: 3.25rem;
  height: 3.25rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: color-mix(in srgb, var(--surface-4) 92%, transparent);
  color: var(--text-primary);
  box-shadow: 0 12px 34px rgba(0, 0, 0, 0.26);
  backdrop-filter: blur(14px);
  transition: transform 0.18s ease, opacity 0.18s ease, background-color 0.18s ease;
}

.selection-rail-button:active {
  transform: scale(0.94);
}

.selection-rail-button:disabled {
  cursor: not-allowed;
  opacity: 0.45;
}

.selection-rail-button--danger {
  color: var(--error);
}

.selection-rail-button--close {
  width: 3.75rem;
  height: 3.75rem;
  margin-top: 0.3rem;
  background: color-mix(in srgb, var(--surface-4) 96%, transparent);
}

.delete-confirm-float {
  position: absolute;
  inset-inline: 1rem;
  bottom: 1rem;
  z-index: 35;
  border: 1px solid color-mix(in srgb, var(--error) 35%, transparent);
  border-radius: 1.35rem;
  background: var(--surface-4);
  padding: 0.85rem;
  box-shadow: 0 18px 48px rgba(0, 0, 0, 0.28);
}

.delete-confirm-action {
  background: var(--error);
  color: var(--text-inverse);
}

.selection-rail-enter-active,
.selection-rail-leave-active,
.delete-confirm-float-enter-active,
.delete-confirm-float-leave-active {
  transition: opacity 180ms ease, transform 180ms ease;
}

.selection-rail-enter-from,
.selection-rail-leave-to {
  opacity: 0;
  transform: translateY(-50%) translateX(0.5rem) scale(0.96);
}

.delete-confirm-float-enter-from,
.delete-confirm-float-leave-to {
  opacity: 0;
  transform: translateY(0.5rem) scale(0.98);
}

@media (prefers-reduced-motion: reduce) {
  .selection-rail-enter-active,
  .selection-rail-leave-active,
  .delete-confirm-float-enter-active,
  .delete-confirm-float-leave-active {
    transition-duration: 0.01ms;
  }
}
</style>
