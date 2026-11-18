<template>
  <div class="now-playing-layout flex h-full min-h-0 min-w-0 flex-col items-center overflow-hidden gap-5" :class="music ? 'is-playing' : 'is-empty'">
    <div
      v-if="music || player.isLoading || player.isBuffering || player.isErrorState"
      class="now-playing-cover-stage relative flex min-h-0 w-full items-center justify-center"
    >
      <div
        class="now-playing-cover relative aspect-square shrink-0 overflow-hidden rounded-xl bg-surface-raised"
        :class="player.isPaused ? 'saturate-[0.86]' : 'saturate-100'"
        @click="handleCoverClick"
      >
        <CoverImage
          :src="currentCover"
          :alt="trackTitle"
          class="h-full w-full"
          loading="eager"
        />

        <div v-if="player.isLoading || player.isBuffering" class="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-[var(--media-overlay)] text-text-primary backdrop-blur-xl">
          <div class="h-12 w-12 animate-spin rounded-full border-[3px] border-[var(--surface-control-active)] border-t-[var(--accent)]" />
          <span class="font-micro text-micro uppercase text-text-secondary">{{ player.isBuffering ? t('player.buffering') : t('player.loading') }}</span>
        </div>

        <div v-if="player.isErrorState" class="absolute inset-x-5 bottom-5 rounded-lg border border-error/30 bg-[var(--error-soft-bg)] px-4 py-3 backdrop-blur-md">
          <div class="font-micro text-micro uppercase text-[var(--error-soft-text)]">{{ t('player.playbackError') }}</div>
          <div class="mt-1 text-xs text-[var(--error-soft-text)] opacity-80">{{ t('player.loadError') }}</div>
        </div>
      </div>
    </div>

    <div v-else class="now-playing-cover-stage relative flex min-h-0 w-full items-center justify-center" aria-hidden="true"></div>

    <div
      class="player-console flex min-h-0 flex-col justify-end gap-1.5 bg-transparent pt-5"
    >
      <div v-if="music" class="shrink-0 text-center">
        <div class="mb-2 flex min-w-0 items-center justify-center gap-2">
          <span class="rounded bg-[var(--surface-control)] px-2 py-1 font-micro text-micro uppercase text-text-muted">{{ platformLabel }}</span>
          <span v-if="requesterName" class="min-w-0 max-w-[150px] truncate rounded bg-[var(--surface-control)] px-2 py-1 font-micro text-micro text-text-muted">{{ t('player.requestedBy', { name: requesterName }) }}</span>
        </div>
        <h1 class="track-title line-clamp-2 break-words font-extrabold" :title="trackTitle">{{ trackTitle }}</h1>
        <p class="mt-0.5 truncate font-title text-[14px] leading-tight text-primary/80" :title="artistLine">{{ artistLine }}</p>
      </div>

      <div class="player-controls flex shrink-0 flex-col gap-3">
        <div class="flex items-center justify-center gap-3">
          <div class="flex items-center justify-end gap-1 w-14">
            <span v-if="!canSeek && durationMs > 0" class="material-symbols-outlined text-[12px] text-text-muted opacity-50" :title="t('player.onlyRequester')">lock</span>
            <span class="text-right font-micro text-micro tabular-nums text-text-muted">{{ formatDuration(progressMs) }}</span>
          </div>
          <button
            class="h-4 flex-1 cursor-pointer rounded-full py-[6px]"
            :class="canSeek ? '' : 'cursor-not-allowed opacity-60'"
            @click="handleSeek"
            :aria-label="t('player.seek')"
            :title="canSeek ? t('player.seek') : t('player.onlyRequester')"
          >
            <span class="block relative h-1 overflow-hidden rounded-full bg-[var(--progress-track)]">
              <span v-if="bufferedPercent !== '0%'" class="block absolute inset-0 h-full rounded-full bg-[var(--progress-loaded)] pointer-events-none transition-[width] duration-200" :style="{ width: bufferedPercent }"></span>
              <span class="block relative h-full rounded-full bg-primary transition-[width] duration-200" :style="{ width: progressPercent }" />
            </span>
          </button>
          <span class="w-14 font-micro text-micro tabular-nums text-text-muted">{{ formatDuration(durationMs) }}</span>
        </div>

        <div class="player-transport flex w-full items-center justify-between gap-1">
          <button
            class="player-icon-button"
            :class="{ 'is-active': player.isShuffle }"
            :disabled="player.isShuffleLocked"
            @click="player.toggleShuffle"
            :aria-label="t('player.shuffle')"
            :title="t('player.shuffle')"
          >
            <Shuffle :stroke-width="1.75" aria-hidden="true" :title="playModeLabel" />
          </button>
          <button
            class="player-icon-button"
            :disabled="player.isSkipLocked || !player.canOperate || !player.historyCursor"
            @click="player.playPrevious"
            :aria-label="player.historyCursor ? t('player.prev') : t('player.prevUnavailable')"
            :title="player.historyCursor ? t('player.prev') : t('player.prevUnavailable')"
          >
            <SkipBack :stroke-width="1.75" aria-hidden="true" />
          </button>
          <button
            class="player-play-button"
            :disabled="player.isPauseLocked && !player.isPaused"
            @click="player.togglePause"
            :aria-label="player.isPaused ? t('player.play') : t('player.pause')"
            :title="player.isPaused ? t('player.play') : t('player.pause')"
          >
            <Play v-if="player.isPaused" :stroke-width="1.75" fill="currentColor" aria-hidden="true" />
            <Pause v-else :stroke-width="1.75" fill="currentColor" aria-hidden="true" />
          </button>
          <button
            class="player-icon-button"
            :disabled="player.isSkipLocked || !player.canOperate"
            @click="player.playNext"
            :aria-label="t('player.next')"
            :title="t('player.next')"
          >
            <SkipForward :stroke-width="1.75" aria-hidden="true" />
          </button>
          <button
            class="player-icon-button"
            :class="{ 'is-active': isLiked }"
            @click="toggleLike"
            :aria-label="isLiked ? t('player.unlike') : t('player.like')"
            :title="isLiked ? t('player.unlike') : t('player.like')"
          >
            <Heart :stroke-width="1.75" :fill="isLiked ? 'currentColor' : 'none'" aria-hidden="true" />
          </button>
        </div>

        <div class="player-footer-row flex w-full items-center justify-between border-t border-border-subtle pt-2.5">
          <div class="flex min-w-0 items-center gap-3">
            <span class="material-symbols-outlined text-[20px] text-primary" style="font-variation-settings: 'FILL' 1;">graphic_eq</span>
            <div class="min-w-0 text-left">
              <div class="player-footer-label font-micro text-micro uppercase leading-none text-text-muted">{{ t('player.nowPlayingIn') }}</div>
              <div class="mt-1 flex items-center gap-1.5 truncate leading-none">
                <span class="font-compact text-compact font-bold text-primary tabular-nums">{{ activeUserCount }}</span>
                <span class="font-micro text-micro text-text-secondary uppercase">{{ t('settings.active') }}</span>
              </div>
            </div>
          </div>
          <div class="flex min-w-0 items-center gap-2">
            <div class="flex min-w-0 items-center gap-2">
              <button class="player-icon-button player-icon-button--compact" @click="toggleMute" :aria-label="t('player.volume')" :title="t('player.volume')">
                <VolumeX v-if="uiStore.volume === 0" :stroke-width="1.75" aria-hidden="true" />
                <Volume2 v-else :stroke-width="1.75" aria-hidden="true" />
              </button>
              <div class="relative flex min-w-0 items-center">
                <input
                  v-model.number="uiStore.volume"
                  class="lounge-volume min-w-0 w-20"
                  min="0"
                  max="1"
                  step="0.01"
                  type="range"
                  :aria-label="t('player.volume')"
                  @focus="showVolumeValue"
                  @blur="hideVolumeValueSoon"
                  @input="showVolumeValueSoon"
                  @keydown="showVolumeValueSoon"
                  @pointerdown="showVolumeValue"
                  @pointerup="hideVolumeValueSoon"
                  @pointercancel="hideVolumeValueSoon"
                />
                <span
                  class="pointer-events-none absolute top-full mt-1 rounded-md border border-border-subtle bg-surface-panel px-1.5 py-0.5 font-mono text-[10px] font-bold tabular-nums text-text-muted shadow-lg transition-[left,opacity] duration-75"
                  :style="{ left: `calc(${Math.round(uiStore.volume * 100)}% - 16px)` }"
                  :class="volumeValueVisible ? 'opacity-100' : 'opacity-0'"
                >
                  {{ Math.round(uiStore.volume * 100) }}%
                </span>
              </div>
            </div>
            <button
              class="player-icon-button player-icon-button--compact"
              v-if="!isQueuePlaced"
              :aria-label="t('nav.queue')"
              :title="t('nav.queue')"
              @click="openQueueModule"
            >
              <ListMusic :stroke-width="1.75" aria-hidden="true" />
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, onBeforeUnmount, ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { Shuffle, SkipBack, Play, Pause, SkipForward, Heart, VolumeX, Volume2, ListMusic } from 'lucide-vue-next';
import CoverImage from '../CoverImage.vue';
import { useNowPlayingViewModel } from '../../composables/useNowPlayingViewModel';
import { useLayoutStore } from '../../stores/layout';
import { formatDuration } from '../../utils/format';

const { t } = useI18n();
const layoutStore = useLayoutStore();

const {
  player,
  ui: uiStore,
  music,
  coverUrl: currentCover,
  trackTitle,
  artistLine,
  platformLabel,
  requesterName,
  durationMs,
  progressMs,
  progressPercent,
  bufferedPercent,
  canSeek,
  isLiked,
  activeUserCount,
  seekToRatio,
  toggleLike
} = useNowPlayingViewModel({ artistSeparator: ' • ' });

const volumeValueVisible = ref(false);
let volumeValueTimer;

const isQueuePlaced = computed(() => layoutStore.placedModuleIds.includes('queue'));
const playModeLabel = computed(() => player.isShuffle ? t('player.shuffle') : t('player.listPlayback'));

const openQueueModule = () => {
  const lastCol = layoutStore.sortedColumns[layoutStore.sortedColumns.length - 1];
  if (lastCol) layoutStore.addModule('queue', lastCol.id);
};

const handleSeek = (event) => {
  if (!canSeek.value) return;
  const rect = event.currentTarget.getBoundingClientRect();
  const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
  seekToRatio(ratio);
};

const handleCoverClick = () => {
  toggleLike();
};

const toggleMute = () => {
  uiStore.volume = uiStore.volume === 0 ? 0.75 : 0;
  showVolumeValueSoon();
};

const showVolumeValue = () => {
  clearTimeout(volumeValueTimer);
  volumeValueVisible.value = true;
};

const hideVolumeValueSoon = () => {
  clearTimeout(volumeValueTimer);
  volumeValueTimer = setTimeout(() => {
    volumeValueVisible.value = false;
  }, 700);
};

const showVolumeValueSoon = () => {
  showVolumeValue();
  hideVolumeValueSoon();
};

onBeforeUnmount(() => {
  clearTimeout(volumeValueTimer);
});
</script>

<style scoped>
.now-playing-layout {
  container-type: size;
  --player-console-min-height: 204px;
  justify-content: center;
}

.now-playing-layout.is-playing {
  --player-console-min-height: 256px;
}

.now-playing-cover-stage,
.player-console {
  width: min(100cqw, calc(100cqh - var(--player-console-min-height) - 20px));
  max-width: 100%;
}

.now-playing-cover-stage {
  height: min(100cqw, calc(100cqh - var(--player-console-min-height) - 20px));
  flex: none;
}

.player-console {
  min-height: var(--player-console-min-height);
  padding-bottom: 0;
  flex: 0 1 auto;
}

.player-console > * {
  width: 100%;
}

.player-footer-label {
  white-space: nowrap;
}

.track-title {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  white-space: normal;
  overflow-wrap: anywhere;
  font-family: var(--font-display, Geist, sans-serif);
  font-size: clamp(1.25rem, 1.8vw, 1.625rem);
  line-height: 1.12;
  letter-spacing: 0;
  color: var(--text-primary);
}

.player-transport > .player-icon-button {
  min-width: 24px;
  flex: 0 1 44px;
}

.lounge-volume {
  accent-color: var(--accent);
}

.lounge-volume::-webkit-slider-runnable-track {
  background: var(--volume-track);
}

.lounge-volume::-moz-range-track {
  background: var(--volume-track);
}

.now-playing-cover {
  width: min(100cqw, 100cqh);
  height: min(100cqw, 100cqh);
  aspect-ratio: 1 / 1;
  flex: none;
  border: 0;
  outline: 0;
  min-width: 0;
}

.now-playing-cover-stage {
  container-type: size;
}
</style>
