import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useQuery } from '@tanstack/vue-query'
import { playlistsApi } from '../api/playlists'
import { queryClient } from '../app/providers'
import {
  createPlaylist as createScopedPlaylist,
  importPlaylist as importScopedPlaylist,
  listPlaylists,
  listPlaylistTracks,
  removePlaylist as removeScopedPlaylist,
  removePlaylistTrack,
  renamePlaylist as renameScopedPlaylist,
  reorderPlaylistTracks,
  roomPlaylistListQueryOptions,
  roomPlaylistTracksQueryOptions,
  type PlaylistScope
} from '../domains/playlists/playlistService'
import { useRoomStore } from './room'
import { useRoomCommandStore } from '../domains/realtime/roomCommandStore'

export const useRoomPlaylistsStore = defineStore('roomPlaylists', () => {
  const roomStore = useRoomStore()
  const commands = useRoomCommandStore()
  const selectedPlaylistId = ref('')
  const error = ref('')

  const roomId = computed(() => roomStore.currentRoomId || 'lounge')
  const scope = computed<Extract<PlaylistScope, { type: 'room' }>>(() => ({ type: 'room', roomId: roomId.value }))
  const listQuery = useQuery(computed(() => roomPlaylistListQueryOptions(scope.value)), queryClient)
  const tracksQuery = useQuery(computed(() => ({
    ...roomPlaylistTracksQueryOptions(scope.value, selectedPlaylistId.value, 200),
    enabled: Boolean(selectedPlaylistId.value),
  })), queryClient)
  const playlists = computed(() => listQuery.data.value ?? [])
  const selectedPlaylist = computed(() => playlists.value.find(item => item.id === selectedPlaylistId.value) ?? null)
  const selectedTracks = computed(() => selectedPlaylistId.value ? tracksQuery.data.value ?? [] : [])
  const loading = computed(() => listQuery.isFetching.value || tracksQuery.isFetching.value)

  watch(roomId, () => { selectedPlaylistId.value = '' })

  async function loadPlaylists() {
    error.value = ''
    try {
      const values = await listPlaylists(scope.value)
      if (!values.some(item => item.id === selectedPlaylistId.value)) selectedPlaylistId.value = values[0]?.id ?? ''
      if (selectedPlaylistId.value) await loadTracks(selectedPlaylistId.value)
      return values
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'Failed to load playlists'
      return []
    }
  }

  async function loadTracks(playlistId = selectedPlaylistId.value) {
    return playlistId ? listPlaylistTracks(scope.value, playlistId, 200) : []
  }

  async function createPlaylist(name: string): Promise<void> {
    const playlist = await createScopedPlaylist(scope.value, name)
    selectedPlaylistId.value = playlist.id
  }

  async function renamePlaylist(playlistId: string, name: string): Promise<void> {
    await renameScopedPlaylist(scope.value, playlistId, name)
  }

  async function deletePlaylist(playlistId: string): Promise<void> {
    await removeScopedPlaylist(scope.value, playlistId)
    if (selectedPlaylistId.value === playlistId) selectedPlaylistId.value = ''
  }

  async function deleteTrack(playlistId: string, trackId: string): Promise<void> {
    await removePlaylistTrack(scope.value, playlistId, trackId)
  }

  async function reorderTracks(playlistId: string, trackIds: string[]): Promise<void> {
    await reorderPlaylistTracks(scope.value, playlistId, trackIds)
  }

  async function importExternal(platform: string, externalPlaylistId: string): Promise<void> {
    if (!selectedPlaylistId.value) return
    await importScopedPlaylist(scope.value, selectedPlaylistId.value, platform, externalPlaylistId)
  }

  function playPlaylist(playlistId = selectedPlaylistId.value): boolean {
    return Boolean(playlistId && commands.enqueueRoomPlaylist(playlistId))
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
