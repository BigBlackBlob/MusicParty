<template>
  <div class="apple-stage relative h-full w-full overflow-hidden" :class="stageClass">
    <div class="pointer-events-none absolute inset-0">
      <div class="absolute inset-[-12%] scale-110 blur-3xl opacity-60 transition-all duration-700" :style="coverBackgroundStyle"></div>
      <div class="absolute inset-0 transition-all duration-700" :class="stageOverlayClass"></div>
    </div>

    <div class="relative z-10 flex h-full w-full items-stretch px-5 pb-8 pt-7 md:px-10 md:pb-10 md:pt-10 xl:px-14">
      <div
        class="mx-auto grid w-full max-w-[1500px] flex-1 grid-cols-1 gap-8 lg:gap-12 xl:gap-16"
        :class="stageLayoutClass"
      >
        <section class="flex min-h-0 flex-col items-center justify-center" :class="showLyricsPanel ? 'lg:items-start' : 'lg:items-center'">
          <div class="relative w-full" :class="showLyricsPanel ? 'max-w-[420px] lg:max-w-[460px]' : 'max-w-[460px] lg:max-w-[560px] xl:max-w-[600px]'">
            <div class="absolute inset-8 rounded-[2.5rem] blur-3xl transition-all duration-500" :class="coverShadowClass"></div>
            <div
              v-if="!showLyricsPanel"
              class="pointer-events-none absolute inset-x-[18%] -bottom-14 h-24 rounded-full blur-2xl"
              :class="ambientGlowClass"
            ></div>
            <div
              v-if="!showLyricsPanel"
              class="pointer-events-none absolute left-1/2 top-1/2 h-[118%] w-[78%] -translate-x-1/2 -translate-y-1/2 rounded-full blur-[72px]"
              :class="ambientHaloClass"
            ></div>

            <div
              id="tutorial-like"
              class="group relative aspect-square w-full cursor-pointer overflow-hidden rounded-[2.35rem] bg-[var(--surface-4)] transition-all duration-500"
              :class="[
                player.isPaused ? 'scale-[0.985] saturate-[0.88]' : 'scale-100',
                hasLiked ? 'cursor-default' : 'cursor-pointer',
                showLyricsPanel ? 'shadow-[0_38px_120px_rgba(0,0,0,0.34)]' : 'shadow-[0_48px_160px_rgba(0,0,0,0.42)]'
              ]"
              @mouseenter="!isMobile && (isHovering = true)"
              @mouseleave="!isMobile && (isHovering = false)"
              @click="handleCoverClick"
            >
              <div v-if="player.isLoading" class="absolute inset-0 z-50 flex flex-col items-center justify-center gap-4 bg-black/38 text-white backdrop-blur-xl">
                <div class="h-12 w-12 animate-spin rounded-full border-[3px] border-white/20 border-t-white"></div>
                <span class="text-xs font-medium tracking-[0.18em] text-white/75">Loading track</span>
              </div>

              <CoverImage
                :src="currentCover"
                :alt="player.nowPlaying ? `${player.nowPlaying.music.name} 封面` : '歌曲封面'"
                loading="eager"
                decoding="async"
                class="absolute inset-0 h-full w-full object-cover transition-transform duration-700"
                :class="player.isPaused ? 'scale-[1.01]' : (showLyricsPanel ? 'scale-[1.04] group-hover:scale-[1.07]' : 'scale-[1.06] group-hover:scale-[1.09]')"
              />

              <div class="absolute inset-0 transition-all duration-500" :class="coverMaskClass"></div>
              <div class="absolute inset-x-0 top-0 h-28 transition-all duration-500" :class="coverSheenClass"></div>

              <Transition
                enter-active-class="transition-all duration-300 ease-out"
                enter-from-class="opacity-0 scale-90"
                enter-to-class="opacity-100 scale-100"
                leave-active-class="transition-all duration-300 ease-in"
                leave-from-class="opacity-100 scale-100"
                leave-to-class="opacity-0 scale-95"
              >
                <div
                  v-if="isBursting || (!hasLiked && (isHovering || mobileLikePending)) || hasLiked"
                  class="absolute inset-0 z-40 flex items-center justify-center select-none"
                  :class="[(isBursting || (!hasLiked && (isHovering || mobileLikePending))) ? 'bg-black/30 backdrop-blur-[2px]' : '']"
                >
                  <div class="relative flex flex-col items-center justify-center gap-3">
                    <div v-if="isBursting" class="absolute -inset-8 rounded-full bg-[var(--accent)]/18 blur-2xl"></div>
                    <div
                      class="relative z-10 flex h-20 w-20 items-center justify-center rounded-full bg-black/24 text-white transition-all duration-300"
                      :class="[
                        isBursting ? 'scale-125 bg-[var(--accent)]/24 text-[var(--accent)]' :
                        hasLiked ? 'scale-100 bg-[var(--accent)]/18 text-[var(--accent)]' :
                        'scale-100 group-hover:scale-110'
                      ]"
                    >
                      <Activity v-if="!hasLiked && (isHovering || mobileLikePending) && !isBursting" class="h-9 w-9" />
                      <Zap v-else class="h-9 w-9" :class="hasLiked || isBursting ? 'fill-current stroke-none' : ''" />
                    </div>

                    <div
                      class="text-[11px] font-medium tracking-[0.22em] uppercase transition-colors duration-300"
                      :class="isBursting || hasLiked ? 'text-[var(--accent)]' : 'text-white/72'"
                    >
                      <span v-if="isBursting">Lovely choice</span>
                      <span v-else-if="hasLiked">Saved to the room</span>
                      <span v-else>Tap to react</span>
                    </div>
                  </div>
                </div>
              </Transition>
            </div>
          </div>
        </section>

        <section v-if="showLyricsPanel" class="flex min-h-0 flex-col justify-center">
          <AppleLyricsPanel
            :lyrics="player.lyricDetail.lyric || player.lyricText"
            :translated-lyrics="player.lyricDetail.translatedLyric"
            :current-time="player.localProgress / 1000"
            :is-playing="!player.isPaused"
            :is-dark-mode="isDarkMode"
            :bg-color="dominantCoverColor"
          />
        </section>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { usePlayerStore } from '../stores/player';
import { useUserStore } from '../stores/user';
import { useUiStore } from '../stores/ui';
import { useWindowSize } from '@vueuse/core';
import { Activity, Zap } from 'lucide-vue-next';
import AppleLyricsPanel from './AppleLyricsPanel.vue';
import CoverImage from './CoverImage.vue';
import { parseLyrics } from '../utils/parser';

const userStore = useUserStore();
const player = usePlayerStore();
const uiStore = useUiStore();
const { width } = useWindowSize();
const isMobile = computed(() => width.value < 768);
const isDarkMode = computed(() => uiStore.isDarkMode);
const currentCover = computed(() => player.nowPlaying?.music.coverUrl || '');
const hasCover = computed(() => !!currentCover.value);
const dominantCoverColor = computed(() => {
  if (uiStore.dynamicAccent?.accent) return uiStore.dynamicAccent.accent;
  if (!hasCover.value) return isDarkMode.value ? 'rgba(28,28,28,1)' : 'rgba(246,246,248,1)';
  return isDarkMode.value ? 'rgba(24,24,28,1)' : 'rgba(245,244,248,1)';
});

const isHovering = ref(false);
const mobileLikePending = ref(false);
const mobileTimer = ref(null);
const isBursting = ref(false);

const hasLiked = computed(() => player.nowPlaying?.likedUserIds?.includes(userStore.userToken));
const MIN_DISPLAY_LYRIC_LINES = 5;
const parsedLyricLines = computed(() => parseLyrics(player.lyricDetail.lyric || player.lyricText));
const hasLyrics = computed(() => parsedLyricLines.value.length >= MIN_DISPLAY_LYRIC_LINES);
const showLyricsPanel = computed(() => !!player.nowPlaying && hasLyrics.value);
const stageLayoutClass = computed(() => {
  if (!showLyricsPanel.value) return 'place-items-center';
  return 'md:grid-cols-[minmax(260px,380px)_minmax(0,1fr)] lg:grid-cols-[minmax(320px,480px)_minmax(0,1fr)]';
});

const stageClass = computed(() => {
  if (hasCover.value) return isDarkMode.value ? 'apple-stage-with-cover-dark' : 'apple-stage-with-cover-light';
  return isDarkMode.value ? 'apple-stage-empty-dark' : 'apple-stage-empty-light';
});

const stageOverlayClass = computed(() => {
  if (hasCover.value) {
    return isDarkMode.value
      ? 'bg-[linear-gradient(180deg,rgba(6,6,6,0.08),rgba(8,8,8,0.32)_42%,rgba(6,6,6,0.62))]'
      : 'bg-[linear-gradient(180deg,rgba(255,255,255,0.08),rgba(255,255,255,0.3)_42%,rgba(244,244,246,0.72))]';
  }
  return isDarkMode.value
    ? 'bg-[linear-gradient(180deg,rgba(12,12,12,0.06),rgba(12,12,12,0.18)_42%,rgba(12,12,12,0.34))]'
    : 'bg-[linear-gradient(180deg,rgba(255,255,255,0.18),rgba(255,255,255,0.4)_42%,rgba(248,248,249,0.75))]';
});

const ambientGlowClass = computed(() => (
  isDarkMode.value
    ? 'bg-[radial-gradient(circle,rgba(255,255,255,0.14),rgba(255,255,255,0.02)_48%,transparent_72%)]'
    : 'bg-[radial-gradient(circle,rgba(124,92,191,0.12),rgba(124,92,191,0.03)_48%,transparent_72%)]'
));

const ambientHaloClass = computed(() => (
  isDarkMode.value
    ? 'bg-[radial-gradient(circle,rgba(211,194,243,0.18),rgba(211,194,243,0.06)_42%,transparent_72%)]'
    : 'bg-[radial-gradient(circle,rgba(124,92,191,0.11),rgba(124,92,191,0.03)_42%,transparent_72%)]'
));

const coverShadowClass = computed(() => (isDarkMode.value ? 'bg-black/38' : 'bg-[rgba(124,92,191,0.12)]'));

const coverMaskClass = computed(() => (
  isDarkMode.value
    ? 'bg-[linear-gradient(180deg,rgba(0,0,0,0.02),rgba(0,0,0,0.18)_45%,rgba(0,0,0,0.55))]'
    : 'bg-[linear-gradient(180deg,rgba(255,255,255,0.05),rgba(255,255,255,0.02)_45%,rgba(255,255,255,0.22))]'
));

const coverSheenClass = computed(() => (
  isDarkMode.value
    ? 'bg-[linear-gradient(180deg,rgba(255,255,255,0.16),transparent)]'
    : 'bg-[linear-gradient(180deg,rgba(255,255,255,0.34),rgba(255,255,255,0.02))]'
));

const coverBackgroundStyle = computed(() => {
  if (!currentCover.value) {
    return {
      background: isDarkMode.value
        ? 'radial-gradient(circle at 20% 20%, rgba(211,194,243,0.18), transparent 28%), radial-gradient(circle at 80% 25%, rgba(120,138,255,0.12), transparent 26%), linear-gradient(135deg, rgba(255,255,255,0.02), rgba(255,255,255,0.01))'
        : 'radial-gradient(circle at 18% 18%, rgba(124,92,191,0.12), transparent 28%), radial-gradient(circle at 82% 22%, rgba(160,178,255,0.12), transparent 26%), linear-gradient(135deg, rgba(255,255,255,0.16), rgba(255,255,255,0.04))'
    };
  }

  return {
    backgroundImage: `${
      isDarkMode.value
        ? 'linear-gradient(135deg, rgba(17,17,17,0.25), rgba(17,17,17,0.55))'
        : 'linear-gradient(135deg, rgba(255,255,255,0.12), rgba(248,248,249,0.4))'
    }, url("${currentCover.value}")`,
    backgroundPosition: 'center',
    backgroundSize: 'cover'
  };
});

const EFFECT_COOLDOWN = 1000;
let lastEffectTime = 0;

const handleCoverClick = () => {
  if (hasLiked.value) return;
  if (isMobile.value) {
    if (!mobileLikePending.value) {
      mobileLikePending.value = true;
      mobileTimer.value = setTimeout(() => {
        mobileLikePending.value = false;
      }, 2000);
    } else {
      clearTimeout(mobileTimer.value);
      mobileLikePending.value = false;
      confirmLike();
    }
  } else {
    confirmLike();
  }
};

const confirmLike = () => {
  player.sendLike();
  triggerBurst();
};

const triggerBurst = () => {
  const now = Date.now();
  if (now - lastEffectTime < EFFECT_COOLDOWN) return;
  lastEffectTime = now;
  isBursting.value = true;
  setTimeout(() => {
    isBursting.value = false;
  }, 500);
};

watch(currentCover, (coverUrl) => {
  uiStore.updateAccentFromCover(coverUrl);
}, { immediate: true });

</script>

<style scoped>
.apple-stage {
  background: var(--surface-2);
}

.apple-stage-empty-dark {
  background:
    radial-gradient(circle at top, rgba(255, 255, 255, 0.04), transparent 30%),
    var(--surface-2);
}

.apple-stage-empty-light {
  background:
    radial-gradient(circle at top, rgba(124, 92, 191, 0.06), transparent 30%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.84), rgba(244, 244, 246, 0.96)),
    var(--surface-2);
}

.apple-stage-with-cover-dark {
  background:
    radial-gradient(circle at top, rgba(255, 255, 255, 0.05), transparent 28%),
    var(--surface-2);
}

.apple-stage-with-cover-light {
  background:
    radial-gradient(circle at top, rgba(124, 92, 191, 0.04), transparent 28%),
    linear-gradient(180deg, rgba(255, 255, 255, 0.7), rgba(244, 244, 246, 0.92)),
    var(--surface-2);
}
</style>
