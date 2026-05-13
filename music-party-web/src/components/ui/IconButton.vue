<template>
  <button
    class="relative inline-flex items-center justify-center transition-all active:scale-[0.92] disabled:opacity-40 disabled:active:scale-100 outline-none focus-visible:ring-2 focus-visible:ring-[var(--accent-muted)]
           before:absolute before:inset-[-8px] before:content-[''] md:before:hidden"
    :class="[
      sizeClasses[size],
      variantClasses[variant],
      radiusClass
    ]"
    :disabled="disabled"
    :aria-label="ariaLabel || $attrs.title"
    v-bind="$attrs"
  >
    <slot />
  </button>
</template>

<script setup>
import { computed } from 'vue';

const props = defineProps({
  variant: {
    type: String,
    default: 'ghost' // ghost, outline, primary, secondary
  },
  size: {
    type: String,
    default: 'md' // sm, md, lg, xl
  },
  radius: {
    type: String,
    default: 'md' // none, sm, md, lg, full
  },
  disabled: {
    type: Boolean,
    default: false
  },
  ariaLabel: {
    type: String,
    default: ''
  }
});

const sizeClasses = {
  sm: 'w-8 h-8 p-1',
  md: 'w-10 h-10 p-2',
  lg: 'w-12 h-12 p-3',
  xl: 'w-14 h-14 p-4'
};

const variantClasses = {
  ghost: 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--surface-3)]',
  outline: 'border border-[var(--border-default)] text-[var(--text-secondary)] hover:border-[var(--border-accent)] hover:text-[var(--text-primary)]',
  primary: 'bg-[var(--accent)] text-[var(--text-inverse)] hover:bg-[var(--accent-hover)]',
  secondary: 'bg-[var(--surface-3)] text-[var(--text-primary)] hover:bg-[var(--surface-4)]'
};

const radiusClass = computed(() => {
  if (props.radius === 'full') return 'rounded-full';
  if (props.radius === 'none') return 'rounded-none';
  return `rounded-[var(--radius-${props.radius})]`;
});
</script>
