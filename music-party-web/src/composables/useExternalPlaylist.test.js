import { describe, expect, it } from 'vitest';
import { useExternalPlaylist, parseNeteasePlaylistId } from './useExternalPlaylist';

describe('parseNeteasePlaylistId', () => {
  it('accepts a raw numeric playlist id', () => {
    expect(parseNeteasePlaylistId('123456')).toBe('123456');
  });

  it('extracts playlist ids from NetEase URLs', () => {
    expect(parseNeteasePlaylistId('https://music.163.com/#/playlist?id=987654&userid=1')).toBe('987654');
    expect(parseNeteasePlaylistId('https://music.163.com/playlist/555666')).toBe('555666');
  });

  it('rejects invalid playlist input', () => {
    expect(parseNeteasePlaylistId('abc')).toBeNull();
    expect(parseNeteasePlaylistId('')).toBeNull();
  });
});

describe('useExternalPlaylist cache hydration', () => {
  it('ignores corrupt cached playlist songs', () => {
    localStorage.setItem('mp_search_playlist_songs', '{bad json');

    const playlist = useExternalPlaylist();

    expect(playlist.playlistSongs.value).toEqual([]);
    expect(localStorage.getItem('mp_search_playlist_songs')).toBeNull();
  });
});
