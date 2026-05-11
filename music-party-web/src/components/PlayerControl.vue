<template>
  <div class="h-28 bg-[var(--surface-4)]/90 backdrop-blur-xl border-t border-[var(--border-subtle)] flex items-center px-4 md:px-8 relative z-50 shadow-lg">
    <!-- 封面 -->
    <div
        id="tutorial-source"
        @click="openSourcePage"
        class="w-16 h-16 md:w-20 md:h-20 -mt-6 md:mt-0 shadow-lg border border-[var(--border-default)] rounded-xl flex-shrink-0 relative z-10 bg-[var(--surface-2)] cursor-pointer group overflow-hidden"
        title="Open Source Page"
    >
      <CoverImage :src="nowPlaying?.music.coverUrl" class="w-full h-full transition-transform duration-300 group-hover:scale-105 group-hover:opacity-70" />

      <!-- 悬浮时的遮罩和图标 -->
      <div class="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity duration-300 bg-black/35">
        <ExternalLink class="w-6 h-6 text-white" />
      </div>
    </div>

    <!-- 中间：信息与进度 -->
    <div class="flex-1 ml-4 mr-4 md:mr-8 flex flex-col justify-center min-w-0">
      <div class="flex justify-between items-end mb-1">
        <div class="overflow-hidden w-full">
          <!-- 标题显示逻辑与样式 -->
          <h2 class="text-lg md:text-xl font-semibold truncate leading-tight transition-colors duration-300"
              :class="!player.connected ? 'text-[var(--error-soft-text)]' : 'text-[var(--text-primary)]'"
          >
            {{
              !player.connected
                  ? '!CONNECTION LOST!'
                  : (nowPlaying ? nowPlaying.music.name : 'WAITING FOR SIGNAL...')
            }}
          </h2>

          <!-- 副标题显示逻辑与样式 -->
          <p class="text-xs font-sans truncate transition-colors duration-300"
             :class="!player.connected ? 'text-[var(--error)]' : 'text-[var(--text-secondary)]'"
          >
            {{
              !player.connected
                  ? 'RECONNECT SERVER...'
                  : (nowPlaying ? nowPlaying.music.artists.join(' / ') : 'SYSTEM STANDBY')
            }}
          </p>
        </div>

        <!-- 时间显示 -->
        <div class="hidden md:block font-mono text-xs text-[var(--text-tertiary)] flex-shrink-0 ml-2">
           <span v-if="player.isLoading" class="text-accent animate-pulse">SYNCING SERVER...</span>
           <span v-if="player.isBuffering" class="animate-pulse text-accent">BUFFERING...</span>
           <span v-else>{{ formatDuration(player.localProgress) }} / {{ formatDuration(nowPlaying?.music.duration || 0) }}</span>
        </div>
      </div>

      <!-- 进度条 -->
      <div
          ref="progressTrackRef"
          class="h-3 w-full relative flex items-center cursor-pointer touch-none"
          :class="{ 'opacity-60 cursor-not-allowed': !canSeek }"
          @pointerdown="handleProgressPointerDown"
      >
        <div class="h-1.5 bg-[var(--progress-track)] w-full relative rounded-full overflow-hidden">
          <div
              class="h-full transition-all duration-200 ease-linear relative rounded-full"
              :class="player.isErrorState ? 'bg-red-500' : 'bg-[var(--accent)]'"
              :style="{ width: displayProgressPercent + '%' }"
          >
            <div
                v-if="displayProgressPercent > 0 && !player.isErrorState"
                class="absolute right-0 top-1/2 -translate-y-1/2 w-3 h-3 rounded-full transition-all duration-200 bg-[var(--accent)] shadow-lg"
            ></div>
          </div>
        </div>

        <div
            v-for="(marker, index) in likeMarkers"
            :key="index"
            class="absolute z-20 transition-all duration-300 transform -translate-y-1/2 top-1/2"
            :style="{ left: (marker / (nowPlaying?.music.duration || 1)) * 100 + '%' }"
        >
          <Zap
              class="w-3 h-3 drop-shadow-md"
              :class="'text-accent fill-accent'"
          />
        </div>
      </div>
      
      <!-- 移动端简易控制 -->
      <div class="flex md:hidden justify-end gap-3 mt-2">
        <button id="tutorial-download-mobile" @click="downloadCurrentMusic" class="p-2 bg-[var(--surface-2)] border border-[var(--border-default)] rounded-lg text-[var(--text-secondary)] active:bg-[var(--surface-3)]">
          <Download class="w-4 h-4" />
        </button>
        <button
            id="tutorial-random-mobile"
            @click="player.toggleShuffle"
            :disabled="!player.connected || player.isShuffleLocked"
            class="p-2 border rounded-lg disabled:opacity-50 transition-colors border-[var(--border-default)]"
            :class="player.isShuffle
                ? 'bg-accent text-white border-accent'
                : 'bg-[var(--surface-2)] text-[var(--text-secondary)]'"
        >
          <Shuffle class="w-4 h-4" />
        </button>
         <button id="tutorial-pause-mobile" @click="player.togglePause" :disabled="player.isPauseLocked && !player.isPaused" class="p-2 bg-[var(--surface-2)] rounded-lg disabled:opacity-50 border border-[var(--border-default)]">
             <Lock v-if="player.isPauseLocked && !player.isPaused" class="w-4 h-4 text-[var(--text-tertiary)]" />
             <template v-else>
                 <Play v-if="player.isPaused" class="w-4 h-4" />
                 <Pause v-else class="w-4 h-4" />
             </template>
         </button>
         <button @click="player.playNext" :disabled="player.isSkipLocked" class="p-2 bg-[var(--surface-2)] rounded-lg disabled:opacity-50 border border-[var(--border-default)]">
             <SkipForward class="w-4 h-4" />
         </button>
       </div>
    </div>

    <!-- PC端：右侧控制区 -->
    <div class="hidden md:flex items-center gap-6 flex-shrink-0">
      
      <!-- 播放控制 -->
      <div class="flex items-center gap-4 border-r border-[var(--border-default)] pr-6">
        <!-- 新增：下载按钮 (放在 Shuffle 旁边或者 Next 后面) -->
        <button id="tutorial-download" @click="downloadCurrentMusic" class="text-[var(--text-tertiary)] hover:text-accent transition-colors" title="Download">
          <Download class="w-5 h-5" />
        </button>

        <button id="tutorial-random" @click="player.toggleShuffle" :disabled="player.isShuffleLocked" :class="[player.isShuffle ? 'text-accent' : 'text-[var(--text-tertiary)]', player.isShuffleLocked ? 'opacity-50 cursor-not-allowed' : '']" title="Shuffle">
            <Shuffle class="w-5 h-5" />
        </button>
        
        <button 
            id="tutorial-pause"
            @click="player.togglePause" 
            :disabled="player.isPauseLocked && !player.isPaused"
            class="w-10 h-10 bg-[var(--accent)] text-[var(--text-inverse)] flex items-center justify-center hover:bg-[var(--accent-hover)] transition-colors rounded-full disabled:opacity-50 disabled:hover:bg-[var(--accent)] disabled:cursor-not-allowed shadow-md"
        >
            <Lock v-if="player.isPauseLocked && !player.isPaused" class="w-4 h-4 text-white" />
            <template v-else>
                <Play v-if="player.isPaused" class="w-4 h-4 fill-current" />
                <Pause v-else class="w-4 h-4 fill-current" />
            </template>
        </button>

        <button @click="player.playNext" :disabled="player.isSkipLocked" class="text-[var(--text-secondary)] hover:text-accent transition-colors disabled:opacity-50 disabled:cursor-not-allowed" title="Next">
            <SkipForward class="w-6 h-6 fill-current" />
        </button>
      </div>

      <!-- 音量控制 -->
      <div class="flex items-center gap-2 group">
        <button @click="toggleMute" class="text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors">
          <VolumeX v-if="ui.volume === 0" class="w-5 h-5" />
          <Volume1 v-else-if="ui.volume < 0.5" class="w-5 h-5" />
          <Volume2 v-else class="w-5 h-5" />
        </button>

        <!-- 滑块容器 -->
        <div
            ref="volumeTrackRef"
            class="w-24 h-6 flex items-center relative cursor-pointer touch-none"
            @mousedown="handleVolumeMouseDown"
        >
          <!-- 灰色轨道 -->
          <div class="w-full h-1 bg-[var(--progress-track)] relative rounded-full overflow-hidden">
            <div
                class="h-full bg-[var(--accent)] group-hover:bg-[var(--accent-hover)] transition-colors relative rounded-full"
                :style="{ width: (ui.volume * 100) + '%' }"
            >
              <!-- 装饰滑块 (只在悬停时显示) -->
              <div class="absolute right-0 top-1/2 -translate-y-1/2 w-3 h-3 bg-white border border-[var(--border-default)] rounded-full group-hover:scale-100 scale-0 transition-transform"></div>
            </div>
          </div>
        </div>

        <div class="w-8 text-[10px] font-mono text-[var(--text-tertiary)] text-right">
          {{ Math.round(ui.volume * 100) }}%
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onUnmounted } from 'vue';
import { usePlayerStore } from '../stores/player';
import { useUiStore } from '../stores/ui';
import { formatDuration } from '../utils/format';
import { Download, Shuffle, SkipForward, Play, Pause, Volume2, Volume1, VolumeX, ExternalLink, Zap, Lock } from 'lucide-vue-next';
import CoverImage from './CoverImage.vue';
import { useToast } from '../composables/useToast';
import { useUserStore } from '../stores/user';

const player = usePlayerStore();
const ui = useUiStore();
const userStore = useUserStore();
const volumeTrackRef = ref(null);
const progressTrackRef = ref(null);
const { info, error } = useToast();

const nowPlaying = computed(() => player.nowPlaying);
const likeMarkers = computed(() => nowPlaying.value?.likeMarkers || []);
const canSeek = computed(() => !!nowPlaying.value && nowPlaying.value.enqueuedById === userStore.userToken);
const isDraggingProgress = ref(false);
const dragProgressMs = ref(0);
const progressPercent = computed(() => {
  if (!nowPlaying.value || nowPlaying.value.music.duration === 0) return 0;
  return Math.min(100, (player.localProgress / nowPlaying.value.music.duration) * 100);
});
const displayProgressMs = computed(() => {
  if (!nowPlaying.value) return 0;
  return isDraggingProgress.value ? dragProgressMs.value : player.localProgress;
});
const displayProgressPercent = computed(() => {
  if (!nowPlaying.value || nowPlaying.value.music.duration === 0) return 0;
  return Math.min(100, (displayProgressMs.value / nowPlaying.value.music.duration) * 100);
});

const getProgressMsFromEvent = (e) => {
  if (!progressTrackRef.value || !nowPlaying.value) return 0;
  const rect = progressTrackRef.value.getBoundingClientRect();
  const x = Math.max(0, Math.min(rect.width, e.clientX - rect.left));
  const percent = rect.width === 0 ? 0 : x / rect.width;
  return Math.round(percent * nowPlaying.value.music.duration);
};

const handleProgressPointerDown = (e) => {
  if (!canSeek.value || !nowPlaying.value) return;
  isDraggingProgress.value = true;
  dragProgressMs.value = getProgressMsFromEvent(e);
  player.setSeekingPreview?.(true);
  window.addEventListener('pointermove', handleProgressPointerMove);
  window.addEventListener('pointerup', handleProgressPointerUp);
};

const handleProgressPointerMove = (e) => {
  if (!isDraggingProgress.value) return;
  dragProgressMs.value = getProgressMsFromEvent(e);
};

const handleProgressPointerUp = () => {
  if (isDraggingProgress.value) {
    player.seek(dragProgressMs.value);
  }
  isDraggingProgress.value = false;
  player.setSeekingPreview?.(false);
  window.removeEventListener('pointermove', handleProgressPointerMove);
  window.removeEventListener('pointerup', handleProgressPointerUp);
};

// --- 音量逻辑 ---
const lastVolume = ref(0.5);
const isDraggingVolume = ref(false);

const toggleMute = () => {
  if (ui.volume > 0) {
    lastVolume.value = ui.volume;
    ui.setVolume(0);
  } else {
    ui.setVolume(lastVolume.value > 0 ? lastVolume.value : 0.5);
  }
};

// 音量拖拽
const updateVolumeByMouse = (e) => {
  if (!volumeTrackRef.value) return;
  const rect = volumeTrackRef.value.getBoundingClientRect();
  const x = e.clientX - rect.left;
  const percentage = Math.max(0, Math.min(1, x / rect.width));
  ui.setVolume(parseFloat(percentage.toFixed(2)));
};

const handleVolumeMouseDown = (e) => {
  isDraggingVolume.value = true;
  updateVolumeByMouse(e);
  window.addEventListener('mousemove', handleVolumeMouseMove);
  window.addEventListener('mouseup', handleVolumeMouseUp);
};
const handleVolumeMouseMove = (e) => { if (isDraggingVolume.value) updateVolumeByMouse(e); };
const handleVolumeMouseUp = () => {
  isDraggingVolume.value = false;
  window.removeEventListener('mousemove', handleVolumeMouseMove);
  window.removeEventListener('mouseup', handleVolumeMouseUp);
};

// --- 下载逻辑 ---
const downloadCurrentMusic = async () => {
  if (!nowPlaying.value) return;
  const music = nowPlaying.value.music;
  info(`Starting download: ${music.name}...`);
  try {
    const response = await fetch(music.url);
    if (!response.ok) throw new Error('Network error');
    const blob = await response.blob();
    const blobUrl = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = blobUrl;
    link.download = `${music.name} - ${music.artists[0]}.mp3`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(blobUrl);
  } catch (e) {
    window.open(music.url, '_blank');
    error('Blob download failed, opening new tab.');
  }
};

// 跳转源页面
const openSourcePage = () => {
  if (!nowPlaying.value) return;
  const { platform, id } = nowPlaying.value.music;
  let url = platform === 'netease' ? `https://music.163.com/#/song?id=${id}` : `https://www.bilibili.com/video/${id}`;
  if (url) window.open(url, '_blank');
};

onUnmounted(() => {
  window.removeEventListener('mousemove', handleVolumeMouseMove);
  window.removeEventListener('mouseup', handleVolumeMouseUp);
  window.removeEventListener('pointermove', handleProgressPointerMove);
  window.removeEventListener('pointerup', handleProgressPointerUp);
});
</script>
