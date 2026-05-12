<template>
  <section class="flex h-full flex-col overflow-y-auto px-4 py-4">
    <div
      class="flex flex-1 flex-col items-center"
      :class="hasLyrics ? 'justify-start gap-0 pt-4' : 'justify-center gap-5 pb-2'"
    >
      <div
        class="relative aspect-square w-full overflow-hidden rounded-[2rem] bg-[var(--surface-3)] shadow-2xl transition-all"
        :class="hasLyrics ? 'max-w-[220px]' : 'max-w-[280px]'"
      >
        <CoverImage
          :src="music?.coverUrl"
          :alt="music ? `${music.name} 封面` : '歌曲封面'"
          loading="eager"
          decoding="async"
          class="h-full w-full"
        />
        <div class="absolute inset-0 bg-[linear-gradient(180deg,transparent_48%,rgba(0,0,0,0.38))]"></div>
      </div>

      <div
        class="w-full text-left"
        :class="hasLyrics ? 'mt-9' : 'mt-1'"
      >
        <MobileMiniLyrics
          v-if="hasLyrics"
          class="mb-2"
          :lyrics="player.lyricText"
          :current-time-ms="player.localProgress"
          @open="lyricsExpanded = true"
        />
        <div class="mb-2 text-[11px] font-semibold tracking-[0.18em] text-[var(--accent)]">
          {{ player.isLoading ? '同步中' : nowPlaying ? '正在播放' : '等待播放' }}
        </div>
        <h1 class="max-w-[22rem] text-2xl font-bold leading-tight text-[var(--text-primary)]">
          {{ music?.name || '系统待机' }}
        </h1>
        <p class="mt-2 max-w-[22rem] text-sm leading-relaxed text-[var(--text-secondary)]">
          {{ music?.artists?.join(' / ') || '暂无播放内容' }}
        </p>
        <p v-if="nowPlaying?.enqueuedByName" class="mt-2 text-xs text-[var(--text-tertiary)]">
          来自 {{ nowPlaying.enqueuedByName }}
        </p>
      </div>

    </div>

    <div class="mt-4 flex-shrink-0 rounded-3xl border border-[var(--border-default)] bg-[var(--surface-4)] p-4 shadow-xl">
      <div class="mb-2 flex justify-between font-mono text-[11px] text-[var(--text-tertiary)]">
        <span>{{ formatDuration(player.localProgress) }}</span>
        <span>{{ formatDuration(music?.duration || 0) }}</span>
      </div>
      <div
        ref="progressTrackRef"
        class="relative flex h-7 touch-none items-center"
        :class="canSeek ? 'cursor-pointer' : 'opacity-70'"
        role="slider"
        tabindex="0"
        :aria-valuemin="0"
        :aria-valuemax="music?.duration || 0"
        :aria-valuenow="Math.round(displayProgressMs)"
        :aria-disabled="!canSeek"
        :aria-label="canSeek ? '拖拽调整播放进度' : '只有点播者可以调整进度'"
        @pointerdown="handleProgressPointerDown"
        @pointercancel="handleProgressPointerCancel"
      >
        <div class="h-2 w-full overflow-hidden rounded-full bg-[var(--progress-track)]">
          <div class="h-full rounded-full bg-[var(--accent)]" :style="{ width: displayProgressPercent + '%' }"></div>
        </div>
        <div
          class="absolute top-1/2 h-4 w-4 -translate-y-1/2 rounded-full bg-[var(--accent)] shadow-lg"
          :class="{ 'opacity-0': !canSeek }"
          :style="{ left: `calc(${displayProgressPercent}% - 0.5rem)` }"
        ></div>
      </div>

      <div class="mt-5 flex items-center justify-center gap-4">
        <button class="mobile-control" :class="player.isShuffle ? 'text-[var(--accent)]' : ''" @click="player.toggleShuffle" aria-label="随机播放">
          <Shuffle class="h-5 w-5" />
        </button>
        <button class="mobile-primary-control" @click="player.togglePause" aria-label="播放或暂停">
          <Play v-if="player.isPaused" class="h-6 w-6 fill-current" />
          <Pause v-else class="h-6 w-6 fill-current" />
        </button>
        <button class="mobile-control" @click="player.playNext" aria-label="下一首">
          <SkipForward class="h-5 w-5" />
        </button>
      </div>
    </div>

    <div v-if="lyricsExpanded" class="fixed inset-0 z-[var(--z-modal)] bg-[var(--surface-0)] p-4 pt-[calc(env(safe-area-inset-top)+1rem)]" @click.self="lyricsExpanded = false">
      <div class="mb-4 flex items-center justify-between">
        <div>
          <div class="text-sm font-bold">歌词</div>
          <div class="text-xs text-[var(--text-tertiary)]">{{ music?.name || '' }}</div>
        </div>
        <button class="min-h-[44px] min-w-[44px] rounded-xl bg-[var(--surface-3)] text-[var(--text-secondary)]" @click="lyricsExpanded = false" aria-label="关闭歌词">
          关闭
        </button>
      </div>
      <AppleLyricsPanel
        :lyrics="player.lyricText"
        :current-time="player.localProgress / 1000"
        :is-playing="!player.isPaused"
        :is-dark-mode="ui.isDarkMode"
        :bg-color="ui.dynamicAccent?.accent || ''"
      />
    </div>
  </section>
</template>

<script setup>
import { computed, onUnmounted, ref, watch } from 'vue';
import { Pause, Play, Shuffle, SkipForward } from 'lucide-vue-next';
import { usePlayerStore } from '../../stores/player';
import { useUiStore } from '../../stores/ui';
import { useUserStore } from '../../stores/user';
import { useToast } from '../../composables/useToast';
import { formatDuration } from '../../utils/format';
import { parseLyrics } from '../../utils/parser';
import AppleLyricsPanel from '../AppleLyricsPanel.vue';
import CoverImage from '../CoverImage.vue';
import MobileMiniLyrics from './MobileMiniLyrics.vue';

defineEmits(['open-search']);

const player = usePlayerStore();
const ui = useUiStore();
const user = useUserStore();
const { error } = useToast();
const nowPlaying = computed(() => player.nowPlaying);
const music = computed(() => nowPlaying.value?.music);
const progressTrackRef = ref(null);
const lyricsExpanded = ref(false);
const isDraggingProgress = ref(false);
const dragProgressMs = ref(0);
const activeProgressPointerId = ref(null);
const lyricLines = computed(() => parseLyrics(player.lyricText));
const hasLyrics = computed(() => lyricLines.value.length >= 5);
const canSeek = computed(() => !!nowPlaying.value && nowPlaying.value.enqueuedById === user.userToken);
const displayProgressMs = computed(() => isDraggingProgress.value ? dragProgressMs.value : player.localProgress);
const displayProgressPercent = computed(() => {
  if (!music.value?.duration) return 0;
  return Math.min(100, (displayProgressMs.value / music.value.duration) * 100);
});

const getProgressMsFromEvent = (e) => {
  if (!progressTrackRef.value || !music.value?.duration) return 0;
  const rect = progressTrackRef.value.getBoundingClientRect();
  const x = Math.max(0, Math.min(rect.width, e.clientX - rect.left));
  return Math.round((x / rect.width) * music.value.duration);
};

const handleProgressPointerDown = (e) => {
  if (!canSeek.value || !music.value) {
    if (music.value) error('只有点播者可以调整这首歌的进度');
    return;
  }
  e.preventDefault();
  isDraggingProgress.value = true;
  activeProgressPointerId.value = e.pointerId;
  dragProgressMs.value = getProgressMsFromEvent(e);
  progressTrackRef.value?.setPointerCapture?.(e.pointerId);
  player.setSeekingPreview?.(true);
  window.addEventListener('pointermove', handleProgressPointerMove);
  window.addEventListener('pointerup', handleProgressPointerUp);
  window.addEventListener('pointercancel', handleProgressPointerCancel);
};

const handleProgressPointerMove = (e) => {
  if (!isDraggingProgress.value) return;
  if (activeProgressPointerId.value !== null && e.pointerId !== activeProgressPointerId.value) return;
  dragProgressMs.value = getProgressMsFromEvent(e);
};

const cleanupProgressDrag = (e) => {
  const pointerId = activeProgressPointerId.value ?? e?.pointerId;
  if (pointerId !== undefined && progressTrackRef.value?.hasPointerCapture?.(pointerId)) {
    progressTrackRef.value.releasePointerCapture(pointerId);
  }
  activeProgressPointerId.value = null;
  isDraggingProgress.value = false;
  player.setSeekingPreview?.(false);
  window.removeEventListener('pointermove', handleProgressPointerMove);
  window.removeEventListener('pointerup', handleProgressPointerUp);
  window.removeEventListener('pointercancel', handleProgressPointerCancel);
};

const handleProgressPointerUp = (e) => {
  if (activeProgressPointerId.value !== null && e.pointerId !== activeProgressPointerId.value) return;
  if (isDraggingProgress.value) player.seek(dragProgressMs.value);
  cleanupProgressDrag(e);
};

const handleProgressPointerCancel = (e) => {
  if (activeProgressPointerId.value !== null && e.pointerId !== activeProgressPointerId.value) return;
  cleanupProgressDrag(e);
};

watch(() => music.value?.coverUrl, (coverUrl) => {
  ui.updateAccentFromCover(coverUrl);
}, { immediate: true });

onUnmounted(() => {
  window.removeEventListener('pointermove', handleProgressPointerMove);
  window.removeEventListener('pointerup', handleProgressPointerUp);
  window.removeEventListener('pointercancel', handleProgressPointerCancel);
});
</script>

<style scoped>
.mobile-control {
  display: inline-flex;
  min-width: 3rem;
  min-height: 3rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  border: 1px solid var(--border-default);
  background: var(--surface-2);
  color: var(--text-secondary);
}

.mobile-primary-control {
  display: inline-flex;
  min-width: 4rem;
  min-height: 4rem;
  align-items: center;
  justify-content: center;
  border-radius: 9999px;
  background: var(--accent);
  color: var(--text-inverse);
  box-shadow: 0 18px 40px color-mix(in srgb, var(--accent) 24%, transparent);
}
</style>
