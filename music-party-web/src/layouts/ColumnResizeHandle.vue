<template>
  <div
    class="resize-handle group relative flex h-full cursor-col-resize items-center justify-center transition-all"
    :class="[
      layoutStore.isEditMode ? 'w-[6px] hover:bg-accent/30' : 'w-0 pointer-events-none',
      isResizing ? 'bg-accent/50' : ''
    ]"
    @pointerdown="onPointerDown"
  >
    <div
      v-if="layoutStore.isEditMode"
      class="h-8 w-1 rounded-full bg-border-subtle group-hover:bg-accent transition-colors"
      :class="{ 'bg-accent h-12': isResizing }"
    />
  </div>
</template>

<script setup>
import { ref, onBeforeUnmount } from 'vue';
import { useLayoutStore } from '../stores/layout';

const props = defineProps({
  leftColId: {
    type: String,
    required: true
  },
  rightColId: {
    type: String,
    required: true
  }
});

const layoutStore = useLayoutStore();
const isResizing = ref(false);
let startX = 0;
let containerWidth = 0;

const onPointerDown = (e) => {
  if (!layoutStore.isEditMode) return;
  
  isResizing.value = true;
  startX = e.clientX;
  containerWidth = e.currentTarget.parentElement.clientWidth;
  
  e.currentTarget.setPointerCapture(e.pointerId);
  window.addEventListener('pointermove', onPointerMove);
  window.addEventListener('pointerup', onPointerUp);
};

const onPointerMove = (e) => {
  if (!isResizing.value) return;
  
  const deltaX = e.clientX - startX;
  const deltaPercent = (deltaX / containerWidth) * 100;
  
  if (Math.abs(deltaPercent) > 0.1) {
    layoutStore.resizeColumns(props.leftColId, props.rightColId, deltaPercent);
    startX = e.clientX; // Update startX for the next move
  }
};

const onPointerUp = () => {
  isResizing.value = false;
  window.removeEventListener('pointermove', onPointerMove);
  window.removeEventListener('pointerup', onPointerUp);
};

onBeforeUnmount(() => {
  window.removeEventListener('pointermove', onPointerMove);
  window.removeEventListener('pointerup', onPointerUp);
});
</script>

<style scoped>
.resize-handle {
  z-index: 10;
}
</style>
