<template>
  <TrackListItem
    :title="item.music.name"
    :artist="artistLine"
    :cover-url="item.music.coverUrl"
    :active="selected"
    :class="selectionMode ? 'cursor-pointer' : ''"
    @click="handleRowClick"
  >
    <template #prefix>
      <div v-if="selectionMode" class="flex h-5 w-5 items-center justify-center rounded-full border transition-colors" :class="selected ? 'border-primary bg-primary text-on-primary' : 'border-border-default bg-surface-raised text-transparent'" @click.stop="emit('toggle-select')">
        <span class="material-symbols-outlined text-[12px]">check</span>
      </div>
      <div v-else-if="!userStore.isGuest" class="drag-handle cursor-grab active:cursor-grabbing text-text-muted hover:text-text-primary p-1">
        <span class="material-symbols-outlined text-[18px]">drag_indicator</span>
      </div>
    </template>

    <template #meta>
      <!-- Meta fallback is duration, but maybe we just show status or index -->
      <span v-if="index !== undefined">{{ String(index + 1).padStart(2, '0') }}</span>
      <span v-if="item.status === 'DOWNLOADING' || item.status === 'PENDING'" class="text-accent animate-pulse ml-2">{{ t('queue.loading') }}</span>
      <span v-if="item.status === 'FAILED'" class="text-error ml-2">{{ t('queue.failed') }}</span>
    </template>

    <template #suffix>
      <button v-if="!userStore.isGuest && !selectionMode" @click.stop="player.topSong(item.queueId)" class="hover:text-primary transition-colors" :title="t('queue.playNext')" :aria-label="t('queue.playNext')">
        <span class="material-symbols-outlined text-[20px]">keyboard_double_arrow_up</span>
      </button>
      <button v-if="!userStore.isGuest && !selectionMode" @click.stop="player.removeSong(item.queueId)" class="text-error hover:text-red-400 transition-colors" :title="t('queue.remove')" :aria-label="t('queue.remove')">
        <span class="material-symbols-outlined text-[20px]">delete</span>
      </button>
    </template>
  </TrackListItem>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { usePlayerStore } from '../stores/player';
import { useUserStore } from '../stores/user';
import TrackListItem from './ui/TrackListItem.vue';

const props = defineProps({
  item: {
    type: Object,
    required: true
  },
  index: {
    type: Number,
    default: undefined
  },
  selectionMode: {
    type: Boolean,
    default: false
  },
  selected: {
    type: Boolean,
    default: false
  }
});

const emit = defineEmits(['toggle-select']);

const { t } = useI18n();
const player = usePlayerStore();
const userStore = useUserStore();
const artistLine = computed(() => Array.isArray(props.item.music?.artists) && props.item.music.artists.length
  ? props.item.music.artists.join(' / ')
  : t('common.unknownArtist'));

const handleRowClick = () => {
  if (props.selectionMode) {
    emit('toggle-select');
  }
};
</script>
