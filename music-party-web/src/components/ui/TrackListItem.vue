<template>
  <div
    class="group flex min-h-[60px] items-center gap-3 rounded-md border p-2.5 transition-colors"
    :class="active ? 'border-border-subtle bg-[var(--surface-control-active)]' : 'border-transparent hover:bg-[var(--surface-control-hover)]'"
  >
    <!-- Prefix Slot (optional) -->
    <div v-if="$slots.prefix" class="flex-shrink-0">
      <slot name="prefix" />
    </div>

    <!-- Cover -->
    <CoverImage v-if="coverUrl" :src="coverUrl" :alt="title" class="h-10 w-10 flex-shrink-0 rounded object-cover" />
    <div v-else class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded bg-[var(--surface-control)]">
      <span class="material-symbols-outlined text-text-muted text-[20px]">music_note</span>
    </div>

    <!-- Main Content -->
    <div class="flex min-w-0 flex-1 flex-col justify-center overflow-hidden">
      <p class="text-text-primary font-compact truncate" :class="active ? 'text-primary' : ''">{{ title }}</p>
      <p class="text-text-muted text-caption truncate">{{ artist }}</p>
    </div>

    <!-- Meta / Suffix Area -->
    <div class="flex flex-shrink-0 items-center gap-2">
      <!-- Only show meta (like duration) when not hovering (unless active, maybe?) -->
      <span class="text-text-muted font-micro text-micro transition-opacity" :class="{ 'group-hover:hidden': $slots.suffix && !active }">
        <slot name="meta" />
      </span>

      <!-- Active Equalizer icon -->
      <span v-if="active" class="material-symbols-outlined text-primary text-[20px]">equalizer</span>

      <!-- Suffix (Actions) -->
      <div 
        v-if="$slots.suffix && !active" 
        class="hidden group-hover:flex items-center text-text-primary gap-2"
      >
        <slot name="suffix" />
      </div>
      <!-- If it's active we can also show suffix, maybe always flex if active? We'll see. -->
      <div 
        v-if="$slots.suffix && active" 
        class="flex items-center text-text-primary gap-2"
      >
        <slot name="suffix" />
      </div>
    </div>
  </div>
</template>

<script setup>
import CoverImage from '../CoverImage.vue';

defineProps({
  title: String,
  artist: String,
  coverUrl: String,
  active: Boolean,
  unplayable: Boolean
});
</script>
