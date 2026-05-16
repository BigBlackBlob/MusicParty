import { describe, expect, it } from 'vitest';
import {
  addAlbumSelections,
  clearAlbumSelections,
  hasAlbumSelections,
  makeAlbumSongSelectionKey,
  selectedAlbumSongs
} from './selection';

describe('album song selection helpers', () => {
  const songs = [
    { id: '1', name: 'A' },
    { id: '2', name: 'B' }
  ];

  it('scopes song selections by platform and album', () => {
    const selected = new Set([
      makeAlbumSongSelectionKey('netease', 'album-a', '1'),
      makeAlbumSongSelectionKey('netease', 'album-b', '1'),
      makeAlbumSongSelectionKey('other', 'album-a', '2')
    ]);

    expect(selectedAlbumSongs(selected, 'netease', 'album-a', songs)).toEqual([{ id: '1', name: 'A' }]);
  });

  it('clears only the current album selections', () => {
    const selected = new Set([
      makeAlbumSongSelectionKey('netease', 'album-a', '1'),
      makeAlbumSongSelectionKey('netease', 'album-b', '1')
    ]);

    clearAlbumSelections(selected, 'netease', 'album-a', songs);

    expect(selected.has(makeAlbumSongSelectionKey('netease', 'album-a', '1'))).toBe(false);
    expect(selected.has(makeAlbumSongSelectionKey('netease', 'album-b', '1'))).toBe(true);
  });

  it('detects and adds current album selections', () => {
    const selected = new Set();

    addAlbumSelections(selected, 'netease', 'album-a', songs);

    expect(hasAlbumSelections(selected, 'netease', 'album-a', songs)).toBe(true);
    expect(hasAlbumSelections(selected, 'netease', 'album-b', songs)).toBe(false);
  });
});
