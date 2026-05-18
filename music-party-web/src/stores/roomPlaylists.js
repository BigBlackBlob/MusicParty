import { defineStore } from 'pinia';
import { computed, ref } from 'vue';
import { playlistsApi } from '../api/playlists';
import { socketService } from '../services/socket';
import { WS_DEST } from '../constants/api';
import { useRoomStore } from './room';
import { useUserStore } from './user';

export const useRoomPlaylistsStore = defineStore('roomPlaylists', () => {
  const playlists = ref([]);
  const tracksByPlaylist = ref({});
  const selectedPlaylistId = ref('');
  const loading = ref(false);
  const error = ref('');

  const roomId = computed(() => useRoomStore().currentRoomId || 'lounge');
  const selectedPlaylist = computed(() => playlists.value.find(item => item.id === selectedPlaylistId.value) || null);
  const selectedTracks = computed(() => tracksByPlaylist.value[selectedPlaylistId.value] || []);

  const loadPlaylists = async () => {
    loading.value = true;
    error.value = '';
    try {
      playlists.value = await playlistsApi.list(roomId.value);
      if (!selectedPlaylistId.value && playlists.value.length) selectedPlaylistId.value = playlists.value[0].id;
      if (selectedPlaylistId.value) await loadTracks(selectedPlaylistId.value);
    } catch (e) {
      error.value = e?.response?.data?.message || e?.message || 'Failed to load playlists';
    } finally {
      loading.value = false;
    }
  };

  const loadTracks = async (playlistId = selectedPlaylistId.value) => {
    if (!playlistId) return;
    tracksByPlaylist.value = {
      ...tracksByPlaylist.value,
      [playlistId]: await playlistsApi.tracks(roomId.value, playlistId, 0, 200)
    };
  };

  const createPlaylist = async (name) => {
    const playlist = await playlistsApi.create(roomId.value, name);
    selectedPlaylistId.value = playlist.id;
    await loadPlaylists();
  };

  const renamePlaylist = async (playlistId, name) => {
    await playlistsApi.rename(roomId.value, playlistId, name);
    await loadPlaylists();
  };

  const deletePlaylist = async (playlistId) => {
    await playlistsApi.remove(roomId.value, playlistId);
    if (selectedPlaylistId.value === playlistId) selectedPlaylistId.value = '';
    await loadPlaylists();
  };

  const deleteTrack = async (playlistId, trackId) => {
    await playlistsApi.removeTrack(roomId.value, playlistId, trackId);
    await loadTracks(playlistId);
  };

  const reorderTracks = async (playlistId, trackIds) => {
    await playlistsApi.reorder(roomId.value, playlistId, trackIds);
    await loadTracks(playlistId);
  };

  const importExternal = async (platform, externalPlaylistId) => {
    if (!selectedPlaylistId.value) return;
    await playlistsApi.importExternal(roomId.value, selectedPlaylistId.value, platform, externalPlaylistId, useUserStore().sessionToken);
    await loadPlaylists();
    await loadTracks(selectedPlaylistId.value);
  };

  const playPlaylist = (playlistId = selectedPlaylistId.value) => {
    if (!playlistId) return false;
    return socketService.send(WS_DEST.ENQUEUE_ROOM_PLAYLIST, { playlistId });
  };

  const exportPlaylist = async (format = 'txt', playlistId = selectedPlaylistId.value) => {
    if (!playlistId) return '';
    return playlistsApi.exportPlaylist(roomId.value, playlistId, format);
  };

  return {
    playlists,
    tracksByPlaylist,
    selectedPlaylistId,
    selectedPlaylist,
    selectedTracks,
    loading,
    error,
    loadPlaylists,
    loadTracks,
    createPlaylist,
    renamePlaylist,
    deletePlaylist,
    deleteTrack,
    reorderTracks,
    importExternal,
    playPlaylist,
    exportPlaylist
  };
});
