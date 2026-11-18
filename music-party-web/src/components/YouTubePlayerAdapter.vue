<template>
  <div class="youtube-player-host">
    <div id="yt-player-container"></div>
  </div>
  <button
    v-if="needsUserGesture"
    type="button"
    class="fixed bottom-24 left-1/2 z-[var(--z-toast)] -translate-x-1/2 rounded-full bg-primary px-5 py-3 text-sm font-black text-on-primary shadow-xl shadow-black/20"
    @click="enableSound"
  >
    {{ ui.locale === 'zh' ? '\u70b9\u51fb\u542f\u7528\u58f0\u97f3' : 'Tap to enable sound' }}
  </button>
</template>

<script setup>
import { ref, watch, onMounted, onUnmounted } from 'vue';
import { useAudioPlayerAdapter } from '../composables/useAudioPlayerAdapter';
import { useUiStore } from '../stores/ui';

let youtubeApiPromise;

function loadYouTubeAPI() {
  if (window.YT?.Player) return Promise.resolve(window.YT);
  if (youtubeApiPromise) return youtubeApiPromise;

  youtubeApiPromise = new Promise((resolve) => {
    const previousCallback = window.onYouTubeIframeAPIReady;
    window.onYouTubeIframeAPIReady = () => {
      previousCallback?.();
      resolve(window.YT);
    };
    if (!document.querySelector('script[src="https://www.youtube.com/iframe_api"]')) {
      const script = document.createElement('script');
      script.src = 'https://www.youtube.com/iframe_api';
      script.async = true;
      document.head.appendChild(script);
    }
  });
  return youtubeApiPromise;
}

const player = useAudioPlayerAdapter();
const ui = useUiStore();
const needsUserGesture = ref(false);
let ytPlayer;
let destroyed = false;
let syncInterval;
let bufferingTimeout;
let autoplayTimeout;
let autoplayPending = false;
let bufferingReported = false;
let currentVideoId = '';

function clearPlaybackTimers() {
  clearInterval(syncInterval);
  clearTimeout(bufferingTimeout);
  clearTimeout(autoplayTimeout);
  syncInterval = undefined;
  bufferingTimeout = undefined;
  autoplayTimeout = undefined;
}

function seekToRoomPosition() {
  if (!ytPlayer || !currentVideoId) return;
  const positionMs = player.getCurrentProgress();
  ytPlayer.seekTo(Math.max(0, positionMs) / 1000, true);
  player.forceNextSyncSeek = false;
}

function loadCurrentVideo(embedId) {
  if (!embedId || !ytPlayer) return;
  currentVideoId = embedId;
  needsUserGesture.value = false;
  autoplayPending = !player.isPaused;
  if (player.isPaused) {
    ytPlayer.cueVideoById({
      videoId: embedId,
      startSeconds: Math.max(0, player.getCurrentProgress()) / 1000,
    });
  } else {
    ytPlayer.loadVideoById({ videoId: embedId });
  }
}

function reportError() {
  const now = player.nowPlaying;
  if (now?.queueId) {
    player.reportPlayback('playback.error', {
      playEpoch: now.playEpoch,
      queueId: now.queueId,
      code: 'stream-unavailable',
    });
  }
}

function onPlayerStateChange(event) {
  const YT = window.YT;
  if (!YT) return;
  if (event.data === YT.PlayerState.PLAYING) {
    autoplayPending = false;
    needsUserGesture.value = false;
    player.isBuffering = false;
    bufferingReported = false;
    clearTimeout(bufferingTimeout);
    bufferingTimeout = undefined;
    const now = player.nowPlaying;
    if (now?.queueId) {
      player.reportPlayback('playback.started', {
        playEpoch: now.playEpoch,
        queueId: now.queueId,
        positionMs: Math.round(ytPlayer.getCurrentTime() * 1000),
        clientTime: player.serverNow(),
        durationMs: Math.round(ytPlayer.getDuration() * 1000),
      });
    }
    clearInterval(syncInterval);
    syncInterval = setInterval(() => {
      player.setPlaybackPosition(Math.round(ytPlayer.getCurrentTime() * 1000));
    }, 250);
  } else if (event.data === YT.PlayerState.BUFFERING) {
    player.isBuffering = true;
    clearTimeout(bufferingTimeout);
    bufferingReported = false;
    bufferingTimeout = setTimeout(() => {
      if (destroyed || !player.isBuffering || bufferingReported) return;
      bufferingReported = true;
      const now = player.nowPlaying;
      if (now?.queueId) {
        player.reportPlayback('playback.stalled', {
          playEpoch: now.playEpoch,
          queueId: now.queueId,
          positionMs: Math.round(ytPlayer.getCurrentTime() * 1000),
        });
      }
    }, 3000);
  } else if (event.data === YT.PlayerState.PAUSED) {
    clearInterval(syncInterval);
    syncInterval = undefined;
    player.isBuffering = false;
    if (autoplayPending && !player.isPaused) {
      clearTimeout(autoplayTimeout);
      autoplayTimeout = setTimeout(() => {
        if (autoplayPending && !player.isPaused && !destroyed) needsUserGesture.value = true;
      }, 500);
    }
  } else if (event.data === YT.PlayerState.ENDED) {
    clearInterval(syncInterval);
    syncInterval = undefined;
    player.isBuffering = false;
    autoplayPending = false;
    const now = player.nowPlaying;
    if (now?.queueId) player.reportPlayback('playback.ended', { playEpoch: now.playEpoch, queueId: now.queueId });
  }
}

function onPlayerError(event) {
  reportError();
  if (event.data === 5) needsUserGesture.value = true;
}

function enableSound() {
  if (!ytPlayer) return;
  autoplayPending = true;
  ytPlayer.playVideo();
  needsUserGesture.value = false;
}

watch(() => player.nowPlaying?.music?.embedId, loadCurrentVideo);
watch(() => player.isPaused, (paused) => {
  if (!ytPlayer || !currentVideoId) return;
  if (paused) {
    ytPlayer.pauseVideo();
  } else {
    autoplayPending = true;
    ytPlayer.playVideo();
  }
});
watch(() => player.forceNextSyncSeek, (shouldSeek) => {
  if (shouldSeek) seekToRoomPosition();
});

onMounted(async () => {
  const YT = await loadYouTubeAPI();
  if (destroyed) return;
  ytPlayer = new YT.Player('yt-player-container', {
    width: '1',
    height: '1',
    videoId: player.nowPlaying?.music?.embedId,
    playerVars: {
      autoplay: 1,
      controls: 0,
      disablekb: 1,
      enablejsapi: 1,
      origin: window.location.origin,
      rel: 0,
    },
    events: {
      onReady: () => loadCurrentVideo(player.nowPlaying?.music?.embedId),
      onStateChange: onPlayerStateChange,
      onError: onPlayerError,
    },
  });
});

onUnmounted(() => {
  destroyed = true;
  clearPlaybackTimers();
  ytPlayer?.destroy();
  ytPlayer = undefined;
  player.isBuffering = false;
});
</script>

<style scoped>
.youtube-player-host {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
  overflow: hidden;
}
</style>
