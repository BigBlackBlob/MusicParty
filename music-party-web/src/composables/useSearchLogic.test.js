import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useSearchLogic } from './useSearchLogic.js';

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key) => key })
}));

vi.mock('./useToast', () => ({
  useToast: () => ({ success: vi.fn(), error: vi.fn() })
}));

vi.mock('./usePlatforms.js', () => ({
  usePlatforms: () => ({
    platforms: [],
    supportsAlbumSearch: { value: true },
    loadPlatforms: vi.fn()
  })
}));

vi.mock('./useExternalPlaylist.js', async () => {
  const actual = await vi.importActual('./useExternalPlaylist.js');
  return {
    ...actual,
    useExternalPlaylist: () => ({
      playlistSongs: { value: [] },
      playlistId: { value: '' },
      currentPlaylistPage: { value: 1 },
      canGoPlaylistNext: { value: true },
      loadNeteasePlaylistPage: vi.fn(),
      clearExternalPlaylist: vi.fn()
    })
  };
});

describe('useSearchLogic cache hydration', () => {
  beforeEach(() => {
    localStorage.clear();
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('ignores corrupt cached search results during initialization', () => {
    localStorage.setItem('mp_search_songs', '{bad json');
    localStorage.setItem('mp_search_albums', '{}');

    const search = useSearchLogic();

    expect(search.songs.value).toEqual([]);
    expect(search.albums.value).toEqual([]);
    expect(localStorage.getItem('mp_search_songs')).toBeNull();
    expect(localStorage.getItem('mp_search_albums')).toBeNull();
  });
});
