<template>
  <button
    v-if="hasDisplayableLyrics"
    type="button"
    class="mini-lyrics"
    aria-label="打开歌词"
    @click="$emit('open')"
  >
    <Transition name="mini-lyric-line" mode="out-in">
      <span :key="currentLineKey" class="mini-lyrics__line-group">
        <span class="mini-lyrics__line">
          {{ currentLine?.text || '' }}
        </span>
        <span v-if="currentLine?.translation" class="mini-lyrics__translation">
          {{ currentLine.translation }}
        </span>
        <span v-else class="mini-lyrics__translation-spacer" aria-hidden="true"></span>
      </span>
    </Transition>
  </button>
</template>

<script setup>
import { computed } from 'vue';
import { mergeTranslatedLyrics } from '../../utils/parser';

const props = defineProps({
  lyrics: {
    type: String,
    default: ''
  },
  translatedLyrics: {
    type: String,
    default: ''
  },
  currentTimeMs: {
    type: Number,
    default: 0
  },
  minLines: {
    type: Number,
    default: 5
  }
});

defineEmits(['open']);

const lines = computed(() => {
  const parsed = mergeTranslatedLyrics(props.lyrics, props.translatedLyrics);
  return parsed.length >= props.minLines ? parsed : [];
});

const currentLineIndex = computed(() => {
  if (!lines.value.length) return -1;
  let low = 0;
  let high = lines.value.length - 1;
  let answer = -1;

  while (low <= high) {
    const mid = Math.floor((low + high) / 2);
    if (props.currentTimeMs >= lines.value[mid].time) {
      answer = mid;
      low = mid + 1;
    } else {
      high = mid - 1;
    }
  }

  return answer;
});

const currentLine = computed(() => {
  if (currentLineIndex.value < 0) return null;
  return lines.value[currentLineIndex.value];
});

const hasDisplayableLyrics = computed(() => lines.value.length > 0);

const currentLineKey = computed(() => {
  if (!currentLine.value) return 'empty';
  return `${currentLine.value.time}-${currentLineIndex.value}`;
});
</script>

<style scoped>
.mini-lyrics {
  display: flex;
  min-height: 3.9rem;
  width: 100%;
  flex-direction: column;
  align-items: center;
  justify-content: flex-start;
  overflow: hidden;
  border: 0;
  background: transparent;
  padding: 0;
  color: var(--text-primary);
  text-align: left;
  touch-action: manipulation;
}

.mini-lyrics__line {
  display: block;
  width: 100%;
  min-height: 1.7rem;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: clamp(1rem, 4.5vw, 1.35rem);
  font-weight: 750;
  line-height: 1.25;
  letter-spacing: 0;
  text-shadow: 0 2px 14px rgba(0, 0, 0, 0.22);
}

.mini-lyrics__line-group {
  display: block;
  width: 100%;
  min-height: 3.05rem;
}

.mini-lyrics__translation,
.mini-lyrics__translation-spacer {
  display: block;
  width: 100%;
  min-height: 1.25rem;
  overflow: hidden;
  font-size: 0.875rem;
  line-height: 1.25rem;
}

.mini-lyrics__translation {
  color: var(--text-secondary);
  font-weight: 560;
  opacity: 0.82;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.mini-lyrics__translation-spacer {
  opacity: 0;
  pointer-events: none;
  user-select: none;
}

.mini-lyric-line-enter-active,
.mini-lyric-line-leave-active {
  transition: opacity 360ms ease-in-out, transform 360ms ease-in-out;
}

.mini-lyric-line-enter-from {
  opacity: 0;
  transform: translate3d(0, 0.45rem, 0);
}

.mini-lyric-line-leave-to {
  opacity: 0;
  transform: translate3d(0, -0.45rem, 0);
}

@media (prefers-reduced-motion: reduce) {
  .mini-lyric-line-enter-active,
  .mini-lyric-line-leave-active {
    transition-duration: 0.01ms;
  }
}
</style>
