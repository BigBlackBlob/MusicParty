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
          @click="$emit('open-settings')"
        >
          <span class="material-symbols-outlined">settings</span>
        </button>
      </div>
    </header>

    <!-- Main Content Area -->
    <main class="flex-1 flex flex-col px-md w-full overflow-y-auto overflow-x-hidden pb-[92px]">
      <!-- Artwork Section -->
      <div class="w-full aspect-square mt-2 mb-6 rounded-2xl overflow-hidden shadow-[0_30px_60px_rgba(0,0,0,0.6)] flex-shrink-0 relative z-10 group">
        <CoverImage
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

      <!-- Track Info & Like -->
      <div class="flex items-center justify-between mb-8 flex-shrink-0 z-10 px-1">
        <div class="flex flex-col overflow-hidden">
          <h1 class="font-title text-title text-primary truncate tracking-tight">{{ trackTitle }}</h1>
          <p class="font-body text-body text-text-secondary truncate mt-1">{{ artistLine }}</p>
        </div>
        <button 
          @click="toggleLike"
          :disabled="!music"
          class="ml-4 w-[44px] h-[44px] flex items-center justify-center rounded-full text-primary hover:bg-accent-subtle transition-all active:scale-90 flex-shrink-0"
        >
          <span class="material-symbols-outlined text-[28px]" :style="isLiked ? 'font-variation-settings: \'FILL\' 1;' : ''">favorite</span>
        </button>
      </div>

      <!-- Progress Bar -->
      <div class="flex flex-col gap-xs mb-8 flex-shrink-0 z-10 px-1">
        <ProgressScrubber
          :current-ms="player.playbackPositionMs"
          :duration="durationMs"
          :can-seek="canSeek"
          :is-error="player.isErrorState"
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
      <div class="flex items-center justify-between mb-8 flex-shrink-0 px-1 z-10 w-full max-w-[400px] mx-auto">
        <div class="relative" ref="volumePanelContainerRef">
          <button 
            @click="toggleVolumePanel"
            class="w-[36px] h-[36px] flex items-center justify-center transition-colors active:scale-90"
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

        <button 
          @click="player.toggleShuffle"
          :disabled="player.isShuffleLocked"
          class="w-[36px] h-[36px] flex items-center justify-center transition-colors active:scale-90"
          :class="player.isShuffle ? 'text-primary' : 'text-text-secondary hover:text-primary'"
        >
          <span class="material-symbols-outlined text-[24px]">shuffle</span>
        </button>
        <button 
          @click="player.playPrevious"
          class="w-[44px] h-[44px] flex items-center justify-center text-primary hover:text-white transition-colors active:scale-90"
        >
          <span class="material-symbols-outlined text-[36px]" style="font-variation-settings: 'FILL' 1;">skip_previous</span>
        </button>
        <button 
          @click="player.togglePause"
          :disabled="player.isPauseLocked && !player.isPaused"
          class="w-[64px] h-[64px] flex items-center justify-center text-primary hover:text-white hover:scale-105 transition-all active:scale-95 shadow-lg shadow-primary/20 rounded-full bg-accent-subtle"
        >
          <span class="material-symbols-outlined text-[40px]" style="font-variation-settings: 'FILL' 1;">
            {{ player.isPaused ? 'play_arrow' : 'pause' }}
          </span>
        </button>
        <button 
          @click="player.playNext"
          :disabled="player.isSkipLocked"
          class="w-[44px] h-[44px] flex items-center justify-center text-primary hover:text-white transition-colors active:scale-90"
        >
          <span class="material-symbols-outlined text-[36px]" style="font-variation-settings: 'FILL' 1;">skip_next</span>
        </button>
        <button 
          @click="player.toggleRepeat"
          class="w-[36px] h-[36px] flex items-center justify-center transition-colors active:scale-90"
          :class="player.repeatMode !== 'none' ? 'text-primary' : 'text-text-secondary hover:text-primary'"
        >
          <span class="material-symbols-outlined text-[24px]">
            {{ player.repeatMode === 'one' ? 'repeat_one' : 'repeat' }}
          </span>
        </button>
      </div>

      <!-- Integrated Scrolling Lyrics Section -->
      <div v-if="hasLyrics" class="flex-1 min-h-[300px] relative -mx-md px-md mb-8">
        <!-- Top Gradient Fade for Lyrics -->
        <div class="absolute top-0 left-0 w-full h-12 bg-gradient-to-b from-bg-base via-bg-base/80 to-transparent z-10 pointer-events-none"></div>
        
        <div class="flex flex-col gap-6 pt-6 pb-12">
          <AppleLyricsPanel
            class="h-full"
            :lyrics="player.lyricDetail.lyric || player.lyricText"
            :translated-lyrics="player.lyricDetail.translatedLyric"
            :show-translation="ui.showLyricTranslation"
            :current-time-ms="player.playbackPositionMs"
            :is-playing="!player.isPaused"
            :is-dark-mode="ui.isDarkMode"
            :bg-color="'transparent'"
            mobile
            @toggle-translation="ui.toggleLyricTranslation"
          />
        </div>

        <!-- Bottom Gradient Fade for Lyrics -->
        <div class="absolute bottom-0 left-0 w-full h-24 bg-gradient-to-t from-bg-base via-bg-base/90 to-transparent z-10 pointer-events-none"></div>
      </div>

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
import { parseLyrics } from '../../utils/parser';
import AppleLyricsPanel from '../AppleLyricsPanel.vue';
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

const lyricLines = computed(() => parseLyrics(player.lyricDetail.lyric || player.lyricText));

const hasLyrics = computed(() => lyricLines.value.length >= 5);

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
