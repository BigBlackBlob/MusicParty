<template>
  <div class="edit-toolbar flex items-center justify-between px-6 py-3 bg-surface-overlay border-b border-border-accent shadow-lg animate-in slide-in-from-top duration-300">
    <div class="flex items-center gap-6">
      <div class="flex items-center gap-2">
        <span class="material-symbols-outlined text-accent">view_column</span>
        <span class="text-xs font-bold uppercase tracking-widest text-text-secondary">{{ t('layout.columns') }}</span>
      </div>
      
      <div class="flex bg-surface-raised p-1 rounded-lg">
        <button
          v-for="n in [2, 3, 4]"
          :key="n"
          @click="layoutStore.setColumnCount(n)"
          class="px-4 py-1.5 rounded-md text-sm font-bold transition-all"
          :class="layoutStore.layout.columnCount === n ? 'bg-accent text-on-accent shadow-md' : 'text-text-muted hover:text-text-primary'"
        >
          {{ n }}
        </button>
      </div>
    </div>

    <div class="flex items-center gap-3">
      <button
        @click="layoutStore.resetToDefault"
        class="flex items-center gap-2 px-4 py-2 rounded-lg text-text-secondary hover:bg-surface-raised transition-colors"
      >
        <span class="material-symbols-outlined text-[20px]">restart_alt</span>
        <span class="text-xs font-bold uppercase tracking-widest">{{ t('layout.reset') }}</span>
      </button>
      
      <div class="w-px h-6 bg-border-subtle mx-2" />
      
      <button
        @click="layoutStore.exitEditMode"
        class="flex items-center gap-2 px-6 py-2 rounded-lg bg-accent text-on-accent hover:bg-accent-hover shadow-lg shadow-accent/20 transition-all active:scale-95"
      >
        <span class="material-symbols-outlined text-[20px]">check</span>
        <span class="text-xs font-bold uppercase tracking-widest">{{ t('common.done') }}</span>
      </button>
    </div>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n';
import { useLayoutStore } from '../stores/layout';

const { t } = useI18n();
const layoutStore = useLayoutStore();
</script>

<style scoped>
.edit-toolbar {
  z-index: 50;
}
</style>
