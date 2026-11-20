<template>
  <div
    class="resize-handle group relative flex h-full cursor-col-resize items-center justify-center"
    :class="[
      layoutStore.isEditMode ? 'is-edit' : 'is-hidden',
      { 'is-resizing': isResizing }
    ]"
    @pointerdown="onPointerDown"
  >
    <div
      v-if="layoutStore.isEditMode"
      class="resize-pill"
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
  transition: background-color var(--motion-enter) var(--motion-ease-out), width var(--motion-enter) var(--motion-ease-out);
}
.resize-handle.is-hidden {
  width: 0;
  pointer-events: none;
}
.resize-handle.is-edit {
  width: 8px;
}
.resize-handle.is-edit:hover {
  background: var(--accent-subtle);
}
.resize-handle.is-resizing {
  background: var(--accent-muted);
}
.resize-pill {
  width: 3px;
  height: 32px;
  border-radius: var(--radius-full);
  background: var(--border-strong);
  transition: height var(--motion-enter) var(--motion-ease-out), background-color var(--motion-enter) var(--motion-ease-out), box-shadow var(--motion-enter) var(--motion-ease-out);
}
.resize-handle.is-edit:hover .resize-pill {
  height: 44px;
  background: var(--accent);
}
.resize-handle.is-resizing .resize-pill {
  height: 60px;
  background: var(--accent);
  box-shadow: 0 0 12px var(--accent-muted);
}
</style>
