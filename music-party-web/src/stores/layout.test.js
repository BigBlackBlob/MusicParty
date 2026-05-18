import { beforeEach, describe, expect, it } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { LAYOUT_STORAGE_KEY, useLayoutStore } from './layout';

describe('layout store', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
  });

  it('falls back to the default layout when saved layout is invalid JSON', () => {
    localStorage.setItem(LAYOUT_STORAGE_KEY, '{broken');

    const store = useLayoutStore();

    expect(store.layout.columnCount).toBe(3);
    expect(store.placedModuleIds).toEqual(['nowplaying', 'lyrics', 'queue']);
    expect(localStorage.getItem(LAYOUT_STORAGE_KEY)).toBeNull();
  });

  it('sanitizes duplicate and unknown modules from saved layouts', () => {
    localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify({
      version: 3,
      columnCount: 2,
      columns: [
        { id: 'left', order: 0, widthPercent: 50, modules: ['queue', 'missing'] },
        { id: 'right', order: 1, widthPercent: 50, modules: ['queue', 'nowplaying'] }
      ]
    }));

    const store = useLayoutStore();

    expect(store.layout.columnCount).toBe(2);
    expect(store.layout.columns[0].modules).toEqual(['queue']);
    expect(store.layout.columns[1].modules).toEqual(['nowplaying']);
  });

  it('keeps only one module in each saved layout column', () => {
    localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify({
      version: 3,
      columnCount: 2,
      columns: [
        { id: 'left', order: 0, widthPercent: 50, modules: ['lyrics', 'queue'] },
        { id: 'right', order: 1, widthPercent: 50, modules: ['nowplaying'] }
      ]
    }));

    const store = useLayoutStore();

    expect(store.layout.columns[0].modules).toEqual(['lyrics']);
    expect(store.layout.columns[1].modules).toEqual(['nowplaying']);
  });

  it('does not add or move a module into an occupied column', () => {
    const store = useLayoutStore();

    store.removeModule('queue', 'col-2');
    store.addModule('queue', 'col-0');

    expect(store.layout.columns[0].modules).toEqual(['nowplaying']);
    expect(store.layout.columns[2].modules).toEqual([]);

    store.addModule('queue', 'col-2');
    store.moveModule('lyrics', 'col-1', 'col-2', 0);

    expect(store.layout.columns[1].modules).toEqual(['lyrics']);
    expect(store.layout.columns[2].modules).toEqual(['queue']);
  });
});
