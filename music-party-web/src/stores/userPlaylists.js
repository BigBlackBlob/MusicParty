import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { personalPlaylistsApi } from '../api/personalPlaylists';
import { useUserStore } from './user';

export const useUserPlaylistsStore = defineStore('userPlaylists', () => {
  const playlists = ref([]);
  const tracksByPlaylist = ref({});
  const selectedPlaylistId = ref('');
  const loading = ref(false);
  const error = ref('');

  const userStore = useUserStore();
  const selectedPlaylist = computed(() => playlists.value.find(item => item.id === selectedPlaylistId.value) || null);
  const selectedTracks = computed(() => tracksByPlaylist.value[selectedPlaylistId.value] || []);

  const requireNamedUser = () => {
    if (userStore.isGuest) {
      userStore.showNameModal = true;
      return false;
    }
    return true;
  };

  const loadPlaylists = async () => {
    if (!requireNamedUser()) return [];
    loading.value = true;
    error.value = '';
    try {
      playlists.value = await personalPlaylistsApi.list(userStore.sessionToken);
      if (!selectedPlaylistId.value && playlists.value.length) selectedPlaylistId.value = playlists.value[0].id;
      return playlists.value;
    } catch (e) {
      error.value = e?.response?.data?.message || e?.message || 'Failed to load personal playlists';
      throw e;
    } finally {
      loading.value = false;
    }
  };

  const ensurePlaylist = async (name = '我的歌单') => {
    if (!requireNamedUser()) return null;
    if (!playlists.value.length) await loadPlaylists();
    if (selectedPlaylistId.value) return selectedPlaylistId.value;
    const playlist = await createPlaylist(name);
    return playlist?.id || '';
  };

  const createPlaylist = async (name) => {
    if (!requireNamedUser()) return null;
    const playlist = await personalPlaylistsApi.create(userStore.sessionToken, name);
    selectedPlaylistId.value = playlist.id;
    await loadPlaylists();
    return playlist;
  };

  const renamePlaylist = async (playlistId, name) => {
    await personalPlaylistsApi.rename(userStore.sessionToken, playlistId, name);
    await loadPlaylists();
  };

  const deletePlaylist = async (playlistId) => {
    await personalPlaylistsApi.remove(userStore.sessionToken, playlistId);
    if (selectedPlaylistId.value === playlistId) selectedPlaylistId.value = '';
    await loadPlaylists();
  };

  const loadTracks = async (playlistId = selectedPlaylistId.value) => {
    if (!playlistId || !requireNamedUser()) return [];
    const tracks = await personalPlaylistsApi.tracks(userStore.sessionToken, playlistId, 0, 500);
    tracksByPlaylist.value = { ...tracksByPlaylist.value, [playlistId]: tracks };
    return tracks;
  };

  const addTracks = async (playlistId, musics) => {
    if (!playlistId || !requireNamedUser()) return null;
    const result = await personalPlaylistsApi.addTracks(userStore.sessionToken, playlistId, musics);
    await loadPlaylists();
    await loadTracks(playlistId);
    return result;
  };

  const addTracksToSelected = async (musics, fallbackName = '我的歌单') => {
    const playlistId = await ensurePlaylist(fallbackName);
    if (!playlistId) return null;
    return addTracks(playlistId, musics);
  };

  const importNetease = async (playlistId, externalPlaylistId) => {
    const result = await personalPlaylistsApi.importNetease(userStore.sessionToken, playlistId, externalPlaylistId);
    await loadPlaylists();
    await loadTracks(playlistId);
    return result;
  };

  const enqueue = async (playlistId = selectedPlaylistId.value) => {
    if (!playlistId || !requireNamedUser()) return false;
    await personalPlaylistsApi.enqueue(userStore.sessionToken, playlistId);
    return true;
  };

  return {
    playlists,
    tracksByPlaylist,
    selectedPlaylistId,
    selectedPlaylist,
    selectedTracks,
    loading,
    error,
    requireNamedUser,
    loadPlaylists,
    loadTracks,
    ensurePlaylist,
    createPlaylist,
    renamePlaylist,
    deletePlaylist,
    addTracks,
    addTracksToSelected,
    importNetease,
    enqueue
  };
});
