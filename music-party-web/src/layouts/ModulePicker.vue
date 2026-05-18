<template>
  <div class="fixed inset-0 z-[100] flex items-center justify-center p-4">
    <!-- Backdrop -->
    <div
      class="absolute inset-0 bg-black/60 backdrop-blur-sm animate-in fade-in duration-300"
      @click="$emit('close')"
    />

    <!-- Modal Content -->
    <div class="relative w-full max-w-md bg-surface-overlay rounded-2xl shadow-2xl overflow-hidden animate-in zoom-in-95 duration-200">
      <div class="flex items-center justify-between px-6 py-4 border-b border-border-subtle">
        <h3 class="text-sm font-bold uppercase tracking-widest text-text-primary">{{ t('layout.addModule') }}</h3>
        <button
          @click="$emit('close')"
          class="p-1 hover:bg-surface-raised rounded-full transition-colors text-text-muted hover:text-text-primary"
        >
          <span class="material-symbols-outlined text-[20px]">close</span>
        </button>
      </div>

      <div class="p-6">
        <div v-if="availableModules.length === 0" class="text-center py-8">
          <p class="text-sm text-text-muted">{{ t('layout.noAvailableModules') }}</p>
        </div>
        
        <div class="grid grid-cols-1 gap-3">
          <button
            v-for="mod in availableModules"
            :key="mod.id"
            @click="addModule(mod.id)"
            class="group flex items-center gap-4 p-4 rounded-xl border border-border-subtle bg-surface-raised hover:border-accent hover:bg-accent/5 transition-all text-left"
          >
            <div class="w-10 h-10 flex items-center justify-center rounded-lg bg-surface-overlay border border-border-subtle text-text-muted group-hover:text-accent group-hover:border-accent/50 transition-colors">
              <span class="material-symbols-outlined text-[24px]">{{ mod.icon }}</span>
            </div>
            <div class="flex-1 min-w-0">
              <div class="text-sm font-bold text-text-primary group-hover:text-accent transition-colors">
                {{ t(`layout.modules.${mod.id}`) || mod.label[locale] }}
              </div>
              <div class="text-[10px] text-text-muted uppercase tracking-wider mt-0.5">
                {{ mod.id }}
              </div>
            </div>
            <span class="material-symbols-outlined text-text-muted group-hover:text-accent opacity-0 group-hover:opacity-100 transition-all transform translate-x-2 group-hover:translate-x-0">
              add_circle
            </span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useLayoutStore } from '../stores/layout';

const props = defineProps({
  columnId: {
    type: String,
    required: true
  }
});

const emit = defineEmits(['close']);

const { t, locale } = useI18n();
const layoutStore = useLayoutStore();

const availableModules = computed(() => {
  return layoutStore.availableModules;
});

const addModule = (moduleId) => {
  layoutStore.addModule(moduleId, props.columnId);
  emit('close');
};
</script>
