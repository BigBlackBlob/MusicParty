<template>
  <div class="flex items-center justify-between gap-4 px-4 py-3 border-b border-[var(--border-default)]">
    <div class="flex items-center gap-3 min-w-0">
      <slot name="left" />
      <h3 v-if="title" class="text-sm font-semibold truncate text-[var(--text-primary)]">
        {{ title }}
      </h3>
    </div>
    <div class="flex items-center gap-2 flex-shrink-0">
      <slot name="right" />
    </div>
  </div>
</template>

<script setup>
defineProps({
  title: String
});
</script>
