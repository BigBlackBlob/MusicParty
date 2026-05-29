import { describe, expect, it } from 'vitest';
import { getSongExternalUrl } from './likedSongs';

describe('liked song export helpers', () => {
  it('maps YouTube tracks to watch URLs', () => {
    expect(getSongExternalUrl({ platform: 'youtube', id: 'abc123' }))
      .toBe('https://www.youtube.com/watch?v=abc123');
  });
});
