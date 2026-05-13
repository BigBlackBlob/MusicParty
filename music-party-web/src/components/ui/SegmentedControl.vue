<template>
  <TabsRoot
    :model-value="modelValue"
    @update:model-value="$emit('update:modelValue', $event)"
    class="flex flex-col w-full"
  >
    <TabsList class="flex p-1 bg-[var(--surface-3)] rounded-[var(--radius-lg)]">
      <TabsTrigger
        v-for="option in options"
        :key="option.value"
        :value="option.value"
        class="flex-1 min-h-[32px] px-3 py-1.5 text-xs font-bold transition-all duration-200 rounded-[var(--radius-md)] outline-none
               data-[state=active]:bg-[var(--surface-1)] data-[state=active]:text-[var(--text-primary)] data-[state=active]:shadow-sm
               data-[state=inactive]:text-[var(--text-secondary)] hover:data-[state=inactive]:text-[var(--text-primary)]
               focus-visible:ring-2 focus-visible:ring-[var(--accent-muted)]"
      >
        {{ option.label }}
      </TabsTrigger>
    </TabsList>
  </TabsRoot>
</template>

<script setup>
import { TabsRoot, TabsList, TabsTrigger } from 'reka-ui';

defineProps({
  modelValue: [String, Number],
  options: {
    type: Array,
    required: true
  }
});
defineEmits(['update:modelValue']);
</script>
