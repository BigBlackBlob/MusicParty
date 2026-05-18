import { defineStore } from 'pinia';
import { ref, computed, watch } from 'vue';
import { MODULE_MANIFEST } from '../layouts/moduleManifest';

export const LAYOUT_STORAGE_KEY = 'mp-layout-v1';
export const MAX_MODULES_PER_COLUMN = 1;

const DEFAULT_LAYOUT = {
  version: 3,
  columnCount: 3,
  columns: [
    { id: 'col-0', order: 0, widthPercent: 28, modules: ['nowplaying'] },
    { id: 'col-1', order: 1, widthPercent: 44, modules: ['lyrics'] },
    { id: 'col-2', order: 2, widthPercent: 28, modules: ['queue'] }
  ]
};

const cloneDefaultLayout = () => JSON.parse(JSON.stringify(DEFAULT_LAYOUT));

const normalizeColumnWidths = (columns) => {
  if (!Array.isArray(columns) || columns.length === 0) return columns;
  const width = 100 / columns.length;
  return columns.map((column, index) => ({
    ...column,
    order: index,
    widthPercent: Number.isFinite(column.widthPercent) ? column.widthPercent : width,
    modules: Array.isArray(column.modules) ? column.modules : []
  }));
};

const sanitizeLayout = (candidate) => {
  if (!candidate || typeof candidate !== 'object') return cloneDefaultLayout();
  if (candidate.version !== DEFAULT_LAYOUT.version) return cloneDefaultLayout();
  if (!Array.isArray(candidate.columns) || candidate.columns.length < 2 || candidate.columns.length > 4) {
    return cloneDefaultLayout();
  }

  const columns = normalizeColumnWidths(candidate.columns)
    .map((column, index) => ({
      id: typeof column.id === 'string' && column.id ? column.id : `col-${index}`,
      order: Number.isFinite(column.order) ? column.order : index,
      widthPercent: column.widthPercent,
      modules: column.modules.filter(moduleId => MODULE_MANIFEST[moduleId]).slice(0, MAX_MODULES_PER_COLUMN)
    }))
    .sort((a, b) => a.order - b.order)
    .map((column, index) => ({ ...column, order: index }));

  const placed = new Set();
  columns.forEach((column) => {
    column.modules = column.modules.filter((moduleId) => {
      const manifest = MODULE_MANIFEST[moduleId];
      if (!manifest) return false;
      if (manifest.maxInstances !== -1 && placed.has(moduleId)) return false;
      placed.add(moduleId);
      return true;
    });
  });

  if (!placed.has('nowplaying')) {
    columns[Math.min(1, columns.length - 1)].modules.unshift('nowplaying');
  }

  return {
    version: DEFAULT_LAYOUT.version,
    columnCount: columns.length,
    columns
  };
};

const loadInitialLayout = () => {
  try {
    const savedLayout = localStorage.getItem(LAYOUT_STORAGE_KEY);
    return savedLayout ? sanitizeLayout(JSON.parse(savedLayout)) : cloneDefaultLayout();
  } catch {
    localStorage.removeItem(LAYOUT_STORAGE_KEY);
    return cloneDefaultLayout();
  }
};

export const useLayoutStore = defineStore('layout', () => {
  const isEditMode = ref(false);
  const layout = ref(loadInitialLayout());

  // Computed
  const sortedColumns = computed(() => {
    return [...layout.value.columns].sort((a, b) => a.order - b.order);
  });

  const placedModuleIds = computed(() => {
    const ids = [];
    layout.value.columns.forEach(col => {
      ids.push(...col.modules);
    });
    return ids;
  });

  const availableModules = computed(() => {
    return Object.values(MODULE_MANIFEST).filter(mod => {
      if (mod.maxInstances === -1) return true;
      const count = placedModuleIds.value.filter(id => id === mod.id).length;
      return count < mod.maxInstances;
    });
  });

  // Actions
  function enterEditMode() {
    isEditMode.value = true;
  }

  function exitEditMode() {
    isEditMode.value = false;
  }

  function resetToDefault() {
    layout.value = cloneDefaultLayout();
  }

  function setColumnCount(n) {
    if (n < 2 || n > 4) return;
    const currentCount = layout.value.columnCount;
    if (n === currentCount) return;

    const sortedColumns = [...layout.value.columns].sort((a, b) => a.order - b.order);
    const newColumns = [];
    const avgWidth = 100 / n;

    if (n > currentCount) {
      // Adding columns
      for (let i = 0; i < n; i++) {
        if (i < currentCount) {
          newColumns.push({ ...sortedColumns[i], order: i, widthPercent: avgWidth });
        } else {
          newColumns.push({
            id: `col-${Date.now()}-${i}`,
            order: i,
            widthPercent: avgWidth,
            modules: []
          });
        }
      }
    } else {
      // Removing columns: merge modules to the last remaining column
      const removedModules = [];
      for (let i = 0; i < currentCount; i++) {
        if (i < n) {
          newColumns.push({ ...sortedColumns[i], order: i, widthPercent: avgWidth });
        } else {
          removedModules.push(...sortedColumns[i].modules);
        }
      }
      // Add removed modules to the last column
      newColumns[n - 1].modules.push(...removedModules);
    }

    layout.value.columnCount = n;
    layout.value.columns = newColumns;
  }

  function moveModule(moduleId, fromColId, toColId, toIndex) {
    const fromCol = layout.value.columns.find(c => c.id === fromColId);
    const toCol = layout.value.columns.find(c => c.id === toColId);
    
    if (!fromCol || !toCol) return;
    if (fromCol !== toCol && toCol.modules.length >= MAX_MODULES_PER_COLUMN) return;

    const fromIndex = fromCol.modules.indexOf(moduleId);
    if (fromIndex === -1) return;

    // Remove from source
    fromCol.modules.splice(fromIndex, 1);
    
    // Add to destination
    if (toIndex !== undefined) {
      toCol.modules.splice(toIndex, 0, moduleId);
    } else {
      toCol.modules.push(moduleId);
    }
  }

  function addModule(moduleId, colId, index) {
    const col = layout.value.columns.find(c => c.id === colId);
    if (!col) return;
    if (col.modules.length >= MAX_MODULES_PER_COLUMN) return;

    // Check manifest constraints
    const manifest = MODULE_MANIFEST[moduleId];
    if (!manifest) return;

    if (manifest.maxInstances !== -1) {
      const count = placedModuleIds.value.filter(id => id === moduleId).length;
      if (count >= manifest.maxInstances) return;
    }

    if (index !== undefined) {
      col.modules.splice(index, 0, moduleId);
    } else {
      col.modules.push(moduleId);
    }
  }

  function removeModule(moduleId, colId) {
    const col = layout.value.columns.find(c => c.id === colId);
    if (!col) return;

    const manifest = MODULE_MANIFEST[moduleId];
    if (manifest && !manifest.removable) return;

    const index = col.modules.indexOf(moduleId);
    if (index !== -1) {
      col.modules.splice(index, 1);
    }
  }

  function swapColumns(indexA, indexB) {
    const cols = layout.value.columns;
    if (indexA < 0 || indexA >= cols.length || indexB < 0 || indexB >= cols.length) return;
    
    // Swap order
    const tempOrder = cols[indexA].order;
    cols[indexA].order = cols[indexB].order;
    cols[indexB].order = tempOrder;
  }

  function resizeColumns(leftColId, rightColId, deltaPercent) {
    const leftCol = layout.value.columns.find(c => c.id === leftColId);
    const rightCol = layout.value.columns.find(c => c.id === rightColId);
    
    if (!leftCol || !rightCol) return;

    const newLeftWidth = leftCol.widthPercent + deltaPercent;
    const newRightWidth = rightCol.widthPercent - deltaPercent;

    // Constraint: min 15% width
    if (newLeftWidth < 15 || newRightWidth < 15) return;

    leftCol.widthPercent = newLeftWidth;
    rightCol.widthPercent = newRightWidth;
  }

  // Persistence
  watch(layout, (newVal) => {
    localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify(newVal));
  }, { deep: true });

  return {
    layout,
    isEditMode,
    sortedColumns,
    placedModuleIds,
    availableModules,
    maxModulesPerColumn: MAX_MODULES_PER_COLUMN,
    enterEditMode,
    exitEditMode,
    resetToDefault,
    setColumnCount,
    moveModule,
    addModule,
    removeModule,
    swapColumns,
    resizeColumns
  };
});
