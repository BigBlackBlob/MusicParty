import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type { Music } from '../contracts/generated/models'
import { personalPlaylistsApi } from '../api/personalPlaylists'
import { queryClient } from '../app/providers'
import {
  addPersonalTracks,
  cachedPlaylists,
  cachedPlaylistTracks,
  createPlaylist as createScopedPlaylist,
  importPlaylist as importScopedPlaylist,
  listPlaylists,
  listPlaylistTracks,
  removePlaylist as removeScopedPlaylist,
  removePlaylistTrack,
  renamePlaylist as renameScopedPlaylist,
  type PlaylistScope
} from '../domains/playlists/playlistService'
import { useUserStore } from './user'

const scope: PlaylistScope = { type: 'personal' }

export const useUserPlaylistsStore = defineStore('userPlaylists', () => {
  const selectedPlaylistId = ref('')
  const loading = ref(false)
  const error = ref('')
  const cacheRevision = ref(0)
  queryClient.getQueryCache().subscribe(() => { cacheRevision.value += 1 })
  const userStore = useUserStore()

  const playlists = computed(() => { cacheRevision.value; return cachedPlaylists(scope) })
  const selectedPlaylist = computed(() => playlists.value.find(item => item.id === selectedPlaylistId.value) ?? null)
  const selectedTracks = computed(() => {
    cacheRevision.value
    return selectedPlaylistId.value ? cachedPlaylistTracks(scope, selectedPlaylistId.value) : []
  })
  const likedPlaylist = computed(() => playlists.value.find(item => item.systemKey === 'liked-songs') ?? null)

  function requireNamedUser(): boolean {
    if (!userStore.isGuest) return true
    userStore.showNameModal = true
    return false
  }

  async function loadPlaylists() {
    if (!requireNamedUser()) return []
    loading.value = true
    error.value = ''
    try {
      const values = await listPlaylists(scope)
      if (!selectedPlaylistId.value && values.length) selectedPlaylistId.value = values[0]?.id ?? ''
      return values
    } catch (cause) {
      error.value = cause instanceof Error ? cause.message : 'Failed to load personal playlists'
      throw cause
    } finally {
      loading.value = false
    }
  }

  async function loadTracks(playlistId = selectedPlaylistId.value) {
    if (!playlistId || !requireNamedUser()) return []
    return listPlaylistTracks(scope, playlistId)
  }

  async function createPlaylist(name: string) {
    if (!requireNamedUser()) return null
    const playlist = await createScopedPlaylist(scope, name)
    selectedPlaylistId.value = playlist.id
    await loadPlaylists()
    return playlist
  }

  async function ensurePlaylist(name = '我的歌单'): Promise<string | null> {
    if (!requireNamedUser()) return null
    if (!playlists.value.length) await loadPlaylists()
    if (selectedPlaylistId.value) return selectedPlaylistId.value
    return (await createPlaylist(name))?.id ?? ''
  }

  async function renamePlaylist(playlistId: string, name: string): Promise<void> {
    await renameScopedPlaylist(scope, playlistId, name)
    await loadPlaylists()
  }

  async function deletePlaylist(playlistId: string): Promise<void> {
    await removeScopedPlaylist(scope, playlistId)
    if (selectedPlaylistId.value === playlistId) selectedPlaylistId.value = ''
    await loadPlaylists()
  }

  async function addTracks(playlistId: string, musics: Music[]) {
    if (!playlistId || !requireNamedUser()) return null
    const result = await addPersonalTracks(playlistId, musics)
    await Promise.all([loadPlaylists(), loadTracks(playlistId)])
    return result
  }

  async function addTracksToSelected(musics: Music[], fallbackName = '我的歌单') {
    const playlistId = await ensurePlaylist(fallbackName)
    return playlistId ? addTracks(playlistId, musics) : null
  }

  async function importPlaylist(playlistId: string, platform: string, externalPlaylistId: string) {
    const result = await importScopedPlaylist(scope, playlistId, platform, externalPlaylistId)
    await Promise.all([loadPlaylists(), loadTracks(playlistId)])
    return result
  }

  async function removeTrack(playlistId: string, trackId: string): Promise<void> {
    await removePlaylistTrack(scope, playlistId, trackId)
    await Promise.all([loadPlaylists(), loadTracks(playlistId)])
  }

  async function exportPlaylist(playlistId: string, format = 'txt'): Promise<string> {
    if (!playlistId || !requireNamedUser()) return ''
    return personalPlaylistsApi.exportPlaylist(playlistId, format)
  }

  async function enqueue(playlistId = selectedPlaylistId.value): Promise<boolean> {
    if (!playlistId || !requireNamedUser()) return false
    await personalPlaylistsApi.enqueue(playlistId)
    return true
  }

  return {
    playlists, selectedPlaylistId, selectedPlaylist, selectedTracks, likedPlaylist, loading, error,
    requireNamedUser, loadPlaylists, loadTracks, ensurePlaylist, createPlaylist, renamePlaylist,
    deletePlaylist, addTracks, addTracksToSelected,
    importNetease: (playlistId: string, externalPlaylistId: string) => importPlaylist(playlistId, 'netease', externalPlaylistId),
    importPlaylist, removeTrack, deleteTrack: removeTrack, exportPlaylist, enqueue
  }
})
