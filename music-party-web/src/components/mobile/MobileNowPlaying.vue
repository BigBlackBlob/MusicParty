<template>
  <section class="mobile-now-page" :class="`mobile-now-page--${ui.mobileNowDensity}`">
    <div class="mobile-stage">
      <div class="mobile-cover-container">
        <button
          type="button"
          class="mobile-cover group"
          :class="{ 'mobile-cover--paused': player.isPaused }"
          :disabled="!music"
          @click="toggleLike"
          :aria-label="isLiked ? t('player.unlike') : t('player.like')"
        >
          <CoverImage
            :src="music?.coverUrl"
            :alt="trackTitle"
            loading="eager"
            decoding="async"
            class="h-full w-full object-cover transition-transform duration-700 ease-out group-active:scale-[0.98]"
          />
          <div class="absolute inset-0 rounded-2xl shadow-[inset_0_0_0_1px_rgba(255,255,255,0.08)] pointer-events-none" />

          <span v-if="player.isLoading || player.isBuffering" class="mobile-cover__status">
            <span class="h-8 w-8 animate-spin rounded-full border-[2.5px] border-white/20 border-t-white" />
            <span class="tracking-widest mt-3">{{ player.isBuffering ? t('player.buffering') : t('player.loading') }}</span>
          </span>
        </button>
      </div>

      <div class="mobile-track-info">
        <div class="mobile-track-info__meta font-mono">
          <span v-if="platformLabel" class="meta-pill">{{ platformLabel }}</span>
          <span v-if="requesterName" class="meta-pill">{{ t('player.requestedByShort') }}: {{ requesterName }}</span>
          <span v-if="player.streamListenerCount > 0" class="meta-pill">{{ player.streamListenerCount }} {{ t('player.listenersShort') }}</span>
        </div>
        <h1 class="mobile-track-info__title">{{ trackTitle }}</h1>
        <p class="mobile-track-info__artist">{{ artistLine }}</p>

        <MobileMiniLyrics
          v-if="hasLyrics"
          class="mobile-mini-lyrics-slot w-full flex-shrink-0"
          :lyrics="player.lyricDetail.lyric || player.lyricText"
          :translated-lyrics="ui.showLyricTranslation ? player.lyricDetail.translatedLyric : ''"
          :show-translation="ui.showLyricTranslation"
          :current-time-ms="player.playbackPositionMs"
          @open="lyricsExpanded = true"
        />
      </div>

      <div v-if="player.isErrorState" class="mobile-error mt-4 mx-4">
        <strong>{{ t('player.playbackError') }}</strong>
        <span>{{ t('player.loadError') }}</span>
      </div>
    </div>

    <div class="mobile-controls">
      <div class="mobile-progress-wrap">
        <ProgressScrubber
          :current-ms="player.playbackPositionMs"
          :duration="durationMs"
          :can-seek="canSeek"
          :is-error="player.isErrorState"
          :markers="player.nowPlaying?.likeMarkers || []"
          @preview-start="player.setSeekingPreview(true)"
          @preview-end="player.setSeekingPreview(false)"
          @seek="player.seek"
        />
      </div>

      <div class="mobile-transport">
        <IconButton
          @click="toggleVolumePanel"
          :variant="volumePanelOpen ? 'secondary' : 'ghost'"
          class="text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
          :aria-label="t('player.volume')"
        >
          <VolumeX v-if="ui.volume === 0" class="h-5 w-5" />
          <Volume1 v-else-if="ui.volume < 0.5" class="h-5 w-5" />
          <Volume2 v-else class="h-5 w-5" />
        </IconButton>

        <div class="flex items-center gap-6 md:gap-8">
          <IconButton
            @click="player.playNext"
            :disabled="player.isSkipLocked"
            :aria-label="t('player.next')"
            class="text-[var(--text-primary)] transition-transform active:scale-90"
          >
            <SkipForward class="h-7 w-7 fill-current" />
          </IconButton>

          <button
            @click="player.togglePause"
            :disabled="player.isPauseLocked && !player.isPaused"
            class="mobile-play-btn"
            :aria-label="player.isPaused ? t('player.play') : t('player.pause')"
          >
            <Play v-if="player.isPaused" class="ml-1 h-9 w-9 fill-current" />
            <Pause v-else class="h-9 w-9 fill-current" />
          </button>

          <IconButton
            @click="toggleLike"
            :variant="isLiked ? 'secondary' : 'ghost'"
            :disabled="!music"
            class="transition-transform active:scale-90"
            :aria-label="isLiked ? t('player.unlike') : t('player.like')"
          >
            <Heart class="h-7 w-7 transition-colors duration-300" :class="isLiked ? 'fill-[var(--accent)] text-[var(--accent)]' : 'text-[var(--text-primary)]'" />
          </IconButton>
        </div>

        <IconButton
          @click="player.toggleShuffle"
          :variant="player.isShuffle ? 'secondary' : 'ghost'"
          :disabled="player.isShuffleLocked"
          class="text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
          :aria-label="t('player.shuffle')"
        >
          <Shuffle class="h-5 w-5" />
        </IconButton>
      </div>

      <Transition name="mobile-volume-popover">
        <div v-if="volumePanelOpen" ref="volumePanelRef" class="mobile-volume-panel">
          <button class="mobile-volume-panel__mute" type="button" @click="toggleMute">
            {{ ui.volume === 0 ? t('player.muted') : `${Math.round(ui.volume * 100)}%` }}
          </button>
          <input
            :value="ui.volume"
            type="range"
            min="0"
            max="1"
            step="0.01"
            :aria-label="t('player.volume')"
            class="accent-[var(--accent)]"
            @input="event => ui.setVolume(Number(event.target.value))"
          />
        </div>
      </Transition>
    </div>

    <Transition name="slide-up">
      <div v-if="lyricsExpanded" class="mobile-lyrics-overlay">
        <div class="mobile-lyrics-overlay__bar">
          <div class="min-w-0">
            <h2 class="text-sm font-bold text-[var(--text-primary)] truncate">{{ trackTitle }}</h2>
            <p class="text-[11px] text-[var(--text-tertiary)] truncate mt-0.5">{{ artistLine }}</p>
          </div>
          <IconButton variant="secondary" size="sm" @click="lyricsExpanded = false" :aria-label="t('player.closeLyrics')" class="rounded-full bg-black/20 hover:bg-black/40">
            <X class="h-5 w-5" />
          </IconButton>
        </div>
        <AppleLyricsPanel
          class="min-h-0 flex-1"
          :lyrics="player.lyricDetail.lyric || player.lyricText"
          :translated-lyrics="player.lyricDetail.translatedLyric"
          :show-translation="ui.showLyricTranslation"
          :current-time-ms="player.playbackPositionMs"
          :is-playing="!player.isPaused"
          :is-dark-mode="ui.isDarkMode"
          :bg-color="ui.dynamicAccent?.accent || ''"
          mobile
          @toggle-translation="ui.toggleLyricTranslation"
        />
      </div>
    </Transition>
  </section>
</template>

<script setup>
import { computed, onUnmounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Heart, Pause, Play, Shuffle, SkipForward, Volume1, Volume2, VolumeX, X } from 'lucide-vue-next';
import { useNowPlayingViewModel } from '../../composables/useNowPlayingViewModel';
import { parseLyrics } from '../../utils/parser';
import AppleLyricsPanel from '../AppleLyricsPanel.vue';
import CoverImage from '../CoverImage.vue';
import IconButton from '../ui/IconButton.vue';
import ProgressScrubber from '../ui/ProgressScrubber.vue';
import MobileMiniLyrics from './MobileMiniLyrics.vue';

const { t } = useI18n();
const {
  player,
  ui,
  music,
  trackTitle,
  artistLine,
  platformLabel,
  requesterName,
  durationMs,
  canSeek,
  isLiked,
  toggleLike
} = useNowPlayingViewModel({ artistSeparator: ' / ' });
const lyricsExpanded = ref(false);
const volumePanelOpen = ref(false);
const volumePanelRef = ref(null);

const lyricLines = computed(() => parseLyrics(player.lyricDetail.lyric || player.lyricText));
const hasLyrics = computed(() => lyricLines.value.length >= 5);

const toggleVolumePanel = () => {
  volumePanelOpen.value = !volumePanelOpen.value;
};

const toggleMute = () => {
  ui.setVolume(ui.volume === 0 ? 0.75 : 0);
};

const handleDocumentPointerDown = (event) => {
  if (!volumePanelOpen.value) return;
  if (volumePanelRef.value?.contains(event.target)) return;
  volumePanelOpen.value = false;
};

watch(volumePanelOpen, (open) => {
  if (open) document.addEventListener('pointerdown', handleDocumentPointerDown, true);
  else document.removeEventListener('pointerdown', handleDocumentPointerDown, true);
});

onUnmounted(() => {
  document.removeEventListener('pointerdown', handleDocumentPointerDown, true);
});
</script>

<style scoped>
.mobile-now-page {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
  --mobile-controls-height: auto;
  --mobile-cover-vw: 76vw;
  --mobile-cover-max: 320px;
  --mobile-stage-pad-top: clamp(12px, 2.6dvh, 22px);
  --mobile-cover-pad-x: 24px;
  --mobile-cover-pad-bottom: 14px;
  --mobile-track-gap: 4px;
  --mobile-meta-gap: 6px;
  --mobile-meta-margin-bottom: 6px;
  --mobile-title-max: 32px;
  --mobile-artist-max: 18px;
  --mobile-mini-lyrics-margin: 8px;
  --mobile-progress-pad-x: 20px;
  --mobile-progress-pad-top: 8px;
  --mobile-progress-pad-bottom: 16px;
  --mobile-transport-pad-x: 24px;
  --mobile-transport-pad-bottom: 24px;
  --mobile-play-size: 72px;
  --mobile-transport-center-gap: 24px;
}

.mobile-now-page--compact {
  --mobile-cover-vw: 68vw;
  --mobile-cover-max: 260px;
  --mobile-stage-pad-top: clamp(10px, 2dvh, 16px);
  --mobile-cover-pad-bottom: 10px;
  --mobile-track-gap: 3px;
  --mobile-meta-margin-bottom: 4px;
  --mobile-title-max: 28px;
  --mobile-artist-max: 16px;
  --mobile-mini-lyrics-margin: 6px;
  --mobile-progress-pad-top: 4px;
  --mobile-progress-pad-bottom: 12px;
  --mobile-transport-pad-bottom: 16px;
  --mobile-play-size: 66px;
  --mobile-transport-center-gap: 20px;
}

.mobile-now-page--relaxed {
  --mobile-cover-vw: 84vw;
  --mobile-cover-max: 360px;
  --mobile-stage-pad-top: clamp(16px, 3.4dvh, 28px);
  --mobile-cover-pad-bottom: 18px;
  --mobile-track-gap: 6px;
  --mobile-mini-lyrics-margin: 10px;
  --mobile-progress-pad-bottom: 18px;
  --mobile-transport-pad-bottom: 28px;
}

.mobile-stage {
  flex: 1;
  display: flex;
  min-height: 0;
  flex-direction: column;
  overflow-y: auto;
  overflow-x: hidden;
  padding-bottom: 24px;
}

.mobile-cover-container {
  width: 100%;
  padding: var(--mobile-stage-pad-top) var(--mobile-cover-pad-x) var(--mobile-cover-pad-bottom);
  display: flex;
  justify-content: center;
}

.mobile-cover {
  position: relative;
  display: block;
  width: min(var(--mobile-cover-vw), var(--mobile-cover-max));
  max-width: 100%;
  aspect-ratio: 1;
  border-radius: 16px;
  background: var(--surface-2);
  box-shadow: 0 32px 64px -12px rgba(0, 0, 0, 0.5), 0 16px 24px -8px rgba(0, 0, 0, 0.3);
  transition: filter 0.5s ease;
}

.mobile-cover:disabled {
  cursor: default;
}

.mobile-cover--paused {
  filter: saturate(0.8) brightness(0.85);
}

.mobile-cover__status {
  position: absolute;
  inset: 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: var(--surface-glass-bg);
  backdrop-filter: blur(4px);
  color: white;
  border-radius: inherit;
  font-size: 10px;
  font-weight: 700;
}

.mobile-track-info {
  width: 100%;
  padding: 0 24px;
  text-align: left;
  display: flex;
  flex-direction: column;
  gap: var(--mobile-track-gap);
}

.mobile-track-info__meta {
  display: flex;
  flex-wrap: wrap;
  gap: var(--mobile-meta-gap);
  align-items: center;
  margin-bottom: var(--mobile-meta-margin-bottom);
}

.meta-pill {
  background: var(--surface-glass-control);
  border: 1px solid var(--surface-glass-border);
  padding: 3px 8px;
  border-radius: var(--radius-xs);
  color: var(--text-primary);
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  backdrop-filter: blur(8px);
  box-shadow: 0 2px 8px var(--surface-glass-bg);
}

.mobile-track-info__title {
  color: var(--text-primary);
  font-size: clamp(22px, 6.4vw, var(--mobile-title-max));
  font-weight: 800;
  line-height: 1.1;
  letter-spacing: -0.01em;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  overflow: hidden;
  text-shadow: 0 2px 12px var(--surface-glass-bg);
}

.mobile-track-info__artist {
  color: var(--text-secondary);
  font-size: clamp(15px, 4vw, var(--mobile-artist-max));
  font-weight: 500;
  line-height: 1.3;
  opacity: 0.95;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 1;
  overflow: hidden;
  text-shadow: 0 1px 8px var(--surface-glass-bg);
}

.mobile-error {
  display: flex;
  flex-direction: column;
  gap: 2px;
  border-left: 3px solid var(--error);
  background: var(--error-soft-bg);
  padding: 12px 16px;
  color: var(--text-primary);
  font-size: 13px;
  border-radius: 0 8px 8px 0;
}

.mobile-controls {
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  background: transparent;
  position: relative;
  z-index: 10;
}

.mobile-progress-wrap {
  width: 100%;
  padding: var(--mobile-progress-pad-top) var(--mobile-progress-pad-x) var(--mobile-progress-pad-bottom);
}

.mobile-transport {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0 var(--mobile-transport-pad-x) var(--mobile-transport-pad-bottom);
}

.mobile-transport > .flex {
  gap: var(--mobile-transport-center-gap);
}

.mobile-mini-lyrics-slot {
  margin-top: var(--mobile-mini-lyrics-margin);
}

.mobile-play-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: var(--mobile-play-size);
  height: var(--mobile-play-size);
  border-radius: 50%;
  background: var(--text-primary);
  color: var(--surface-0);
  transition: all 0.2s cubic-bezier(0.16, 1, 0.3, 1);
  box-shadow: 0 8px 24px rgba(0,0,0,0.3);
}

.mobile-play-btn:active {
  transform: scale(0.92);
}

.mobile-play-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
  transform: none;
}

.mobile-volume-panel {
  position: absolute;
  inset-inline: 24px;
  bottom: calc(100% + 16px);
  z-index: 20;
  display: flex;
  align-items: center;
  gap: 16px;
  border: 1px solid var(--surface-glass-border);
  border-radius: 12px;
  background: var(--surface-glass-panel);
  backdrop-filter: blur(24px);
  padding: 14px 20px;
  box-shadow: 0 24px 48px var(--surface-glass-bg);
}

.mobile-volume-panel__mute {
  flex-shrink: 0;
  color: var(--text-primary);
  font-size: 12px;
  font-weight: 700;
  font-variant-numeric: tabular-nums;
  width: 44px;
  text-align: left;
}

.mobile-volume-panel input[type=range] {
  flex: 1;
  min-width: 0;
}

.mobile-lyrics-overlay {
  position: fixed;
  inset: 0;
  z-index: var(--z-tutorial);
  display: flex;
  flex-direction: column;
  background: var(--surface-0);
  padding-top: env(safe-area-inset-top);
}

.mobile-lyrics-overlay__bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-default);
  background: var(--surface-1);
  position: relative;
  z-index: 10;
}

.mobile-volume-popover-enter-active,
.mobile-volume-popover-leave-active,
.slide-up-enter-active,
.slide-up-leave-active {
  transition: opacity 240ms cubic-bezier(0.16, 1, 0.3, 1), transform 240ms cubic-bezier(0.16, 1, 0.3, 1);
}

.mobile-volume-popover-enter-from,
.mobile-volume-popover-leave-to {
  opacity: 0;
  transform: translateY(12px) scale(0.95);
}

.slide-up-enter-from,
.slide-up-leave-to {
  opacity: 0;
  transform: translateY(100%);
}

@media (max-height: 700px) {
  .mobile-now-page {
    --mobile-cover-vw: 68vw;
    --mobile-cover-max: 270px;
    --mobile-stage-pad-top: clamp(10px, 2dvh, 16px);
    --mobile-cover-pad-x: 32px;
    --mobile-cover-pad-bottom: 8px;
    --mobile-title-max: 28px;
    --mobile-artist-max: 16px;
    --mobile-progress-pad-top: 4px;
    --mobile-progress-pad-bottom: 12px;
    --mobile-transport-pad-bottom: 16px;
    --mobile-play-size: 64px;
    --mobile-transport-center-gap: 20px;
  }

  .mobile-now-page--compact {
    --mobile-cover-vw: 62vw;
    --mobile-cover-max: 240px;
  }
}

@media (max-height: 620px) {
  .mobile-now-page {
    --mobile-cover-vw: 58vw;
    --mobile-cover-max: 220px;
    --mobile-stage-pad-top: 8px;
    --mobile-cover-pad-bottom: 6px;
    --mobile-track-gap: 2px;
    --mobile-meta-gap: 4px;
    --mobile-meta-margin-bottom: 2px;
    --mobile-mini-lyrics-margin: 4px;
    --mobile-progress-pad-top: 2px;
    --mobile-progress-pad-bottom: 10px;
    --mobile-transport-pad-bottom: 12px;
    --mobile-play-size: 60px;
    --mobile-transport-center-gap: 16px;
  }

  .mobile-track-info__meta {
    max-height: 22px;
    overflow: hidden;
  }
}
</style>
