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
          :lyrics="player.lyricDetail.lyric || player.lyricText"
          :translated-lyrics="player.lyricDetail.translatedLyric"
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

      <div class="mobile-controls-row mt-5">
        <div class="relative">
          <button
            class="mobile-control"
            :class="volumePanelOpen ? 'border-[var(--border-accent)] text-[var(--accent)]' : ''"
            @click="toggleVolumePanel"
            :aria-label="`音量 ${Math.round(ui.volume * 100)}%`"
            :aria-expanded="volumePanelOpen"
            aria-controls="mobile-volume-panel"
          >
            <VolumeX v-if="ui.volume === 0" class="h-5 w-5" />
            <Volume1 v-else-if="ui.volume < 0.5" class="h-5 w-5" />
            <Volume2 v-else class="h-5 w-5" />
          </button>

          <Transition name="mobile-volume-popover">
            <div
              v-if="volumePanelOpen"
              id="mobile-volume-panel"
              ref="volumePanelRef"
              class="mobile-volume-panel"
              role="group"
              aria-label="音量调节"
            >
              <div class="text-[11px] font-bold tabular-nums text-[var(--text-primary)]">
                {{ Math.round(ui.volume * 100) }}%
              </div>
              <div
                ref="volumeTrackRef"
                class="mobile-volume-track"
                role="slider"
                tabindex="0"
                aria-label="音量"
                aria-valuemin="0"
                aria-valuemax="100"
                :aria-valuenow="Math.round(ui.volume * 100)"
                @pointerdown="handleVolumePointerDown"
                @pointercancel="handleVolumePointerCancel"
                @keydown="handleVolumeKeydown"
              >
                <div class="mobile-volume-track__rail">
                  <div class="mobile-volume-track__fill" :style="{ height: `${ui.volume * 100}%` }"></div>
                </div>
                <div class="mobile-volume-track__thumb" :style="{ bottom: `calc(${ui.volume * 100}% - 0.45rem)` }"></div>
              </div>
            </div>
          </Transition>
        </div>
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
        <button
          class="mobile-control"
          :class="hasLiked ? 'border-[var(--border-accent)] text-[var(--accent)]' : ''"
          @click="handleMobileLike"
          :aria-label="hasLiked ? '已喜欢当前歌曲' : '喜欢当前歌曲'"
          :aria-pressed="hasLiked"
        >
          <Heart class="h-5 w-5" :class="hasLiked ? 'fill-current stroke-none' : ''" />
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
        :lyrics="player.lyricDetail.lyric || player.lyricText"
        :translated-lyrics="player.lyricDetail.translatedLyric"
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
import { Heart, Pause, Play, Shuffle, SkipForward, Volume1, Volume2, VolumeX } from 'lucide-vue-next';
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
const volumePanelOpen = ref(false);
const volumePanelRef = ref(null);
const volumeTrackRef = ref(null);
const activeVolumePointerId = ref(null);
const lyricLines = computed(() => parseLyrics(player.lyricDetail.lyric || player.lyricText));
const hasLyrics = computed(() => lyricLines.value.length >= 5);
const canSeek = computed(() => !!nowPlaying.value && nowPlaying.value.enqueuedById === user.userToken);
const hasLiked = computed(() => (
  player.nowPlaying?.likedUserIds?.includes(user.userToken) || player.isSongLiked(music.value)
));
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

const handleVolumeKeydown = (e) => {
  const step = e.shiftKey ? 0.1 : 0.05;
  if (e.key === 'ArrowUp' || e.key === 'ArrowRight') {
    e.preventDefault();
    ui.setVolume(ui.volume + step);
  }
  if (e.key === 'ArrowDown' || e.key === 'ArrowLeft') {
    e.preventDefault();
    ui.setVolume(ui.volume - step);
  }
  if (e.key === 'Home') {
    e.preventDefault();
    ui.setVolume(0);
  }
  if (e.key === 'End') {
    e.preventDefault();
    ui.setVolume(1);
  }
  if (e.key === 'Escape') {
    e.preventDefault();
    closeVolumePanel();
  }
};

const handleMobileLike = () => {
  if (!nowPlaying.value || hasLiked.value) return;
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
  window.removeEventListener('pointermove', handleProgressPointerMove);
  window.removeEventListener('pointerup', handleProgressPointerUp);
  window.removeEventListener('pointercancel', handleProgressPointerCancel);
  window.removeEventListener('pointermove', handleVolumePointerMove);
  window.removeEventListener('pointerup', handleVolumePointerUp);
  window.removeEventListener('pointercancel', handleVolumePointerCancel);
  document.removeEventListener('pointerdown', handleDocumentPointerDown, true);
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
