<template>
  <div class="lyrics-shell relative flex h-full w-full flex-col" :style="shellStyle">
    <div class="lyrics-shell__inner relative flex h-full w-full flex-col overflow-hidden px-3 py-4 md:px-4 md:py-5">
      <div class="flex h-full w-full flex-1 flex-col items-center justify-center min-h-0">
        <div v-if="showEmptyState" class="lyrics-empty-state flex min-h-0 w-full flex-1 items-center justify-center text-center text-sm font-medium" :class="emptyStateClass">
          暂无歌词
        </div>

        <template v-else>
          <div
            ref="scrollRef"
            class="lyrics-scroll relative w-full flex-1 min-h-0 overflow-y-auto overflow-x-hidden"
            @wheel.passive="handleUserScrollIntent"
            @touchstart.passive="handleTouchStart"
            @touchmove.passive="handleTouchMove"
            @touchend.passive="handleTouchEnd"
          >
            <div class="mx-auto flex min-h-full w-full max-w-[min(760px,88vw)] flex-col justify-center py-4 md:py-5">
              <div class="relative flex flex-col gap-[0.18em] md:gap-[0.24em]" aria-live="off">
              <div
                v-for="(line, index) in displayLines"
                :key="`${line.time}-${index}`"
                :ref="(el) => setLineRef(el, index)"
                class="lyrics-line origin-center transition-[opacity,transform,color,text-shadow] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]"
                :class="getLineClass(index)"
                :style="getLineStyle(index)"
              >
                <span class="lyrics-line__primary">{{ line.text }}</span>
                <span v-if="line.translation" class="lyrics-line__translation">{{ line.translation }}</span>
              </div>
              </div>
            </div>
          </div>

          <div class="mt-3 flex items-center gap-2 md:mt-4">
            <button class="lyrics-control" type="button" @click="decreaseFont" aria-label="减小歌词字号">
              <span class="text-lg leading-none">A−</span>
            </button>
            <button class="lyrics-control" type="button" @click="increaseFont" aria-label="增大歌词字号">
              <span class="text-lg leading-none">A+</span>
            </button>
            <button
              class="lyrics-control lyrics-control--text"
              type="button"
              :class="showTranslation ? 'lyrics-control--active' : ''"
              :aria-pressed="showTranslation"
              aria-label="切换译文显示"
              @click="$emit('toggle-translation')"
            >
              译文
            </button>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { mergeTranslatedLyrics } from '../utils/parser';

const props = defineProps({
  lyrics: {
    type: String,
    default: ''
  },
  translatedLyrics: {
    type: String,
    default: ''
  },
  showTranslation: {
    type: Boolean,
    default: true
  },
  currentTime: {
    type: Number,
    default: 0
  },
  isPlaying: {
    type: Boolean,
    default: false
  },
  isDarkMode: {
    type: Boolean,
    default: true
  },
  bgColor: {
    type: String,
    default: ''
  },
  lyricsLoaded: {
    type: Boolean,
    default: true
  }
});
defineEmits(['toggle-translation']);

const MIN_DISPLAY_LYRIC_LINES = 5;
const lines = computed(() => mergeTranslatedLyrics(
  props.lyrics,
  props.showTranslation ? props.translatedLyrics : ''
));
const displayLines = computed(() => lines.value.length >= MIN_DISPLAY_LYRIC_LINES ? lines.value : []);
const showEmptyState = computed(() => props.lyricsLoaded && !displayLines.value.length);
const scrollRef = ref(null);
const activeIndex = ref(-1);
const isUserScrolling = ref(false);
const manualScrollTimer = ref(null);
const lastAutoScrollAt = ref(0);
const lastScrolledIndex = ref(-2);
const touchStartY = ref(0);
const touchMoved = ref(false);
const fontScale = ref(-2);

const scaledFont = (base) => `${Math.max(10, Math.round(base + fontScale.value * 2)) / 16}rem`;

const shellStyle = computed(() => ({
  '--lyrics-text-active': props.isDarkMode ? 'rgba(255,255,255,0.98)' : 'rgba(26,26,26,0.98)',
  '--lyrics-text-mid': props.isDarkMode ? 'rgba(255,255,255,0.6)' : 'rgba(26,26,26,0.6)',
  '--lyrics-text-low': props.isDarkMode ? 'rgba(255,255,255,0.3)' : 'rgba(26,26,26,0.3)',
  '--lyrics-translation-active': props.isDarkMode ? 'rgba(255,255,255,0.68)' : 'rgba(26,26,26,0.58)',
  '--lyrics-translation-mid': props.isDarkMode ? 'rgba(255,255,255,0.42)' : 'rgba(26,26,26,0.38)',
  '--lyrics-translation-low': props.isDarkMode ? 'rgba(255,255,255,0.22)' : 'rgba(26,26,26,0.22)',
  '--lyrics-empty': props.isDarkMode ? 'rgba(255,255,255,0.5)' : 'rgba(26,26,26,0.55)',
  '--lyrics-accent-glow': props.bgColor || (props.isDarkMode ? 'rgba(255,255,255,0.08)' : 'rgba(26,26,26,0.06)'),
  '--lyrics-active-size': `clamp(${scaledFont(34)}, 4.3vw, ${scaledFont(56)})`,
  '--lyrics-mid-size': `clamp(${scaledFont(20)}, 2vw, ${scaledFont(28)})`,
  '--lyrics-low-size': `clamp(${scaledFont(15)}, 1.35vw, ${scaledFont(19)})`
}));

const emptyStateClass = computed(() => ({
  color: 'var(--lyrics-empty)'
}));

const getActiveIndex = () => {
  if (!displayLines.value.length) return -1;
  if (props.currentTime < displayLines.value[0].time / 1000) return -1;

  const currentMs = props.currentTime * 1000;
  let low = 0;
  let high = displayLines.value.length - 1;
  let answer = -1;

  while (low <= high) {
    const mid = Math.floor((low + high) / 2);
    if (currentMs >= displayLines.value[mid].time) {
      answer = mid;
      low = mid + 1;
    } else {
      high = mid - 1;
    }
  }

  return answer;
};

const getLineClass = (index) => {
  const distance = activeIndex.value === -1 ? index : Math.abs(index - activeIndex.value);
  if (index === activeIndex.value) return 'lyrics-line--active';
  if (distance <= 2) return 'lyrics-line--mid';
  return 'lyrics-line--low';
};

const getLineStyle = (index) => {
  const distance = activeIndex.value === -1 ? index : Math.abs(index - activeIndex.value);
  if (index === activeIndex.value) {
    return {
      opacity: 1,
      transform: 'scale(1) translate3d(0,0,0)',
      fontSize: 'var(--lyrics-active-size)',
      fontWeight: 700,
      lineHeight: 1.3,
      color: 'var(--lyrics-text-active)',
      '--lyrics-translation-size': 'clamp(0.95rem, 1.1vw, 1.28rem)',
      '--lyrics-translation-color': 'var(--lyrics-translation-active)',
      textShadow: props.isDarkMode
        ? '0 8px 24px rgba(0,0,0,0.22), 0 0 22px color-mix(in srgb, var(--lyrics-accent-glow) 36%, transparent)'
        : '0 6px 18px rgba(255,255,255,0.08)'
    };
  }

  if (distance <= 2) {
    const offset = activeIndex.value === -1 ? 0 : (index < activeIndex.value ? -8 : 8);
    return {
      opacity: 0.6,
      transform: `scale(0.84) translate3d(0, ${offset}px, 0)`,
      fontSize: 'var(--lyrics-mid-size)',
      fontWeight: 600,
      lineHeight: 1.28,
      color: 'var(--lyrics-text-mid)',
      '--lyrics-translation-size': 'clamp(0.82rem, 0.95vw, 1.05rem)',
      '--lyrics-translation-color': 'var(--lyrics-translation-mid)'
    };
  }

  const offset = activeIndex.value === -1 ? 0 : (index < activeIndex.value ? -14 : 14);
  return {
    opacity: 0.3,
    transform: `scale(0.72) translate3d(0, ${offset}px, 0)`,
    fontSize: 'var(--lyrics-low-size)',
    fontWeight: 500,
    lineHeight: 1.28,
    color: 'var(--lyrics-text-low)',
    '--lyrics-translation-size': 'clamp(0.75rem, 0.8vw, 0.92rem)',
    '--lyrics-translation-color': 'var(--lyrics-translation-low)'
  };
};

const lineRefs = [];
const setLineRef = (el, index) => {
  if (el) lineRefs[index] = el;
};

const clearManualTimer = () => {
  if (manualScrollTimer.value) {
    clearTimeout(manualScrollTimer.value);
    manualScrollTimer.value = null;
  }
};

const syncActiveLine = async () => {
  const nextIndex = getActiveIndex();
  const indexChanged = nextIndex !== activeIndex.value;
  activeIndex.value = nextIndex;
  await nextTick();
  if (!props.isPlaying || isUserScrolling.value) return;
  if (!indexChanged && lastScrolledIndex.value === nextIndex) return;
  if (Date.now() - lastAutoScrollAt.value < 120) return;
  if (activeIndex.value < 0 || !scrollRef.value) return;

  const el = lineRefs[activeIndex.value];
  if (!el) return;

  const container = scrollRef.value;
  const targetTop = el.offsetTop - container.clientHeight / 2 + el.offsetHeight / 2;
  const maxTop = Math.max(0, container.scrollHeight - container.clientHeight);
  const nextTop = Math.max(0, Math.min(maxTop, targetTop));

  lastAutoScrollAt.value = Date.now();
  lastScrolledIndex.value = nextIndex;
  container.scrollTo({ top: nextTop, behavior: 'smooth' });
};

const handleUserScrollIntent = () => {
  isUserScrolling.value = true;
  clearManualTimer();
  manualScrollTimer.value = setTimeout(() => {
    if (props.isPlaying) {
      isUserScrolling.value = false;
      manualScrollTimer.value = null;
      syncActiveLine();
    }
  }, 3000);
};

const handleTouchStart = (e) => {
  touchStartY.value = e.touches?.[0]?.clientY || 0;
  touchMoved.value = false;
};

const handleTouchMove = (e) => {
  const nextY = e.touches?.[0]?.clientY || 0;
  if (Math.abs(nextY - touchStartY.value) > 8) {
    touchMoved.value = true;
    handleUserScrollIntent();
  }
};

const handleTouchEnd = () => {
  if (touchMoved.value) handleUserScrollIntent();
};

const increaseFont = () => {
  fontScale.value = Math.min(4, fontScale.value + 1);
};

const decreaseFont = () => {
  fontScale.value = Math.max(-2, fontScale.value - 1);
};

watch(() => [props.currentTime, props.lyrics, props.translatedLyrics, props.showTranslation, props.isPlaying], () => {
  syncActiveLine();
}, { immediate: true });

watch(displayLines, () => {
  lineRefs.length = 0;
  lastScrolledIndex.value = -2;
  syncActiveLine();
});

watch(() => props.isPlaying, (playing) => {
  if (playing && isUserScrolling.value) syncActiveLine();
});

onBeforeUnmount(() => {
  clearManualTimer();
});
</script>

<style scoped>
.lyrics-shell {
  background: transparent;
}

.lyrics-scroll {
  scrollbar-width: none;
  -ms-overflow-style: none;
  max-height: 100%;
  scroll-behavior: auto;
  contain: layout style paint;
}

.lyrics-scroll::-webkit-scrollbar {
  display: none;
}

.lyrics-line {
  display: flex;
  flex-direction: column;
  gap: 0.18em;
  max-width: min(760px, 88vw);
  word-break: break-word;
  will-change: transform, opacity;
  letter-spacing: 0;
}

.lyrics-line__primary,
.lyrics-line__translation {
  display: block;
  max-width: min(760px, 88vw);
  word-break: break-word;
  letter-spacing: 0;
}

.lyrics-line__translation {
  color: var(--lyrics-translation-color);
  font-size: var(--lyrics-translation-size);
  font-weight: 560;
  line-height: 1.35;
  text-shadow: none;
}

.lyrics-control {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2.75rem;
  height: 2.75rem;
  border-radius: 9999px;
  border: 1px solid var(--border-default);
  background: var(--surface-3);
  color: var(--text-primary);
  transition: background-color 0.2s ease, color 0.2s ease, border-color 0.2s ease;
  opacity: 0.72;
}

.lyrics-control:hover {
  background: var(--surface-4);
  opacity: 1;
}

.lyrics-control:focus-visible {
  border-color: var(--border-accent);
  box-shadow: 0 0 0 3px var(--accent-muted);
  outline: none;
  opacity: 1;
}

.lyrics-control--text {
  width: auto;
  min-width: 3.25rem;
  padding: 0 0.85rem;
  font-size: 0.78rem;
  font-weight: 800;
}

.lyrics-control--active {
  border-color: var(--border-accent);
  background: var(--accent-subtle);
  color: var(--accent);
  opacity: 1;
}

@media (prefers-reduced-motion: reduce) {
  .lyrics-line {
    transition-duration: 0.01ms;
  }

  .lyrics-control {
    transition-duration: 0.01ms;
  }
}
</style>
