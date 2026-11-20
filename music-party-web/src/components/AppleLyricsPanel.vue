<template>
  <div class="lyrics-shell relative flex h-full w-full flex-col" :class="{ 'lyrics-shell--mobile': mobile }" :style="shellStyle">
    <div class="lyrics-shell__inner relative flex h-full w-full min-w-0 flex-col overflow-hidden px-3 md:px-4">
      <div
        class="relative flex h-full w-full flex-1 flex-col items-center justify-center min-h-0"
        @mouseenter="revealControls"
        @mouseleave="scheduleControlsHide"
        @focusin="showControlsNow"
        @focusout="scheduleControlsHide"
      >
        <Transition name="lyrics-stage-fade" mode="out-in">
        <div v-if="showEmptyState" key="empty" class="lyrics-empty-state flex min-h-0 w-full flex-1 flex-col items-center justify-center gap-3 text-center">
          <span class="lyrics-empty-state__icon material-symbols-outlined" aria-hidden="true">music_note</span>
          <span class="lyrics-empty-state__label text-sm font-medium" :style="emptyStateClass">{{ t('lyrics.empty') }}</span>
        </div>

        <div v-else key="lines" class="relative flex h-full w-full flex-1 flex-col items-center justify-center min-h-0">
          <div
            ref="scrollRef"
            class="lyrics-scroll relative w-full flex-1 min-h-0 overflow-y-auto overflow-x-hidden [mask-image:linear-gradient(to_bottom,transparent,black_10%,black_88%,transparent)]"
            @mouseenter="isHoveringList = true"
            @mouseleave="isHoveringList = false"
            @wheel.passive="handleUserScrollIntent"
            @touchstart.passive="handleTouchStart"
            @touchmove.passive="handleTouchMove"
            @touchend.passive="handleTouchEnd"
            @scroll.passive="handleScroll"
          >
            <div class="lyrics-scroll__content mx-auto flex w-full max-w-[min(760px,88vw)] flex-col" :class="containerAlignmentClass">
              <div class="lyrics-scroll__spacer w-full shrink-0"></div>
              <div class="relative flex w-full min-w-0 shrink-0 flex-col gap-[0.55em] md:gap-[0.72em]" aria-live="off" :class="containerAlignmentClass">
              <div
                v-for="(line, index) in displayLines"
                :key="`${line.time}-${index}`"
                :ref="(el) => setLineRef(el, index)"
                class="lyrics-line"
                :class="[getLineClass(index), alignmentClass, { 'lyrics-line--clear': isClearReadingMode }]"
                :style="getLineStyle(index)"
              >
                <span v-if="index === activeIndex && line.segments" class="lyrics-line__primary lyrics-line__words">
                  <span
                    v-for="(segment, segmentIndex) in line.segments"
                    :key="segmentIndex"
                    class="lyrics-word"
                    :class="wordStateClass(segment)"
                    :style="wordStateStyle(segment)"
                  >{{ segment.text }}</span>
                </span>
                <span v-else class="lyrics-line__primary">{{ line.text }}</span>
                <span v-if="line.translation" class="lyrics-line__translation">{{ line.translation }}</span>
                <span v-if="line.romanization" class="lyrics-line__romanization">{{ line.romanization }}</span>
              </div>
              </div>
              <div class="lyrics-scroll__spacer w-full shrink-0"></div>
            </div>
          </div>

          <div class="lyrics-controls flex items-center justify-center gap-2" :class="{ 'lyrics-controls--visible': controlsVisible }">
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
            <button
              v-if="hasRomanization"
              class="lyrics-control lyrics-control--text"
              type="button"
              :class="showRomanization ? 'lyrics-control--active' : ''"
              :aria-pressed="showRomanization"
              :aria-label="t('lyrics.romanization')"
              :title="t('lyrics.romanization')"
              @click="$emit('toggle-romanization')"
            >
              {{ t('lyrics.romanization') }}
            </button>
          </div>
        </div>
        </Transition>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { mergeTranslatedLyrics, parseLyrics } from '../utils/parser';
import { pairLyricExtras, parseYrc } from '../utils/yrc';
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
  romanizedLyrics: {
    type: String,
    default: ''
  },
  wordLyric: {
    type: String,
    default: ''
  },
  wordTranslatedLyric: {
    type: String,
    default: ''
  },
  wordRomanizedLyric: {
    type: String,
    default: ''
  },
  showTranslation: {
    type: Boolean,
    default: true
  },
  showRomanization: {
    type: Boolean,
    default: false
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
defineEmits(['toggle-translation', 'toggle-romanization']);

const MIN_DISPLAY_LYRIC_LINES = 5;
const wordLines = computed(() => parseYrc(props.wordLyric));
const hasRomanization = computed(() =>
  parseYrc(props.wordRomanizedLyric).length > 0 || parseLyrics(props.romanizedLyrics).length > 0
);
const lines = computed(() => {
  if (wordLines.value.length) {
    const translations = props.showTranslation
      ? pairLyricExtras(wordLines.value, props.wordTranslatedLyric, props.lyrics, props.translatedLyrics) : [];
    const romanizations = props.showRomanization
      ? pairLyricExtras(wordLines.value, props.wordRomanizedLyric, props.lyrics, props.romanizedLyrics) : [];
    return wordLines.value.map((line, index) => ({
      time: line.start,
      text: line.text,
      translation: translations[index] || '',
      romanization: romanizations[index] || '',
      segments: line.segments
    }));
  }
  const plain = mergeTranslatedLyrics(
    props.lyrics,
    props.showTranslation ? props.translatedLyrics : ''
  );
  if (!props.showRomanization || !plain.length) return plain;
  const romanByTime = new Map(parseLyrics(props.romanizedLyrics).map((line) => [line.time, line.text]));
  return plain.map((line) => ({ ...line, romanization: romanByTime.get(line.time) || '' }));
});
const displayLines = computed(() => lines.value.length >= MIN_DISPLAY_LYRIC_LINES ? lines.value : []);
const showEmptyState = computed(() => props.lyricsLoaded && !displayLines.value.length);
const activeTimeMs = computed(() => {
  if (Number.isFinite(props.currentTimeMs)) return Math.max(0, props.currentTimeMs);
  return Math.max(0, props.currentTime * 1000);
});
const scrollRef = ref(null);
const activeIndex = ref(-1);
const isUserScrolling = ref(false);
const isHoveringList = ref(false);
const isClearReadingMode = computed(() => isUserScrolling.value || isHoveringList.value);
const controlsVisible = ref(false);
const manualScrollTimer = ref(null);
const controlsTimer = ref(null);
const lastAutoScrollAt = ref(0);
const lastScrolledIndex = ref(-2);
const lastScrollTop = ref(0);
const touchStartY = ref(0);
const touchMoved = ref(false);
const fontScale = ref(-2);
let syncRequestId = 0;
let scrollFrameId = null;
let smoothScrollRafId = null;
let programmaticScroll = false;

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
  syncActiveLine(true);
};

const scaledFont = (base) => `${Math.max(10, Math.round(base + fontScale.value * 2)) / 16}rem`;

const shellStyle = computed(() => ({
  '--lyrics-accent-glow': props.bgColor || 'var(--lyrics-glow-fallback)',
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
  const hasActive = activeIndex.value !== -1;
  const signedDelta = hasActive ? index - activeIndex.value : index;
  const distance = Math.abs(signedDelta);
  const clearMode = isClearReadingMode.value;
  // Subtle Apple Music wave stagger on auto-follow; instant response on hover/manual scroll
  const staggerMs = clearMode || !hasActive ? 0 : Math.min(distance, 5) * 28;

  if (index === activeIndex.value) {
    return {
      opacity: 1,
      transform: 'scale(1) translate3d(0, 0, 0)',
      filter: 'blur(0px)',
      transitionDelay: '0ms',
      fontSize: 'var(--lyrics-active-size)',
      fontWeight: 700,
      lineHeight: 1.24,
      color: 'var(--lyrics-text-active)',
      '--lyrics-translation-size': 'var(--lyrics-translation-active-size)',
      '--lyrics-translation-color': 'var(--lyrics-translation-active)',
      textShadow: '0 0 26px var(--lyrics-active-glow)'
    };
  }

  // Constant fontSize + GPU-only scale eliminates layout reflow jump between lines
  const scale = distance === 1 ? 0.91 : distance === 2 ? 0.86 : 0.82;
  const offsetY = !hasActive ? 0 : (signedDelta < 0 ? -Math.min(distance * 3, 10) : Math.min(distance * 3, 10));
  const opacity = clearMode
    ? (distance === 1 ? 0.72 : distance === 2 ? 0.56 : 0.44)
    : (distance === 1 ? 0.52 : distance === 2 ? 0.34 : 0.2);
  const blurPx = clearMode ? 0 : (distance === 1 ? 0.65 : distance === 2 ? 1.35 : 2.1);

  return {
    opacity,
    transform: `scale(${scale}) translate3d(0, ${offsetY}px, 0)`,
    filter: `blur(${blurPx}px)`,
    transitionDelay: `${staggerMs}ms`,
    fontSize: 'var(--lyrics-active-size)',
    fontWeight: distance === 1 ? 650 : 600,
    lineHeight: 1.24,
    color: distance <= 2 ? 'var(--lyrics-text-mid)' : 'var(--lyrics-text-low)',
    '--lyrics-translation-size': 'var(--lyrics-translation-active-size)',
    '--lyrics-translation-color': distance <= 2 ? 'var(--lyrics-translation-mid)' : 'var(--lyrics-translation-low)',
    textShadow: 'none'
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

const clearControlsTimer = () => {
  if (controlsTimer.value) {
    clearTimeout(controlsTimer.value);
    controlsTimer.value = null;
  }
};

const showControlsNow = () => {
  clearControlsTimer();
  controlsVisible.value = true;
};

const scheduleControlsHide = () => {
  clearControlsTimer();
  controlsTimer.value = setTimeout(() => {
    controlsVisible.value = false;
    controlsTimer.value = null;
  }, 2500);
};

const revealControls = () => {
  showControlsNow();
  scheduleControlsHide();
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
  // Optical center (~43% from top, matching Apple Music focus sweet spot)
  const opticalAnchor = containerRect.top + containerRect.height * 0.43;
  const visualDelta = (lineRect.top + lineRect.height / 2) - opticalAnchor;
  const maxTop = Math.max(0, container.scrollHeight - container.clientHeight);
  const nextTop = Math.max(0, Math.min(maxTop, container.scrollTop + visualDelta));

  if (scrollFrameId !== null) cancelAnimationFrame(scrollFrameId);
  if (smoothScrollRafId !== null) {
    cancelAnimationFrame(smoothScrollRafId);
    smoothScrollRafId = null;
  }
  scrollFrameId = requestAnimationFrame(() => {
    scrollFrameId = null;
    if (requestId !== syncRequestId || !scrollRef.value) return;

    lastAutoScrollAt.value = Date.now();
    lastScrolledIndex.value = targetIndex;

    const shouldAnimate = indexChanged && props.isPlaying && !force;
    if (!shouldAnimate) {
      programmaticScroll = true;
      scrollRef.value.scrollTop = nextTop;
      lastScrollTop.value = scrollRef.value.scrollTop;
      programmaticScroll = false;
      return;
    }

    const startTop = scrollRef.value.scrollTop;
    const delta = nextTop - startTop;
    if (Math.abs(delta) < 1) return;

    const duration = 560;
    const startTime = performance.now();
    programmaticScroll = true;

    const step = (now) => {
      if (!scrollRef.value || isUserScrolling.value) {
        smoothScrollRafId = null;
        programmaticScroll = false;
        return;
      }
      const t = Math.min(1, (now - startTime) / duration);
      // Quartic ease-out: snappy pull with velvety Apple Music landing
      const eased = 1 - Math.pow(1 - t, 4);
      scrollRef.value.scrollTop = startTop + delta * eased;
      lastScrollTop.value = scrollRef.value.scrollTop;
      if (t < 1) {
        smoothScrollRafId = requestAnimationFrame(step);
      } else {
        smoothScrollRafId = null;
        programmaticScroll = false;
      }
    };
    smoothScrollRafId = requestAnimationFrame(step);
  });
};

const handleUserScrollIntent = () => {
  if (smoothScrollRafId !== null) {
    cancelAnimationFrame(smoothScrollRafId);
    smoothScrollRafId = null;
    programmaticScroll = false;
  }
  revealControls();
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

const handleScroll = () => {
  if (!scrollRef.value) return;
  const nextTop = scrollRef.value.scrollTop;
  const changedByUser = !programmaticScroll && Math.abs(nextTop - lastScrollTop.value) > 1;
  lastScrollTop.value = nextTop;
  if (changedByUser) handleUserScrollIntent();
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
  revealControls();
  syncActiveLine(true);
};

const decreaseFont = () => {
  fontScale.value = Math.max(-2, fontScale.value - 1);
  revealControls();
  syncActiveLine(true);
};

// The room clock ticks at ~200ms cadence, too coarse for a word sweep.
// While the active line has word segments, extrapolate from the last tick with
// a requestAnimationFrame anchor; each new tick re-bases the anchor.
const karaokeNow = ref(activeTimeMs.value);
let karaokeAnchor = activeTimeMs.value;
let karaokeAnchorWall = 0;
let karaokeFrame = null;

const karaokeTicking = computed(() => {
  if (!props.isPlaying || activeIndex.value < 0) return false;
  return !!displayLines.value[activeIndex.value]?.segments;
});

const advanceKaraoke = () => {
  if (!karaokeTicking.value) {
    karaokeFrame = null;
    karaokeAnchor = activeTimeMs.value;
    karaokeAnchorWall = performance.now();
    karaokeNow.value = karaokeAnchor;
    return;
  }
  karaokeNow.value = karaokeAnchor + (performance.now() - karaokeAnchorWall);
  karaokeFrame = requestAnimationFrame(advanceKaraoke);
};

watch(activeTimeMs, (value) => {
  karaokeAnchor = value;
  karaokeAnchorWall = performance.now();
  if (!karaokeTicking.value) karaokeNow.value = value;
}, { immediate: true });

watch(karaokeTicking, (ticking) => {
  if (ticking && karaokeFrame === null) {
    karaokeFrame = requestAnimationFrame(advanceKaraoke);
  } else if (!ticking && karaokeFrame !== null) {
    cancelAnimationFrame(karaokeFrame);
    karaokeFrame = null;
    karaokeNow.value = activeTimeMs.value;
  }
});

const wordStateClass = (segment) => {
  const now = karaokeNow.value;
  if (segment.dur <= 0) return now > segment.start ? 'lyrics-word--done' : 'lyrics-word--todo';
  if (now >= segment.end) return 'lyrics-word--done';
  if (now >= segment.start) return 'lyrics-word--active';
  return 'lyrics-word--todo';
};

const wordStateStyle = (segment) => {
  const now = karaokeNow.value;
  if (segment.dur <= 0 || now < segment.start || now >= segment.end) return null;
  const ratio = Math.min(1, Math.max(0, (now - segment.start) / segment.dur));
  return { '--word-fill': `${(ratio * 100).toFixed(1)}%` };
};

watch(() => [activeTimeMs.value, props.showTranslation, props.showRomanization], () => {
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
  clearControlsTimer();
  if (scrollFrameId !== null) cancelAnimationFrame(scrollFrameId);
  if (smoothScrollRafId !== null) cancelAnimationFrame(smoothScrollRafId);
  if (karaokeFrame !== null) cancelAnimationFrame(karaokeFrame);
});
</script>

<style scoped>
.lyrics-shell {
  background: transparent;
}

@media (min-width: 768px) {
  .lyrics-scroll__content {
    max-width: 820px;
  }

  /* The panel owns the inset; each line gets the remaining column width. */
  .lyrics-shell:not(.lyrics-shell--mobile) .lyrics-line,
  .lyrics-shell:not(.lyrics-shell--mobile) .lyrics-line__primary,
  .lyrics-shell:not(.lyrics-shell--mobile) .lyrics-line__translation,
  .lyrics-shell:not(.lyrics-shell--mobile) .lyrics-line__romanization {
    width: 100%;
    max-width: 100%;
    word-break: normal;
    line-break: strict;
    text-wrap: pretty;
  }

  .lyrics-line--active {
    transform-origin: center center;
  }
}

.lyrics-scroll {
  scrollbar-width: none;
  -ms-overflow-style: none;
  max-height: 100%;
  scroll-behavior: auto;
  contain: layout style paint;
  padding-bottom: 4.5rem;
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
  will-change: transform, opacity, filter;
  letter-spacing: -0.015em;
  transition:
    transform 560ms cubic-bezier(0.22, 1, 0.36, 1),
    opacity 480ms cubic-bezier(0.22, 1, 0.36, 1),
    filter 420ms cubic-bezier(0.22, 1, 0.36, 1),
    color 380ms cubic-bezier(0.22, 1, 0.36, 1),
    text-shadow 480ms cubic-bezier(0.22, 1, 0.36, 1);
}

.lyrics-line--clear:not(.lyrics-line--active):hover {
  opacity: 0.92 !important;
  filter: blur(0px) !important;
  transition-delay: 0ms !important;
}

@container stage-column (max-width: 400px) {
  .lyrics-line--active {
    font-size: clamp(1rem, 7cqw, 1.75rem) !important;
  }

  .lyrics-line--mid {
    font-size: clamp(0.9rem, 5cqw, 1.25rem) !important;
  }
}

.lyrics-line__primary,
.lyrics-line__translation {
  display: block;
  max-width: min(760px, 88vw);
  overflow-wrap: anywhere;
  word-break: break-word;
  letter-spacing: 0;
}

.lyrics-line__words {
  white-space: pre-wrap;
}

.lyrics-word {
  display: inline-block;
  white-space: pre-wrap;
  transition: transform 240ms cubic-bezier(0.22, 1, 0.36, 1),
    text-shadow 260ms cubic-bezier(0.22, 1, 0.36, 1);
}

.lyrics-word--done {
  color: var(--lyrics-text-active);
  transform: translateY(0);
}

.lyrics-word--todo {
  color: var(--lyrics-word-todo);
  transform: translateY(0);
}

.lyrics-word--active {
  background-image: linear-gradient(
    90deg,
    var(--lyrics-text-active) 0%,
    var(--lyrics-text-active) max(0%, calc(var(--word-fill, 0%) - 10%)),
    var(--lyrics-word-todo) min(100%, calc(var(--word-fill, 0%) + 14%)),
    var(--lyrics-word-todo) 100%
  );
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  transform: translateY(-1.5px);
  text-shadow: 0 0 18px var(--lyrics-active-glow);
}

.lyrics-line__romanization {
  color: var(--lyrics-translation-color);
  font-size: var(--lyrics-translation-size);
  font-weight: 500;
  line-height: 1.3;
  opacity: 0.75;
}

.lyrics-shell--mobile .lyrics-shell__inner {
  padding: 8px 18px calc(10px + env(safe-area-inset-bottom));
}

.lyrics-shell--mobile .lyrics-scroll {
  width: 100%;
  padding-top: 4px;
  padding-bottom: calc(5rem + env(safe-area-inset-bottom));
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
  display: grid;
  grid-template-columns: 2.5rem 2.5rem 2.5rem minmax(3.4rem, auto);
  gap: 8px;
  padding-bottom: calc(8px + env(safe-area-inset-bottom));
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

.lyrics-empty-state__icon {
  font-size: 36px;
  font-variation-settings: 'wght' 300;
  color: var(--lyrics-accent-glow, var(--accent));
  opacity: 0.55;
  filter: drop-shadow(0 0 18px color-mix(in srgb, var(--lyrics-accent-glow, var(--accent)) 40%, transparent));
}

.lyrics-empty-state__label {
  letter-spacing: 0.01em;
}

.lyrics-controls {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 2;
  flex-shrink: 0;
  justify-content: center;
  min-height: 4rem;
  padding: 10px 12px calc(10px + env(safe-area-inset-bottom));
  background: linear-gradient(to top, color-mix(in srgb, var(--surface-0) 88%, transparent), transparent);
  opacity: 0;
  pointer-events: none;
  transform: translateY(12px);
  transition: opacity 0.22s ease, transform 0.22s ease;
}

.lyrics-controls--visible,
.lyrics-controls:focus-within {
  opacity: 1;
  pointer-events: auto;
  transform: translateY(0);
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

.lyrics-stage-fade-enter-active {
  transition: opacity var(--motion-panel) var(--motion-ease-standard);
}

.lyrics-stage-fade-leave-active {
  transition: opacity var(--motion-exit) var(--motion-ease-in);
}

.lyrics-stage-fade-enter-from,
.lyrics-stage-fade-leave-to {
  opacity: 0;
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
</style>
