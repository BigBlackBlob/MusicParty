<template>
  <div class="flex h-full min-h-0 min-w-0 flex-col justify-center overflow-hidden">
    <AppleLyricsPanel
      class="h-full w-full px-4 md:px-8"
      :lyrics="player.lyricDetail.lyric || player.lyricText"
      :translated-lyrics="player.lyricDetail.translatedLyric"
      :show-translation="uiStore.showLyricTranslation"
      :current-time-ms="progressMs"
      :is-playing="!player.isPaused"
      :is-dark-mode="uiStore.isDarkMode"
      :bg-color="ambientAccent"
      :lyrics-loaded="!!currentMusic"
      @toggle-translation="uiStore.toggleLyricTranslation"
    />
  </div>
</template>

<script setup>
import AppleLyricsPanel from '../AppleLyricsPanel.vue';
import { useNowPlayingViewModel } from '../../composables/useNowPlayingViewModel';

const {
  player,
  ui: uiStore,
  music: currentMusic,
  progressMs,
  ambientAccent
} = useNowPlayingViewModel();
</script>
