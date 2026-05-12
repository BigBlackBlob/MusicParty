<template>
  <div class="flex h-screen w-screen items-center justify-center overflow-hidden bg-[var(--surface-0)] p-4">
    <div class="absolute left-4 top-4 z-[var(--z-header)] flex max-w-[calc(100vw-2rem)] items-center gap-2 overflow-x-auto rounded-2xl border border-[var(--border-default)] bg-[var(--surface-4)]/95 p-2 shadow-xl backdrop-blur">
      <button
        v-for="size in sizes"
        :key="size"
        type="button"
        class="min-h-[36px] rounded-xl px-3 text-xs font-semibold transition-colors"
        :class="ui.mobilePreviewWidth === size ? 'bg-[var(--accent)] text-[var(--text-inverse)]' : 'text-[var(--text-secondary)] hover:bg-[var(--surface-3)]'"
        @click="ui.setMobilePreviewWidth(size)"
      >
        {{ size }}
      </button>
      <button
        type="button"
        class="min-h-[36px] rounded-xl px-3 text-xs font-semibold text-[var(--text-secondary)] transition-colors hover:bg-[var(--surface-3)]"
        @click="ui.setForceMobileLayout(false)"
      >
        退出
      </button>
    </div>

    <div
      class="h-[min(calc(100dvh-2rem),932px)] overflow-hidden rounded-[2rem] border border-[var(--border-default)] bg-[var(--surface-0)] shadow-2xl"
      :style="{ width: `${ui.mobilePreviewWidth}px` }"
    >
      <slot />
    </div>
  </div>
</template>

<script setup>
import { useUiStore } from '../../stores/ui';

const ui = useUiStore();
const sizes = [375, 390, 430, 768];
</script>
