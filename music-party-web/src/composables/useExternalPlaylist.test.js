import { describe, expect, it } from 'vitest';
import { parseNeteasePlaylistId } from './useExternalPlaylist';

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
