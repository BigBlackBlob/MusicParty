<template>
  <div class="flex flex-col w-full group/progress">
    <div v-if="!hideLabels" class="flex items-center justify-between mb-1 px-0.5">
      <div class="flex items-center gap-1.5">
        <span class="text-[10px] font-mono font-bold text-[var(--text-tertiary)] tabular-nums">
          {{ formatDuration(displayProgressMs) }}
        </span>
        <Lock v-if="!canSeek && duration > 0" class="w-2.5 h-2.5 text-[var(--text-tertiary)] opacity-40" />
      </div>
      <span class="text-[10px] font-mono font-bold text-[var(--text-tertiary)] tabular-nums">
        {{ formatDuration(duration) }}
      </span>
    </div>
    
    <SliderRoot
      :model-value="[displayProgressMs]"
      :max="duration"
      :step="1000"
      :disabled="!canSeek"
      aria-label="播放进度"
      class="relative flex items-center select-none touch-none w-full h-4 group"
      :class="canSeek ? 'cursor-pointer' : 'cursor-not-allowed'"
      @update:model-value="handleUpdate"
      @value-commit="handleCommit"
    >
      <SliderTrack 
        class="relative grow rounded-full h-1 overflow-hidden transition-colors"
        :class="canSeek ? 'bg-[var(--progress-track)]' : 'bg-[var(--progress-track)]/40'"
      >
        <!-- 缓冲进度 (已加载部分) -->
        <div
          v-if="loadedPercent > 0"
          class="absolute top-0 left-0 h-full bg-[var(--progress-loaded)] pointer-events-none"
          :style="{ width: loadedPercent + '%' }"
        />
        <SliderRange
          class="absolute h-full transition-colors duration-200"
          :class="isError ? 'bg-[var(--error)]' : 'bg-[var(--accent)]'"
          :style="{ opacity: canSeek ? 1 : 0.4 }"
        />
        
        <!-- Markers -->
        <div
          v-for="(marker, index) in markers"
          :key="index"
          class="absolute top-0 h-full w-1 bg-white/30 rounded-full"
          :style="{ left: (marker / (duration || 1)) * 100 + '%' }"
        />
      </SliderTrack>
      
      <SliderThumb
        v-if="canSeek"
        class="block w-3 h-3 bg-white border border-[var(--border-accent)] shadow-md rounded-full transition-transform duration-150 scale-0 group-hover:scale-100 focus:scale-100 focus:outline-none focus:ring-2 focus:ring-[var(--accent-muted)]"
      />
    </SliderRoot>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { SliderRoot, SliderTrack, SliderRange, SliderThumb } from 'reka-ui';
import { Lock } from 'lucide-vue-next';
import { formatDuration } from '../../utils/format';

const props = defineProps({
  currentMs: Number,
  duration: Number,
  canSeek: Boolean,
  isError: Boolean,
  hideLabels: Boolean,
  loadedMs: {
    type: Number,
    default: 0
  },
  markers: {
    type: Array,
    default: () => []
  }
});

const emit = defineEmits(['seek', 'preview-start', 'preview-end']);

const isDragging = ref(false);
const localPreviewMs = ref(0);

const displayProgressMs = computed(() => {
  return isDragging.value ? localPreviewMs.value : props.currentMs;
});

// 已缓冲进度占整条进度的百分比，用于在进度条上展示缓冲进度
const loadedPercent = computed(() => {
  if (!props.duration || props.duration <= 0) return 0;
  const ratio = (props.loadedMs || 0) / props.duration;
  return Math.max(0, Math.min(100, ratio * 100));
});

const handleUpdate = (val) => {
  if (!isDragging.value) {
    isDragging.value = true;
    emit('preview-start');
  }
  localPreviewMs.value = val[0];
};

const handleCommit = (val) => {
  isDragging.value = false;
  emit('preview-end');
  emit('seek', val[0]);
};

// Sync local preview when not dragging
watch(() => props.currentMs, (val) => {
  if (!isDragging.value) {
    localPreviewMs.value = val;
  }
});
</script>
