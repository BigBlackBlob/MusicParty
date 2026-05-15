import { describe, expect, it } from 'vitest';
import { withNavidromeResourceToken, withPlaybackToken } from './audioUrl';

describe('audioUrl helpers', () => {
  it('adds session token to Navidrome playback URLs only', () => {
    expect(withPlaybackToken({ platform: 'navidrome', url: '/api/navidrome/stream/song' }, 'secret'))
      .toBe('/api/navidrome/stream/song?token=secret');
    expect(withPlaybackToken({ platform: 'netease', url: 'https://example.test/song.mp3' }, 'secret'))
      .toBe('https://example.test/song.mp3');
  });

  it('adds session token to Navidrome resource URLs only', () => {
    expect(withNavidromeResourceToken('/api/navidrome/cover/id', 'secret token'))
      .toBe('/api/navidrome/cover/id?token=secret%20token');
    expect(withNavidromeResourceToken('https://example.test/cover.jpg', 'secret'))
      .toBe('https://example.test/cover.jpg');
  });
});
