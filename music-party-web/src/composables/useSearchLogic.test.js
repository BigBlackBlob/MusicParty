import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useSearchLogic } from './useSearchLogic.js';
import { searchTracks } from '../domains/search/searchService';

vi.mock('../domains/search/searchService', () => ({
  clearSearchCache: vi.fn().mockResolvedValue(undefined),
  searchTracks: vi.fn(),
  searchAlbums: vi.fn(),
  hasMissingCovers: vi.fn().mockReturnValue(0),
  loadAlbumTracks: vi.fn()
}));

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

  it('clears failure feedback when a later search succeeds with no results', async () => {
    const search = useSearchLogic();
    search.keyword.value = 'review';
    vi.mocked(searchTracks).mockRejectedValueOnce(new Error('Service unavailable'));
    await search.doSearch();
    expect(search.searchError.value).toBe('Service unavailable');
    expect(search.loading.value).toBe(false);

    vi.mocked(searchTracks).mockResolvedValueOnce([]);
    await search.doSearch();
    expect(search.searchError.value).toBe('');
    expect(search.hasSubmittedSearch.value).toBe(true);
    expect(search.songs.value).toEqual([]);
  });

  it('searches the full local library when the keyword is empty', async () => {
    const search = useSearchLogic();
    search.platform.value = 'local';
    const localTrack = { id: 'local-1', title: 'All Night' };
    vi.mocked(searchTracks).mockResolvedValue([localTrack]);

    await search.doSearch();

    expect(searchTracks).toHaveBeenLastCalledWith('local', '%', 0, 20, 'lounge');
    expect(search.songs.value).toEqual([localTrack]);
  });
});
