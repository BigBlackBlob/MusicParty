<template>
  <div class="flex h-full min-h-0 w-full flex-col overflow-hidden bg-transparent">
    <!-- Header -->
    <div class="flex h-12 w-full shrink-0 items-center justify-between gap-1 border-b border-border-subtle px-2">
      <div class="flex h-full min-w-0 flex-1 items-stretch gap-1">
        <button
          class="queue-tab relative flex shrink-0 items-center gap-1.5 whitespace-nowrap px-2 text-xs font-semibold transition-colors"
          :class="activeView === 'queue' ? 'text-text-primary' : 'text-text-tertiary hover:text-text-secondary'"
          @click="activeView = 'queue'"
        >
          {{ t('queue.upNext') }}
          <span class="text-[11px] font-normal text-[var(--accent)]">{{ queue.length > 99 ? '99+' : queue.length }}</span>
          <span v-if="activeView === 'queue'" class="absolute inset-x-2 bottom-0 h-[2px] bg-[var(--accent)]"></span>
        </button>
        <button
          class="queue-tab relative flex shrink-0 items-center gap-1.5 whitespace-nowrap px-2 text-xs font-semibold transition-colors"
          :class="activeView === 'liked' ? 'text-text-primary' : 'text-text-tertiary hover:text-text-secondary'"
          @click="activeView = 'liked'"
        >
          {{ t('queue.liked') }}
          <span class="text-[11px] font-normal text-[var(--accent)]">{{ liked.likedSongs.length > 99 ? '99+' : liked.likedSongs.length }}</span>
          <span v-if="activeView === 'liked'" class="absolute inset-x-2 bottom-0 h-[2px] bg-[var(--accent)]"></span>
        </button>
      </div>

      <div class="flex flex-shrink-0 items-center gap-1">
        <button
          v-if="activeView === 'queue' && queue.length > 0"
          @click="toggleSelectionMode"
          class="flex h-8 w-8 items-center justify-center rounded-md"
          :class="selectionMode ? 'bg-[var(--accent-subtle)] text-[var(--accent)]' : 'text-text-secondary hover:bg-[var(--surface-control-hover)] hover:text-text-primary'"
          :aria-label="t('queue.selectTracks')"
          :title="t('queue.selectTracks')"
          :aria-pressed="selectionMode"
        >
          <span class="material-symbols-outlined text-[16px]">checklist</span>
        </button>

        <button
          v-if="activeView === 'liked' && liked.likedSongs.length > 0"
          @click="exportLikedSongs"
          class="flex h-8 w-8 items-center justify-center rounded-md text-text-muted hover:bg-[var(--surface-control-hover)] hover:text-text-primary"
          :aria-label="t('common.export')"
          :title="t('common.export')"
        >
          <span class="material-symbols-outlined text-[16px]">download</span>
        </button>
        <button
          v-if="!layoutStore.isEditMode"
          @click="layoutStore.removeModuleById('queue')"
          class="flex h-8 w-8 items-center justify-center rounded-md text-text-muted hover:bg-[var(--surface-control-hover)] hover:text-text-primary"
          :aria-label="t('common.close')"
          :title="t('common.close')"
        >
          <span class="material-symbols-outlined text-[16px]">close</span>
        </button>
      </div>
    </div>

    <div v-if="selectionMode && activeView === 'queue'" class="mx-4 mt-4 mb-3 flex flex-shrink-0 items-center gap-2 rounded-md border border-border-subtle bg-[var(--surface-control)] px-2 py-2">
      <span class="min-w-0 flex-1 truncate text-xs text-text-muted">{{ selectedCount }} {{ t('queue.selected') }}</span>
      <button class="h-8 rounded-md px-2 text-xs text-text-secondary hover:bg-[var(--surface-control-hover)] hover:text-text-primary" @click="selectAll">{{ t('queue.all') }}</button>
      <button class="h-8 rounded-md px-2 text-xs text-text-secondary hover:bg-[var(--surface-control-hover)] hover:text-text-primary disabled:opacity-40" :disabled="!hasSelection" @click="batchTop">{{ t('queue.top') }}</button>
      <button class="h-8 rounded-md px-2 text-xs text-error hover:bg-[var(--surface-control-hover)] disabled:opacity-40" :disabled="!hasSelection" @click="batchRemove">{{ t('queue.remove') }}</button>
    </div>

    <div ref="queueScrollerRef" class="flex min-h-0 flex-1 flex-col gap-2 overflow-y-auto p-4" @scroll.passive="onScroll">

      <!-- Liked View -->
      <div v-if="activeView === 'liked'" class="flex flex-col gap-2">
        <div v-if="liked.likedSongs.length === 0" class="py-16 text-center">
          <span class="queue-empty__icon mx-auto mb-3 flex h-10 w-10 items-center justify-center rounded-full text-[20px]" aria-hidden="true"><span class="material-symbols-outlined">favorite</span></span>
          <div class="text-sm font-bold text-text-primary">{{ t('queue.noLiked') }}</div>
          <div class="mt-1 text-xs text-text-muted">{{ t('queue.likedDesc') }}</div>
        </div>

        <TrackListItem
          v-for="song in liked.likedSongs"
          :key="song.key"
          :title="song.name"
            :artist="formatArtists(song.artists)"
          :cover-url="song.coverUrl"
        >
          <template #meta>
            {{ song.platform }}
          </template>
          <template #suffix>
            <button @click="removeLikedSong(song.key)" class="text-error hover:text-red-400" :aria-label="t('queue.remove')" :title="t('queue.remove')">
              <span class="material-symbols-outlined text-[18px]">delete</span>
            </button>
          </template>
        </TrackListItem>
      </div>

      <!-- Queue View -->
      <div v-else class="flex min-h-0 flex-1 flex-col gap-2">
        <div v-if="queue.length === 0" class="flex min-h-0 flex-1 flex-col items-center justify-center text-center">
          <span class="queue-empty__icon flex h-10 w-10 items-center justify-center rounded-full text-[20px]" aria-hidden="true"><span class="material-symbols-outlined">queue_music</span></span>
          <div class="mt-3 text-sm font-bold text-text-primary">{{ t('queue.empty') }}</div>
          <div class="mt-1 max-w-[180px] text-xs leading-relaxed text-text-muted">{{ t('queue.emptyDesc') }}</div>
          <button
            v-if="openSearch"
            class="mt-3 rounded-md border border-[var(--border-accent)] bg-[var(--accent)] px-4 py-2 text-xs font-semibold text-[var(--on-primary)] transition-colors hover:bg-[var(--accent-hover)]"
            @click="openSearch()"
          >{{ t('search.searchAndAdd') }}</button>
        </div>

          <div v-else class="flex min-h-0 flex-1 flex-col gap-2">
            <div ref="queueListRef" class="flex flex-col gap-2">
              <QueueItem
              v-for="(item, index) in interactiveQueue"
              :key="item.queueId || `${item.music?.platform || 'track'}:${item.music?.id || index}`"
              :data-queue-id="item.queueId"
              :item="item"
              :index="index"
              :selection-mode="selectionMode"
              :selected="isSelected(item.queueId)"
              @toggle-select="toggleSelected(item.queueId)"
              />
            </div>
            <template v-if="virtualized">
              <div aria-hidden="true" :style="{ height: `${topPadding}px` }"></div>
              <QueueItem
                v-for="(item, index) in visibleQueue"
                :key="item.queueId || `${item.music?.platform || 'track'}:${item.music?.id || index}`"
                :item="item"
                :index="50 + Math.floor(topPadding / 72) + index"
                :selection-mode="selectionMode"
                :selected="isSelected(item.queueId)"
                @toggle-select="toggleSelected(item.queueId)"
              />
              <div aria-hidden="true" :style="{ height: `${bottomPadding}px` }"></div>
            </template>
        </div>
      </div>

    </div>
  </div>
</template>

<script setup>
import { computed, ref, inject, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import { useI18n } from 'vue-i18n';
import Sortable from 'sortablejs';
import { useRoomCommandStore } from '../domains/realtime/roomCommandStore';
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore';
import { useLikedSongsStore } from '../domains/playback/likedSongs';
import { useUserStore } from '../stores/user';
import { useLayoutStore } from '../stores/layout';
import QueueItem from './QueueItem.vue';
import TrackListItem from './ui/TrackListItem.vue';
import { createLikedSongsFilename, createLikedSongsText } from '../utils/likedSongs';
import { useQueueSelection } from '../composables/useQueueSelection';
import { useVirtualQueue } from '../composables/useVirtualQueue';
import { buildQueueReorderPayload, buildQueueReorderPayloadFromDom, isQueueReorderSourceCurrent, restoreDraggedNode } from '../utils/queueReorder';

const player = useRoomCommandStore();
const runtime = useRoomRuntimeStore();
const liked = useLikedSongsStore();
const user = useUserStore();
const layoutStore = useLayoutStore();
const openSearch = inject('openSearch', null);
const { t } = useI18n();
const queue = computed(() => runtime.queue);
const activeView = ref('queue');
const queueListRef = ref(null);
const queueScrollerRef = ref(null);
let sortableInstance = null;
let dragStartedInInteractiveZone = false;
let dragAnchor = null;
const { virtualized, interactiveQueue, visibleQueue, topPadding, bottomPadding, onScroll, observeScroller } = useVirtualQueue(queue);

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

onMounted(() => {
  observeScroller(queueScrollerRef.value);
  initSortable();
});

const dragDisabled = computed(() => selectionMode.value || !player.canOperate);

watch([activeView, dragDisabled, queueListRef], async () => {
  await nextTick();
  if (activeView.value === 'queue') {
    if (sortableInstance && sortableInstance.el !== queueListRef.value) destroySortable();
    if (!sortableInstance) {
      initSortable();
    } else {
      sortableInstance.option('disabled', dragDisabled.value);
    }
  } else {
    destroySortable();
  }
});

const destroySortable = () => {
  if (!sortableInstance) return;
  sortableInstance.destroy();
  sortableInstance = null;
};

const initSortable = () => {
  if (!queueListRef.value) return;
  sortableInstance = new Sortable(queueListRef.value, {
    animation: 150,
    ghostClass: 'opacity-40',
    disabled: dragDisabled.value,
    // The queue itself is the scroll container. Explicit values keep long
    // drags reliable when the list is nested in the panel's flex layout.
    scroll: queueScrollerRef.value,
    scrollSensitivity: 80,
    scrollSpeed: 20,
    // onChoose runs before Sortable inserts its hidden clone next to the item.
    onChoose: (evt) => { dragAnchor = evt.item.nextSibling; },
    onStart: (evt) => { dragStartedInInteractiveZone = evt.oldIndex < 50; },
    onMove: (evt) => dragStartedInInteractiveZone && [...queueListRef.value.children].indexOf(evt.related) < 50,
    onEnd: (evt) => {
      if (!dragStartedInInteractiveZone) {
        restoreDraggedNode(evt, dragAnchor);
        dragAnchor = null;
        return;
      }
      const payload = buildQueueReorderPayloadFromDom(evt)
        || (isQueueReorderSourceCurrent(queue.value, evt)
          ? buildQueueReorderPayload(queue.value, evt.oldIndex, evt.newIndex)
          : null);
      restoreDraggedNode(evt, dragAnchor);
      dragAnchor = null;
      if (payload && payload.oldIndex < 50 && payload.newIndex < 50) player.reorderQueue(payload.oldIndex, payload.newIndex, payload.queueId, payload.targetQueueId, payload.position);
      else if (evt.oldIndex !== evt.newIndex) player.requestResync('queue-reorder-stale-dom', true);
      dragStartedInInteractiveZone = false;
    }
  });
};

onBeforeUnmount(() => {
  destroySortable();
});

const batchTop = () => {
  const ids = selectedIds.value;
  if (!ids.length) return;
  player.topSongs(ids);
  exitSelectionMode();
};

const batchRemove = () => {
  const ids = selectedIds.value;
  if (!ids.length) return;
  player.removeSongs(ids);
  exitSelectionMode();
};

const exportLikedSongs = () => {
  if (!liked.likedSongs.length) return;
  const blob = new Blob([createLikedSongsText(liked.likedSongs)], { type: 'text/plain;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = createLikedSongsFilename();
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
};

const removeLikedSong = (key) => liked.removeAndSync(key, user.isGuest);

const formatArtists = (artists) => Array.isArray(artists) && artists.length ? artists.join(' / ') : t('common.unknownArtist');

</script>

<style scoped>
.queue-empty__icon {
  color: var(--accent);
  background: var(--accent-muted);
}
</style>
