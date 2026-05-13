<template>
  <div class="h-28 bg-[var(--surface-4)]/90 backdrop-blur-xl border-t border-[var(--border-subtle)] flex items-center px-4 md:px-8 relative z-[var(--z-header)] shadow-lg">
    <!-- 封面 -->
    <div
        id="tutorial-source"
        @click="openSourcePage"
        class="w-16 h-16 md:w-20 md:h-20 -mt-6 md:mt-0 shadow-lg border border-[var(--border-default)] rounded-xl flex-shrink-0 relative z-10 bg-[var(--surface-2)] cursor-pointer group overflow-hidden"
        title="Open Source Page"
        :aria-label="nowPlaying ? `${nowPlaying.music.name} 封面，点击打开来源页面` : '打开来源页面'"
    >
      <CoverImage
          :src="nowPlaying?.music.coverUrl"
          :alt="nowPlaying ? `${nowPlaying.music.name} 封面` : '歌曲封面'"
          loading="eager"
          decoding="async"
          class="w-full h-full transition-transform duration-300 group-hover:scale-105 group-hover:opacity-70"
      />

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
                  ? '连接已断开'
                  : (nowPlaying ? nowPlaying.music.name : '等待播放')
            }}
          </h2>

          <!-- 副标题显示逻辑与样式 -->
          <p class="text-xs font-sans truncate transition-colors duration-300"
             :class="!player.connected ? 'text-[var(--error)]' : 'text-[var(--text-secondary)]'"
          >
            {{
              !player.connected
                  ? '正在重连'
                  : (nowPlaying ? nowPlaying.music.artists.join(' / ') : '暂无播放内容')
            }}
          </p>
        </div>

        <!-- 时间显示 -->
        <div class="hidden md:block font-mono text-xs text-[var(--text-tertiary)] flex-shrink-0 ml-2">
           <span v-if="player.isLoading" class="text-accent animate-pulse">同步中...</span>
           <span v-if="player.isBuffering" class="animate-pulse text-accent">缓冲中...</span>
           <span v-else>{{ formatDuration(player.localProgress) }} / {{ formatDuration(nowPlaying?.music.duration || 0) }}</span>
        </div>
      </div>

      <!-- 进度条 -->
      <div
          ref="progressTrackRef"
          class="h-3 w-full relative flex items-center touch-none"
          :class="canSeek ? 'cursor-pointer' : 'opacity-60 cursor-not-allowed'"
          role="slider"
          tabindex="0"
          :aria-valuemin="0"
          :aria-valuemax="nowPlaying?.music.duration || 0"
          :aria-valuenow="Math.round(displayProgressMs)"
          :aria-disabled="!canSeek"
          :aria-label="canSeek ? '拖拽调整播放进度' : '只有点播者可以调整进度'"
          :title="canSeek ? '拖拽调整播放进度' : '只有点播者可以调整进度'"
          @pointerdown="handleProgressPointerDown"
          @pointercancel="handleProgressPointerCancel"
      >
          <div class="h-1.5 bg-[var(--progress-track)] w-full relative rounded-full overflow-hidden">
            <div
              class="h-full relative rounded-full"
              :class="player.isErrorState ? 'bg-red-500' : 'bg-[var(--accent)]'"
              :style="{ width: displayProgressPercent + '%' }"
            >
            <div
                v-if="displayProgressPercent > 0 && !player.isErrorState"
                class="absolute right-0 top-1/2 -translate-y-1/2 w-3 h-3 rounded-full transition-all duration-200 bg-[var(--accent)] shadow-lg"
                :class="{ 'opacity-0': !canSeek }"
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
        <button id="tutorial-download-mobile" @click="downloadCurrentMusic" class="min-w-[44px] min-h-[44px] p-2 bg-[var(--surface-2)] border border-[var(--border-default)] rounded-lg text-[var(--text-secondary)] active:bg-[var(--surface-3)] active:scale-[0.96]" aria-label="下载当前歌曲">
          <Download class="w-4 h-4" />
        </button>
        <button
            id="tutorial-random-mobile"
            @click="player.toggleShuffle"
            :disabled="!player.connected || player.isShuffleLocked"
            class="min-w-[44px] min-h-[44px] p-2 border rounded-lg disabled:opacity-50 transition-colors border-[var(--border-default)] active:scale-[0.96]"
            :class="player.isShuffle
                ? 'bg-accent text-white border-accent'
                : 'bg-[var(--surface-2)] text-[var(--text-secondary)]'"
            aria-label="随机播放"
        >
          <Shuffle class="w-4 h-4" />
        </button>
         <button id="tutorial-pause-mobile" @click="player.togglePause" :disabled="player.isPauseLocked && !player.isPaused" class="min-w-[44px] min-h-[44px] p-2 bg-[var(--surface-2)] rounded-lg disabled:opacity-50 border border-[var(--border-default)] active:scale-[0.96]" aria-label="播放或暂停">
             <Lock v-if="player.isPauseLocked && !player.isPaused" class="w-4 h-4 text-[var(--text-tertiary)]" />
             <template v-else>
                 <Play v-if="player.isPaused" class="w-4 h-4" />
                 <Pause v-else class="w-4 h-4" />
             </template>
         </button>
         <button @click="player.playNext" :disabled="player.isSkipLocked" class="min-w-[44px] min-h-[44px] p-2 bg-[var(--surface-2)] rounded-lg disabled:opacity-50 border border-[var(--border-default)] active:scale-[0.96]" aria-label="下一首">
             <SkipForward class="w-4 h-4" />
         </button>
       </div>
    </div>

    <!-- PC端：右侧控制区 -->
    <div class="hidden md:flex items-center gap-6 flex-shrink-0">
      
      <!-- 播放控制 -->
      <div class="flex items-center gap-4 border-r border-[var(--border-default)] pr-6">
        <!-- 新增：下载按钮 (放在 Shuffle 旁边或者 Next 后面) -->
        <button id="tutorial-download" @click="downloadCurrentMusic" class="min-w-[44px] min-h-[44px] inline-flex items-center justify-center text-[var(--text-tertiary)] hover:text-accent active:scale-[0.96] transition-colors" title="Download" aria-label="下载当前歌曲">
          <Download class="w-5 h-5" />
        </button>

        <button id="tutorial-random" @click="player.toggleShuffle" :disabled="player.isShuffleLocked" :class="['min-w-[44px] min-h-[44px] inline-flex items-center justify-center active:scale-[0.96]', player.isShuffle ? 'text-accent' : 'text-[var(--text-tertiary)]', player.isShuffleLocked ? 'opacity-50 cursor-not-allowed' : '']" title="Shuffle" aria-label="随机播放">
            <Shuffle class="w-5 h-5" />
        </button>
        
        <button 
            id="tutorial-pause"
            @click="player.togglePause" 
            :disabled="player.isPauseLocked && !player.isPaused"
            class="min-w-[44px] min-h-[44px] bg-[var(--accent)] text-[var(--text-inverse)] flex items-center justify-center hover:bg-[var(--accent-hover)] active:scale-[0.96] transition-colors rounded-full disabled:opacity-50 disabled:hover:bg-[var(--accent)] disabled:cursor-not-allowed shadow-md"
            aria-label="播放或暂停"
        >
            <Lock v-if="player.isPauseLocked && !player.isPaused" class="w-4 h-4 text-white" />
            <template v-else>
                <Play v-if="player.isPaused" class="w-4 h-4 fill-current" />
                <Pause v-else class="w-4 h-4 fill-current" />
            </template>
        </button>

        <button @click="player.playNext" :disabled="player.isSkipLocked" class="min-w-[44px] min-h-[44px] inline-flex items-center justify-center text-[var(--text-secondary)] hover:text-accent active:scale-[0.96] transition-colors disabled:opacity-50 disabled:cursor-not-allowed" title="Next" aria-label="下一首">
            <SkipForward class="w-6 h-6 fill-current" />
        </button>

        <button
            @click="likeCurrentMusic"
            :disabled="!nowPlaying || hasLiked"
            class="min-w-[44px] min-h-[44px] inline-flex items-center justify-center active:scale-[0.96] transition-colors disabled:cursor-default"
            :class="hasLiked ? 'text-[var(--accent)]' : 'text-[var(--text-secondary)] hover:text-accent'"
            :title="hasLiked ? '已喜欢' : '喜欢当前歌曲'"
            :aria-label="hasLiked ? '已喜欢当前歌曲' : '喜欢当前歌曲'"
            :aria-pressed="hasLiked"
        >
            <Heart class="w-5 h-5" :class="hasLiked ? 'fill-current stroke-none' : ''" />
        </button>
      </div>

      <!-- 音量控制 -->
      <div class="flex items-center gap-2 group">
        <button @click="toggleMute" class="min-w-[44px] min-h-[44px] inline-flex items-center justify-center text-[var(--text-secondary)] hover:text-[var(--text-primary)] active:scale-[0.96] transition-colors" aria-label="切换静音">
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

        <div class="w-8 text-xs font-mono text-[var(--text-tertiary)] text-right">
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
import { Download, Shuffle, SkipForward, Play, Pause, Volume2, Volume1, VolumeX, ExternalLink, Zap, Lock, Heart } from 'lucide-vue-next';
import CoverImage from './CoverImage.vue';
import { useToast } from '../composables/useToast';
import { useUserStore } from '../stores/user';
import { withPlaybackToken } from '../utils/audioUrl';

const player = usePlayerStore();
const ui = useUiStore();
const userStore = useUserStore();
const volumeTrackRef = ref(null);
const progressTrackRef = ref(null);
const { info, error } = useToast();

const nowPlaying = computed(() => player.nowPlaying);
const likeMarkers = computed(() => nowPlaying.value?.likeMarkers || []);
const canSeek = computed(() => !!nowPlaying.value && nowPlaying.value.enqueuedById === userStore.userToken);
const isLocallyLiked = computed(() => player.isSongLiked(nowPlaying.value?.music));
const hasLiked = computed(() => isLocallyLiked.value);
const isDraggingProgress = ref(false);
const dragProgressMs = ref(0);
const activeProgressPointerId = ref(null);
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
  if (!canSeek.value || !nowPlaying.value) {
    if (nowPlaying.value) error('只有点播者可以调整这首歌的进度');
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
  if (isDraggingProgress.value) {
    player.seek(dragProgressMs.value);
  }
  cleanupProgressDrag(e);
};

const handleProgressPointerCancel = (e) => {
  if (activeProgressPointerId.value !== null && e.pointerId !== activeProgressPointerId.value) return;
  cleanupProgressDrag(e);
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
  const url = withPlaybackToken(music, userStore.userToken);
  info(`开始下载：${music.name}...`);
  try {
    const response = await fetch(url);
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
    window.open(url, '_blank');
    error('直接下载失败，已尝试在新标签页打开。');
  }
};

const likeCurrentMusic = () => {
  if (!nowPlaying.value || hasLiked.value) return;
  player.sendLike();
};

// 跳转源页面
const openSourcePage = () => {
  if (!nowPlaying.value) return;
  const { platform, id } = nowPlaying.value.music;
  if (platform === 'navidrome') {
    info('Navidrome 没有公开源页面');
    return;
  }
  let url = platform === 'netease' ? `https://music.163.com/#/song?id=${id}` : `https://www.bilibili.com/video/${id}`;
  if (url) window.open(url, '_blank');
};

onUnmounted(() => {
  window.removeEventListener('mousemove', handleVolumeMouseMove);
  window.removeEventListener('mouseup', handleVolumeMouseUp);
  window.removeEventListener('pointermove', handleProgressPointerMove);
  window.removeEventListener('pointerup', handleProgressPointerUp);
  window.removeEventListener('pointercancel', handleProgressPointerCancel);
});
</script>
