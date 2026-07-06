<template>
  <div class="hidden">
    <!-- 主播放器 -->
    <audio
        ref="audioRef"
        :src="audioSrc"
        crossorigin="anonymous"
        @error="handleError"
        @waiting="onPlaybackStalled('waiting')"
        @stalled="onPlaybackStalled('stalled')"
        @suspend="onPlaybackStalled('suspend')"
        @playing="onPlaybackHealthy"
        @canplay="onCanPlay"
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
import { usePlayerStore } from '../stores/player';
import { useUiStore } from '../stores/ui';
import { useUserStore } from '../stores/user';
import { useAudio } from '../composables/useAudio';
import { withPlaybackToken } from '../utils/audioUrl';

const player = usePlayerStore();
const ui = useUiStore();
const user = useUserStore();
const audioRef = ref(null);
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

const audioSrc = computed(() => withPlaybackToken(player.nowPlaying?.music, user.sessionToken));

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
};

const onPlaybackStalled = (reason) => {
  // 用 isActuallyStalled 过滤误判：suspend 在"下够了"时也会发，emptied 是切歌导致
  if (!isActuallyStalled(reason)) return;
  player.isBuffering = true;
  armStalledWatchdog(reason);
};

onMounted(() => {
  // 初始尝试播放静默音轨
  if (!player.isPaused) {
    silentAudioRef.value?.play().catch(() => {});
  }
});
</script>

