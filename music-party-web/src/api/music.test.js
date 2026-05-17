import { beforeEach, describe, expect, it, vi } from 'vitest';

const get = vi.fn();

vi.mock('axios', () => ({
  default: {
    create: vi.fn(() => ({
      get,
      interceptors: {
        response: {
          use: vi.fn()
        }
      }
    }))
  }
}));

const { musicApi } = await import('./music');

describe('musicApi album endpoints', () => {
  beforeEach(() => {
    get.mockReset();
  });

  it('searches albums by platform and passes tokens for Subsonic platforms only', () => {
    musicApi.searchAlbums('navidrome', 'blue', 'session-token', 'room-1');

    expect(get).toHaveBeenCalledWith('/api/album/search/navidrome', {
        params: {
            keyword: 'blue',
            roomId: 'room-1',
            token: 'session-token'
        }
    });

    musicApi.searchAlbums('subsonic-squidify', 'blue', 'session-token', 'room-1');

    expect(get).toHaveBeenLastCalledWith('/api/album/search/subsonic-squidify', {
        params: {
            keyword: 'blue',
            roomId: 'room-1',
            token: 'session-token'
        }
    });

    musicApi.searchAlbums('netease', 'blue', 'session-token', 'room-1');

    expect(get).toHaveBeenLastCalledWith('/api/album/search/netease', {
        params: {
            keyword: 'blue',
            roomId: 'room-1'
        }
    });
  });

  it('loads album songs by platform and passes tokens for Subsonic platforms only', () => {
    musicApi.getAlbumSongs('navidrome', 'album-1', 'session-token', 'room-1');

    expect(get).toHaveBeenCalledWith('/api/album/songs/navidrome/album-1', {
        params: {
            roomId: 'room-1',
            token: 'session-token'
        }
    });

    musicApi.getAlbumSongs('subsonic-squidify', 'album-1', 'session-token', 'room-1');

    expect(get).toHaveBeenLastCalledWith('/api/album/songs/subsonic-squidify/album-1', {
        params: {
            roomId: 'room-1',
            token: 'session-token'
        }
    });

    musicApi.getAlbumSongs('netease', 'album-1', 'session-token', 'room-1');

    expect(get).toHaveBeenLastCalledWith('/api/album/songs/netease/album-1', {
      params: {
        roomId: 'room-1'
      }
    });
  });
});
