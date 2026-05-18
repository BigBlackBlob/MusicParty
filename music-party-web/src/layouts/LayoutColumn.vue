<template>
  <div class="layout-column relative flex h-full min-h-0 min-w-0 flex-col overflow-hidden">
    <div
      ref="moduleListRef"
      class="module-list flex h-full min-h-0 flex-1 flex-col gap-4 overflow-y-auto"
      :class="{ 'edit-mode': layoutStore.isEditMode }"
    >
      <div
        v-for="moduleId in column.modules"
        :key="moduleId"
        class="layout-module-wrapper flex flex-1 flex-col min-h-0 transition-all overflow-hidden"
        :class="{ 'edit-mode-module rounded-xl border border-transparent bg-surface-raised/30': layoutStore.isEditMode }"
        :data-id="moduleId"
      >
        <!-- Module Header (Edit Mode Only) -->
        <div
          v-if="layoutStore.isEditMode"
          class="module-header flex items-center justify-between px-3 py-2 bg-surface-raised border-b border-border-subtle cursor-default"
        >
          <div class="flex items-center gap-2 overflow-hidden">
            <span class="module-drag-handle material-symbols-outlined text-[18px] cursor-grab active:cursor-grabbing text-text-muted">drag_indicator</span>
            <span class="text-[10px] font-bold uppercase tracking-wider text-text-muted truncate">
              {{ t(`layout.modules.${moduleId}`) || MODULE_MANIFEST[moduleId]?.label[locale] }}
            </span>
          </div>
          <button
            v-if="MODULE_MANIFEST[moduleId]?.removable"
            @click="layoutStore.removeModule(moduleId, column.id)"
            class="p-1 hover:bg-error/10 hover:text-error rounded transition-colors"
          >
            <span class="material-symbols-outlined text-[16px]">close</span>
          </button>
        </div>

        <!-- Module Content -->
        <div class="module-content flex-1 min-h-0" :class="{ 'pointer-events-none opacity-60': layoutStore.isEditMode }">
          <component :is="MODULE_MANIFEST[moduleId]?.component" />
        </div>
      </div>

      <!-- Add Module Button (Edit Mode Only) -->
      <div
        v-if="layoutStore.isEditMode && column.modules.length < layoutStore.maxModulesPerColumn"
        class="mt-auto py-4 flex justify-center"
      >
        <button
          @click="$emit('add-module', column.id)"
          class="flex items-center gap-2 px-4 py-2 rounded-full border border-dashed border-border-accent text-accent hover:bg-accent/5 transition-colors"
        >
          <span class="material-symbols-outlined text-[20px]">add</span>
          <span class="text-xs font-bold uppercase tracking-widest">{{ t('layout.addModule') }}</span>
        </button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, onBeforeUnmount, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useLayoutStore } from '../stores/layout';
import { MODULE_MANIFEST } from './moduleManifest';
import Sortable from 'sortablejs';

const props = defineProps({
  column: {
    type: Object,
    required: true
  }
});

defineEmits(['add-module']);

const { t, locale } = useI18n();
const layoutStore = useLayoutStore();
const moduleListRef = ref(null);
let sortableInstance = null;

const initSortable = () => {
  if (!moduleListRef.value || sortableInstance) return;

  sortableInstance = new Sortable(moduleListRef.value, {
    group: 'layout-modules',
    handle: '.module-drag-handle',
    animation: 180,
    ghostClass: 'sortable-ghost',
    dragClass: 'sortable-drag',
    onEnd: (evt) => {
      // Restore DOM state to let Vue handle re-render
      const { from, to, item, oldIndex, newIndex } = evt;
      const moduleId = item.getAttribute('data-id');
      const fromColId = from.getAttribute('data-col-id');
      const toColId = to.getAttribute('data-col-id');

      if (from !== to) {
        from.insertBefore(item, from.children[oldIndex] || null);
      } else {
        const ref = from.children[oldIndex < newIndex ? oldIndex : oldIndex + 1] || null;
        from.insertBefore(item, ref);
      }

      layoutStore.moveModule(moduleId, fromColId, toColId, newIndex);
    }
  });

  moduleListRef.value.setAttribute('data-col-id', props.column.id);
};

const destroySortable = () => {
  if (sortableInstance) {
    sortableInstance.destroy();
    sortableInstance = null;
  }
};

watch(() => layoutStore.isEditMode, (isEdit) => {
  if (isEdit) {
    initSortable();
  } else {
    destroySortable();
  }
}, { immediate: true });

onBeforeUnmount(() => {
  destroySortable();
});
</script>

<style scoped>
.module-list {
  scrollbar-width: none;
}
.module-list::-webkit-scrollbar {
  display: none;
}

.edit-mode.module-list {
  padding: 1rem;
  border: 1px dashed var(--border-subtle);
  background: rgba(var(--accent-rgb, 211, 194, 243), 0.02);
}

.edit-mode-module {
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.1);
  border-color: var(--border-subtle);
}

.sortable-ghost {
  opacity: 0.2;
  background: var(--accent-muted) !important;
}

.sortable-drag {
  cursor: grabbing !important;
  box-shadow: 0 10px 25px rgba(0, 0, 0, 0.2) !important;
}
</style>
