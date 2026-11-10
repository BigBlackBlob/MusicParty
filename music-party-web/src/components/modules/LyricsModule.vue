<template>
  <div class="flex h-full min-h-0 min-w-0 flex-col justify-center overflow-hidden">
    <AppleLyricsPanel
      v-if="currentMusic || player.isLoading || player.isBuffering || player.isErrorState"
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
    <section v-else class="room-waiting-stage" role="status" :aria-label="t('player.nowPlaying')">
      <div class="room-waiting-stage__copy">
        <div class="font-micro text-micro uppercase text-text-muted">{{ t('player.nowPlaying') }}</div>
        <h2 class="room-waiting-stage__title">{{ trackTitle }}</h2>
        <p class="room-waiting-stage__hint">{{ t('player.waitingFirstTrackHint') }}</p>
      </div>
      <span class="material-symbols-outlined room-waiting-stage__icon" aria-hidden="true">graphic_eq</span>
    </section>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n';
import AppleLyricsPanel from '../AppleLyricsPanel.vue';
import { useNowPlayingViewModel } from '../../composables/useNowPlayingViewModel';

const { t } = useI18n();
const {
  player,
  ui: uiStore,
  music: currentMusic,
  trackTitle,
  progressMs,
  ambientAccent
} = useNowPlayingViewModel();
</script>

<style scoped>
.room-waiting-stage {
  display: grid;
  width: min(100%, 720px);
  height: 100%;
  min-height: 0;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  justify-content: center;
  gap: 24px;
  overflow: hidden;
  margin: 0 auto;
  padding: 24px clamp(0px, 3vw, 40px);
}

.room-waiting-stage__copy {
  min-width: 0;
}

.room-waiting-stage__icon {
  flex: 0 0 auto;
  align-self: center;
  padding: 0 4px 9px;
  border-bottom: 1px solid var(--accent);
  color: var(--accent);
  font-size: 42px;
  opacity: 0.88;
  filter: drop-shadow(0 0 22px color-mix(in srgb, var(--accent) 38%, transparent));
}

.room-waiting-stage__title {
  max-width: 520px;
  margin: 12px 0 0;
  color: var(--text-primary);
  font-size: 32px;
  font-weight: 650;
  line-height: 1.12;
  letter-spacing: 0;
}

.room-waiting-stage__hint {
  margin: 10px 0 0;
  color: var(--text-secondary);
  font-size: 14px;
  line-height: 1.5;
}

@media (max-width: 640px) {
  .room-waiting-stage {
    width: 100%;
    gap: 16px;
    padding-inline: 4px;
  }

  .room-waiting-stage__title {
    font-size: 26px;
  }

  .room-waiting-stage__icon {
    font-size: 34px;
  }
}
</style>
