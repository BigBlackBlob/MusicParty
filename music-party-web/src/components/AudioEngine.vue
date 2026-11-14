<template>
  <div class="hidden">
    <!-- 主播放器 -->
    <audio
        :ref="setAudioElement(0)"
        :src="activeElement === 0 ? audioSrc : nextAudioSrc"
        crossorigin="anonymous"
        @error="handleError"
        @waiting="onPlaybackStalled('waiting')"
        @stalled="onPlaybackStalled('stalled')"
        @suspend="onPlaybackStalled('suspend')"
        @playing="onPlaybackHealthy"
        @ended="onPlaybackEnded"
        @canplay="onCanPlay"
        referrerpolicy="no-referrer"
    ></audio>
    <audio
        :ref="setAudioElement(1)"
        :src="activeElement === 1 ? audioSrc : nextAudioSrc"
        crossorigin="anonymous"
        preload="auto"
        @error="handleError"
        @waiting="onPlaybackStalled('waiting')"
        @stalled="onPlaybackStalled('stalled')"
        @playing="onPlaybackHealthy"
        @ended="onPlaybackEnded"
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
const nextAudioSrc = computed(() => withPlaybackToken(player.queue?.[0]?.music));
const preloadedTrackId = computed(() => player.queue?.[0]?.music?.id || '');
watch(() => player.nowPlaying?.music?.id, (id, oldId) => {
  if (!id || !oldId || id !== preloadedTrackId.value) return;
  const standby = 1 - activeElement.value;
  audioElements[activeElement.value].value?.pause();
  activeElement.value = standby;
});

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

const onCanPlay = () => {
  markPlaybackHealthy();
  checkAutoPlay();
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

