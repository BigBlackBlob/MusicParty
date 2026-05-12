<template>
  <div
      class="group relative mb-2 flex h-16 items-center gap-3 rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)] px-3 py-2 transition-colors hover:bg-[var(--surface-3)]"
  >
    <div class="flex w-7 flex-shrink-0 flex-col items-center justify-center">
      <div v-if="index !== undefined" class="font-mono text-xs text-[var(--text-tertiary)]">
        {{ String(index + 1).padStart(2, '0') }}
      </div>
      <div v-else class="font-mono text-xs text-[var(--text-tertiary)]">#</div>
      <div v-if="item.queueId.startsWith('TOP-')" class="mt-1 h-2 w-2 rounded-full bg-[var(--accent)]" title="全局置顶"></div>
      <div v-else-if="item.queueId.startsWith('USERTOP-')" class="mt-1 h-2.5 w-2.5 rounded-full bg-[var(--accent)]/90" title="个人置顶"></div>
    </div>

    <div class="h-10 w-10 flex-shrink-0 overflow-hidden rounded-xl bg-[var(--surface-3)]">
      <CoverImage
          :src="item.music.coverUrl"
          :alt="`${item.music.name} 封面`"
          loading="lazy"
          decoding="async"
          class="h-full w-full"
      />
    </div>

    <div class="min-w-0 flex-1">
      <div class="truncate text-sm font-semibold text-[var(--text-primary)]">{{ item.music.name }}</div>
      <div class="mt-0.5 flex min-w-0 items-center gap-2">
        <div v-if="!item.status || item.status === 'READY'" class="min-w-0 flex-1 truncate text-xs text-[var(--text-secondary)]">
          {{ item.music.artists.join(' / ') }}
        </div>

        <div v-else-if="item.status === 'DOWNLOADING' || item.status === 'PENDING'" class="min-w-0 flex-1 truncate text-xs font-semibold text-[var(--accent)]">
          <Loader2 class="h-3.5 w-3.5 animate-spin" /> 加载中...
        </div>

        <div v-else-if="item.status === 'FAILED'" class="min-w-0 flex-1 truncate text-xs font-semibold text-[var(--error)]">
          下载失败
        </div>

        <div class="ml-auto max-w-[7rem] flex-shrink-0 truncate rounded-full border border-[var(--border-default)] bg-[var(--surface-3)] px-2 py-0.5 text-[10px] text-[var(--text-tertiary)]">
          {{ userStore.resolveName(item.enqueuedBy.token, item.enqueuedBy.name) }}
        </div>
      </div>
    </div>

    <div
        v-if="!userStore.isGuest"
        class="pointer-events-none absolute right-2 top-1/2 flex -translate-y-1/2 items-center gap-1 rounded-full border border-[var(--border-default)] bg-[var(--surface-3)]/95 px-1.5 py-1 opacity-0 shadow-lg backdrop-blur-sm transition-opacity group-hover:pointer-events-auto group-hover:opacity-100"
    >
      <button @click="player.topSong(item.queueId)" title="Top" class="min-w-[44px] min-h-[44px] rounded-full p-1.5 text-[var(--text-tertiary)] transition-colors hover:text-[var(--accent)] active:scale-[0.96]" aria-label="置顶">
        <ArrowUpToLine class="h-4 w-4" />
      </button>
      <button @click="player.removeSong(item.queueId)" title="Remove" class="min-w-[44px] min-h-[44px] rounded-full p-1.5 text-[var(--text-tertiary)] transition-colors hover:text-[var(--error)] active:scale-[0.96]" aria-label="移除">
        <Trash2 class="h-4 w-4" />
      </button>
    </div>
  </div>
</template>

<script setup>
import { usePlayerStore } from '../stores/player';
import { useUserStore } from '../stores/user';
import { Trash2, ArrowUpToLine, Loader2 } from 'lucide-vue-next';
import CoverImage from './CoverImage.vue';

const props = defineProps({
  item: {
    type: Object,
    required: true
  },
  index: {
    type: Number,
    default: undefined
  }
});

const player = usePlayerStore();
const userStore = useUserStore();
</script>
