<template>
  <div class="soundcloud-player-host">
    <iframe
      ref="iframeRef"
      :src="widgetUrl"
      title="SoundCloud player"
      allow="autoplay"
      tabindex="-1"
    ></iframe>
  </div>
  <button
    v-if="needsUserGesture"
    type="button"
    class="fixed bottom-24 left-1/2 z-[var(--z-toast)] -translate-x-1/2 rounded-full bg-primary px-5 py-3 text-sm font-black text-on-primary shadow-xl shadow-black/20"
    @click="enableSound"
  >
    {{ ui.locale === 'zh' ? '点击启用声音' : 'Tap to enable sound' }}
  </button>
</template>

<script lang="ts">
type WidgetEvent = 'READY' | 'PLAY' | 'PAUSE' | 'FINISH' | 'ERROR';

const widgetOptions = {
  buying: false, liking: false, download: false, sharing: false,
  show_artwork: false, show_comments: false, show_playcount: false,
  show_user: false, hide_related: true, visual: false,
};

type WidgetOptions = typeof widgetOptions & { auto_play: boolean; callback: () => void };
type SoundCloudWidget = {
  bind: (event: string, handler: () => void) => void;
  unbind: (event: string) => void;
  load: (url: string, options: WidgetOptions) => void;
  play: () => void;
  pause: () => void;
  seekTo: (positionMs: number) => void;
  setVolume: (_volume: number) => void;
  getPosition: (callback: (positionMs: number) => void) => void;
  getDuration: (_callback: (_durationMs: number) => void) => void;
};
type SoundCloudAPI = {
  Widget: ((iframe: HTMLIFrameElement) => SoundCloudWidget) & { Events: Record<WidgetEvent, string> };
};

let soundCloudApiPromise: Promise<SoundCloudAPI> | undefined;

function loadSoundCloudAPI() {
  const host = window as Window & { SC?: SoundCloudAPI };
  if (host.SC?.Widget) return Promise.resolve(host.SC);
  if (soundCloudApiPromise) return soundCloudApiPromise;

  soundCloudApiPromise = new Promise<SoundCloudAPI>((resolve, reject) => {
    const script = document.createElement('script');
    script.src = 'https://w.soundcloud.com/player/api.js';
    script.async = true;
    script.onload = () => {
      if (host.SC?.Widget) resolve(host.SC);
      else reject(new Error('SoundCloud Widget API unavailable'));
    };
    script.onerror = () => reject(new Error('Failed to load SoundCloud Widget API'));
    document.head.appendChild(script);
  });
  return soundCloudApiPromise;
}
</script>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue';
import { useAudioPlayerAdapter } from '../composables/useAudioPlayerAdapter';
import { useUiStore } from '../stores/ui';
import { getWidgetVolume } from '../utils/playbackVolume';

const player = useAudioPlayerAdapter();
const ui = useUiStore();
const effectiveVolume = computed(() => getWidgetVolume(ui.volume, ui.getPlatformVolumeOffset(player.nowPlaying?.music?.platform)));
const iframeRef = ref<HTMLIFrameElement | null>(null);
const needsUserGesture = ref(false);

function trackURL(embedId: string) {
  return `https://api.soundcloud.com/tracks/${encodeURIComponent(embedId)}`;
}

// Keep the iframe src fixed; load each track without autoplay so READY can set its volume first.
let loadedId = player.nowPlaying?.music?.embedId;
let loadedEpoch = player.nowPlaying?.playEpoch;
const initialQuery = new URLSearchParams({
  ...Object.fromEntries(Object.entries(widgetOptions).map(([key, value]) => [key, String(value)])),
  url: loadedId ? trackURL(loadedId) : '',
  auto_play: 'false',
});
const widgetUrl = loadedId ? `https://w.soundcloud.com/player/?${initialQuery}` : 'about:blank';
let widget: SoundCloudWidget | undefined;
let events: SoundCloudAPI['Widget']['Events'] | undefined;
let destroyed = false;
let ready = false;
let autoplayPending = false;
let syncInterval: ReturnType<typeof setInterval> | undefined;
let autoplayTimeout: ReturnType<typeof setTimeout> | undefined;

function stopSync() {
  clearInterval(syncInterval);
  syncInterval = undefined;
}

function stopAutoplayCheck() {
  clearTimeout(autoplayTimeout);
  autoplayPending = false;
  needsUserGesture.value = false;
}

function watchForBlockedAutoplay() {
  clearTimeout(autoplayTimeout);
  autoplayTimeout = setTimeout(() => {
    if (autoplayPending && !player.isPaused) needsUserGesture.value = true;
  }, 500);
}

function play() {
  autoplayPending = true;
  watchForBlockedAutoplay();
  widget?.play();
}

function reportError() {
  stopSync();
  stopAutoplayCheck();
  player.isBuffering = false;
  const now = player.nowPlaying;
  if (now?.queueId && now.music.embedId === loadedId) {
    player.reportPlayback('playback.error', {
      playEpoch: now.playEpoch,
      queueId: now.queueId,
      code: 'stream-unavailable',
    });
  }
}

function seekToRoomPosition() {
  if (!ready || !widget) return;
  const position = Math.max(0, player.getCurrentProgress());
  widget.seekTo(position);
  player.setPlaybackPosition(position);
  player.forceNextSyncSeek = false;
}

function onWidgetReady() {
  if (destroyed) return;
  const embedId = player.nowPlaying?.music?.embedId;
  if (embedId && (embedId !== loadedId || player.nowPlaying?.playEpoch !== loadedEpoch)) {
    loadTrack(embedId);
    return;
  }
  ready = true;
  widget?.setVolume(effectiveVolume.value);
  seekToRoomPosition();
  if (player.isPaused) widget?.pause();
  else play();
}

function loadTrack(embedId: string) {
  ready = false;
  stopSync();
  stopAutoplayCheck();
  loadedId = embedId;
  loadedEpoch = player.nowPlaying?.playEpoch;
  widget?.load(trackURL(embedId), {
    ...widgetOptions, auto_play: false, callback: onWidgetReady,
  });
}

function onWidgetPlay() {
  const now = player.nowPlaying;
  if (!ready || !widget || !now?.queueId || now.music.embedId !== loadedId) return;
  if (player.isPaused) {
    widget.pause();
    return;
  }
  stopAutoplayCheck();
  player.isBuffering = false;
  stopSync();
  const interval = setInterval(() => {
    widget?.getPosition((positionMs) => {
      if (syncInterval === interval && player.nowPlaying?.playEpoch === now.playEpoch) {
        player.setPlaybackPosition(positionMs);
      }
    });
  }, 250);
  syncInterval = interval;
  widget.getPosition((positionMs) => {
    if (syncInterval !== interval || player.nowPlaying?.playEpoch !== now.playEpoch) return;
    widget?.getDuration((durationMs) => {
      if (syncInterval !== interval || player.nowPlaying?.playEpoch !== now.playEpoch) return;
      player.reportPlayback('playback.started', {
        playEpoch: now.playEpoch,
        queueId: now.queueId,
        positionMs: Math.round(positionMs),
        clientTime: player.serverNow(),
        durationMs: Number.isFinite(durationMs) && durationMs > 0 ? Math.round(durationMs) : now.music.duration,
      });
    });
  });
}

function onWidgetPause() {
  stopSync();
  player.isBuffering = false;
  if (ready && autoplayPending && !player.isPaused) watchForBlockedAutoplay();
}

function onWidgetFinish() {
  stopSync();
  stopAutoplayCheck();
  player.isBuffering = false;
  const now = player.nowPlaying;
  if (ready && now?.queueId && now.music.embedId === loadedId) {
    player.reportPlayback('playback.ended', { playEpoch: now.playEpoch, queueId: now.queueId });
  }
}

function enableSound() {
  if (!ready) return;
  needsUserGesture.value = false;
  play();
}

watch(() => [player.nowPlaying?.music?.embedId, player.nowPlaying?.playEpoch] as const, ([embedId, epoch]) => {
  if (ready && embedId && (embedId !== loadedId || epoch !== loadedEpoch)) loadTrack(embedId);
});

watch(effectiveVolume, (volume) => {
  if (ready) widget?.setVolume(volume);
});

watch(() => player.isPaused, (paused) => {
  if (!ready) return;
  if (paused) {
    stopSync();
    stopAutoplayCheck();
    widget?.pause();
  } else {
    play();
  }
});

watch(() => player.forceNextSyncSeek, (shouldSeek) => {
  if (shouldSeek) seekToRoomPosition();
});

onMounted(async () => {
  try {
    const SC = await loadSoundCloudAPI();
    if (destroyed || !iframeRef.value) return;
    widget = SC.Widget(iframeRef.value);
    const widgetEvents = SC.Widget.Events;
    events = widgetEvents;
    widget.bind(widgetEvents.PLAY, onWidgetPlay);
    widget.bind(widgetEvents.FINISH, onWidgetFinish);
    widget.bind(widgetEvents.PAUSE, onWidgetPause);
    widget.bind(widgetEvents.ERROR, reportError);
    if (loadedId) {
      widget.bind(widgetEvents.READY, () => {
        widget?.unbind(widgetEvents.READY);
        onWidgetReady();
      });
    }
  } catch {
    if (!destroyed) reportError();
  }
});

onUnmounted(() => {
  destroyed = true;
  stopSync();
  stopAutoplayCheck();
  if (widget && events) {
    for (const event of ['READY', 'PLAY', 'FINISH', 'PAUSE', 'ERROR'] as const) widget.unbind(events[event]);
  }
  widget = undefined;
  player.isBuffering = false;
});
</script>

<style scoped>
.soundcloud-player-host {
  position: absolute;
  width: 1px;
  height: 1px;
  opacity: 0;
  pointer-events: none;
  overflow: hidden;
}

iframe {
  width: 1px;
  height: 1px;
  border: 0;
}
</style>
