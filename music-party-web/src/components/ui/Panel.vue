<template>
  <div
    class="flex flex-col overflow-hidden bg-[var(--surface-1)] border-[var(--border-default)]"
    :class="[
      border ? (borderSide === 'right' ? 'border-r' : borderSide === 'left' ? 'border-l' : borderSide === 'top' ? 'border-t' : borderSide === 'bottom' ? 'border-b' : 'border') : '',
      radius ? 'rounded-[var(--radius-md)]' : ''
    ]"
  >
    <slot name="header" />
    <div class="flex-1 overflow-y-auto">
      <slot />
    </div>
    <slot name="footer" />
  </div>
</template>

<script setup>
defineProps({
  border: {
    type: Boolean,
    default: true
  },
  borderSide: {
    type: String,
    default: 'all' // all, left, right, top, bottom
  },
  radius: {
    type: Boolean,
    default: false
  }
});
</script>
