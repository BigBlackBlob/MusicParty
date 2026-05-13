<template>
  <section class="flex h-full flex-col overflow-y-auto px-6 pt-8 pb-10 safe-area-bottom">
    <div
      class="flex flex-1 flex-col items-center justify-center gap-8"
      :class="hasLyrics ? 'md:pt-4' : 'pb-10'"
    >
      <!-- Cover Art -->
      <div
        class="relative aspect-square w-full max-w-[300px] overflow-hidden rounded-[var(--radius-lg)] bg-[var(--surface-3)] shadow-2xl transition-all border border-[var(--border-default)]"
        :class="hasLyrics ? 'scale-90' : 'scale-100'"
      >
        <CoverImage
          :src="music?.coverUrl"
          :alt="music?.name"
          loading="eager"
          decoding="async"
          class="h-full w-full object-cover transition-transform duration-700"
          :class="player.isPaused ? 'scale-100' : 'scale-110'"
        />
      </div>

      <!-- Info Area -->
      <div class="w-full text-left">
        <MobileMiniLyrics
          v-if="hasLyrics"
          class="mb-6"
          :lyrics="player.lyricDetail.lyric || player.lyricText"
          :translated-lyrics="ui.showLyricTranslation ? player.lyricDetail.translatedLyric : ''"
          :show-translation="ui.showLyricTranslation"
          :current-time-ms="player.playbackPositionMs"
          @open="lyricsExpanded = true"
        />

        <div class="flex items-center gap-2 mb-2">
          <div class="h-1.5 w-1.5 rounded-full bg-[var(--accent)] animate-pulse" v-if="!player.isPaused"></div>
          <span class="text-[10px] font-black uppercase tracking-[0.2em] text-[var(--accent)]">
            {{ player.isLoading ? 'Syncing' : nowPlaying ? 'Now Playing' : 'Idle' }}
          </span>
        </div>

        <h1 class="text-3xl font-black leading-tight text-[var(--text-primary)] tracking-tighter line-clamp-2">
          {{ music?.name || 'Waiting...' }}
        </h1>
        <p class="mt-1 text-base font-bold text-[var(--text-secondary)] truncate">
          {{ music?.artists?.join(' / ') || 'Music Party' }}
        </p>
        <p v-if="nowPlaying?.enqueuedByName" class="mt-4 text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-widest">
          Requested by {{ nowPlaying.enqueuedByName }}
        </p>
      </div>
    </div>

    <!-- Controls Card -->
    <div class="mt-auto p-6 rounded-[var(--radius-lg)] border border-[var(--border-default)] bg-[var(--surface-1)] shadow-xl flex flex-col gap-6">
      <ProgressScrubber
        :current-ms="player.playbackPositionMs"
        :duration="music?.duration || 0"
        :can-seek="canSeek"
        :is-error="player.isErrorState"
        @seek="player.seek"
      />

      <div class="flex items-center justify-between">
        <div class="relative">
          <IconButton @click="toggleVolumePanel" :variant="volumePanelOpen ? 'primary' : 'secondary'">
            <VolumeX v-if="ui.volume === 0" class="h-5 w-5" />
            <Volume1 v-else-if="ui.volume < 0.5" class="h-5 w-5" />
            <Volume2 v-else class="h-5 w-5" />
          </IconButton>

          <Transition name="mobile-volume-popover">
            <div v-if="volumePanelOpen" ref="volumePanelRef" class="absolute bottom-full left-1/2 -translate-x-1/2 mb-4 p-4 bg-[var(--surface-4)] border border-[var(--border-default)] rounded-[var(--radius-lg)] shadow-2xl flex flex-col items-center gap-4 min-w-[64px]">
              <span class="text-[10px] font-bold text-[var(--text-primary)]">{{ Math.round(ui.volume * 100) }}%</span>
              <div ref="volumeTrackRef" class="h-32 w-8 relative flex flex-col items-center" @pointerdown="handleVolumePointerDown">
                <div class="w-1.5 h-full bg-[var(--progress-track)] rounded-full overflow-hidden relative">
                  <div class="absolute bottom-0 left-0 right-0 bg-[var(--accent)] rounded-full" :style="{ height: ui.volume * 100 + '%' }"></div>
                </div>
              </div>
            </div>
          </Transition>
        </div>

        <IconButton @click="player.toggleShuffle" :variant="player.isShuffle ? 'primary' : 'ghost'">
          <Shuffle class="h-5 w-5" />
        </IconButton>

        <IconButton size="xl" variant="primary" radius="lg" @click="player.togglePause">
          <Play v-if="player.isPaused" class="h-8 w-8 fill-current ml-1" />
          <Pause v-else class="h-8 w-8 fill-current" />
        </IconButton>

        <IconButton @click="player.playNext">
          <SkipForward class="h-5 w-5 fill-current" />
        </IconButton>

        <IconButton @click="handleMobileLike" :variant="hasLiked ? 'primary' : 'ghost'">
          <Heart class="h-5 w-5" :class="hasLiked ? 'fill-current' : ''" />
        </IconButton>
      </div>
    </div>

    <!-- Lyrics Overlay -->
    <Transition name="slide-up">
      <div v-if="lyricsExpanded" class="fixed inset-0 z-[var(--z-modal)] bg-[var(--surface-0)] flex flex-col" @click.self="lyricsExpanded = false">
        <div class="px-6 py-4 flex items-center justify-between border-b border-[var(--border-default)] pt-safe">
          <div class="min-w-0">
            <h3 class="text-sm font-bold text-[var(--text-primary)] truncate">{{ music?.name }}</h3>
            <p class="text-[10px] font-bold text-[var(--text-tertiary)] uppercase tracking-widest">Full Lyrics</p>
          </div>
          <IconButton variant="secondary" size="sm" @click="lyricsExpanded = false">
            <X class="h-4 w-4" />
          </IconButton>
        </div>
        <div class="flex-1 overflow-hidden p-6 bg-[var(--surface-1)]">
          <AppleLyricsPanel
            :lyrics="player.lyricDetail.lyric || player.lyricText"
            :translated-lyrics="player.lyricDetail.translatedLyric"
            :show-translation="ui.showLyricTranslation"
            :current-time-ms="player.playbackPositionMs"
            :is-playing="!player.isPaused"
            :is-dark-mode="ui.isDarkMode"
            :bg-color="ui.dynamicAccent?.accent || ''"
            @toggle-translation="ui.toggleLyricTranslation"
          />
        </div>
      </div>
    </Transition>
  </section>
</template>

<script setup>
import { computed, onUnmounted, ref, watch } from 'vue';
import { Heart, Pause, Play, Shuffle, SkipForward, Volume1, Volume2, VolumeX, X } from 'lucide-vue-next';
import { usePlayerStore } from '../../stores/player';
import { useUiStore } from '../../stores/ui';
import { useUserStore } from '../../stores/user';
import { useToast } from '../../composables/useToast';
import { parseLyrics } from '../../utils/parser';
import AppleLyricsPanel from '../AppleLyricsPanel.vue';
import CoverImage from '../CoverImage.vue';
import MobileMiniLyrics from './MobileMiniLyrics.vue';

// UI Primitives
import IconButton from '../ui/IconButton.vue';
import ProgressScrubber from '../ui/ProgressScrubber.vue';

defineEmits(['open-search']);

const player = usePlayerStore();
const ui = useUiStore();
const user = useUserStore();
const { error } = useToast();
const nowPlaying = computed(() => player.nowPlaying);
const music = computed(() => nowPlaying.value?.music);
const lyricsExpanded = ref(false);
const volumePanelOpen = ref(false);
const volumePanelRef = ref(null);
const volumeTrackRef = ref(null);
const activeVolumePointerId = ref(null);
const lyricLines = computed(() => parseLyrics(player.lyricDetail.lyric || player.lyricText));
const hasLyrics = computed(() => lyricLines.value.length >= 5);
const canSeek = computed(() => !!nowPlaying.value && nowPlaying.value.enqueuedById === user.userToken);
const hasLiked = computed(() => player.isSongLiked(music.value));

const toggleVolumePanel = () => {
  volumePanelOpen.value = !volumePanelOpen.value;
};

const closeVolumePanel = () => {
  volumePanelOpen.value = false;
};

const setVolumeFromPointer = (e) => {
  if (!volumeTrackRef.value) return;
  const rect = volumeTrackRef.value.getBoundingClientRect();
  const y = Math.max(0, Math.min(rect.height, e.clientY - rect.top));
  const nextVolume = 1 - (y / rect.height);
  ui.setVolume(parseFloat(nextVolume.toFixed(2)));
};

const handleVolumePointerDown = (e) => {
  e.preventDefault();
  activeVolumePointerId.value = e.pointerId;
  volumeTrackRef.value?.setPointerCapture?.(e.pointerId);
  setVolumeFromPointer(e);
  window.addEventListener('pointermove', handleVolumePointerMove);
  window.addEventListener('pointerup', handleVolumePointerUp);
  window.addEventListener('pointercancel', handleVolumePointerCancel);
};

const handleVolumePointerMove = (e) => {
  if (activeVolumePointerId.value !== null && e.pointerId !== activeVolumePointerId.value) return;
  setVolumeFromPointer(e);
};

const cleanupVolumeDrag = (e) => {
  const pointerId = activeVolumePointerId.value ?? e?.pointerId;
  if (pointerId !== undefined && volumeTrackRef.value?.hasPointerCapture?.(pointerId)) {
    volumeTrackRef.value.releasePointerCapture(pointerId);
  }
  activeVolumePointerId.value = null;
  window.removeEventListener('pointermove', handleVolumePointerMove);
  window.removeEventListener('pointerup', handleVolumePointerUp);
  window.removeEventListener('pointercancel', handleVolumePointerCancel);
};

const handleVolumePointerUp = (e) => {
  if (activeVolumePointerId.value !== null && e.pointerId !== activeVolumePointerId.value) return;
  cleanupVolumeDrag(e);
};

const handleVolumePointerCancel = (e) => {
  if (activeVolumePointerId.value !== null && e.pointerId !== activeVolumePointerId.value) return;
  cleanupVolumeDrag(e);
};

const handleMobileLike = () => {
  if (!nowPlaying.value) return;
  player.sendLike();
};

const handleDocumentPointerDown = (e) => {
  if (!volumePanelOpen.value) return;
  if (volumePanelRef.value?.contains(e.target)) return;
  closeVolumePanel();
};

watch(() => music.value?.coverUrl, (coverUrl) => {
  ui.updateAccentFromCover(coverUrl);
}, { immediate: true });

watch(volumePanelOpen, (open) => {
  if (open) {
    document.addEventListener('pointerdown', handleDocumentPointerDown, true);
  } else {
    document.removeEventListener('pointerdown', handleDocumentPointerDown, true);
  }
});

onUnmounted(() => {
  window.removeEventListener('pointermove', handleVolumePointerMove);
  window.removeEventListener('pointerup', handleVolumePointerUp);
  window.removeEventListener('pointercancel', handleVolumePointerCancel);
  document.removeEventListener('pointerdown', handleDocumentPointerDown, true);
});
</script>

<style scoped>
.safe-area-bottom {
  padding-bottom: calc(2rem + env(safe-area-inset-bottom));
}

.pt-safe {
  padding-top: env(safe-area-inset-top);
}

.mobile-volume-popover-enter-active,
.mobile-volume-popover-leave-active {
  transition: opacity 0.2s ease, transform 0.2s cubic-bezier(0.16, 1, 0.3, 1);
}

.mobile-volume-popover-enter-from,
.mobile-volume-popover-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(8px) scale(0.95);
}

.slide-up-enter-active, .slide-up-leave-active {
  transition: transform 0.3s cubic-bezier(0.16, 1, 0.3, 1), opacity 0.2s ease;
}
.slide-up-enter-from { transform: translateY(100%); opacity: 0; }
.slide-up-leave-to { transform: translateY(100%); opacity: 0; }
</style>

<style scoped>
.mobile-now-playing {
  padding-bottom: calc(1rem + env(safe-area-inset-bottom));
  overscroll-behavior: contain;
}

.mobile-lyrics-overlay {
  display: flex;
  height: var(--app-height, 100dvh);
  min-height: var(--app-height, 100dvh);
  flex-direction: column;
  overflow: hidden;
  padding-top: calc(env(safe-area-inset-top) + 1rem);
  padding-bottom: calc(env(safe-area-inset-bottom) + 1rem);
}

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

.mobile-controls-row {
  display: grid;
  grid-template-columns: 3rem 3rem 4rem 3rem 3rem;
  align-items: center;
  justify-content: center;
  gap: 1rem;
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

.mobile-volume-panel {
  position: absolute;
  bottom: calc(100% + 0.75rem);
  left: 50%;
  z-index: 20;
  display: flex;
  min-width: 4.75rem;
  min-height: 12rem;
  transform: translateX(-50%);
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  border: 1px solid var(--border-default);
  border-radius: 1.5rem;
  background: var(--surface-4);
  padding: 0.85rem 0.75rem;
  box-shadow: 0 22px 60px rgba(0, 0, 0, 0.28);
}

.mobile-volume-track {
  position: relative;
  display: flex;
  width: 2.75rem;
  min-height: 8.5rem;
  touch-action: none;
  align-items: center;
  justify-content: center;
  outline: none;
}

.mobile-volume-track:focus-visible {
  border-radius: 1rem;
  box-shadow: 0 0 0 3px var(--accent-muted);
}

.mobile-volume-track__rail {
  position: relative;
  height: 8rem;
  width: 0.55rem;
  overflow: hidden;
  border-radius: 9999px;
  background: var(--progress-track);
}

.mobile-volume-track__fill {
  position: absolute;
  inset-inline: 0;
  bottom: 0;
  border-radius: 9999px;
  background: var(--accent);
}

.mobile-volume-track__thumb {
  position: absolute;
  left: 50%;
  width: 1.15rem;
  height: 1.15rem;
  transform: translateX(-50%);
  border: 3px solid var(--surface-4);
  border-radius: 9999px;
  background: var(--accent);
  box-shadow: 0 8px 18px rgba(0, 0, 0, 0.24);
}

.mobile-volume-popover-enter-active,
.mobile-volume-popover-leave-active {
  transition: opacity 180ms ease, transform 180ms ease;
}

.mobile-volume-popover-enter-from,
.mobile-volume-popover-leave-to {
  opacity: 0;
  transform: translateX(-50%) translateY(0.35rem) scale(0.96);
}

@media (prefers-reduced-motion: reduce) {
  .mobile-volume-popover-enter-active,
  .mobile-volume-popover-leave-active {
    transition-duration: 0.01ms;
  }
}
</style>
