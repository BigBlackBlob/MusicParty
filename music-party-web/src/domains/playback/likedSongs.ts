import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Music } from '../../contracts/generated/models'
import { personalPlaylistsApi, type LikedSongTrack } from '../../api/personalPlaylists'
import { STORAGE_KEYS } from '../../constants/keys'

export interface LikedSong extends Music {
  key: string
  likedAt: number
}

const songKey = (music: Pick<Music, 'platform' | 'id'>): string => `${music.platform}:${music.id}`
const toLikedSong = (music: Music, likedAt = Date.now()): LikedSong => ({
  ...music,
  artists: music.artists ?? [],
  duration: music.duration ?? 0,
  coverUrl: music.coverUrl ?? '',
  key: songKey(music),
  likedAt
})
const fromTrack = (track: LikedSongTrack): LikedSong => toLikedSong(track.music, track.createdAt)

export const useLikedSongsStore = defineStore('liked-songs', () => {
  const likedSongs = ref<LikedSong[]>([])
  let activeAccountId = ''

  const storageKey = (accountId: string): string => `${STORAGE_KEYS.LIKED_SONGS}:${accountId}`
  const readCached = (accountId: string): LikedSong[] => {
    if (!accountId) return []
    try {
      const saved: unknown = JSON.parse(localStorage.getItem(storageKey(accountId)) ?? '[]')
      return Array.isArray(saved) ? saved as LikedSong[] : []
    } catch {
      return []
    }
  }

  function persist(): void {
    if (!activeAccountId) return
    localStorage.setItem(storageKey(activeAccountId), JSON.stringify(likedSongs.value))
  }

  function add(music: Music): void {
    const next = toLikedSong(music)
    likedSongs.value = [next, ...likedSongs.value.filter(song => song.key !== next.key)].slice(0, 500)
    persist()
  }

  function remove(key: string): LikedSong | undefined {
    const existing = likedSongs.value.find(song => song.key === key)
    likedSongs.value = likedSongs.value.filter(song => song.key !== key)
    persist()
    return existing
  }

  function includes(music: Music | null | undefined): boolean {
    return Boolean(music && likedSongs.value.some(song => song.key === songKey(music)))
  }

  async function syncFromServer(accountId: string, isGuest: boolean): Promise<void> {
    activeAccountId = accountId
    likedSongs.value = readCached(accountId)
    if (isGuest || !accountId) return
    const serverSongs = (await personalPlaylistsApi.likedSongs()).map(fromTrack)
    if (activeAccountId !== accountId) return
    likedSongs.value = serverSongs
    persist()
  }

  function reset(): void {
    activeAccountId = ''
    likedSongs.value = []
  }

  async function removeAndSync(key: string, isGuest: boolean): Promise<void> {
    const song = remove(key)
    if (!song || isGuest) return
    try {
      await personalPlaylistsApi.unlikeSong(song.platform, song.id)
    } catch (error) {
      add(song)
      throw error
    }
  }

  async function toggle(music: Music): Promise<'liked' | 'unliked'> {
    const key = songKey(music)
    if (includes(music)) {
      remove(key)
      try {
        await personalPlaylistsApi.unlikeSong(music.platform, music.id)
      } catch (error) {
        add(music)
        throw error
      }
      return 'unliked'
    }
    add(music)
    try {
      await personalPlaylistsApi.likeSong(music)
    } catch (error) {
      remove(key)
      throw error
    }
    return 'liked'
  }

  return { likedSongs, add, remove, includes, syncFromServer, removeAndSync, toggle, reset }
})
