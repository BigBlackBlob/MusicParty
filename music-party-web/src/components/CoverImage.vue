<template>
  <div class="relative flex items-center justify-center overflow-hidden bg-[var(--surface-3)]">
    <img
        v-if="!hasError && normalizedSrc"
        :src="normalizedSrc"
        class="h-full w-full object-cover transition-opacity duration-300"
        @error="hasError = true"
    />

    <div v-else class="flex h-full w-full items-center justify-center bg-[linear-gradient(180deg,rgba(255,255,255,0.03),rgba(255,255,255,0))] text-[var(--text-tertiary)]">
      <ImageOff class="w-1/2 h-1/2" :stroke-width="1.5" />
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue';
import { ImageOff } from 'lucide-vue-next';

const props = defineProps(['src']);
const hasError = ref(false);

const normalizedSrc = computed(() => {
  if (typeof props.src !== 'string') return '';

  const trimmed = props.src.trim();
  if (!trimmed) return '';
  if (trimmed.startsWith('//')) return `https:${trimmed}`;
  if (trimmed.startsWith('http://')) return trimmed.replace('http://', 'https://');

  return /^https?:\/\//.test(trimmed) ? trimmed : '';
});

watch(() => props.src, () => hasError.value = false);
</script>
