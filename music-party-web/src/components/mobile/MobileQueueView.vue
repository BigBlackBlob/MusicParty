<template>
  <section class="mobile-work-page">
    <header class="mobile-work-header">
      <div class="mobile-work-header__top">
        <div class="min-w-0">
          <h2>{{ activeView === 'queue' ? t('queue.title') : t('queue.liked') }}</h2>
          <p>{{ activeCount }} {{ t('queue.tracks') }}</p>
        </div>
        <div class="flex items-center gap-2">
          <IconButton
            v-if="activeView === 'queue' && player.queue.length > 0"
            :variant="selectionMode ? 'secondary' : 'ghost'"
            size="sm"
            @click="toggleSelectionMode"
            :aria-label="t('queue.selectTracks')"
          >
            <X v-if="selectionMode" class="h-4 w-4" />
            <CheckSquare v-else class="h-4 w-4" />
          </IconButton>
          <IconButton
            v-if="activeView === 'liked'"
            variant="ghost"
            size="sm"
            @click="exportLikedSongs"
            :disabled="player.likedSongs.length === 0"
            :aria-label="t('queue.exportLiked')"
          >
            <Download class="h-4 w-4" />
          </IconButton>
        </div>
      </div>

      <SegmentedControl
        v-model="activeView"
        :options="[
          { label: t('nav.queue'), value: 'queue' },
          { label: t('queue.liked'), value: 'liked' }
        ]"
      />
    </header>

    <div class="mobile-work-list" :class="{ 'mobile-work-list--with-action-bar': selectionMode }">
      <template v-if="activeView === 'queue'">
        <div v-if="player.queue.length === 0" class="mobile-empty">
          <strong>{{ t('queue.empty') }}</strong>
          <span>{{ t('queue.emptyDesc') }}</span>
        </div>

        <div v-else ref="queueListRef" class="flex flex-col gap-[6px]">
          <TrackListItem
            v-for="(item, index) in player.queue"
            :key="item.queueId || `${item.music?.platform}:${item.music?.id}:${index}`"
            :title="item.music?.name"
            :artist="formatArtists(item.music?.artists)"
            :cover-url="item.music?.coverUrl"
            :active="isSelected(item.queueId)"
            clickable
            @click="handleQueueItemClick(item.queueId)"
            @activate="handleQueueItemClick(item.queueId)"
            @pointerdown="startLongPress(item.queueId)"
            @pointerup="clearLongPress"
            @pointerleave="clearLongPress"
            @pointercancel="clearLongPress"
          >
            <template #prefix>
              <div class="mobile-list-prefix">
                <div v-if="!selectionMode && !user.isGuest" class="mobile-drag-handle mr-2 text-text-muted">
                  <GripVertical class="h-4 w-4" />
                </div>
                <span v-if="!selectionMode">{{ index + 1 }}</span>
                <span
                  v-else
                  class="mobile-check"
                  :class="{ 'mobile-check--selected': isSelected(item.queueId) }"
                >
                  <Check v-if="isSelected(item.queueId)" class="h-3 w-3" />
                </span>
              </div>
            </template>
            <template #meta>
              {{ item.status || '' }}
            </template>
            <template #suffix>
              <div v-if="!selectionMode && !user.isGuest" class="flex items-center gap-1">
                <IconButton size="sm" variant="ghost" @click.stop="player.topSong(item.queueId)" :aria-label="t('queue.top')">
                  <ArrowUpToLine class="h-3.5 w-3.5" />
                </IconButton>
                <IconButton size="sm" variant="ghost" @click.stop="player.removeSong(item.queueId)" :aria-label="t('queue.remove')">
                  <Trash2 class="h-3.5 w-3.5 text-[var(--error)]" />
                </IconButton>
              </div>
            </template>
          </TrackListItem>
        </div>
      </template>

      <template v-else>
        <div v-if="player.likedSongs.length === 0" class="mobile-empty">
          <strong>{{ t('queue.noLiked') }}</strong>
          <span>{{ t('queue.likedDesc') }}</span>
        </div>

        <TrackListItem
          v-for="song in player.likedSongs"
          v-else
          :key="song.key"
          :title="song.name"
          :artist="formatArtists(song.artists)"
          :cover-url="song.coverUrl"
        >
          <template #meta>
            {{ song.platform }}
          </template>
          <template #suffix>
            <IconButton size="sm" variant="ghost" @click="player.removeLikedSong(song.key)" :aria-label="t('queue.remove')">
              <Trash2 class="h-3.5 w-3.5 text-[var(--error)]" />
            </IconButton>
          </template>
        </TrackListItem>
      </template>
    </div>

    <Transition name="action-bar">
      <div v-if="selectionMode" class="mobile-action-bar">
        <div v-if="pendingDelete" class="mobile-delete-confirm">
          <span>{{ t('queue.deleteConfirm', { count: selectedCount }) }}</span>
          <div class="grid grid-cols-2 gap-2">
            <button type="button" class="mobile-danger-action" @click="confirmDeleteSelected">{{ t('queue.remove') }}</button>
            <button type="button" class="mobile-secondary-action" @click="pendingDelete = false">{{ t('queue.cancel') }}</button>
          </div>
        </div>
        <template v-else>
          <div class="min-w-0">
            <strong>{{ t('queue.selected') }} {{ selectedCount }}</strong>
            <button type="button" @click="selectAll">{{ t('queue.all') }}</button>
          </div>
          <div class="flex items-center gap-2">
            <IconButton variant="secondary" @click="topSelected" :disabled="!hasSelection" :aria-label="t('queue.top')">
              <ArrowUpToLine class="h-5 w-5" />
            </IconButton>
            <IconButton variant="secondary" @click="requestDeleteSelected" :disabled="!hasSelection" :aria-label="t('queue.removeAll')">
              <Trash2 class="h-5 w-5 text-[var(--error)]" />
            </IconButton>
            <IconButton variant="primary" @click="cancelSelection" :aria-label="t('queue.cancel')">
              <X class="h-5 w-5" />
            </IconButton>
          </div>
        </template>
      </div>
    </Transition>
  </section>
</template>

<script setup>
import { computed, ref, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { ArrowUpToLine, Check, CheckSquare, Download, Trash2, X, GripVertical } from 'lucide-vue-next';
import Sortable from 'sortablejs';
import { usePlayerStore } from '../../stores/player';
import { useUserStore } from '../../stores/user';
import { createLikedSongsFilename, createLikedSongsText } from '../../utils/likedSongs';
import { useQueueSelection } from '../../composables/useQueueSelection';
import IconButton from '../ui/IconButton.vue';
import SegmentedControl from '../ui/SegmentedControl.vue';
import TrackListItem from '../ui/TrackListItem.vue';

const { t } = useI18n();
const player = usePlayerStore();
const user = useUserStore();
const activeView = ref('queue');
const queue = computed(() => player.queue);
const pendingDelete = ref(false);
const longPressTimer = ref(null);
const longPressTriggered = ref(false);
const LONG_PRESS_MS = 450;

const queueListRef = ref(null);
let sortableInstance = null;

const {
  selectionMode,
  selectedCount,
  selectedIds,
  hasSelection,
  enterSelectionMode,
  exitSelectionMode,
  toggleSelectionMode,
  toggleSelected,
  isSelected,
  selectAll
} = useQueueSelection(queue);

onMounted(() => {
  initSortable();
});

watch([activeView, selectionMode, queueListRef], () => {
  if (activeView.value === 'queue' && !selectionMode.value) {
    if (!sortableInstance) initSortable();
  } else {
    if (sortableInstance) {
      sortableInstance.destroy();
      sortableInstance = null;
    }
  }
});

const initSortable = () => {
  if (!queueListRef.value) return;
  sortableInstance = new Sortable(queueListRef.value, {
    animation: 150,
    handle: '.mobile-drag-handle',
    ghostClass: 'opacity-40',
    delay: 100, // 给长按留一点空间
    onEnd: (evt) => {
      if (evt.oldIndex !== evt.newIndex) {
        player.reorderQueue(evt.oldIndex, evt.newIndex);
      }
    }
  });
};

const activeCount = computed(() => activeView.value === 'queue' ? player.queue.length : player.likedSongs.length);

const formatArtists = (artists) => Array.isArray(artists) && artists.length ? artists.join(' / ') : t('common.unknownArtist');

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

const startLongPress = (queueId) => {
  if (user.isGuest || selectionMode.value || !queueId) return;
  clearLongPress();
  longPressTriggered.value = false;
  longPressTimer.value = setTimeout(() => {
    longPressTriggered.value = true;
    enterSelectionMode(queueId);
  }, LONG_PRESS_MS);
};

const clearLongPress = () => {
  if (!longPressTimer.value) return;
  clearTimeout(longPressTimer.value);
  longPressTimer.value = null;
};

const handleQueueItemClick = (queueId) => {
  if (!queueId) return;
  if (longPressTriggered.value) {
    longPressTriggered.value = false;
    return;
  }
  if (selectionMode.value) toggleSelected(queueId);
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

watch(activeView, () => {
  cancelSelection();
});
</script>

<style scoped>
.mobile-work-page {
  position: relative;
  display: grid;
  height: 100%;
  min-height: 0;
  grid-template-rows: auto minmax(0, 1fr);
  overflow: hidden;
  background: transparent;
}

.mobile-work-header {
  display: flex;
  flex-direction: column;
  gap: 14px;
  border-bottom: 1px solid var(--surface-glass-border);
  background: var(--surface-glass-bg);
  backdrop-filter: blur(20px);
  padding: 14px 16px;
}

.mobile-work-header__top {
  display: flex;
  min-height: 36px;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.mobile-work-header h2 {
  color: var(--text-primary);
  font-size: 20px;
  font-weight: 800;
  line-height: 1.15;
}

.mobile-work-header p {
  margin-top: 2px;
  color: var(--text-tertiary);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
}

.mobile-work-list {
  display: flex;
  min-height: 0;
  flex-direction: column;
  gap: 6px;
  overflow-y: auto;
  padding: 10px 12px calc(12px + env(safe-area-inset-bottom));
}

.mobile-work-list--with-action-bar {
  padding-bottom: calc(112px + env(safe-area-inset-bottom));
}

.mobile-list-prefix {
  display: flex;
  width: 30px;
  justify-content: center;
  color: var(--text-tertiary);
  font-family: var(--font-mono, ui-monospace, monospace);
  font-size: 10px;
  font-weight: 700;
}

.mobile-check {
  display: inline-flex;
  width: 20px;
  height: 20px;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-full);
  color: var(--text-inverse);
}

.mobile-check--selected {
  border-color: var(--accent);
  background: var(--accent);
}

.mobile-empty {
  display: flex;
  min-height: 220px;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  color: var(--text-tertiary);
  text-align: center;
}

.mobile-empty strong {
  color: var(--text-primary);
  font-size: 14px;
}

.mobile-empty span {
  max-width: 220px;
  font-size: 12px;
  line-height: 1.5;
}

.mobile-action-bar {
  position: absolute;
  inset-inline: 0;
  bottom: 0;
  z-index: 3;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid var(--surface-glass-border);
  background: var(--surface-glass-panel);
  backdrop-filter: blur(24px);
  padding: 12px 16px;
}

.mobile-action-bar strong {
  display: block;
  color: var(--text-primary);
  font-size: 14px;
}

.mobile-action-bar button {
  color: var(--accent);
  font-size: 11px;
  font-weight: 700;
}

.mobile-delete-confirm {
  display: grid;
  width: 100%;
  gap: 10px;
}

.mobile-delete-confirm span {
  color: var(--error-soft-text);
  font-size: 13px;
  font-weight: 700;
  text-align: center;
}

.mobile-danger-action,
.mobile-secondary-action {
  min-height: 40px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 800;
}

.mobile-danger-action {
  background: var(--error);
  color: var(--text-inverse) !important;
}

.mobile-secondary-action {
  background: var(--surface-3);
  color: var(--text-primary) !important;
}

.action-bar-enter-active,
.action-bar-leave-active {
  transition: transform 180ms var(--motion-ease-out), opacity 140ms var(--motion-ease-out);
}

.action-bar-enter-from,
.action-bar-leave-to {
  opacity: 0;
  transform: translateY(100%);
}
</style>
