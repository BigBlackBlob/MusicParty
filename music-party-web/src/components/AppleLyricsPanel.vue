<template>
  <div class="lyrics-shell relative flex h-full w-full flex-col" :class="{ 'lyrics-shell--mobile': mobile }" :style="shellStyle">
    <div class="lyrics-shell__inner relative flex h-full w-full min-w-0 flex-col overflow-hidden px-3 md:px-4">
      <div class="flex h-full w-full flex-1 flex-col items-center justify-center min-h-0">
        <div v-if="showEmptyState" class="lyrics-empty-state flex min-h-0 w-full flex-1 items-center justify-center text-center text-sm font-medium" :class="emptyStateClass">
          {{ t('lyrics.empty') }}
        </div>

        <template v-else>
          <div
            ref="scrollRef"
            class="lyrics-scroll relative w-full flex-1 min-h-0 overflow-y-auto overflow-x-hidden [mask-image:linear-gradient(to_bottom,transparent,black_5%,black_95%,transparent)]"
            @wheel.passive="handleUserScrollIntent"
            @touchstart.passive="handleTouchStart"
            @touchmove.passive="handleTouchMove"
            @touchend.passive="handleTouchEnd"
          >
            <div class="lyrics-scroll__content mx-auto flex w-full max-w-[min(760px,88vw)] flex-col" :class="containerAlignmentClass">
              <div class="lyrics-scroll__spacer w-full shrink-0"></div>
              <div class="relative flex w-full min-w-0 shrink-0 flex-col gap-[0.18em] md:gap-[0.24em]" aria-live="off" :class="containerAlignmentClass">
              <div
                v-for="(line, index) in displayLines"
                :key="`${line.time}-${index}`"
                :ref="(el) => setLineRef(el, index)"
                class="lyrics-line transition-[opacity,transform,color,text-shadow] duration-300 ease-[cubic-bezier(0.16,1,0.3,1)]"
                :class="[getLineClass(index), alignmentClass]"
                :style="getLineStyle(index)"
              >
                <span class="lyrics-line__primary">{{ line.text }}</span>
                <span v-if="line.translation" class="lyrics-line__translation">{{ line.translation }}</span>
              </div>
              </div>
              <div class="lyrics-scroll__spacer w-full shrink-0"></div>
            </div>
          </div>

          <div class="lyrics-controls mt-3 flex items-center justify-center gap-2 md:mt-4 w-full shrink-0">
            <button class="lyrics-control" type="button" @click="toggleAlignment" :aria-label="t('lyrics.toggleAlignment')" :title="t('lyrics.toggleAlignment')">
              <span class="material-symbols-outlined text-[18px]">{{ alignmentIcon }}</span>
            </button>
            <button class="lyrics-control" type="button" @click="decreaseFont" :aria-label="t('lyrics.decreaseFont')" :title="t('lyrics.decreaseFont')">
              <span class="lyrics-control__label">A−</span>
            </button>
            <button class="lyrics-control" type="button" @click="increaseFont" :aria-label="t('lyrics.increaseFont')" :title="t('lyrics.increaseFont')">
              <span class="lyrics-control__label">A+</span>
            </button>
            <button
              class="lyrics-control lyrics-control--text"
              type="button"
              :class="showTranslation ? 'lyrics-control--active' : ''"
              :aria-pressed="showTranslation"
              :aria-label="t('lyrics.toggleTranslation')"
              :title="t('lyrics.toggleTranslation')"
              @click="$emit('toggle-translation')"
            >
              {{ t('lyrics.translation') }}
            </button>
          </div>
        </template>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { mergeTranslatedLyrics } from '../utils/parser';
import { useUiStore } from '../stores/ui';

const uiStore = useUiStore();
const { t } = useI18n();

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
  currentTimeMs: {
    type: Number,
    default: null
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
  },
  mobile: {
    type: Boolean,
    default: false
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
const activeTimeMs = computed(() => {
  if (Number.isFinite(props.currentTimeMs)) return Math.max(0, props.currentTimeMs);
  return Math.max(0, props.currentTime * 1000);
});
const scrollRef = ref(null);
const activeIndex = ref(-1);
const isUserScrolling = ref(false);
const manualScrollTimer = ref(null);
const lastAutoScrollAt = ref(0);
const lastScrolledIndex = ref(-2);
const touchStartY = ref(0);
const touchMoved = ref(false);
const fontScale = ref(-2);
let syncRequestId = 0;
let scrollFrameId = null;

const alignmentIcon = computed(() => {
  switch (uiStore.lyricAlignment) {
    case 'left': return 'format_align_left';
    case 'right': return 'format_align_right';
    default: return 'format_align_center';
  }
});

const containerAlignmentClass = computed(() => {
  switch (uiStore.lyricAlignment) {
    case 'left': return 'items-start';
    case 'right': return 'items-end';
    default: return 'items-center';
  }
});

const alignmentClass = computed(() => {
  switch (uiStore.lyricAlignment) {
    case 'left': return 'text-left origin-left';
    case 'right': return 'text-right origin-right';
    default: return 'text-center origin-center';
  }
});

const toggleAlignment = () => {
  const modes = ['center', 'left', 'right'];
  const currentIndex = modes.indexOf(uiStore.lyricAlignment);
  uiStore.setLyricAlignment(modes[(currentIndex + 1) % modes.length]);
};

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
  '--lyrics-active-size': props.mobile
    ? `clamp(${scaledFont(22)}, 7vw, ${scaledFont(34)})`
    : `clamp(${scaledFont(34)}, 4.3vw, ${scaledFont(56)})`,
  '--lyrics-mid-size': props.mobile
    ? `clamp(${scaledFont(16)}, 4.8vw, ${scaledFont(22)})`
    : `clamp(${scaledFont(20)}, 2vw, ${scaledFont(28)})`,
  '--lyrics-low-size': props.mobile
    ? `clamp(${scaledFont(13)}, 3.8vw, ${scaledFont(17)})`
    : `clamp(${scaledFont(15)}, 1.35vw, ${scaledFont(19)})`,
  '--lyrics-translation-active-size': props.mobile ? 'clamp(0.82rem, 3.6vw, 1rem)' : 'clamp(0.95rem, 1.1vw, 1.28rem)',
  '--lyrics-translation-mid-size': props.mobile ? 'clamp(0.76rem, 3.2vw, 0.92rem)' : 'clamp(0.82rem, 0.95vw, 1.05rem)',
  '--lyrics-translation-low-size': props.mobile ? 'clamp(0.7rem, 3vw, 0.84rem)' : 'clamp(0.75rem, 0.8vw, 0.92rem)'
}));

const emptyStateClass = computed(() => ({
  color: 'var(--lyrics-empty)'
}));

const getActiveIndex = () => {
  if (!displayLines.value.length) return -1;
  if (activeTimeMs.value < displayLines.value[0].time) return -1;

  const currentMs = activeTimeMs.value;
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
      '--lyrics-translation-size': 'var(--lyrics-translation-active-size)',
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
      '--lyrics-translation-size': 'var(--lyrics-translation-mid-size)',
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
    '--lyrics-translation-size': 'var(--lyrics-translation-low-size)',
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

const syncActiveLine = async (force = false) => {
  const requestId = ++syncRequestId;
  const nextIndex = getActiveIndex();
  const indexChanged = nextIndex !== activeIndex.value;
  const targetIndex = nextIndex === -1 ? 0 : nextIndex;
  const needsRecentering = force || lastScrolledIndex.value !== targetIndex;

  if (!indexChanged && !needsRecentering) return;

  activeIndex.value = nextIndex;

  await nextTick();

  if (requestId !== syncRequestId) return;
  if (!scrollRef.value || displayLines.value.length === 0) return;
  if (isUserScrolling.value && !force) return;

  if (Date.now() - lastAutoScrollAt.value < 50 && !force) return;

  const el = lineRefs[targetIndex];
  if (!el) return;

  const container = scrollRef.value;
  const containerRect = container.getBoundingClientRect();
  const lineRect = el.getBoundingClientRect();
  const visualDelta = (lineRect.top + lineRect.height / 2) - (containerRect.top + containerRect.height / 2);
  const maxTop = Math.max(0, container.scrollHeight - container.clientHeight);
  const nextTop = Math.max(0, Math.min(maxTop, container.scrollTop + visualDelta));

  if (scrollFrameId !== null) cancelAnimationFrame(scrollFrameId);
  scrollFrameId = requestAnimationFrame(() => {
    scrollFrameId = null;
    if (requestId !== syncRequestId || !scrollRef.value) return;

    lastAutoScrollAt.value = Date.now();
    lastScrolledIndex.value = targetIndex;

    scrollRef.value.scrollTo({
      top: nextTop,
      behavior: (indexChanged && props.isPlaying && !force) ? 'smooth' : 'auto'
    });
  });
};

const handleUserScrollIntent = () => {
  isUserScrolling.value = true;
  clearManualTimer();
  manualScrollTimer.value = setTimeout(() => {
    if (props.isPlaying) {
      isUserScrolling.value = false;
      manualScrollTimer.value = null;
      syncActiveLine(true);
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

watch(() => [activeTimeMs.value, props.showTranslation], () => {
  syncActiveLine();
}, { immediate: true });

watch(displayLines, (newLines, oldLines) => {
  // 只有当行数发生变化或从无到有时，才重置滚动索引和 lineRefs
  // 防止因为 prop 引用变化但内容不变导致的跳动
  if (newLines.length !== oldLines?.length || (newLines.length > 0 && oldLines?.length === 0)) {
    lineRefs.length = 0;
    lastScrolledIndex.value = -2;
    syncActiveLine(true);
  }
}, { immediate: true });

watch(() => props.isPlaying, (playing) => {
  if (playing && isUserScrolling.value) syncActiveLine(true);
});

onBeforeUnmount(() => {
  clearManualTimer();
  if (scrollFrameId !== null) cancelAnimationFrame(scrollFrameId);
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

.lyrics-scroll__spacer {
  height: 45vh;
}

.lyrics-scroll::-webkit-scrollbar {
  display: none;
}

.lyrics-line {
  display: flex;
  flex-direction: column;
  gap: 0.18em;
  max-width: min(760px, 88vw);
  min-width: 0;
  word-break: break-word;
  will-change: transform, opacity;
  letter-spacing: 0;
}

.lyrics-line__primary,
.lyrics-line__translation {
  display: block;
  max-width: min(760px, 88vw);
  overflow-wrap: anywhere;
  word-break: break-word;
  letter-spacing: 0;
}

.lyrics-shell--mobile .lyrics-shell__inner {
  padding: 8px 18px calc(10px + env(safe-area-inset-bottom));
}

.lyrics-shell--mobile .lyrics-scroll {
  width: 100%;
  padding-top: 4px;
  padding-bottom: 6px;
  mask-image: linear-gradient(to bottom, transparent, black 8%, black 92%, transparent);
}

.lyrics-shell--mobile .lyrics-scroll__content {
  max-width: 100%;
}

.lyrics-shell--mobile .lyrics-scroll__spacer {
  height: 38dvh;
}

.lyrics-shell--mobile .lyrics-line {
  max-width: 100%;
  width: 100%;
  transform-origin: inherit;
}

.lyrics-shell--mobile .lyrics-line__primary,
.lyrics-shell--mobile .lyrics-line__translation {
  max-width: 100%;
  width: 100%;
}

.lyrics-shell--mobile .lyrics-line__primary {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.lyrics-shell--mobile .lyrics-line__translation {
  display: -webkit-box;
  overflow: hidden;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.lyrics-shell--mobile .lyrics-controls {
  position: sticky;
  bottom: 0;
  display: grid;
  grid-template-columns: 2.5rem 2.5rem 2.5rem minmax(3.4rem, auto);
  justify-content: center;
  gap: 8px;
  margin-top: 8px;
  padding: 8px 0 calc(4px + env(safe-area-inset-bottom));
  background: color-mix(in srgb, var(--surface-0) 92%, transparent);
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
  width: 2.5rem;
  height: 2.5rem;
  min-width: 2.5rem;
  min-height: 2.5rem;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-default);
  background: var(--surface-3);
  color: var(--text-primary);
  font: inherit;
  line-height: 1;
  transition: background-color 0.2s ease, color 0.2s ease, border-color 0.2s ease;
  opacity: 0.72;
}

.lyrics-controls {
  flex-shrink: 0;
  justify-content: center;
  min-height: 2.5rem;
  padding-bottom: env(safe-area-inset-bottom);
}

.lyrics-control__label {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 1em;
  font-size: 0.98rem;
  font-weight: 800;
  line-height: 1;
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
  min-width: 3.4rem;
  padding: 0 0.85rem;
  font-size: 0.76rem;
  font-weight: 800;
  line-height: 1;
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
