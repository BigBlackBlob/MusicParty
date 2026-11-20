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
        <Check :size="12" :stroke-width="2.25" aria-hidden="true" />
      </div>
      <GripVertical
        v-else-if="player.canOperate && (index === undefined || index < 50)"
        :size="15"
        :stroke-width="1.75"
        class="-ml-1 -mr-1.5 shrink-0 text-text-tertiary opacity-0 transition-opacity group-hover:opacity-60 cursor-grab select-none"
        aria-hidden="true"
      />
    </template>

    <template #meta>
      <!-- Meta fallback is duration, but maybe we just show status or index -->
      <span v-if="index !== undefined">{{ String(index + 1).padStart(2, '0') }}</span>
      <span v-if="item.status === 'DOWNLOADING' || item.status === 'PENDING'" class="text-accent animate-pulse ml-2">{{ t('queue.loading') }}</span>
      <span v-if="item.status === 'FAILED'" class="text-error ml-2">{{ t('queue.failed') }}</span>
    </template>

    <template #suffix>
      <button
        v-if="!selectionMode"
        type="button"
        @click.stop="player.topSong(item.queueId)"
        class="inline-flex h-7 w-7 items-center justify-center rounded-full text-text-tertiary transition-colors hover:bg-[var(--surface-control-hover)] hover:text-primary active:scale-95"
        :title="t('queue.playNext')"
        :aria-label="t('queue.playNext')"
      >
        <ChevronsUp :size="18" :stroke-width="1.75" aria-hidden="true" />
      </button>
      <button
        v-if="!selectionMode"
        type="button"
        @click.stop="player.removeSong(item.queueId)"
        class="inline-flex h-7 w-7 items-center justify-center rounded-full text-text-tertiary transition-colors hover:bg-[var(--error-soft-bg)] hover:text-error active:scale-95"
        :title="t('queue.remove')"
        :aria-label="t('queue.remove')"
      >
        <Trash2 :size="17" :stroke-width="1.75" aria-hidden="true" />
      </button>
    </template>
  </TrackListItem>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { Check, ChevronsUp, GripVertical, Trash2 } from 'lucide-vue-next';
import { useRoomCommandStore } from '../domains/realtime/roomCommandStore';
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
const player = useRoomCommandStore();
const artistLine = computed(() => Array.isArray(props.item.music?.artists) && props.item.music.artists.length
  ? props.item.music.artists.join(' / ')
  : t('common.unknownArtist'));

const handleRowClick = () => {
  if (props.selectionMode) {
    emit('toggle-select');
  }
};

</script>
