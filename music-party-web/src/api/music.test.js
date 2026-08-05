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

  it('searches albums with a room identifier but never a browser-readable token', () => {
    musicApi.searchAlbums('navidrome', 'blue', 'room-1');

    expect(get).toHaveBeenCalledWith('/api/album/search/navidrome', {
        params: {
            keyword: 'blue',
            roomId: 'room-1'
        }
    });

    musicApi.searchAlbums('subsonic-squidify', 'blue', 'room-1');

    expect(get).toHaveBeenLastCalledWith('/api/album/search/subsonic-squidify', {
        params: {
            keyword: 'blue',
            roomId: 'room-1'
        }
    });

    musicApi.searchAlbums('netease', 'blue', 'room-1');

    expect(get).toHaveBeenLastCalledWith('/api/album/search/netease', {
        params: {
            keyword: 'blue',
            roomId: 'room-1'
        }
    });
  });

  it('loads album songs with a room identifier but never a browser-readable token', () => {
    musicApi.getAlbumSongs('navidrome', 'album-1', 'room-1');

    expect(get).toHaveBeenCalledWith('/api/album/songs/navidrome/album-1', {
        params: {
            roomId: 'room-1'
        }
    });

    musicApi.getAlbumSongs('subsonic-squidify', 'album-1', 'room-1');

    expect(get).toHaveBeenLastCalledWith('/api/album/songs/subsonic-squidify/album-1', {
        params: {
            roomId: 'room-1'
        }
    });

    musicApi.getAlbumSongs('netease', 'album-1', 'room-1');

    expect(get).toHaveBeenLastCalledWith('/api/album/songs/netease/album-1', {
      params: {
        roomId: 'room-1'
      }
    });
  });
});
