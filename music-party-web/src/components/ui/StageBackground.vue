<template>
  <div class="stage-background" aria-hidden="true">
    <Transition name="stage-fade">
      <div :key="ui.stageGradient" class="stage-background__layer" :style="{ background: ui.stageGradient }" />
    </Transition>
  </div>
</template>

<script setup>
import { useUiStore } from '../../stores/ui';

const ui = useUiStore();
</script>

<style scoped>
.stage-background,
.stage-background__layer {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.stage-background {
  overflow: hidden;
  isolation: isolate;
}

.stage-fade-enter-active,
.stage-fade-leave-active {
  transition: opacity 600ms ease;
}

.stage-fade-enter-active { z-index: 1; }
.stage-fade-leave-active { z-index: 0; }
.stage-fade-enter-from { opacity: 0; }

@media (prefers-reduced-motion: reduce) {
  .stage-fade-enter-active,
  .stage-fade-leave-active { transition: none !important; }
}
</style>
