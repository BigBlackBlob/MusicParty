<template>
  <div class="hidden">
    <!-- 主播放器 -->
    <audio
        :ref="setAudioElement(0)"
        :src="activeElement === 0 ? audioSrc : nextAudioSrc"
        crossorigin="anonymous"
        @error="onAudioEvent(0, 'error')"
        @waiting="onAudioEvent(0, 'waiting')"
        @stalled="onAudioEvent(0, 'stalled')"
        @suspend="onAudioEvent(0, 'suspend')"
        @playing="onAudioEvent(0, 'playing')"
        @ended="onAudioEvent(0, 'ended')"
        @canplay="onAudioEvent(0, 'canplay')"
        preload="auto"
        referrerpolicy="no-referrer"
    ></audio>
    <audio
        :ref="setAudioElement(1)"
        :src="activeElement === 1 ? audioSrc : nextAudioSrc"
        crossorigin="anonymous"
        preload="auto"
        @error="onAudioEvent(1, 'error')"
        @waiting="onAudioEvent(1, 'waiting')"
        @stalled="onAudioEvent(1, 'stalled')"
        @playing="onAudioEvent(1, 'playing')"
        @ended="onAudioEvent(1, 'ended')"
        @canplay="onAudioEvent(1, 'canplay')"
        referrerpolicy="no-referrer"
    ></audio>

    <!-- 静默保活音轨 (Keep-Alive) -->
    <audio
        ref="silentAudioRef"
        :src="SILENT_WAV"
        loop
    ></audio>
  </div>
  <button
      v-if="needsUserGesture"
      type="button"
      class="fixed bottom-24 left-1/2 z-[var(--z-toast)] -translate-x-1/2 rounded-full bg-primary px-5 py-3 text-sm font-black text-on-primary shadow-xl shadow-black/20"
      @click="safePlay"
  >
    {{ ui.locale === 'zh' ? '点击启用声音' : 'Tap to enable sound' }}
  </button>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue';
import { useAudioPlayerAdapter } from '../composables/useAudioPlayerAdapter';
import { useUiStore } from '../stores/ui';
import { useAudio } from '../composables/useAudio';
import { withPlaybackToken } from '../utils/audioUrl';

const player = useAudioPlayerAdapter();
const ui = useUiStore();
const audioElements = [ref(null), ref(null)];
const activeElement = ref(0);
const audioRef = computed(() => audioElements[activeElement.value].value);
const setAudioElement = (index) => (element) => { audioElements[index].value = element; };
const silentAudioRef = ref(null);

// 极简 1秒 静默 WAV
const SILENT_WAV = 'data:audio/wav;base64,UklGRigAAABXQVZFRm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQQAAAAAAA==';

const {
  localProgress,
  isBuffering,
  bufferedMs,
  isErrorState,
  needsUserGesture,
  safePlay,
  handleError,
  checkAutoPlay,
  armStalledWatchdog,
  markPlaybackHealthy,
  isActuallyStalled
} = useAudio(audioRef, player, computed(() => ui.volume));

const audioSrc = computed(() => withPlaybackToken(player.nowPlaying?.music));
// Queue snapshots and patches carry the server media URL for standby preload.
const queuedAudioSrc = computed(() => withPlaybackToken(player.queue?.[0]?.music));
// A removal patch can precede its player.state by a frame. Keep the buffered
// source until that state arrives, so consuming the queue cannot erase it.
const nextAudioSrc = ref('');
watch([queuedAudioSrc, () => player.isQueueSettled], ([source, settled]) => {
  if (settled) nextAudioSrc.value = source;
}, { flush: 'post', immediate: true });
watch(() => player.nowPlaying?.music?.id, (id, oldId) => {
  if (!id || !oldId) return;
  const standby = 1 - activeElement.value;
  if (audioElements[standby].value?.getAttribute('src') !== audioSrc.value) return;
  audioElements[activeElement.value].value?.pause();
  activeElement.value = standby;
  nextAudioSrc.value = '';
  queueMicrotask(checkAutoPlay);
}, { flush: 'sync' });


// 同步状态到 playerStore
watch(localProgress, (val) => {
  player.setPlaybackPosition(val);
});
watch(isBuffering, (val) => {
  player.isBuffering = val;
});
watch(bufferedMs, (val) => {
  player.bufferedMs = val;
});
watch(isErrorState, (val) => {
  player.isErrorState = val;
});

// 监听播放状态以维持静默音轨
watch(() => player.isPaused, (paused) => {
  if (!paused) {
    silentAudioRef.value?.play().catch(() => {});
  } else {
    silentAudioRef.value?.pause();
  }
});

// Both elements share one engine. A preload failure or stall on the standby
// element must not pause, retry, or report the track that is actually playing.
const onAudioEvent = (index, kind) => {
  if (index !== activeElement.value) return;
  if (kind === 'error') { handleError(); return; }
  if (kind === 'canplay') { markPlaybackHealthy(); checkAutoPlay(); return; }
  if (kind === 'playing') { onPlaybackHealthy(); return; }
  if (kind === 'ended') { onPlaybackEnded(); return; }
  onPlaybackStalled(kind);
};

const onPlaybackHealthy = () => {
  markPlaybackHealthy();
  const now = player.nowPlaying;
  if (now?.queueId) player.reportPlayback('playback.started', { playEpoch: now.playEpoch, queueId: now.queueId, positionMs: Math.round((audioRef.value?.currentTime || 0) * 1000), clientTime: player.serverNow(), durationMs: Math.round((audioRef.value?.duration || 0) * 1000) });
};

const onPlaybackEnded = () => {
  const now = player.nowPlaying;
  if (now?.queueId) player.reportPlayback('playback.ended', { playEpoch: now.playEpoch, queueId: now.queueId });
};

const onPlaybackStalled = (reason) => {
  // 用 isActuallyStalled 过滤误判：suspend 在"下够了"时也会发，emptied 是切歌导致
  if (!isActuallyStalled(reason)) return;
  player.isBuffering = true;
  const now = player.nowPlaying;
  if (now?.queueId) player.reportPlayback('playback.stalled', { playEpoch: now.playEpoch, queueId: now.queueId, positionMs: Math.round((audioRef.value?.currentTime || 0) * 1000) });
  armStalledWatchdog(reason);
};

onMounted(() => {
  // 初始尝试播放静默音轨
  if (!player.isPaused) {
    silentAudioRef.value?.play().catch(() => {});
  }
});
</script>

