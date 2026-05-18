<template>
  <div class="flex h-full min-h-0 min-w-0 flex-col items-center justify-center overflow-hidden gap-5">
    <div class="now-playing-cover-stage relative flex min-h-0 w-full flex-1 items-center justify-center group">
      <div
        class="now-playing-cover relative aspect-square shrink overflow-hidden rounded-xl bg-surface-raised album-shadow transition-transform duration-700 hover:scale-[1.02]"
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
          <span class="font-micro text-micro uppercase text-text-secondary">{{ player.isBuffering ? t('player.buffering') : t('player.loading') }}</span>
        </div>

        <div v-if="player.isErrorState" class="absolute inset-x-5 bottom-5 rounded-lg border border-error/30 bg-error-container/70 px-4 py-3 text-on-error-container backdrop-blur-md">
          <div class="font-section-label text-section-label uppercase">{{ t('player.playbackError') }}</div>
          <div class="mt-1 text-caption text-on-error-container/80">{{ t('player.loadError') }}</div>
        </div>
      </div>
    </div>

    <div class="glass-panel player-console w-full shrink-0 h-[256px] flex flex-col rounded-xl px-6 py-5">
      <div class="text-center">
        <div class="mb-2 flex min-w-0 items-center justify-center gap-2">
          <span class="rounded bg-[var(--surface-control)] px-2 py-1 font-micro text-micro uppercase text-text-muted">{{ platformLabel }}</span>
          <span v-if="requesterName" class="min-w-0 max-w-[150px] truncate rounded bg-[var(--surface-control)] px-2 py-1 font-micro text-micro text-text-muted">{{ t('player.requestedBy', { name: requesterName }) }}</span>
        </div>
        <div ref="titleViewportRef" class="track-title-marquee" :class="{ 'track-title-marquee--scroll': shouldScrollTitle }" :title="trackTitle">
          <span ref="titleTrackRef" class="track-title-marquee__track">
            <span>{{ trackTitle }}</span>
            <span v-if="shouldScrollTitle" aria-hidden="true">{{ trackTitle }}</span>
          </span>
        </div>
        <p class="mt-0.5 truncate font-title text-[14px] leading-tight text-primary/80">{{ artistLine }}</p>
      </div>

      <div class="mt-auto flex flex-col gap-3">
        <div class="flex items-center justify-center gap-3">
          <div class="flex items-center justify-end gap-1 w-14">
            <span v-if="!canSeek && durationMs > 0" class="material-symbols-outlined text-[12px] text-text-muted opacity-50" :title="t('player.onlyRequester')">lock</span>
            <span class="text-right font-micro text-micro tabular-nums text-text-muted">{{ formatDuration(progressMs) }}</span>
          </div>
          <button
            class="h-4 flex-1 cursor-pointer rounded-full py-[6px]"
            :class="canSeek ? '' : 'cursor-not-allowed opacity-60'"
            @click="handleSeek"
            :title="canSeek ? t('player.seek') : t('player.onlyRequester')"
          >
            <span class="block h-1 overflow-hidden rounded-full bg-[var(--progress-track)]">
              <span class="block h-full rounded-full bg-primary transition-[width] duration-200" :style="{ width: progressPercent }" />
            </span>
          </button>
          <span class="w-14 font-micro text-micro tabular-nums text-text-muted">{{ formatDuration(durationMs) }}</span>
        </div>

        <div class="flex items-center justify-center gap-5">
          <button
            class="flex h-10 w-10 items-center justify-center text-text-muted transition-colors hover:text-text-primary disabled:cursor-not-allowed disabled:opacity-35"
            :class="player.isShuffle ? 'text-primary' : ''"
            :disabled="player.isShuffleLocked"
            @click="player.toggleShuffle"
            :title="t('player.shuffle')"
          >
            <span class="material-symbols-outlined text-[20px]">shuffle</span>
          </button>
          <button class="flex h-12 w-12 cursor-not-allowed items-center justify-center rounded-full text-text-secondary opacity-35" :title="t('player.prevUnavailable')">
            <span class="material-symbols-outlined" style="font-variation-settings: 'FILL' 1;">skip_previous</span>
          </button>
          <button
            class="flex h-[52px] w-[52px] items-center justify-center rounded-lg bg-primary text-on-primary shadow-lg transition-colors hover:bg-[var(--accent-hover)] active:bg-primary disabled:cursor-not-allowed disabled:opacity-45"
            :disabled="player.isPauseLocked && !player.isPaused"
            @click="player.togglePause"
            :title="player.isPaused ? t('player.play') : t('player.pause')"
          >
            <span v-if="player.isPaused" class="material-symbols-outlined text-[28px]" style="font-variation-settings: 'FILL' 1;">play_arrow</span>
            <span v-else class="material-symbols-outlined text-[28px]" style="font-variation-settings: 'FILL' 1;">pause</span>
          </button>
          <button
            class="flex h-12 w-12 items-center justify-center rounded-full text-text-secondary transition-all hover:bg-[var(--surface-control-hover)] hover:text-text-primary disabled:cursor-not-allowed disabled:opacity-35"
            :disabled="player.isSkipLocked"
            @click="player.playNext"
            :title="t('player.next')"
          >
            <span class="material-symbols-outlined" style="font-variation-settings: 'FILL' 1;">skip_next</span>
          </button>
          <button
            class="flex h-10 w-10 items-center justify-center rounded-md transition-colors hover:bg-[var(--surface-control-hover)] hover:text-primary"
            :class="isLiked ? 'text-primary' : 'text-text-muted'"
            @click="toggleLike"
            :title="isLiked ? t('player.unlike') : t('player.like')"
          >
            <span class="material-symbols-outlined text-[20px]" :style="isLiked ? `font-variation-settings: 'FILL' 1;` : ''">favorite</span>
          </button>
          <button
            class="flex h-10 w-10 items-center justify-center rounded-md text-text-muted transition-colors hover:bg-[var(--surface-control-hover)] hover:text-primary"
            @click="saveCurrentToPersonalPlaylist"
            :title="t('personalPlaylists.saveSong')"
          >
            <span class="material-symbols-outlined text-[20px]">playlist_add</span>
          </button>
        </div>

        <div class="flex w-full items-center justify-between border-t border-border-subtle pt-2.5">
          <div class="flex min-w-0 items-center gap-3">
            <span class="material-symbols-outlined text-[20px] text-primary" style="font-variation-settings: 'FILL' 1;">graphic_eq</span>
            <div class="min-w-0 text-left">
              <div class="font-micro text-micro uppercase leading-none text-text-muted">{{ t('player.nowPlayingIn') }}</div>
              <div class="mt-1 truncate font-compact text-compact leading-none text-text-primary">{{ t('app.lounge') }} • {{ activeUserCount }} {{ t('settings.active') }}</div>
            </div>
          </div>
          <div class="flex items-center gap-4">
            <div class="flex items-center gap-3">
              <button class="flex items-center text-text-muted transition-colors hover:text-text-primary" @click="toggleMute" :title="t('player.volume')">
                <span class="material-symbols-outlined text-[18px]">{{ uiStore.volume === 0 ? 'volume_off' : 'volume_up' }}</span>
              </button>
              <input
                v-model.number="uiStore.volume"
                class="lounge-volume w-20"
                min="0"
                max="1"
                step="0.01"
                type="range"
                :aria-label="t('player.volume')"
              />
            </div>
            <button
              class="flex items-center justify-center text-primary transition-colors hover:text-text-primary"
              :class="{ 'opacity-50': !isQueuePlaced }"
              :title="t('nav.queue')"
              @click="toggleQueueModule"
            >
              <span class="material-symbols-outlined text-[20px]" style="font-variation-settings: 'FILL' 1;">queue_music</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import CoverImage from '../CoverImage.vue';
import { useNowPlayingViewModel } from '../../composables/useNowPlayingViewModel';
import { useLayoutStore } from '../../stores/layout';
import { useUserPlaylistsStore } from '../../stores/userPlaylists';
import { useToast } from '../../composables/useToast';
import { formatDuration } from '../../utils/format';

const { t } = useI18n();
const layoutStore = useLayoutStore();
const userPlaylistsStore = useUserPlaylistsStore();

const {
  player,
  ui: uiStore,
  coverUrl: currentCover,
  trackTitle,
  artistLine,
  platformLabel,
  requesterName,
  durationMs,
  progressMs,
  progressPercent,
  canSeek,
  isLiked,
  activeUserCount,
  seekToRatio,
  toggleLike
} = useNowPlayingViewModel({ artistSeparator: ' • ' });

const titleViewportRef = ref(null);
const titleTrackRef = ref(null);
const shouldScrollTitle = ref(false);

const isQueuePlaced = computed(() => layoutStore.placedModuleIds.includes('queue'));

const toggleQueueModule = () => {
  if (isQueuePlaced.value) {
    // Remove queue from wherever it is
    layoutStore.layout.columns.forEach(col => {
      layoutStore.removeModule('queue', col.id);
    });
  } else {
    // Add to the last column by default
    const lastCol = layoutStore.sortedColumns[layoutStore.sortedColumns.length - 1];
    layoutStore.addModule('queue', lastCol.id);
  }
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

const saveCurrentToPersonalPlaylist = async () => {
  const music = player.value?.nowPlaying?.music || player.nowPlaying?.music;
  if (!music) return;
  const result = await userPlaylistsStore.addTracksToSelected([music], t('personalPlaylists.defaultName'));
  if (!result) return;
  useToast().success(t('personalPlaylists.savedResult', { added: result.addedCount, skipped: result.skippedCount }));
};

const toggleMute = () => {
  uiStore.volume = uiStore.volume === 0 ? 0.75 : 0;
};

const measureTitleOverflow = () => {
  const viewport = titleViewportRef.value;
  const track = titleTrackRef.value;
  if (!viewport || !track) return;
  shouldScrollTitle.value = track.scrollWidth > viewport.clientWidth + 2;
};

let titleResizeObserver;

onMounted(() => {
  measureTitleOverflow();
  if (typeof ResizeObserver !== 'undefined') {
    titleResizeObserver = new ResizeObserver(measureTitleOverflow);
    if (titleViewportRef.value) titleResizeObserver.observe(titleViewportRef.value);
    if (titleTrackRef.value) titleResizeObserver.observe(titleTrackRef.value);
  }
});

onBeforeUnmount(() => {
  titleResizeObserver?.disconnect();
});

watch(trackTitle, async () => {
  shouldScrollTitle.value = false;
  await nextTick();
  measureTitleOverflow();
});
</script>

<style scoped>
.lounge-volume {
  accent-color: #ede1ff;
}

.player-console {
  min-height: 256px;
}

.now-playing-cover {
  width: min(100cqw, 100cqh);
  max-height: 100%;
  min-width: 0;
}

.now-playing-cover-stage {
  container-type: size;
}

.track-title-marquee {
  position: relative;
  width: 100%;
  overflow: hidden;
  white-space: nowrap;
  font-family: var(--font-display, Geist, sans-serif);
  font-size: 25px;
  font-weight: 800;
  line-height: 1.12;
  letter-spacing: 0;
  color: var(--text-primary);
}

.track-title-marquee__track {
  display: inline-flex;
  gap: 2.5rem;
  flex: 0 0 auto;
  min-width: 0;
  padding-inline: 0.25rem;
  transform: translateX(0);
}

.track-title-marquee--scroll .track-title-marquee__track {
  animation: title-marquee 12s linear infinite;
}

@keyframes title-marquee {
  0%, 14% {
    transform: translateX(0);
  }
  86%, 100% {
    transform: translateX(calc(-50% - 1.25rem));
  }
}
</style>
