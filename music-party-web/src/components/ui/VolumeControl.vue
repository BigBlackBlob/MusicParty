<template>
  <div class="flex items-center gap-2 group/volume">
    <IconButton size="md" @click="$emit('toggle-mute')">
      <VolumeX v-if="modelValue === 0" class="h-5 w-5" />
      <Volume1 v-else-if="modelValue < 0.5" class="h-5 w-5" />
      <Volume2 v-else class="h-5 w-5" />
    </IconButton>

    <SliderRoot
      :model-value="[modelValue]"
      :max="1"
      :step="0.01"
      aria-label="音量调节"
      class="relative flex items-center select-none touch-none w-24 h-6 group"
      @update:model-value="$emit('update:modelValue', $event[0])"
    >
      <SliderTrack class="bg-[var(--progress-track)] relative grow rounded-full h-1 overflow-hidden">
        <SliderRange class="absolute bg-[var(--accent)] h-full" />
      </SliderTrack>

      <SliderThumb
        class="block w-3 h-3 bg-white border border-[var(--border-accent)] shadow-md rounded-full transition-transform duration-150 scale-0 group-hover:scale-100 focus:scale-100 focus:outline-none focus:ring-2 focus:ring-[var(--accent-muted)]"
      />
    </SliderRoot>

    <span class="w-8 text-[10px] font-mono font-bold text-[var(--text-tertiary)] text-right tabular-nums">
      {{ Math.round(modelValue * 100) }}%
    </span>
  </div>
</template>

<script setup>
import { Volume2, Volume1, VolumeX } from 'lucide-vue-next';
import { SliderRoot, SliderTrack, SliderRange, SliderThumb } from 'reka-ui';
import IconButton from './IconButton.vue';

defineProps({
  modelValue: Number
});

defineEmits(['update:modelValue', 'toggle-mute']);
</script>

<style scoped>
/* Removed custom range styles in favor of reka-ui primitives */
</style>

