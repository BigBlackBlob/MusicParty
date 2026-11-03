import { computed, ref, watch } from 'vue'
import { defineStore } from 'pinia'
import { useQuery } from '@tanstack/vue-query'
import { queryClient } from '../app/providers'
import type { Music } from '../contracts/generated/models'
import { personalPlaylistsApi } from '../api/personalPlaylists'
import {
  addPersonalTracks,
  createPlaylist as createScopedPlaylist,
  importPlaylist as importScopedPlaylist,
  listPlaylists,
  listPlaylistTracks,
  playlistListQueryOptions,
  playlistTracksQueryOptions,
  removePlaylist as removeScopedPlaylist,
  removePlaylistTrack,
  renamePlaylist as renameScopedPlaylist,
  type PlaylistScope
} from '../domains/playlists/playlistService'
import { useUserStore } from './user'

export const useUserPlaylistsStore = defineStore('userPlaylists', () => {
  const selectedPlaylistId = ref('')
  const userStore = useUserStore()
  const scope = computed<Extract<PlaylistScope, { type: 'personal' }>>(() => ({ type: 'personal', publicId: userStore.publicId }))
  const canQueryPersonalPlaylists = computed(() => userStore.hasDisplayName && !userStore.isGuest)
  const listQuery = useQuery(computed(() => ({
    ...playlistListQueryOptions(scope.value),
    enabled: canQueryPersonalPlaylists.value,
  })), queryClient)
  const tracksQuery = useQuery(computed(() => ({
    ...playlistTracksQueryOptions(scope.value, selectedPlaylistId.value),
    enabled: Boolean(selectedPlaylistId.value),
  })), queryClient)

  watch(() => userStore.publicId, () => { selectedPlaylistId.value = '' })

  const playlists = computed(() => listQuery.data.value ?? [])
  const selectedPlaylist = computed(() => playlists.value.find(item => item.id === selectedPlaylistId.value) ?? null)
  const selectedTracks = computed(() => selectedPlaylistId.value ? tracksQuery.data.value ?? [] : [])
  const likedPlaylist = computed(() => playlists.value.find(item => item.systemKey === 'liked-songs') ?? null)
  const loading = computed(() => listQuery.isFetching.value || tracksQuery.isFetching.value)
  const error = computed(() => {
    const cause = listQuery.error.value ?? tracksQuery.error.value
    return cause instanceof Error ? cause.message : ''
  })

  function requireNamedUser(): boolean {
    if (!userStore.isGuest) return true
    userStore.showNameModal = true
    return false
  }

  async function loadPlaylists() {
    if (!requireNamedUser()) return []
    const values = await listPlaylists(scope.value)
    if (!selectedPlaylistId.value && values.length) selectedPlaylistId.value = values[0]?.id ?? ''
    return values
  }

  async function loadTracks(playlistId = selectedPlaylistId.value) {
    if (!playlistId || !requireNamedUser()) return []
    return listPlaylistTracks(scope.value, playlistId)
  }

  async function createPlaylist(name: string) {
    if (!requireNamedUser()) return null
    const playlist = await createScopedPlaylist(scope.value, name)
    selectedPlaylistId.value = playlist.id
    return playlist
  }

  async function ensurePlaylist(name = '我的歌单'): Promise<string | null> {
    if (!requireNamedUser()) return null
    if (!playlists.value.length) await loadPlaylists()
    if (selectedPlaylistId.value) return selectedPlaylistId.value
    if (playlists.value.length) {
      selectedPlaylistId.value = playlists.value[0]?.id ?? ''
      if (selectedPlaylistId.value) return selectedPlaylistId.value
    }
    return (await createPlaylist(name))?.id ?? ''
  }

  async function renamePlaylist(playlistId: string, name: string): Promise<void> {
    await renameScopedPlaylist(scope.value, playlistId, name)
  }

  async function deletePlaylist(playlistId: string): Promise<void> {
    await removeScopedPlaylist(scope.value, playlistId)
    if (selectedPlaylistId.value === playlistId) selectedPlaylistId.value = ''
  }

  async function addTracks(playlistId: string, musics: Music[]) {
    if (!playlistId || !requireNamedUser()) return null
    const result = await addPersonalTracks(userStore.publicId, playlistId, musics)
    return result
  }

  async function addTracksToSelected(musics: Music[], fallbackName = '我的歌单') {
    const playlistId = await ensurePlaylist(fallbackName)
    return playlistId ? addTracks(playlistId, musics) : null
  }

  async function importPlaylist(playlistId: string, platform: string, externalPlaylistId: string) {
    const result = await importScopedPlaylist(scope.value, playlistId, platform, externalPlaylistId)
    return result
  }

  async function removeTrack(playlistId: string, trackId: string): Promise<void> {
    await removePlaylistTrack(scope.value, playlistId, trackId)
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
