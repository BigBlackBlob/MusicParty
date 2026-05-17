<template>
  <section class="flex flex-col h-full bg-bg-base relative overflow-hidden">
    <!-- Header / Tab View -->
    <header class="fixed top-0 w-full z-40 bg-surface-panel border-b border-border-default pt-md px-md safe-area-top">
      <div class="flex justify-between items-center mb-md">
        <h1 class="font-title text-title text-primary">{{ roomStore.currentRoom?.name || 'Lounge' }}</h1>
        <div class="flex items-center">
          <button 
            v-if="activeView === 'queue' && player.queue.length > 0"
            @click="toggleSelectionMode"
            class="w-[44px] h-[44px] flex items-center justify-center rounded-full hover:bg-surface-raised transition-colors"
          >
            <span class="material-symbols-outlined text-text-secondary">{{ selectionMode ? 'close' : 'checklist' }}</span>
          </button>
          <button 
            v-if="activeView === 'liked'"
            @click="exportLikedSongs"
            :disabled="player.likedSongs.length === 0"
            class="w-[44px] h-[44px] flex items-center justify-center rounded-full hover:bg-surface-raised transition-colors disabled:opacity-30"
          >
            <span class="material-symbols-outlined text-text-secondary">download</span>
          </button>
          <button class="w-[44px] h-[44px] flex items-center justify-center rounded-full hover:bg-surface-raised transition-colors">
            <span class="material-symbols-outlined text-text-secondary">search</span>
          </button>
        </div>
      </div>
      <!-- Segmented Control -->
      <div class="flex p-xs bg-bg-base rounded-lg mb-md">
        <button 
          @click="activeView = 'queue'"
          class="flex-1 py-sm text-center rounded-md font-section-label text-section-label transition-all"
          :class="activeView === 'queue' ? 'bg-surface-raised text-primary shadow-sm' : 'text-text-secondary hover:text-text-primary'"
        >
          {{ t('nav.queue').toUpperCase() }}
        </button>
        <button 
          @click="activeView = 'liked'"
          class="flex-1 py-sm text-center rounded-md font-section-label text-section-label transition-all"
          :class="activeView === 'liked' ? 'bg-surface-raised text-primary shadow-sm' : 'text-text-secondary hover:text-text-primary'"
        >
          {{ t('queue.liked').toUpperCase() }}
        </button>
      </div>
    </header>

    <!-- Main Content Area -->
    <main class="flex-1 overflow-y-auto pt-[140px] px-md pb-[92px] safe-area-bottom" :class="{ 'pb-[160px]': selectionMode }">
      <template v-if="activeView === 'queue'">
        <div v-if="player.queue.length === 0" class="flex flex-col items-center justify-center py-20 text-center opacity-40">
          <span class="material-symbols-outlined text-[48px] mb-2">queue_music</span>
          <p class="font-compact text-compact uppercase tracking-widest">{{ t('queue.empty') }}</p>
        </div>

        <div v-else ref="queueListRef" class="flex flex-col gap-xs">
          <div
            v-for="(item, index) in player.queue"
            :key="item.queueId || `${item.music?.platform}:${item.music?.id}:${index}`"
            class="flex items-center p-sm rounded-xl transition-colors group cursor-pointer"
            :class="[
              isSelected(item.queueId) ? 'bg-accent-subtle' : 'hover:bg-surface-raised',
              player.nowPlaying?.music?.id === item.music?.id ? 'border border-primary/20' : ''
            ]"
            @click="handleQueueItemClick(item.queueId)"
            @pointerdown="startLongPress(item.queueId)"
            @pointerup="clearLongPress"
            @pointerleave="clearLongPress"
            @pointercancel="clearLongPress"
          >
            <div class="relative w-[44px] h-[44px] rounded-md overflow-hidden flex-shrink-0 mr-sm">
              <CoverImage :src="item.music?.coverUrl" class="w-full h-full object-cover" />
              <div v-if="selectionMode" class="absolute inset-0 bg-black/40 flex items-center justify-center">
                <div class="w-5 h-5 rounded-full border border-white flex items-center justify-center" :class="{ 'bg-primary border-primary': isSelected(item.queueId) }">
                  <span v-if="isSelected(item.queueId)" class="material-symbols-outlined text-on-primary text-[14px]">check</span>
                </div>
              </div>
              <div v-else-if="!user.isGuest" class="mobile-drag-handle absolute inset-0 bg-black/20 opacity-0 group-hover:opacity-100 flex items-center justify-center transition-opacity">
                <span class="material-symbols-outlined text-white">drag_indicator</span>
              </div>
            </div>
            <div class="flex-1 min-w-0">
              <p class="font-compact text-compact truncate font-semibold" :class="isSelected(item.queueId) ? 'text-primary' : 'text-text-primary'">
                {{ item.music?.name }}
              </p>
              <p class="font-caption text-caption truncate" :class="isSelected(item.queueId) ? 'text-primary opacity-80' : 'text-text-secondary'">
                {{ formatArtists(item.music?.artists) }}
              </p>
            </div>
            <div class="flex items-center opacity-0 group-hover:opacity-100 transition-opacity">
              <button 
                v-if="!selectionMode && !user.isGuest"
                @click.stop="player.topSong(item.queueId)"
                class="w-[44px] h-[44px] flex items-center justify-center rounded-full hover:bg-surface-raised transition-colors text-text-secondary hover:text-primary"
              >
                <span class="material-symbols-outlined text-[20px]">arrow_upward</span>
              </button>
              <button 
                v-if="!selectionMode && !user.isGuest"
                @click.stop="player.removeSong(item.queueId)"
                class="w-[44px] h-[44px] flex items-center justify-center rounded-full hover:bg-surface-raised transition-colors text-text-secondary hover:text-error"
              >
                <span class="material-symbols-outlined text-[20px]">delete</span>
              </button>
            </div>
          </div>
        </div>
      </template>

      <template v-else>
        <div v-if="player.likedSongs.length === 0" class="flex flex-col items-center justify-center py-20 text-center opacity-40">
          <span class="material-symbols-outlined text-[48px] mb-2">favorite</span>
          <p class="font-compact text-compact uppercase tracking-widest">{{ t('queue.noLiked') }}</p>
        </div>

        <div v-else class="flex flex-col gap-xs">
          <div
            v-for="song in player.likedSongs"
            :key="song.key"
            class="flex items-center p-sm rounded-xl hover:bg-surface-raised transition-colors group cursor-pointer"
          >
            <div class="relative w-[44px] h-[44px] rounded-md overflow-hidden flex-shrink-0 mr-sm">
              <CoverImage :src="song.coverUrl" class="w-full h-full object-cover" />
            </div>
            <div class="flex-1 min-w-0">
              <p class="font-compact text-compact text-text-primary truncate font-semibold">{{ song.name }}</p>
              <p class="font-caption text-caption text-text-secondary truncate">{{ formatArtists(song.artists) }}</p>
            </div>
            <button 
              @click="player.removeLikedSong(song.key)"
              class="w-[44px] h-[44px] flex items-center justify-center rounded-full hover:bg-surface-raised transition-colors text-text-secondary hover:text-error opacity-0 group-hover:opacity-100"
            >
              <span class="material-symbols-outlined text-[20px]">delete</span>
            </button>
          </div>
        </div>
      </template>
    </main>

    <!-- Action Bar for Selection Mode -->
    <Transition name="action-bar">
      <div v-if="selectionMode" class="fixed bottom-[84px] inset-x-md z-40 bg-surface-panel border border-border-default rounded-2xl p-md shadow-2xl backdrop-blur-xl">
        <div v-if="pendingDelete" class="flex flex-col gap-3">
          <p class="text-sm font-bold text-error text-center">{{ t('queue.deleteConfirm', { count: selectedCount }) }}</p>
          <div class="grid grid-cols-2 gap-2">
            <button @click="confirmDeleteSelected" class="py-sm rounded-lg bg-error text-white font-bold text-sm">{{ t('queue.remove') }}</button>
            <button @click="pendingDelete = false" class="py-sm rounded-lg bg-surface-raised text-text-primary font-bold text-sm">{{ t('queue.cancel') }}</button>
          </div>
        </div>
        <div v-else class="flex items-center justify-between">
          <div class="flex flex-col">
            <span class="text-xs font-bold text-primary">{{ selectedCount }} {{ t('queue.selected') }}</span>
            <button @click="selectAll" class="text-[10px] text-text-secondary uppercase tracking-widest text-left">{{ t('queue.all') }}</button>
          </div>
          <div class="flex items-center gap-1">
            <button @click="topSelected" :disabled="!hasSelection" class="w-[40px] h-[40px] flex items-center justify-center rounded-full bg-surface-raised text-primary disabled:opacity-30">
              <span class="material-symbols-outlined">arrow_upward</span>
            </button>
            <button @click="requestDeleteSelected" :disabled="!hasSelection" class="w-[40px] h-[40px] flex items-center justify-center rounded-full bg-surface-raised text-error disabled:opacity-30">
              <span class="material-symbols-outlined">delete</span>
            </button>
            <button @click="cancelSelection" class="w-[40px] h-[40px] flex items-center justify-center rounded-full bg-primary text-on-primary">
              <span class="material-symbols-outlined">close</span>
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </section>
</template>

<script setup>
import { computed, ref, onMounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import Sortable from 'sortablejs';
import { usePlayerStore } from '../../stores/player';
import { useUserStore } from '../../stores/user';
import { useRoomStore } from '../../stores/room';
import { createLikedSongsFilename, createLikedSongsText } from '../../utils/likedSongs';
import { useQueueSelection } from '../../composables/useQueueSelection';
import CoverImage from '../CoverImage.vue';

const { t } = useI18n();
const player = usePlayerStore();
const user = useUserStore();
const roomStore = useRoomStore();
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
    delay: 100,
    onEnd: (evt) => {
      if (evt.oldIndex !== evt.newIndex) {
        const moved = queue.value[evt.oldIndex];
        const target = queue.value[evt.newIndex];
        if (moved?.queueId && target?.queueId) {
          player.reorderQueue(evt.oldIndex, evt.newIndex, moved.queueId, target.queueId, 'before');
        } else {
          player.reorderQueue(evt.oldIndex, evt.newIndex);
        }
      }
    }
  });
};

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
.safe-area-top {
  padding-top: calc(env(safe-area-inset-top) + 16px);
}
.safe-area-bottom {
  padding-bottom: calc(env(safe-area-inset-bottom) + 92px);
}
.action-bar-enter-active,
.action-bar-leave-active {
  transition: transform 300ms cubic-bezier(0.16, 1, 0.3, 1), opacity 200ms ease;
}
.action-bar-enter-from,
.action-bar-leave-to {
  opacity: 0;
  transform: translateY(20px) scale(0.95);
}
</style>
