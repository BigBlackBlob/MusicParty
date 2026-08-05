import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { playlistsApi } from '../api/playlists'
import { queryClient } from '../app/providers'
import {
  cachedPlaylists,
  cachedPlaylistTracks,
  createPlaylist as createScopedPlaylist,
  importPlaylist as importScopedPlaylist,
  listPlaylists,
  listPlaylistTracks,
  removePlaylist as removeScopedPlaylist,
  removePlaylistTrack,
  renamePlaylist as renameScopedPlaylist,
  reorderPlaylistTracks,
  type PlaylistScope
} from '../domains/playlists/playlistService'
import { realtimeClient } from '../transport/realtimeClient'
import { useRoomStore } from './room'

export const useRoomPlaylistsStore = defineStore('roomPlaylists', () => {
  const roomStore = useRoomStore()
  const selectedPlaylistId = ref('')
  const loading = ref(false)
  const error = ref('')
  const cacheRevision = ref(0)
  queryClient.getQueryCache().subscribe(() => { cacheRevision.value += 1 })

  const roomId = computed(() => roomStore.currentRoomId || 'lounge')
  const scope = computed<PlaylistScope>(() => ({ type: 'room', roomId: roomId.value }))
  const playlists = computed(() => { cacheRevision.value; return cachedPlaylists(scope.value) })
  const selectedPlaylist = computed(() => playlists.value.find(item => item.id === selectedPlaylistId.value) ?? null)
  const selectedTracks = computed(() => {
    cacheRevision.value
    return selectedPlaylistId.value ? cachedPlaylistTracks(scope.value, selectedPlaylistId.value) : []
  })

  watch(roomId, () => { selectedPlaylistId.value = '' })

  async function loadPlaylists() {
    loading.value = true
    error.value = ''
    try {
      const values = await listPlaylists(scope.value)
      if (!values.some(item => item.id === selectedPlaylistId.value)) selectedPlaylistId.value = values[0]?.id ?? ''
      if (selectedPlaylistId.value) await loadTracks(selectedPlaylistId.value)
      return values
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'Failed to load playlists'
      return []
    } finally {
      loading.value = false
    }
  }

  async function loadTracks(playlistId = selectedPlaylistId.value) {
    return playlistId ? listPlaylistTracks(scope.value, playlistId, 200) : []
  }

  async function createPlaylist(name: string): Promise<void> {
    const playlist = await createScopedPlaylist(scope.value, name)
    selectedPlaylistId.value = playlist.id
    await loadPlaylists()
  }

  async function renamePlaylist(playlistId: string, name: string): Promise<void> {
    await renameScopedPlaylist(scope.value, playlistId, name)
    await loadPlaylists()
  }

  async function deletePlaylist(playlistId: string): Promise<void> {
    await removeScopedPlaylist(scope.value, playlistId)
    if (selectedPlaylistId.value === playlistId) selectedPlaylistId.value = ''
    await loadPlaylists()
  }

  async function deleteTrack(playlistId: string, trackId: string): Promise<void> {
    await removePlaylistTrack(scope.value, playlistId, trackId)
    await Promise.all([loadPlaylists(), loadTracks(playlistId)])
  }

  async function reorderTracks(playlistId: string, trackIds: string[]): Promise<void> {
    await reorderPlaylistTracks(scope.value, playlistId, trackIds)
    await loadTracks(playlistId)
  }

  async function importExternal(platform: string, externalPlaylistId: string): Promise<void> {
    if (!selectedPlaylistId.value) return
    await importScopedPlaylist(scope.value, selectedPlaylistId.value, platform, externalPlaylistId)
    await Promise.all([loadPlaylists(), loadTracks(selectedPlaylistId.value)])
  }

  function playPlaylist(playlistId = selectedPlaylistId.value): boolean {
    const mutationId = `room-playlist-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
    return Boolean(playlistId && realtimeClient.send('enqueue.room-playlist', { playlistId, mutationId }))
  }

  async function exportPlaylist(format = 'txt', playlistId = selectedPlaylistId.value): Promise<string> {
    return playlistId ? playlistsApi.exportPlaylist(roomId.value, playlistId, format) : ''
  }

  return {
    playlists, selectedPlaylistId, selectedPlaylist, selectedTracks, loading, error,
    loadPlaylists, loadTracks, createPlaylist, renamePlaylist, deletePlaylist, deleteTrack,
    reorderTracks, importExternal, playPlaylist, exportPlaylist
  }
})
