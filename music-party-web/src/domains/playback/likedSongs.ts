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

  try {
    const saved: unknown = JSON.parse(localStorage.getItem(STORAGE_KEYS.LIKED_SONGS) ?? '[]')
    likedSongs.value = Array.isArray(saved) ? saved as LikedSong[] : []
  } catch {
    likedSongs.value = []
  }

  function persist(): void {
    localStorage.setItem(STORAGE_KEYS.LIKED_SONGS, JSON.stringify(likedSongs.value))
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

  async function syncFromServer(isGuest: boolean): Promise<void> {
    if (isGuest) return
    const localSongs = [...likedSongs.value]
    likedSongs.value = (await personalPlaylistsApi.likedSongs()).map(fromTrack)
    persist()
    const serverKeys = new Set(likedSongs.value.map(song => song.key))
    const missing = localSongs.filter(song => !serverKeys.has(song.key))
    for (const song of missing) await personalPlaylistsApi.likeSong(song)
    if (missing.length) {
      likedSongs.value = (await personalPlaylistsApi.likedSongs()).map(fromTrack)
      persist()
    }
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

  return { likedSongs, add, remove, includes, syncFromServer, removeAndSync, toggle }
})
