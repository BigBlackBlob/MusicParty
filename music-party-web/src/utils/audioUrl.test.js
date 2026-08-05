import { describe, expect, it } from 'vitest';
import { withNavidromeResourceToken, withPlaybackToken } from './audioUrl';

describe('audioUrl helpers', () => {
  it('keeps playback URLs free of browser-readable tokens', () => {
    expect(withPlaybackToken({ platform: 'navidrome', url: '/api/navidrome/stream/song' }, 'secret'))
      .toBe('/api/navidrome/stream/song');
    expect(withPlaybackToken({ platform: 'subsonic-squidify', url: '/api/subsonic/squidify/stream/song' }, 'secret'))
      .toBe('/api/subsonic/squidify/stream/song');
    expect(withPlaybackToken({ platform: 'netease', url: '/api/netease/stream/123' }, 'secret'))
      .toBe('/api/netease/stream/123');
    expect(withPlaybackToken({ platform: 'bilibili', url: '/api/bilibili/stream/BV1xx411c7mD' }, 'secret'))
      .toBe('/api/bilibili/stream/BV1xx411c7mD');
    expect(withPlaybackToken({ platform: 'youtube', url: '/media/youtube-abc123.m4a' }, 'secret'))
      .toBe('/media/youtube-abc123.m4a');
  });

  it('keeps resource URLs free of browser-readable tokens', () => {
    expect(withNavidromeResourceToken('/api/navidrome/cover/id', 'secret token'))
      .toBe('/api/navidrome/cover/id');
    expect(withNavidromeResourceToken('/api/subsonic/squidify/cover/id', 'secret token'))
      .toBe('/api/subsonic/squidify/cover/id');
    expect(withNavidromeResourceToken('https://example.test/cover.jpg', 'secret'))
      .toBe('https://example.test/cover.jpg');
  });
});
