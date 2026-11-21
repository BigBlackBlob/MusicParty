<template>
  <div class="now-playing-layout flex h-full min-h-0 min-w-0 flex-col items-center overflow-hidden gap-3" :class="music ? 'is-playing' : 'is-empty'">
    <div
      v-if="music || player.isLoading || player.isBuffering || player.isErrorState"
      class="now-playing-cover-stage relative flex min-h-0 w-full items-center justify-center"
    >
      <div
        class="now-playing-cover relative aspect-square shrink-0 overflow-hidden rounded-xl bg-surface-raised"
        :class="player.isPaused ? 'is-paused' : 'is-active'"
        @click="handleCoverClick"
      >
        <Transition name="np-cover">
          <CoverImage
            :key="trackKey"
            :src="currentCover"
            :alt="trackTitle"
            class="absolute inset-0 h-full w-full"
            loading="eager"
          />
        </Transition>

        <Transition name="np-overlay">
          <div v-if="player.isLoading || player.isBuffering" class="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-[var(--media-overlay)] text-text-primary backdrop-blur-xl">
            <div class="h-12 w-12 animate-spin rounded-full border-[3px] border-[var(--surface-control-active)] border-t-[var(--accent)]" />
            <span class="font-micro text-micro uppercase text-text-secondary">{{ player.isBuffering ? t('player.buffering') : t('player.loading') }}</span>
          </div>
        </Transition>

        <Transition name="np-overlay">
          <div v-if="player.isErrorState" class="absolute inset-x-5 bottom-5 rounded-lg border border-error/30 bg-[var(--error-soft-bg)] px-4 py-3 backdrop-blur-md">
            <div class="font-micro text-micro uppercase text-[var(--error-soft-text)]">{{ t('player.playbackError') }}</div>
            <div class="mt-1 text-xs text-[var(--error-soft-text)] opacity-80">{{ t('player.loadError') }}</div>
          </div>
        </Transition>
      </div>
    </div>

    <div v-else class="now-playing-cover-stage relative flex min-h-0 w-full items-center justify-center" aria-hidden="true"></div>

    <div
      class="player-console flex min-h-0 flex-col justify-between gap-3 bg-transparent pt-1"
    >
      <div class="player-meta flex min-h-0 flex-1 items-center justify-center">
        <Transition name="np-meta" mode="out-in">
          <div v-if="music" :key="trackKey" class="w-full min-w-0 shrink-0 text-center">
            <div class="mb-2 flex min-w-0 items-center justify-center gap-2">
              <span class="rounded-full border border-border-subtle bg-[var(--surface-control)] px-2.5 py-0.5 font-micro text-micro uppercase text-text-muted">{{ platformLabel }}</span>
              <span v-if="requesterName" class="min-w-0 max-w-[160px] truncate rounded-full border border-border-subtle bg-[var(--surface-control)] px-2.5 py-0.5 font-micro text-micro text-text-muted">{{ t('player.requestedBy', { name: requesterName }) }}</span>
            </div>
            <h1 class="track-title line-clamp-2 break-words font-extrabold" :title="trackTitle">{{ trackTitle }}</h1>
            <p class="mt-0.5 truncate font-title text-[14px] leading-tight text-primary/80" :title="artistLine">{{ artistLine }}</p>
          </div>
        </Transition>
      </div>

      <div class="player-controls flex shrink-0 flex-col gap-3">
        <div class="flex items-center justify-center gap-3">
          <div class="flex items-center justify-end gap-1 w-14">
            <span v-if="!canSeek && durationMs > 0" class="material-symbols-outlined text-[12px] text-text-muted opacity-50" :title="t('player.onlyRequester')">lock</span>
            <span class="text-right font-micro text-micro tabular-nums text-text-muted">{{ formatDuration(progressMs) }}</span>
          </div>
          <button
            class="np-seek-bar flex h-4 flex-1 items-center rounded-full"
            :class="canSeek ? 'is-seekable cursor-pointer' : 'cursor-not-allowed opacity-60'"
            @click="handleSeek"
            :aria-label="t('player.seek')"
            :title="canSeek ? t('player.seek') : t('player.onlyRequester')"
          >
            <span class="np-seek-track relative block w-full rounded-full bg-[var(--progress-track)]">
              <span v-if="bufferedPercent !== '0%'" class="pointer-events-none absolute inset-0 block h-full rounded-full bg-[var(--progress-loaded)] transition-[width] duration-200" :style="{ width: bufferedPercent }"></span>
              <span class="relative block h-full rounded-full bg-primary transition-[width] duration-200" :style="{ width: progressPercent }" />
              <span v-if="canSeek" class="np-seek-thumb" :style="{ left: progressPercent }" aria-hidden="true" />
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
            <Transition name="np-icon" mode="out-in">
              <Play v-if="player.isPaused" key="play" :stroke-width="1.75" fill="currentColor" aria-hidden="true" />
              <Pause v-else key="pause" :stroke-width="1.75" fill="currentColor" aria-hidden="true" />
            </Transition>
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

const trackKey = computed(() => (
  music.value ? `${music.value.platform || ''}:${music.value.id || trackTitle.value}` : 'empty'
));
const isQueuePlaced = computed(() => (
  layoutStore.placedModuleIds.includes('queue-playlists')
));
const playModeLabel = computed(() => player.isShuffle ? t('player.shuffle') : t('player.listPlayback'));

const openQueueModule = () => {
  const lastCol = layoutStore.sortedColumns[layoutStore.sortedColumns.length - 1];
  if (lastCol) layoutStore.addModule('queue-playlists', lastCol.id);
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
  cursor: pointer;
  transition: transform 340ms var(--motion-ease-standard),
    filter 340ms var(--motion-ease-standard),
    box-shadow 340ms var(--motion-ease-standard);
}

.now-playing-cover.is-paused {
  transform: scale(0.95);
  filter: saturate(0.86);
}

.now-playing-cover.is-active {
  transform: scale(1);
  filter: saturate(1);
  box-shadow: 0 20px 44px -18px color-mix(in srgb, var(--surface-0) 70%, transparent);
}

.now-playing-cover-stage {
  container-type: size;
}

/* Progress bar hover polish */
.np-seek-track {
  height: 4px;
  transition: height var(--motion-exit) var(--motion-ease-standard);
}

.np-seek-bar.is-seekable:hover .np-seek-track,
.np-seek-bar.is-seekable:focus-visible .np-seek-track {
  height: 6px;
}

.np-seek-thumb {
  position: absolute;
  top: 50%;
  width: 10px;
  height: 10px;
  border-radius: var(--radius-full);
  background: var(--accent);
  transform: translate(-50%, -50%) scale(0.6);
  opacity: 0;
  pointer-events: none;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent);
  transition: opacity var(--motion-exit) var(--motion-ease-standard),
    transform var(--motion-exit) var(--motion-ease-standard);
}

.np-seek-bar.is-seekable:hover .np-seek-thumb,
.np-seek-bar.is-seekable:focus-visible .np-seek-thumb {
  opacity: 1;
  transform: translate(-50%, -50%) scale(1);
}

/* Restrained track-change & icon transitions */
.np-cover-enter-active,
.np-cover-leave-active {
  transition: opacity 280ms var(--motion-ease-standard);
}

.np-cover-enter-from,
.np-cover-leave-to {
  opacity: 0;
}

.np-meta-enter-active {
  transition: opacity var(--motion-enter) var(--motion-ease-standard),
    transform var(--motion-enter) var(--motion-ease-standard);
}

.np-meta-leave-active {
  transition: opacity 110ms var(--motion-ease-in);
}

.np-meta-enter-from {
  opacity: 0;
  transform: translateY(3px);
}

.np-meta-leave-to {
  opacity: 0;
}

.np-icon-enter-active,
.np-icon-leave-active {
  transition: opacity 110ms var(--motion-ease-standard),
    transform 110ms var(--motion-ease-standard);
}

.np-icon-enter-from,
.np-icon-leave-to {
  opacity: 0;
  transform: scale(0.84);
}

.np-overlay-enter-active,
.np-overlay-leave-active {
  transition: opacity var(--motion-exit) var(--motion-ease-standard);
}

.np-overlay-enter-from,
.np-overlay-leave-to {
  opacity: 0;
}
</style>
