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
import { computed, ref, watch, onMounted, onUnmounted } from 'vue';
import { useAudioPlayerAdapter } from '../composables/useAudioPlayerAdapter';
import { useUiStore } from '../stores/ui';
import { getWidgetVolume } from '../utils/playbackVolume';
import { useToastStore } from '../stores/toast';

let youtubeApiPromise;

function loadYouTubeAPI() {
  if (window.YT?.Player) return Promise.resolve(window.YT);
  if (youtubeApiPromise) return youtubeApiPromise;

  youtubeApiPromise = new Promise((resolve, reject) => {
    const previousCallback = window.onYouTubeIframeAPIReady;
    let script = document.querySelector('script[src="https://www.youtube.com/iframe_api"]');
    const owned = !script;
    const cleanup = () => {
      clearTimeout(timeout);
      script?.removeEventListener('error', fail);
      window.onYouTubeIframeAPIReady = previousCallback;
    };
    const fail = () => {
      cleanup();
      if (owned) script?.remove();
      youtubeApiPromise = undefined;
      reject(new Error('YouTube API unavailable'));
    };
    const timeout = setTimeout(fail, 15000);
    window.onYouTubeIframeAPIReady = () => {
      cleanup();
      previousCallback?.();
      resolve(window.YT);
    };
    if (!script) {
      script = document.createElement('script');
      script.src = 'https://www.youtube.com/iframe_api';
      script.async = true;
    }
    script.addEventListener('error', fail, { once: true });
    if (owned) document.head.appendChild(script);
  });
  return youtubeApiPromise;
}

const player = useAudioPlayerAdapter();
const ui = useUiStore();
const effectiveVolume = computed(() => getWidgetVolume(ui.volume, ui.getPlatformVolumeOffset(player.nowPlaying?.music?.platform)));
const toast = useToastStore();
const needsUserGesture = ref(false);
let ytPlayer;
let ready = false;
let destroyed = false;
let syncInterval;
let bufferingTimeout;
let autoplayTimeout;
let autoplayPending = false;
let bufferingReported = false;
let currentVideoId = '';
let failed = false;

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
  if (!embedId || !ready || !ytPlayer) return;
  clearPlaybackTimers();
  failed = false;
  player.isErrorState = false;
  player.isBuffering = false;
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

function reportError(code) {
  if (destroyed || failed) return;
  failed = true;
  clearPlaybackTimers();
  autoplayPending = false;
  needsUserGesture.value = code === 5;
  player.isBuffering = false;
  player.isErrorState = true;
  const zh = ui.locale === 'zh';
  const hostname = window.location.hostname;
  const ipOrigin = /^\d{1,3}(\.\d{1,3}){3}$/.test(hostname) || hostname.startsWith('[');
  const embedMessage = ipOrigin
    ? (zh ? 'YouTube 拒绝嵌入播放，当前通过 IP 地址访问，请改用域名打开页面。' : 'YouTube refused embedded playback from this IP address. Open this page using a domain name.')
    : (zh ? 'YouTube 拒绝嵌入播放，可在 YouTube 原站打开。' : 'YouTube refused embedded playback. Open the video on YouTube.');
  const messages = {
    2: zh ? 'YouTube 视频链接或 ID 无效。' : 'The YouTube video link or ID is invalid.',
    5: zh ? 'YouTube 播放器加载失败，请点击启用声音重试。' : 'YouTube playback failed. Tap to enable sound to retry.',
    100: zh ? 'YouTube 视频不存在、已删除或设为私密。' : 'This YouTube video is missing, removed or private.',
    101: embedMessage,
    150: embedMessage,
    153: zh ? 'YouTube 无法验证嵌入来源，请检查页面来源和 Referer 设置。' : 'YouTube could not verify the embed origin. Check the page origin and Referer settings.',
  };
  const message = messages[code] ?? (zh ? 'YouTube 播放器无法加载，请检查网络后重试。' : 'YouTube could not load. Check your connection and retry.');
  toast.add({ title: 'YouTube', message: `${message} (${code ?? 'API'})`, type: 'error', duration: 10000 });
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
  // YouTube can emit BUFFERING after a terminal error; keep that failure visible.
  if (destroyed || failed) return;
  const YT = window.YT;
  if (!YT) return;
  if (event.data === YT.PlayerState.PLAYING) {
    autoplayPending = false;
    needsUserGesture.value = false;
    player.isBuffering = false;
    player.isErrorState = false;
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
  reportError(event.data);
}

function enableSound() {
  if (!ytPlayer) return;
  failed = false;
  player.isErrorState = false;
  autoplayPending = true;
  ytPlayer.playVideo();
  needsUserGesture.value = false;
}

watch([
  () => player.nowPlaying?.music?.embedId,
  () => player.nowPlaying?.playEpoch,
  () => player.nowPlaying?.queueId,
], ([embedId]) => loadCurrentVideo(embedId));
watch(effectiveVolume, (volume) => {
  if (ready) ytPlayer?.setVolume(volume);
});

watch(() => player.isPaused, (paused) => {
  if (!ytPlayer || !currentVideoId || failed) return;
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
  try {
    const YT = await loadYouTubeAPI();
    if (destroyed) return;
    ytPlayer = new YT.Player('yt-player-container', {
      width: '1',
      height: '1',
      videoId: player.nowPlaying?.music?.embedId,
      playerVars: {
        autoplay: 0, // onReady applies volume before starting playback.
        controls: 0,
        disablekb: 1,
        enablejsapi: 1,
        origin: window.location.origin,
        rel: 0,
      },
      events: {
        onReady: () => {
          if (destroyed) return;
          ready = true;
          ytPlayer?.setVolume(effectiveVolume.value);
          loadCurrentVideo(player.nowPlaying?.music?.embedId);
        },
        onStateChange: onPlayerStateChange,
        onError: onPlayerError,
      },
    });
  } catch {
    reportError();
  }
});

onUnmounted(() => {
  destroyed = true;
  clearPlaybackTimers();
  ytPlayer?.destroy();
  ytPlayer = undefined;
  player.isBuffering = false;
  player.isErrorState = false;
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
