<template>
  <!-- Floating Album Art & Now Playing Controls -->
  <section class="flex h-full min-h-0 min-w-0 flex-col items-center justify-center overflow-hidden gap-8">
    <div class="relative flex min-h-0 w-full flex-1 items-center justify-center group">
      <div
        class="relative aspect-square h-full max-w-full shrink-0 overflow-hidden rounded-xl bg-surface-raised album-shadow transition-transform duration-700 hover:scale-[1.02]"
        :class="player.isPaused ? 'saturate-[0.86]' : 'saturate-100'"
        @click="handleCoverClick"
      >
        <CoverImage
          :src="currentCover"
          :alt="trackTitle"
          class="absolute inset-0 h-full w-full object-cover transition-transform duration-700 group-hover:scale-[1.04]"
          loading="eager"
        />
        <div class="absolute inset-0 bg-gradient-to-b from-[var(--album-gradient-top)] via-transparent to-[var(--album-gradient-bottom)]" />
        <div class="pointer-events-none absolute inset-0 rounded-xl ring-1 ring-[var(--border-subtle)]" />

        <div v-if="player.isLoading || player.isBuffering" class="absolute inset-0 flex flex-col items-center justify-center gap-4 bg-[var(--media-overlay)] text-text-primary backdrop-blur-xl">
          <div class="h-12 w-12 animate-spin rounded-full border-[3px] border-[var(--surface-control-active)] border-t-[var(--accent)]" />
          <span class="font-micro text-micro uppercase text-text-secondary">{{ player.isBuffering ? 'Buffering' : 'Loading track' }}</span>
        </div>

        <div v-if="player.isErrorState" class="absolute inset-x-5 bottom-5 rounded-lg border border-error/30 bg-error-container/70 px-4 py-3 text-on-error-container backdrop-blur-md">
          <div class="font-section-label text-section-label uppercase">Playback error</div>
          <div class="mt-1 text-caption text-on-error-container/80">The current track could not be loaded.</div>
        </div>
      </div>
    </div>

    <div class="glass-panel w-full shrink-0 max-w-[480px] h-[280px] flex flex-col justify-between rounded-xl p-6 pb-5">
      <div class="text-center">
        <div class="mb-2 flex items-center justify-center gap-2">
          <span class="rounded bg-[var(--surface-control)] px-2 py-1 font-micro text-micro uppercase text-text-muted">{{ platformLabel }}</span>
          <span v-if="requesterName" class="rounded bg-[var(--surface-control)] px-2 py-1 font-micro text-micro uppercase text-text-muted">by {{ requesterName }}</span>
        </div>
        <h1 class="truncate font-display text-[28px] font-bold leading-tight tracking-tight text-text-primary">{{ trackTitle }}</h1>
        <p class="mt-1 truncate font-title text-[15px] text-primary/80">{{ artistLine }}</p>
      </div>

      <div class="mt-auto flex flex-col gap-4">
        <div class="flex items-center justify-center gap-4">
          <span class="w-12 text-right font-micro text-micro tabular-nums text-text-muted">{{ formatDuration(progressMs) }}</span>
          <button
            class="h-4 flex-1 cursor-pointer rounded-full py-[6px]"
            :class="canSeek ? '' : 'cursor-not-allowed opacity-60'"
            @click="handleSeek"
            title="Seek"
          >
            <span class="block h-1 overflow-hidden rounded-full bg-[var(--progress-track)]">
              <span class="block h-full rounded-full bg-primary transition-[width] duration-200" :style="{ width: progressPercent }" />
            </span>
          </button>
          <span class="w-12 font-micro text-micro tabular-nums text-text-muted">{{ formatDuration(durationMs) }}</span>
        </div>

        <div class="flex items-center justify-center gap-6">
          <button
            class="flex h-10 w-10 items-center justify-center text-text-muted transition-colors hover:text-text-primary disabled:cursor-not-allowed disabled:opacity-35"
            :class="player.isShuffle ? 'text-primary' : ''"
            :disabled="player.isShuffleLocked"
            @click="player.toggleShuffle"
            title="Shuffle"
          >
            <span class="material-symbols-outlined text-[20px]">shuffle</span>
          </button>
          <button class="flex h-12 w-12 cursor-not-allowed items-center justify-center rounded-full text-text-secondary opacity-35" title="Previous unavailable">
            <span class="material-symbols-outlined" style="font-variation-settings: 'FILL' 1;">skip_previous</span>
          </button>
          <button
            class="flex h-16 w-16 items-center justify-center rounded-full bg-primary text-on-primary shadow-xl transition-all hover:scale-105 active:scale-95 disabled:cursor-not-allowed disabled:opacity-45"
            :disabled="player.isPauseLocked && !player.isPaused"
            @click="player.togglePause"
            :title="player.isPaused ? 'Play' : 'Pause'"
          >
            <span v-if="player.isPaused" class="material-symbols-outlined text-[32px]" style="font-variation-settings: 'FILL' 1;">play_arrow</span>
            <span v-else class="material-symbols-outlined text-[32px]" style="font-variation-settings: 'FILL' 1;">pause</span>
          </button>
          <button
            class="flex h-12 w-12 items-center justify-center rounded-full text-text-secondary transition-all hover:bg-[var(--surface-control-hover)] hover:text-text-primary disabled:cursor-not-allowed disabled:opacity-35"
            :disabled="player.isSkipLocked"
            @click="player.playNext"
            title="Next"
          >
            <span class="material-symbols-outlined" style="font-variation-settings: 'FILL' 1;">skip_next</span>
          </button>
          <button
            class="flex h-10 w-10 items-center justify-center transition-colors hover:text-primary"
            :class="isLiked ? 'text-primary' : 'text-text-muted'"
            @click="handleLike"
            title="Like"
          >
            <span class="material-symbols-outlined text-[20px]" :style="isLiked ? `font-variation-settings: 'FILL' 1;` : ''">favorite</span>
          </button>
        </div>

        <div class="flex w-full items-center justify-between border-t border-border-subtle pt-3">
          <div class="flex min-w-0 items-center gap-3">
            <span class="material-symbols-outlined text-[20px] text-primary" style="font-variation-settings: 'FILL' 1;">graphic_eq</span>
            <div class="min-w-0 text-left">
              <div class="font-micro text-micro uppercase leading-none text-text-muted">Now playing in</div>
              <div class="mt-1 truncate font-compact text-compact leading-none text-text-primary">Main Lounge • {{ activeUserCount }} Active</div>
            </div>
          </div>
          <div class="flex items-center gap-4">
            <div class="flex items-center gap-3">
              <button class="flex items-center text-text-muted transition-colors hover:text-text-primary" @click="toggleMute" title="Volume">
                <span class="material-symbols-outlined text-[18px]">{{ uiStore.volume === 0 ? 'volume_off' : 'volume_up' }}</span>
              </button>
              <input
                v-model.number="uiStore.volume"
                class="lounge-volume w-20"
                min="0"
                max="1"
                step="0.01"
                type="range"
                aria-label="Volume"
              />
            </div>
            <button class="flex items-center justify-center text-primary transition-colors hover:text-text-primary" title="Queue" @click="emit('toggle-queue')">
              <span class="material-symbols-outlined text-[20px]" style="font-variation-settings: 'FILL' 1;">queue_music</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </section>

  <!-- Scrolling Lyrics Column -->
  <section class="flex h-full min-h-0 min-w-0 flex-col justify-center overflow-hidden">
    <AppleLyricsPanel
      class="h-full w-full"
      :class="isQueueVisible ? 'pr-4' : 'pr-8'"
      :lyrics="player.lyricDetail.lyric || player.lyricText"
      :translated-lyrics="player.lyricDetail.translatedLyric"
      :show-translation="uiStore.showLyricTranslation"
      :current-time-ms="progressMs"
      :is-playing="!player.isPaused"
      :is-dark-mode="uiStore.isDarkMode"
      :bg-color="ambientAccent"
      :lyrics-loaded="!!currentMusic"
      @toggle-translation="uiStore.toggleLyricTranslation"
    />
  </section>
</template>

<script setup>
import { computed, watch } from 'vue';
import CoverImage from './CoverImage.vue';
import AppleLyricsPanel from './AppleLyricsPanel.vue';
import { usePlayerStore } from '../stores/player';
import { useUserStore } from '../stores/user';
import { useUiStore } from '../stores/ui';
import { formatDuration } from '../utils/format';

const player = usePlayerStore();
const userStore = useUserStore();
const uiStore = useUiStore();

defineProps({
  isQueueVisible: {
    type: Boolean,
    default: true
  }
});
const emit = defineEmits(['toggle-queue']);

const currentMusic = computed(() => player.nowPlaying?.music || null);
const currentCover = computed(() => currentMusic.value?.coverUrl || '');
const trackTitle = computed(() => currentMusic.value?.name || 'Waiting for the first track');
const artistLine = computed(() => {
  const artists = currentMusic.value?.artists;
  return Array.isArray(artists) && artists.length ? artists.join(' • ') : 'MusicParty';
});
const platformLabel = computed(() => {
  const platform = currentMusic.value?.platform;
  if (platform === 'netease') return 'Netease';
  if (platform === 'bilibili') return 'Bilibili';
  if (platform === 'navidrome') return 'Navidrome';
  return 'Room';
});
const requesterName = computed(() => {
  const requester = player.nowPlaying?.requestedBy || player.nowPlaying?.userId || player.nowPlaying?.requesterId;
  return requester ? userStore.resolveName(requester) : '';
});
const durationMs = computed(() => currentMusic.value?.duration || 0);
const progressMs = computed(() => player.playbackPositionMs || 0);
const progressPercent = computed(() => {
  if (!durationMs.value) return '0%';
  return `${Math.max(0, Math.min(100, (progressMs.value / durationMs.value) * 100))}%`;
});
const canSeek = computed(() => !!currentMusic.value && !player.isSeekingPreview && durationMs.value > 0);
const isLiked = computed(() => player.isSongLiked(currentMusic.value));
const activeUserCount = computed(() => userStore.onlineUsers.length || 1);
const ambientAccent = computed(() => uiStore.dynamicAccent?.accent || '#ede1ff');

const handleSeek = (event) => {
  if (!canSeek.value) return;
  const rect = event.currentTarget.getBoundingClientRect();
  const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
  player.seek(Math.floor(ratio * durationMs.value));
};

const handleLike = () => {
  if (!currentMusic.value) return;
  player.sendLike();
};

const handleCoverClick = () => {
  handleLike();
};

const toggleMute = () => {
  uiStore.volume = uiStore.volume === 0 ? 0.75 : 0;
};

watch(currentCover, (coverUrl) => {
  uiStore.updateAccentFromCover(coverUrl);
}, { immediate: true });
</script>

<style scoped>
.lounge-volume {
  accent-color: #ede1ff;
}

section {
  --player-panel-height: 280px;
}
</style>
