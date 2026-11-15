<template>
  <div class="layout-renderer-container flex flex-col h-full w-full overflow-hidden">
    <!-- Edit Mode Toolbar -->
    <EditModeToolbar v-if="layoutStore.isEditMode" />

    <!-- Main Grid -->
    <div
      ref="gridRef"
      class="layout-grid flex-1 grid h-full w-full overflow-hidden"
      :style="gridStyle"
    >
      <template v-for="(col, index) in effectiveColumns" :key="col.id">
        <!-- Column -->
        <LayoutColumn
          :column="col"
          @add-module="openModulePicker"
        />

        <!-- Resize Handle (except after last column) -->
        <ColumnResizeHandle
          v-if="layoutStore.isEditMode && index < effectiveColumns.length - 1"
          :left-col-id="col.id"
          :right-col-id="effectiveColumns[index + 1].id"
        />
      </template>
    </div>


    <!-- Module Picker Modal -->
    <ModulePicker
      v-if="showModulePicker"
      :column-id="activeColumnId"
      @close="showModulePicker = false"
    />
  </div>
</template>

<script setup>
import { ref, computed } from 'vue';
import { useElementSize, useWindowSize } from '@vueuse/core';
import { useLayoutStore } from '../stores/layout';
import { useUiStore } from '../stores/ui';
import { MODULE_MANIFEST } from './moduleManifest';
import LayoutColumn from './LayoutColumn.vue';
import ColumnResizeHandle from './ColumnResizeHandle.vue';
import EditModeToolbar from './EditModeToolbar.vue';
import ModulePicker from './ModulePicker.vue';

const layoutStore = useLayoutStore();
const uiStore = useUiStore();
const { width: windowWidth } = useWindowSize();
const gridRef = ref(null);
const { width: gridWidth } = useElementSize(gridRef);
const showModulePicker = ref(false);
const activeColumnId = ref(null);
const DESKTOP_HORIZONTAL_PADDING = 40;
const BASE_STAGE_GAP = 18;
const MIN_STAGE_GAP = 10;
const MAX_STAGE_GAP = 26;
const STAGE_GAP_SCALE_FACTOR = 60;
const ZOOM_GAP_COMPRESSION = 12;

const stageGap = computed(() => (
  Math.round(Math.max(
    MIN_STAGE_GAP,
    Math.min(
      MAX_STAGE_GAP,
      BASE_STAGE_GAP
        + ((uiStore.mainStageScale - 1) * STAGE_GAP_SCALE_FACTOR)
        - ((uiStore.globalZoomLevel - 1) * ZOOM_GAP_COMPRESSION)
    )
  ))
));

const availableStageWidth = computed(() => Math.max(
  0,
  windowWidth.value / uiStore.globalZoomLevel - (DESKTOP_HORIZONTAL_PADDING * 2)
));

const columnMinWidth = (column) => Math.max(
  0,
  ...column.modules.map(moduleId => MODULE_MANIFEST[moduleId]?.minWidthPx || 0)
);

const effectiveColumns = computed(() => {
  const sorted = layoutStore.sortedColumns;
  if (windowWidth.value > 1180 || layoutStore.isEditMode) return sorted;

  const handleWidth = layoutStore.isEditMode ? Math.max(0, sorted.length - 1) * 6 : 0;
  const gapWidth = Math.max(0, sorted.length - 1) * stageGap.value;
  const totalMinWidth = sorted.reduce((sum, column) => sum + columnMinWidth(column), handleWidth);

  if (sorted.length > 2 && totalMinWidth + gapWidth > availableStageWidth.value) {
    const col1 = { ...sorted[0], modules: [...sorted[0].modules] };
    const col2 = { ...sorted[1], modules: [...sorted[1].modules] };
    for (let i = 2; i < sorted.length; i++) {
      col2.modules.push(...sorted[i].modules);
    }
    col1.widthPercent = 50;
    col2.widthPercent = 50;
    return [col1, col2];
  }
  
  return sorted;
});

const gridStyle = computed(() => {
  const cols = effectiveColumns.value;
  if (windowWidth.value <= 1200 && !layoutStore.isEditMode) {
    const gapWidth = Math.max(0, cols.length - 1) * stageGap.value;
    // Budget against the grid itself so padding and zoom cannot add hidden overflow.
    const availableWidth = Math.max(0, gridWidth.value - gapWidth);
    const scale = Math.max(0, windowWidth.value - 1024);
    const minimumWidths = cols.map(column => {
      if (column.modules.includes('nowplaying')) return Math.min(340, 320 + scale * 0.05);
      if (column.modules.includes('queue')) return Math.min(300, 288 + scale * 0.03);
      if (column.modules.includes('lyrics')) return 300;
      return 0;
    });
    const minimumTotal = minimumWidths.reduce((sum, width) => sum + width, 0);
    const remaining = Math.max(0, availableWidth - minimumTotal);
    const flexibleColumns = cols.map(column => !column.modules.includes('nowplaying') && !column.modules.includes('queue'));
    const hasFlexibleColumn = flexibleColumns.some(Boolean);
    const weights = cols.map((column, index) => !hasFlexibleColumn || flexibleColumns[index] ? column.widthPercent : 0);
    const totalWeight = weights.reduce((sum, weight) => sum + weight, 0);
    const compression = minimumTotal > 0 ? Math.min(1, availableWidth / minimumTotal) : 1;
    const widths = minimumWidths.map((minimum, index) => minimum * compression + (totalWeight > 0 ? remaining * weights[index] / totalWeight : 0));

    return {
      // The last track absorbs subpixel rounding without extending past the grid.
      gridTemplateColumns: widths.map((width, index) => index === cols.length - 1 ? 'minmax(0, 1fr)' : `${width}px`).join(' '),
      columnGap: `${stageGap.value}px`,
      transition: 'grid-template-columns 0.2s ease'
    };
  }

  const templateParts = [];
  
  cols.forEach((col, index) => {
    // Column width
    // Use 1fr for the last column to absorb rounding errors
    if (index === cols.length - 1) {
      templateParts.push('1fr');
    } else {
      templateParts.push(`${col.widthPercent}%`);
    }
    
    if (layoutStore.isEditMode && index < cols.length - 1) {
      templateParts.push(layoutStore.isEditMode ? '6px' : '0px');
    }
  });
  
  return {
    gridTemplateColumns: templateParts.join(' '),
    columnGap: layoutStore.isEditMode ? '0px' : `${stageGap.value}px`,
    transition: 'grid-template-columns 0.2s ease'
  };
});


const openModulePicker = (columnId) => {
  activeColumnId.value = columnId;
  showModulePicker.value = true;
};
</script>

<style scoped>
.layout-grid {
  gap: 0;
  padding: 0;
}
</style>
