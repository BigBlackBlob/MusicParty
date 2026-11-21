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
    expect(store.placedModuleIds).toEqual(['nowplaying', 'lyrics', 'queue-playlists']);
    expect(localStorage.getItem(LAYOUT_STORAGE_KEY)).toBeNull();
  });

  it('sanitizes duplicate and unknown modules from saved layouts', () => {
    localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify({
      version: 3,
      columnCount: 3,
      columns: [
        { id: 'left', order: 0, widthPercent: 30, modules: ['missing', 'lyrics'] },
        { id: 'middle', order: 1, widthPercent: 30, modules: ['lyrics'] },
        { id: 'right', order: 2, widthPercent: 40, modules: ['nowplaying'] }
      ]
    }));

    const store = useLayoutStore();

    expect(store.layout.columns.map(column => column.modules)).toEqual([['lyrics'], [], ['nowplaying']]);
  });

  it('keeps only one module in each saved layout column', () => {
    localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify({
      version: 3,
      columnCount: 2,
      columns: [
        { id: 'left', order: 0, widthPercent: 50, modules: ['lyrics', 'chat'] },
        { id: 'right', order: 1, widthPercent: 50, modules: ['nowplaying'] }
      ]
    }));

    const store = useLayoutStore();

    expect(store.layout.columns[0].modules).toEqual(['lyrics']);
    expect(store.layout.columns[1].modules).toEqual(['nowplaying']);
  });

  it.each([
    ['queue and playlists', ['queue'], ['playlists'], [], 'queue'],
    ['playlists alone', [], ['playlists'], [], 'playlists'],
    ['already merged', ['queue'], ['playlists'], ['queue-playlists'], 'merged']
  ])('migrates saved %s without changing column widths or order', (_, queue, playlists, merged, keptColumn) => {
    const columns = [
      { id: 'queue', order: 2, widthPercent: 30, modules: queue },
      { id: 'playlists', order: 0, widthPercent: 25, modules: playlists },
      { id: 'merged', order: 3, widthPercent: 20, modules: merged },
      { id: 'player', order: 1, widthPercent: 25, modules: ['nowplaying'] }
    ];
    localStorage.setItem(LAYOUT_STORAGE_KEY, JSON.stringify({ version: 3, columnCount: 4, columns }));

    const store = useLayoutStore();

    expect(store.layout.columnCount).toBe(4);
    expect(store.sortedColumns).toEqual(
      [...columns].sort((a, b) => a.order - b.order).map(column => ({
        ...column,
        modules: column.id === keptColumn ? ['queue-playlists'] : column.id === 'player' ? ['nowplaying'] : []
      }))
    );
    expect(JSON.parse(localStorage.getItem(LAYOUT_STORAGE_KEY))).toEqual(store.layout);
  });

  it('only offers the combined module when its slot is empty', () => {
    const store = useLayoutStore();
    store.removeModule('queue-playlists', 'col-2');

    const availableIds = store.availableModules.map(module => module.id);
    expect(availableIds).toContain('queue-playlists');
    expect(availableIds).not.toContain('queue');
    expect(availableIds).not.toContain('playlists');
  });

  it('does not add or move a module into an occupied column', () => {
    const store = useLayoutStore();

    store.removeModule('queue-playlists', 'col-2');
    store.addModule('queue-playlists', 'col-0');

    expect(store.layout.columns[0].modules).toEqual(['nowplaying']);
    expect(store.layout.columns[2].modules).toEqual([]);

    store.addModule('queue-playlists', 'col-2');
    store.moveModule('lyrics', 'col-1', 'col-2', 0);

    expect(store.layout.columns[1].modules).toEqual(['lyrics']);
    expect(store.layout.columns[2].modules).toEqual(['queue-playlists']);
  });
});
