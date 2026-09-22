<template>
  <div class="flex h-full min-h-0 min-w-0 flex-col justify-center overflow-hidden">
    <AppleLyricsPanel
      class="h-full w-full px-4 md:px-8"
      :lyrics="player.lyricDetail.lyric || player.lyricText"
      :translated-lyrics="player.lyricDetail.translatedLyric"
      :romanized-lyrics="player.lyricDetail.romanizedLyric"
      :word-lyric="player.lyricDetail.wordLyric"
      :word-translated-lyric="player.lyricDetail.wordTranslatedLyric"
      :word-romanized-lyric="player.lyricDetail.wordRomanizedLyric"
      :show-translation="uiStore.showLyricTranslation"
      :show-romanization="uiStore.showLyricRomanization"
      :current-time-ms="progressMs"
      :is-playing="!player.isPaused"
      :is-dark-mode="uiStore.isDarkMode"
      :bg-color="ambientAccent"
      :lyrics-loaded="!!currentMusic"
      @toggle-translation="uiStore.toggleLyricTranslation"
      @toggle-romanization="uiStore.toggleLyricRomanization"
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
