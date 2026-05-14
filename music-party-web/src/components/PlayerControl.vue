<template>
  <div class="w-full h-full flex items-center justify-between px-6">
    <div class="flex items-center gap-4 w-1/3 min-w-0">
      <!-- Now Playing Mini -->
      <img v-if="currentCover" :src="currentCover" class="w-12 h-12 rounded object-cover" />
      <div v-if="player.nowPlaying" class="flex flex-col min-w-0 flex-1">
        <span class="text-sm font-bold truncate">{{ player.nowPlaying.music.name }}</span>
        <span class="text-xs text-[#8A8A8A] truncate">{{ player.nowPlaying.music.artists.join(' / ') }}</span>
      </div>
    </div>

    <div class="flex flex-col items-center justify-center w-1/3 gap-2">
      <!-- Controls -->
      <div class="flex items-center gap-4 text-sm font-bold">
        <button @click="player.toggleShuffle" class="text-[#8A8A8A] hover:text-white transition-colors" :class="{ 'text-[#D3C2F3]': player.isShuffle }">{{ t('player.shuffle') }}</button>
        <button class="text-[#8A8A8A] hover:text-white transition-colors opacity-50 cursor-not-allowed">{{ t('player.prev') }}</button>
        <button @click="player.togglePause" class="text-black bg-[#D3C2F3] px-3 py-1 rounded hover:bg-white transition-colors">
          {{ player.isPaused ? t('player.play') : t('player.pause') }}
        </button>
        <button @click="player.playNext" class="text-[#8A8A8A] hover:text-white transition-colors">{{ t('player.next') }}</button>
      </div>
      <!-- Progress -->
      <div class="w-full max-w-md h-1.5 bg-[#303033] rounded overflow-hidden cursor-pointer" @click="handleSeek">
        <div class="h-full bg-[#D3C2F3]" :style="{ width: `${(player.playbackPositionMs / (player.nowPlaying?.music.duration || 1)) * 100}%` }"></div>
      </div>
    </div>

    <div class="w-1/3 flex items-center justify-end gap-4">
       <!-- Volume / Extra -->
       <button @click="player.sendLike" class="text-[#8A8A8A] hover:text-[#D3C2F3] text-sm font-bold transition-colors">{{ t('player.like') }}</button>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { usePlayerStore } from '../stores/player';

const { t } = useI18n();
const player = usePlayerStore();
const currentCover = computed(() => player.nowPlaying?.music.coverUrl || '');

const handleSeek = (e) => {
  if (!player.nowPlaying || !player.nowPlaying.music.duration) return;
  const rect = e.currentTarget.getBoundingClientRect();
  const clickX = e.clientX - rect.left;
  const percentage = Math.max(0, Math.min(1, clickX / rect.width));
  player.seek(Math.floor(percentage * player.nowPlaying.music.duration));
};
</script>
