<template>
  <section class="flex h-full flex-col overflow-hidden bg-[var(--surface-1)]">
    <div class="px-4 py-4 border-b border-[var(--border-default)] bg-[var(--surface-1)]">
      <div class="flex items-center justify-between mb-4">
        <div>
          <h2 class="text-xl font-bold text-[var(--text-primary)]">
            {{ activeView === 'queue' ? '播放队列' : '喜欢的歌' }}
          </h2>
          <p class="text-[11px] font-bold text-[var(--text-tertiary)] uppercase tracking-wider mt-0.5">
            {{ activeView === 'queue' ? `${player.queue.length} Tracks` : `${player.likedSongs.length} Favorites` }}
          </p>
        </div>
        <IconButton v-if="activeView === 'liked'" variant="primary" @click="exportLikedSongs" :disabled="player.likedSongs.length === 0" title="导出">
          <Download class="h-4 w-4" />
        </IconButton>
      </div>

      <SegmentedControl
        v-model="activeView"
        :options="[
          { label: '队列', value: 'queue' },
          { label: '喜欢', value: 'liked' }
        ]"
      />
    </div>

    <div class="flex-1 overflow-y-auto px-3 py-3">
      <!-- Queue View -->
      <div v-if="activeView === 'queue'">
        <div v-if="player.queue.length === 0" class="py-20 text-center text-sm text-[var(--text-tertiary)]">
          队列为空
        </div>

        <div v-else class="space-y-1">
          <TrackListItem
            v-for="(item, index) in player.queue"
            :key="item.queueId"
            :title="item.music.name"
            :artist="item.music.artists.join(' / ')"
            :cover-url="item.music.coverUrl"
            :active="isSelected(item.queueId)"
            @click="handleQueueItemClick(item.queueId)"
            @pointerdown="startLongPress(item.queueId)"
            @pointerup="clearLongPress"
            @pointercancel="clearLongPress"
          >
            <template #prefix>
              <div class="flex w-8 flex-shrink-0 items-center justify-center">
                <div v-if="selectionMode" class="h-5 w-5 rounded-full border flex items-center justify-center transition-colors"
                  :class="isSelected(item.queueId) ? 'border-[var(--accent)] bg-[var(--accent)] text-[var(--text-inverse)]' : 'border-[var(--border-default)]'"
                >
                  <Check v-if="isSelected(item.queueId)" class="h-3 w-3" />
                </div>
                <span v-else class="font-mono text-[10px] text-[var(--text-tertiary)]">{{ index + 1 }}</span>
              </div>
            </template>
            <template #suffix>
              <div v-if="!user.isGuest && !selectionMode" class="flex items-center gap-1">
                <IconButton size="sm" @click.stop="player.topSong(item.queueId)"><ArrowUpToLine class="h-3.5 w-3.5" /></IconButton>
                <IconButton size="sm" @click.stop="player.removeSong(item.queueId)"><Trash2 class="h-3.5 w-3.5 text-[var(--error)]" /></IconButton>
              </div>
            </template>
          </TrackListItem>
        </div>
      </div>

      <!-- Liked View -->
      <div v-else>
        <div v-if="player.likedSongs.length === 0" class="py-20 text-center text-sm text-[var(--text-tertiary)] px-8 leading-relaxed">
          还没有喜欢的歌。播放页右侧红心会把当前歌曲保存到这里。
        </div>

        <div v-else class="space-y-1">
          <TrackListItem
            v-for="song in player.likedSongs"
            :key="song.key"
            :title="song.name"
            :artist="song.artists.join(' / ')"
            :cover-url="song.coverUrl"
          >
            <template #suffix>
              <IconButton size="sm" @click="player.removeLikedSong(song.key)">
                <Trash2 class="h-3.5 w-3.5 text-[var(--error)]" />
              </IconButton>
            </template>
          </TrackListItem>
        </div>
      </div>
    </div>

    <!-- Selection ActionBar -->
    <Transition name="action-bar">
      <div v-if="selectionMode" class="px-4 py-3 bg-[var(--surface-2)] border-t border-[var(--border-default)] safe-area-bottom">
        <div v-if="pendingDelete" class="mb-3 p-3 rounded-[var(--radius-md)] bg-[var(--error-soft-bg)]/10 border border-[var(--error)]/20">
          <div class="text-xs font-bold text-[var(--error-soft-text)] mb-3 text-center">确认从队列中删除 {{ selectedCount }} 首歌曲？</div>
          <div class="flex gap-2">
            <button class="flex-1 h-10 bg-[var(--error)] text-[var(--text-inverse)] text-xs font-bold rounded-[var(--radius-sm)]" @click="confirmDeleteSelected">确认删除</button>
            <button class="flex-1 h-10 bg-[var(--surface-3)] text-[var(--text-primary)] text-xs font-bold rounded-[var(--radius-sm)]" @click="pendingDelete = false">取消</button>
          </div>
        </div>
        <div v-else class="flex items-center justify-between">
          <div class="flex flex-col">
            <span class="text-sm font-bold text-[var(--text-primary)]">已选择 {{ selectedCount }}</span>
            <button class="text-[10px] font-bold text-[var(--accent)] uppercase tracking-wider text-left" @click="selectAll">全选所有</button>
          </div>
          <div class="flex items-center gap-2">
            <IconButton variant="secondary" @click="topSelected" :disabled="!hasSelection" title="置顶"><ArrowUpToLine class="h-5 w-5" /></IconButton>
            <IconButton variant="secondary" @click="requestDeleteSelected" :disabled="!hasSelection" title="删除"><Trash2 class="h-5 w-5 text-[var(--error)]" /></IconButton>
            <IconButton variant="primary" @click="cancelSelection" title="关闭"><X class="h-5 w-5" /></IconButton>
          </div>
        </div>
      </div>
    </Transition>
  </section>
</template>

<script setup>
import { computed, ref } from 'vue';
import { ArrowUpToLine, Check, Download, Trash2, X } from 'lucide-vue-next';
import { usePlayerStore } from '../../stores/player';
import { useUserStore } from '../../stores/user';
import { createLikedSongsFilename, createLikedSongsText } from '../../utils/likedSongs';
import { useQueueSelection } from '../../composables/useQueueSelection';

// UI Primitives
import SegmentedControl from '../ui/SegmentedControl.vue';
import TrackListItem from '../ui/TrackListItem.vue';
import IconButton from '../ui/IconButton.vue';

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
  const ids = selectedIds.value;
  if (!player.topSongs(ids)) player.topSongsCompat(ids);
  cancelSelection();
};

const requestDeleteSelected = () => {
  if (!hasSelection.value) return;
  pendingDelete.value = true;
};

const confirmDeleteSelected = () => {
  if (!hasSelection.value) return;
  const ids = selectedIds.value;
  if (!player.removeSongs(ids)) player.removeSongsCompat(ids);
  cancelSelection();
};
</script>

<style scoped>
.action-bar-enter-active,
.action-bar-leave-active {
  transition: transform 0.2s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.15s ease;
}

.action-bar-enter-from,
.action-bar-leave-to {
  transform: translateY(100%);
  opacity: 0;
}

.safe-area-bottom {
  padding-bottom: calc(env(safe-area-inset-bottom) + 12px);
}
</style>

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
  top: 64%;
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
