<template>
  <section class="flex flex-col h-full bg-bg-base relative overflow-hidden">
    <!-- Top Navigation (Modal style) -->
    <header class="flex items-center justify-between px-md w-full h-[64px] flex-shrink-0 z-20 relative">
      <div class="flex-1 flex justify-start">
        <!-- Close button removed by user request -->
      </div>
      <div class="flex-1 flex flex-col items-center justify-center min-w-0">
        <span class="font-micro text-micro text-text-muted uppercase tracking-widest">{{ t('player.nowPlaying') }}</span>
        <span class="font-compact text-compact text-text-primary font-semibold truncate max-w-[180px] w-full text-center">{{ roomStore.currentRoom?.name || 'Music Party' }}</span>
      </div>
      <div class="flex-1 flex items-center justify-end">
        <button 
          class="w-[44px] h-[44px] flex items-center justify-center rounded-full hover:bg-surface-raised transition-colors active:scale-95 text-primary ml-1"
          :aria-label="t('settings.title')"
          :title="t('settings.title')"
          @click="$emit('open-settings')"
        >
          <span class="material-symbols-outlined">settings</span>
        </button>
      </div>
    </header>

    <!-- Main Content Area -->
    <main class="mobile-now-playing-main flex-1 flex flex-col px-md w-full overflow-y-auto overflow-x-hidden py-4">
      <!-- Artwork Section -->
      <div
        v-if="music || player.isLoading || player.isBuffering || player.isErrorState"
        class="mobile-now-playing-art w-full aspect-square max-h-[42dvh] max-w-[42dvh] mb-6 rounded-2xl overflow-hidden flex-shrink-0 relative z-10 group mx-auto"
        :class="music ? 'shadow-[0_30px_60px_rgba(0,0,0,0.6)]' : 'mobile-empty-art'"
      >
        <div v-if="!music && !player.isLoading && !player.isBuffering" class="mobile-empty-art__content" aria-hidden="true">
          <span class="material-symbols-outlined mobile-empty-art__icon">graphic_eq</span>
        </div>
        <CoverImage
          v-else
          :src="music?.coverUrl"
          :alt="trackTitle"
          loading="eager"
          decoding="async"
          class="h-full w-full object-cover transition-transform duration-700 ease-out group-active:scale-[0.98]"
        />
        <div v-if="player.isLoading || player.isBuffering" class="absolute inset-0 flex flex-col items-center justify-center bg-black/40 backdrop-blur-sm text-white">
          <span class="h-8 w-8 animate-spin rounded-full border-[2.5px] border-white/20 border-t-white" />
          <span class="tracking-widest mt-3 text-[10px] font-bold uppercase">{{ player.isBuffering ? t('player.buffering') : t('player.loading') }}</span>
        </div>
      </div>

      <section v-else class="mobile-now-playing-empty" :aria-label="t('player.nowPlaying')">
        <div class="mobile-now-playing-empty__copy">
          <div class="mobile-now-playing-empty__meta">
            <span class="font-micro text-micro uppercase text-text-muted">{{ roomStore.currentRoom?.name || t('app.brand') }} · {{ t('player.nowPlaying') }}</span>
            <button
              @click="toggleLike"
              :disabled="!music"
              class="mobile-now-playing-like flex h-11 w-11 flex-shrink-0 items-center justify-center rounded-full text-primary hover:bg-accent-subtle transition-colors"
              :aria-label="isLiked ? t('player.unlike') : t('player.like')"
              :title="isLiked ? t('player.unlike') : t('player.like')"
            >
              <svg xmlns="http://www.w3.org/2000/svg" height="26" width="26" viewBox="0 -960 960 960" fill="currentColor" aria-hidden="true">
                <path v-if="isLiked" d="m480-120-58-52q-101-91-167-157T150-447.5Q111-500 95.5-544T80-634q0-94 63-157t157-63q52 0 99 22t81 62q34-40 81-62t99-22q94 0 157 63t63 157q0 46-15.5 90T810-447.5Q771-395 705-329T538-172l-58 52Z"/>
                <path v-else d="m480-120-58-52q-101-91-167-157T150-447.5Q111-500 95.5-544T80-634q0-94 63-157t157-63q52 0 99 22t81 62q34-40 81-62t99-22q94 0 157 63t63 157q0 46-15.5 90T810-447.5Q771-395 705-329T538-172l-58 52Zm0-108q96-86 158-147.5t98-107q36-45.5 50-81t14-70.5q0-60-40-100t-100-40q-47 0-87 26.5T518-680h-76q-15-41-55-67.5T300-774q-60 0-100 40t-40 100q0 35 14 70.5t50 81q36 45.5 98 107T480-228Zm0-273Z"/>
              </svg>
            </button>
          </div>
          <h1 class="mobile-now-playing-empty__title">{{ trackTitle }}</h1>
          <p class="mobile-now-playing-empty__hint">{{ t('player.waitingFirstTrackHint') }}</p>
        </div>
        <span class="material-symbols-outlined mobile-now-playing-empty__icon" aria-hidden="true">graphic_eq</span>
      </section>

      <!-- Dynamic spacer: pushes the bottom section down on taller screens -->
      <div class="flex-1 min-h-[15px] flex-shrink-0" aria-hidden="true" />

      <!-- Track Info & Like -->
      <div v-if="music || player.isLoading || player.isBuffering || player.isErrorState" class="mobile-now-playing-track flex items-center justify-between mb-8 flex-shrink-0 z-10 px-1">
        <div class="flex min-w-0 flex-1 flex-col overflow-hidden">
          <h1 class="font-title text-title text-text-primary line-clamp-2 break-words leading-snug tracking-tight">{{ trackTitle }}</h1>
          <p class="font-body text-body text-text-secondary truncate mt-1">{{ artistLine }}</p>
        </div>
        <button 
          @click="toggleLike"
          :disabled="!music"
          class="ml-4 w-[44px] h-[44px] flex items-center justify-center rounded-full text-primary hover:bg-accent-subtle transition-all active:scale-90 flex-shrink-0"
        >
          <svg xmlns="http://www.w3.org/2000/svg" height="28" width="28" viewBox="0 -960 960 960" fill="currentColor" aria-hidden="true">
              <path v-if="isLiked" d="m480-120-58-52q-101-91-167-157T150-447.5Q111-500 95.5-544T80-634q0-94 63-157t157-63q52 0 99 22t81 62q34-40 81-62t99-22q94 0 157 63t63 157q0 46-15.5 90T810-447.5Q771-395 705-329T538-172l-58 52Z"/>
              <path v-else d="m480-120-58-52q-101-91-167-157T150-447.5Q111-500 95.5-544T80-634q0-94 63-157t157-63q52 0 99 22t81 62q34-40 81-62t99-22q94 0 157 63t63 157q0 46-15.5 90T810-447.5Q771-395 705-329T538-172l-58 52Zm0-108q96-86 158-147.5t98-107q36-45.5 50-81t14-70.5q0-60-40-100t-100-40q-47 0-87 26.5T518-680h-76q-15-41-55-67.5T300-774q-60 0-100 40t-40 100q0 35 14 70.5t50 81q36 45.5 98 107T480-228Zm0-273Z"/>
            </svg>
        </button>
      </div>

      <!-- Progress Bar -->
      <div class="mobile-now-playing-progress flex flex-col gap-xs mb-8 flex-shrink-0 z-10 px-1">
        <ProgressScrubber
          :current-ms="player.playbackPositionMs"
          :duration="durationMs"
          :can-seek="canSeek"
          :is-error="player.isErrorState"
          :loaded-ms="player.bufferedMs"
          :markers="player.nowPlaying?.likeMarkers || []"
          :hide-labels="true"
          @preview-start="player.setSeekingPreview(true)"
          @preview-end="player.setSeekingPreview(false)"
          @seek="player.seek"
          class="h-1.5"
        />
        <div class="flex justify-between font-micro text-micro text-text-secondary mt-1">
          <span>{{ formatTime(player.playbackPositionMs) }}</span>
          <span>{{ formatTime(durationMs) }}</span>
        </div>
      </div>

      <!-- Playback Controls -->
      <div class="mobile-now-playing-controls flex items-center flex-shrink-0 z-10 w-full max-w-[420px] mx-auto px-1">
        <!-- 1. Volume -->
        <div class="flex-1 flex justify-center items-center">
          <div class="relative" ref="volumePanelContainerRef">
            <button 
              @click="toggleVolumePanel"
              class="w-[44px] h-[44px] flex items-center justify-center transition-colors active:scale-90"
              :class="volumePanelOpen ? 'text-primary' : 'text-text-secondary hover:text-primary'"
            >
              <span class="material-symbols-outlined text-[24px]">
                {{ ui.volume === 0 ? 'volume_off' : (ui.volume < 0.5 ? 'volume_down' : 'volume_up') }}
              </span>
            </button>

            <Transition name="mobile-volume-popover">
              <div v-if="volumePanelOpen" class="absolute bottom-[100%] left-0 mb-4 z-30 flex items-center gap-4 border border-border-default rounded-2xl bg-surface-panel/90 backdrop-blur-2xl px-4 py-3 shadow-[0_24px_48px_rgba(0,0,0,0.5)] w-[200px] origin-bottom-left">
                <button class="flex-shrink-0 text-text-primary text-xs font-bold font-mono w-[32px] text-left" type="button" @click="toggleMute">
                  {{ ui.volume === 0 ? t('player.muted') : `${Math.round(ui.volume * 100)}%` }}
                </button>
                <input
                  :value="ui.volume"
                  type="range"
                  min="0"
                  max="1"
                  step="0.01"
                  :aria-label="t('player.volume')"
                  class="flex-1 min-w-0 accent-primary"
                  @input="event => ui.setVolume(Number(event.target.value))"
                />
              </div>
            </Transition>
          </div>
        </div>

        <!-- 2. Shuffle -->
        <div class="flex-1 flex justify-center items-center">
          <button 
            @click="player.toggleShuffle"
            :disabled="player.isShuffleLocked"
            class="w-[44px] h-[44px] flex items-center justify-center transition-colors active:scale-90"
            :class="player.isShuffle ? 'text-primary' : 'text-text-secondary hover:text-primary'"
          >
            <span class="material-symbols-outlined text-[24px]" :aria-label="playModeLabel" :title="playModeLabel">shuffle</span>
          </button>
        </div>

        <!-- 3. Play/Pause -->
        <div class="flex-1 flex justify-center items-center">
          <button 
            @click="player.togglePause"
            :disabled="player.isPauseLocked && !player.isPaused"
            class="w-[64px] h-[64px] flex items-center justify-center rounded-full transition-colors active:scale-95"
            :class="music ? 'text-primary bg-accent-subtle shadow-lg shadow-primary/20 hover:text-white hover:scale-105' : 'text-text-secondary bg-surface-raised hover:bg-[var(--surface-control-hover)] hover:text-text-primary'"
          >
            <span class="material-symbols-outlined text-[40px]" style="font-variation-settings: 'FILL' 1;">
              {{ player.isPaused ? 'play_arrow' : 'pause' }}
            </span>
          </button>
        </div>

        <!-- 4. Next -->
        <div class="flex-1 flex justify-center items-center">
          <button
            @click="player.playNext"
            :disabled="player.isSkipLocked || !player.canOperate"
            class="w-[44px] h-[44px] flex items-center justify-center text-primary hover:text-white transition-colors active:scale-90"
          >
            <span class="material-symbols-outlined text-[36px]" style="font-variation-settings: 'FILL' 1;">skip_next</span>
          </button>
        </div>

        <!-- 5. Lyrics -->
        <div class="flex-1 flex justify-center items-center">
          <button
            @click="lyricsOpen = true"
            :disabled="!music"
            class="w-[44px] h-[44px] flex items-center justify-center transition-colors active:scale-90"
            :class="music ? 'text-text-secondary hover:text-primary' : 'text-text-secondary/40'"
            :aria-label="t('lyrics.title', 'Lyrics')"
            :title="t('lyrics.title', 'Lyrics')"
          >
            <span class="material-symbols-outlined text-[24px]">lyrics</span>
          </button>
        </div>
      </div>

      <!-- Ambient Lyrics Overlay -->
      <MobileAmbientLyrics
        :open="lyricsOpen"
        :cover-url="music?.coverUrl || ''"
        :track-title="trackTitle"
        :artist-line="artistLine"
        :lyrics="player.lyricDetail.lyric || player.lyricText"
        :translated-lyrics="player.lyricDetail.translatedLyric"
        :romanized-lyrics="player.lyricDetail.romanizedLyric"
        :word-lyric="player.lyricDetail.wordLyric"
        :word-translated-lyric="player.lyricDetail.wordTranslatedLyric"
        :word-romanized-lyric="player.lyricDetail.wordRomanizedLyric"
        :show-translation="ui.showLyricTranslation"
        :show-romanization="ui.showLyricRomanization"
        :current-time-ms="player.playbackPositionMs"
        :lyrics-loaded="!!music"
        @close="lyricsOpen = false"
        @toggle-translation="ui.toggleLyricTranslation"
        @toggle-romanization="ui.toggleLyricRomanization"
      />

      <div v-if="player.isErrorState" class="mb-8 p-4 bg-error/10 border-l-4 border-error rounded-r-lg">
        <p class="text-sm font-bold text-error">{{ t('player.playbackError') }}</p>
        <p class="text-xs text-text-secondary mt-1">{{ t('player.loadError') }}</p>
      </div>
    </main>
  </section>
</template>

<script setup>
import { computed, ref, onUnmounted, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useNowPlayingViewModel } from '../../composables/useNowPlayingViewModel';
import { useRoomStore } from '../../stores/room';
import MobileAmbientLyrics from './MobileAmbientLyrics.vue';
import CoverImage from '../CoverImage.vue';
import ProgressScrubber from '../ui/ProgressScrubber.vue';

const { t } = useI18n();
const roomStore = useRoomStore();
const {
  player,
  ui,
  music,
  trackTitle,
  artistLine,
  durationMs,
  canSeek,
  isLiked,
  toggleLike
} = useNowPlayingViewModel({ artistSeparator: ' / ' });

const volumePanelOpen = ref(false);
const volumePanelContainerRef = ref(null);
const lyricsOpen = ref(false);
const playModeLabel = computed(() => player.isShuffle ? t('player.shuffle') : t('player.listPlayback'));

const toggleVolumePanel = () => {
  volumePanelOpen.value = !volumePanelOpen.value;
};

const toggleMute = () => {
  ui.setVolume(ui.volume === 0 ? 0.75 : 0);
};

const handleDocumentPointerDown = (event) => {
  if (!volumePanelOpen.value) return;
  if (volumePanelContainerRef.value?.contains(event.target)) return;
  volumePanelOpen.value = false;
};

watch(volumePanelOpen, (open) => {
  if (open) document.addEventListener('pointerdown', handleDocumentPointerDown, true);
  else document.removeEventListener('pointerdown', handleDocumentPointerDown, true);
});

onUnmounted(() => {
  document.removeEventListener('pointerdown', handleDocumentPointerDown, true);
});


const formatTime = (ms) => {
  if (!ms || isNaN(ms)) return '0:00';
  const totalSeconds = Math.floor(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  return `${minutes}:${seconds.toString().padStart(2, '0')}`;
};

defineEmits(['close', 'open-settings']);
</script>

<style scoped>
@media (max-width: 360px) and (max-height: 740px) {
  .mobile-now-playing-main {
    padding-block: 8px;
  }

  .mobile-now-playing-art {
    max-height: 38dvh;
    margin-bottom: 16px;
  }

  .mobile-now-playing-track,
  .mobile-now-playing-progress {
    margin-bottom: 16px;
  }

  .mobile-now-playing-controls {
    margin-bottom: 12px;
  }
}

.mobile-empty-art {
  border: 1px solid var(--border-default);
  background: var(--surface-0);
}

.mobile-empty-art__content {
  display: flex;
  width: 100%;
  height: 100%;
  align-items: center;
  justify-content: center;
}

.mobile-empty-art__icon {
  color: var(--accent);
  font-size: 76px;
  opacity: 0.72;
}

.mobile-now-playing-empty {
  display: flex;
  min-height: 200px;
  flex: 1 0 200px;
  flex-direction: column;
  justify-content: center;
  padding: 14px 4px;
}

.mobile-now-playing-empty__copy {
  min-width: 0;
}

.mobile-now-playing-empty__meta {
  display: flex;
  min-width: 0;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.mobile-now-playing-empty__meta > span {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mobile-now-playing-like:disabled {
  cursor: default;
  opacity: 0.55;
}

.mobile-now-playing-empty__title {
  max-width: 100%;
  margin: 10px 0 0;
  color: var(--text-primary);
  font-size: 26px;
  font-weight: 650;
  line-height: 1.14;
  letter-spacing: 0;
  overflow-wrap: anywhere;
}

.mobile-now-playing-empty__hint {
  margin: 8px 0 0;
  color: var(--text-secondary);
  font-size: 13px;
  line-height: 1.45;
}

.mobile-now-playing-empty__icon {
  align-self: flex-end;
  margin-top: 16px;
  padding: 0 3px 6px;
  border-bottom: 1px solid var(--accent);
  color: var(--accent);
  font-size: 30px;
  opacity: 0.88;
}

.mobile-volume-popover-enter-active,
.mobile-volume-popover-leave-active {
  transition: opacity 240ms cubic-bezier(0.16, 1, 0.3, 1), transform 240ms cubic-bezier(0.16, 1, 0.3, 1);
}

.mobile-volume-popover-enter-from,
.mobile-volume-popover-leave-to {
  opacity: 0;
  transform: translateY(-8px) scale(0.95);
}
</style>
