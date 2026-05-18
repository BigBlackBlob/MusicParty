import { ref } from 'vue';
import { musicApi } from '../api/music';

const PLAYLIST_CACHE_KEY = 'mp_search_playlist_songs';
const PLAYLIST_ID_CACHE_KEY = 'mp_search_playlist_id';
const PLAYLIST_PAGE_CACHE_KEY = 'mp_search_playlist_page';
const PLAYLIST_LIMIT = 50;

export const parseNeteasePlaylistId = (val) => {
  const value = String(val || '').trim();
  if (!value) return null;
  const patterns = [
    /playlist\?id=(\d+)/,
    /playlist\/(\d+)/,
    /music\.163\.com\/.*id=(\d+)/,
    /^(\d+)$/
  ];
  for (const pattern of patterns) {
    const match = value.match(pattern);
    if (match) return match[1];
  }
  return null;
};

export function useExternalPlaylist() {
  const playlistSongs = ref(JSON.parse(localStorage.getItem(PLAYLIST_CACHE_KEY) || '[]'));
  const playlistId = ref(localStorage.getItem(PLAYLIST_ID_CACHE_KEY) || '');
  const currentPlaylistPage = ref(parseInt(localStorage.getItem(PLAYLIST_PAGE_CACHE_KEY) || '1', 10));
  const canGoPlaylistNext = ref(true);

  const loadNeteasePlaylistPage = async (input, page = 1) => {
    const id = parseNeteasePlaylistId(input) || playlistId.value;
    if (!id) return null;
    const offset = (page - 1) * PLAYLIST_LIMIT;
    const data = await musicApi.getPlaylistSongs('netease', id, offset, PLAYLIST_LIMIT);
    playlistSongs.value = data;
    playlistId.value = id;
    currentPlaylistPage.value = page;
    canGoPlaylistNext.value = data.length === PLAYLIST_LIMIT;
    localStorage.setItem(PLAYLIST_CACHE_KEY, JSON.stringify(data));
    localStorage.setItem(PLAYLIST_ID_CACHE_KEY, id);
    localStorage.setItem(PLAYLIST_PAGE_CACHE_KEY, String(page));
    return data;
  };

  const clearExternalPlaylist = () => {
    playlistSongs.value = [];
    playlistId.value = '';
    currentPlaylistPage.value = 1;
    canGoPlaylistNext.value = true;
    localStorage.removeItem(PLAYLIST_CACHE_KEY);
    localStorage.removeItem(PLAYLIST_ID_CACHE_KEY);
    localStorage.removeItem(PLAYLIST_PAGE_CACHE_KEY);
  };

  return {
    playlistSongs,
    playlistId,
    currentPlaylistPage,
    canGoPlaylistNext,
    loadNeteasePlaylistPage,
    clearExternalPlaylist
  };
}
