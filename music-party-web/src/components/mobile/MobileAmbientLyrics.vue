<template>
  <Transition name="ambl-rise">
    <div v-if="open" class="ambl-wrap" role="dialog" aria-modal="true" :aria-label="t('lyrics.title', 'Lyrics')">

      <!-- Blurred cover backdrop -->
      <div class="ambl-bg" aria-hidden="true">
        <img v-if="coverUrl" :src="coverUrl" class="ambl-bg__art" alt="" draggable="false" />
        <div v-else class="ambl-bg__art ambl-bg__art--fallback ambient-canvas" />
        <div class="ambl-bg__scrim" />
        <div class="ambl-bg__vignette" />
      </div>

      <!-- Header: "LYRICS" label + dismiss -->
      <header class="ambl-header">
        <span class="ambl-header__label">{{ t('lyrics.title', 'Lyrics') }}</span>
        <button
          class="ambl-header__close"
          :aria-label="t('common.close', 'Close')"
          @click="$emit('close')"
        >
          <span class="material-symbols-outlined" style="font-size:28px">keyboard_arrow_down</span>
        </button>
      </header>

      <!-- Floating lyric stage -->
      <section class="ambl-stage" aria-live="off">
        <div class="ambl-stage__fade ambl-stage__fade--top" aria-hidden="true" />
        <div class="ambl-stage__fade ambl-stage__fade--bottom" aria-hidden="true" />
        <div v-if="showEmptyState" class="ambl-empty">
          <span
            class="material-symbols-outlined"
            style="font-size:38px; color:var(--accent); opacity:0.4; font-variation-settings:'wght' 300"
            aria-hidden="true"
          >music_note</span>
          <span class="ambl-empty__label">{{ t('lyrics.empty') }}</span>
        </div>
        <template v-else>
          <div class="ambl-glow" aria-hidden="true" />
          <div
            v-for="slot in visibleSlots"
            :key="slot.line.time ?? slot.line.text"
            class="ambl-line"
            :class="`ambl-line--${slot.role}`"
          >
            <span class="ambl-line__text">{{ slot.line.text }}</span>
            <span
              v-if="slot.role === 'active' && slot.line.translation && showTranslation"
              class="ambl-line__tl"
            >{{ slot.line.translation }}</span>
            <span
              v-if="slot.role === 'active' && slot.line.romanization && showRomanization"
              class="ambl-line__ro"
            >{{ slot.line.romanization }}</span>
          </div>
        </template>
      </section>

      <!-- Bottom dock: mini cover + info, translation/romanization toggles -->
      <footer class="ambl-dock">
        <div class="ambl-track">
          <div class="ambl-track__cover" aria-hidden="true">
            <img v-if="coverUrl" :src="coverUrl" alt="" draggable="false" class="ambl-track__cover-img" />
            <div v-else class="ambl-track__cover-fallback ambient-canvas" />
          </div>
          <div class="ambl-track__info">
            <span class="ambl-track__title">{{ trackTitle }}</span>
            <span class="ambl-track__artist">{{ artistLine }}</span>
          </div>
          <div class="ambl-track__pills">
            <button
              class="ambl-pill"
              :class="{ 'ambl-pill--on': showTranslation }"
              :aria-pressed="showTranslation"
              :aria-label="t('lyrics.toggleTranslation')"
              @click="$emit('toggle-translation')"
            >{{ t('lyrics.translation') }}</button>
            <button
              v-if="hasRomanization"
              class="ambl-pill"
              :class="{ 'ambl-pill--on': showRomanization }"
              :aria-pressed="showRomanization"
              :aria-label="t('lyrics.romanization')"
              @click="$emit('toggle-romanization')"
            >{{ t('lyrics.romanization') }}</button>
          </div>
        </div>
      </footer>

    </div>
  </Transition>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { mergeTranslatedLyrics, parseLyrics } from '../../utils/parser';
import { pairLyricExtras, parseYrc } from '../../utils/yrc';

const { t } = useI18n();

const props = defineProps({
  open: { type: Boolean, default: false },
  coverUrl: { type: String, default: '' },
  trackTitle: { type: String, default: '' },
  artistLine: { type: String, default: '' },
  lyrics: { type: String, default: '' },
  translatedLyrics: { type: String, default: '' },
  romanizedLyrics: { type: String, default: '' },
  wordLyric: { type: String, default: '' },
  wordTranslatedLyric: { type: String, default: '' },
  wordRomanizedLyric: { type: String, default: '' },
  showTranslation: { type: Boolean, default: true },
  showRomanization: { type: Boolean, default: false },
  currentTimeMs: { type: Number, default: 0 },
  lyricsLoaded: { type: Boolean, default: true },
});

defineEmits(['close', 'toggle-translation', 'toggle-romanization']);

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
    }));
  }
  const plain = mergeTranslatedLyrics(
    props.lyrics,
    props.showTranslation ? props.translatedLyrics : ''
  );
  if (!props.showRomanization || !plain.length) return plain;
  const romanByTime = new Map(parseLyrics(props.romanizedLyrics).map((l) => [l.time, l.text]));
  return plain.map((l) => ({ ...l, romanization: romanByTime.get(l.time) || '' }));
});

const displayLines = computed(() =>
  lines.value.length >= MIN_DISPLAY_LYRIC_LINES ? lines.value : []
);

const showEmptyState = computed(() => props.lyricsLoaded && !displayLines.value.length);

const activeIndex = computed(() => {
  if (!displayLines.value.length) return -1;
  const ms = Math.max(0, props.currentTimeMs);
  if (ms < displayLines.value[0].time) return -1;
  let lo = 0, hi = displayLines.value.length - 1, ans = -1;
  while (lo <= hi) {
    const mid = (lo + hi) >> 1;
    if (ms >= displayLines.value[mid].time) { ans = mid; lo = mid + 1; }
    else hi = mid - 1;
  }
  return ans;
});

// Four visible slots: prev1 | active | next1 | next2
const visibleSlots = computed(() => {
  const n = displayLines.value.length;
  if (!n) return [];
  const ai = activeIndex.value === -1 ? 0 : activeIndex.value;
  return [
    { role: 'prev1', offset: -1 },
    { role: 'active', offset: 0 },
    { role: 'next1', offset: 1 },
    { role: 'next2', offset: 2 },
  ]
    .filter(({ role }) => !(role === 'prev1' && activeIndex.value <= 0))
    .map(({ role, offset }) => ({
      role,
      line: displayLines.value[(ai + offset + n * 4) % n],
    }));
});
</script>

<style scoped>
.ambl-wrap {
  position: fixed;
  inset: 0;
  z-index: var(--z-overlay);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  padding-top: env(safe-area-inset-top);
  padding-bottom: env(safe-area-inset-bottom);
}

.ambl-bg {
  position: absolute;
  inset: 0;
  z-index: 0;
}
.ambl-bg__art {
  width: 100%;
  height: 100%;
  object-fit: cover;
  filter: var(--ambient-filter);
  opacity: var(--ambient-opacity);
  transform: scale(1.08);
}
.ambl-bg__art--fallback { display: block; }
.ambl-bg__scrim {
  position: absolute;
  inset: 0;
  background: var(--ambient-lyrics-overlay);
}
.ambl-bg__vignette {
  position: absolute;
  inset: 0;
  background: radial-gradient(ellipse at 50% 42%, transparent 30%, rgba(0,0,0,0.55) 100%);
  pointer-events: none;
}

.ambl-header {
  position: relative;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px 4px;
  flex-shrink: 0;
}
.ambl-header__label {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--text-secondary);
  opacity: 0.7;
}
.ambl-header__close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  border-radius: 50%;
  color: var(--text-secondary);
  background: transparent;
  border: none;
  cursor: pointer;
  transition: color var(--motion-press) var(--motion-ease-out),
              background var(--motion-press) var(--motion-ease-out);
  -webkit-tap-highlight-color: transparent;
}
.ambl-header__close:hover { color: var(--text-primary); }
.ambl-header__close:active { background: var(--surface-control-active); }
.ambl-header__close:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--focus-ring);
}

.ambl-stage {
  position: relative;
  z-index: 10;
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 0 28px;
  gap: 0;
  overflow: hidden;
  isolation: isolate;
}
.ambl-stage__fade {
  position: absolute;
  left: 0;
  right: 0;
  z-index: 3;
  height: 24%;
  pointer-events: none;
}
.ambl-stage__fade--top {
  top: 0;
  background: linear-gradient(to bottom, var(--ambient-stage-fade) 0%, transparent 100%);
}
.ambl-stage__fade--bottom {
  bottom: 0;
  background: linear-gradient(to top, var(--ambient-stage-fade) 0%, transparent 100%);
}
.ambl-glow {
  position: absolute;
  width: 70%;
  height: 80px;
  background: radial-gradient(ellipse at center, color-mix(in srgb, var(--accent) 14%, transparent), transparent 70%);
  border-radius: 50%;
  pointer-events: none;
  z-index: 0;
}

.ambl-line {
  position: relative;
  z-index: 1;
  width: 100%;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.3em;
  transition:
    opacity 0.42s cubic-bezier(0.16, 1, 0.3, 1),
    transform 0.42s cubic-bezier(0.16, 1, 0.3, 1),
    font-size 0.42s cubic-bezier(0.16, 1, 0.3, 1);
  will-change: opacity, transform;
  overflow-wrap: break-word;
  word-break: break-word;
}
.ambl-line + .ambl-line { margin-top: 16px; }

.ambl-line--prev1 {
  font-size: clamp(0.82rem, 3.6vw, 1.05rem);
  font-weight: 500;
  opacity: 0.22;
  transform: translateY(-6px);
  color: var(--text-primary);
}
.ambl-line--active {
  font-size: clamp(1.35rem, 6.5vw, 2.1rem);
  font-weight: 700;
  opacity: 1;
  color: var(--text-primary);
  line-height: 1.28;
  text-shadow:
    0 2px 28px color-mix(in srgb, var(--accent) 40%, transparent),
    0 0 60px color-mix(in srgb, var(--accent) 12%, transparent);
}
.ambl-line--next1 {
  font-size: clamp(0.88rem, 4vw, 1.12rem);
  font-weight: 500;
  opacity: 0.28;
  transform: translateY(6px);
  color: var(--text-primary);
}
.ambl-line--next2 {
  font-size: clamp(0.76rem, 3.4vw, 0.96rem);
  font-weight: 400;
  opacity: 0.12;
  transform: translateY(10px);
  color: var(--text-primary);
}
.ambl-line__text {
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 3;
  overflow: hidden;
}
.ambl-line--active .ambl-line__text { -webkit-line-clamp: 4; }

.ambl-line__tl,
.ambl-line__ro {
  display: block;
  font-size: clamp(0.78rem, 3.2vw, 0.96rem);
  font-weight: 400;
  color: var(--accent);
  opacity: 0.82;
  text-shadow: none;
  line-height: 1.4;
  overflow-wrap: break-word;
  word-break: break-word;
}
.ambl-line__ro { color: var(--text-secondary); opacity: 0.65; }

.ambl-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}
.ambl-empty__label {
  font-size: 0.88rem;
  font-weight: 500;
  color: var(--text-secondary);
  opacity: 0.55;
}

.ambl-dock {
  position: relative;
  z-index: 10;
  flex-shrink: 0;
  padding: 10px 20px 14px;
  background: linear-gradient(to top, var(--ambient-stage-fade) 0%, color-mix(in srgb, var(--ambient-stage-fade) 60%, transparent) 60%, transparent 100%);
}
.ambl-track {
  display: flex;
  align-items: center;
  gap: 12px;
}
.ambl-track__cover {
  width: 44px;
  height: 44px;
  border-radius: var(--radius-lg);
  overflow: hidden;
  flex-shrink: 0;
  box-shadow: 0 4px 16px rgba(0,0,0,0.6);
}
.ambl-track__cover-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  display: block;
}
.ambl-track__cover-fallback { width: 100%; height: 100%; }
.ambl-track__info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.ambl-track__title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ambl-track__artist {
  font-size: 11px;
  color: var(--text-secondary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ambl-track__pills {
  display: flex;
  gap: 6px;
  flex-shrink: 0;
}
.ambl-pill {
  font-size: 10px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: var(--radius-full);
  border: 1px solid rgba(255, 255, 255, 0.14);
  color: rgba(255, 255, 255, 0.45);
  background: rgba(255, 255, 255, 0.05);
  cursor: pointer;
  letter-spacing: 0.03em;
  transition:
    background var(--motion-press) var(--motion-ease-out),
    color var(--motion-press) var(--motion-ease-out),
    border-color var(--motion-press) var(--motion-ease-out);
  -webkit-tap-highlight-color: transparent;
}
.ambl-pill:hover {
  color: var(--text-primary);
  border-color: rgba(255, 255, 255, 0.28);
}
.ambl-pill:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px var(--focus-ring);
}
.ambl-pill--on {
  background: color-mix(in srgb, var(--accent) 18%, transparent);
  color: var(--accent);
  border-color: color-mix(in srgb, var(--accent) 36%, transparent);
}

.ambl-rise-enter-active {
  transition:
    opacity var(--motion-enter) var(--motion-ease-out),
    transform var(--motion-enter) var(--motion-ease-out);
}
.ambl-rise-leave-active {
  transition:
    opacity var(--motion-exit) ease-in,
    transform var(--motion-exit) ease-in;
}
.ambl-rise-enter-from {
  opacity: 0;
  transform: translateY(32px);
}
.ambl-rise-leave-to {
  opacity: 0;
  transform: translateY(24px);
}

@media (prefers-reduced-motion: reduce) {
  .ambl-line,
  .ambl-rise-enter-active,
  .ambl-rise-leave-active,
  .ambl-pill { transition-duration: 0.01ms; }
}
</style>
