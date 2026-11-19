import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import type { Music } from '../../contracts/generated/models'
import { personalPlaylistsApi, type LikedSongTrack } from '../../api/personalPlaylists'

export interface LikedSong extends Music {
  key: string
  likedAt: number
}

const songKey = (music: Pick<Music, 'platform' | 'id'>): string => `${music.platform}:${music.id}`
const fromTrack = (track: LikedSongTrack): LikedSong => ({
  ...track.music,
  key: songKey(track.music),
  likedAt: track.createdAt,
})

function removeLegacyCache(): void {
  try {
    const prefix = 'mp_liked_songs'
    for (let index = localStorage.length - 1; index >= 0; index--) {
      const key = localStorage.key(index)
      if (key === prefix || key?.startsWith(`${prefix}:`)) localStorage.removeItem(key)
    }
  } catch {
    // Storage can be unavailable. Liked songs still load through the server.
  }
}

export const useLikedSongsStore = defineStore('liked-songs', () => {
  const tracks = ref<LikedSongTrack[]>([])
  const pending = reactive(new Map<string, { track: LikedSongTrack; liked: boolean; token: symbol }>())
  const likedSongs = computed(() => {
    const changes = [...pending.values()].filter(change => change.liked).reverse().map(change => change.track)
    return [...changes, ...tracks.value.filter(track => !pending.has(songKey(track.music)))].map(fromTrack)
  })
  let activeAccountId = ''
  let generation = 0
  let loadVersion = 0
  let cleaned = false

  function reset(): void {
    activeAccountId = ''
    generation++
    loadVersion++
    tracks.value = []
    pending.clear()
  }

  async function syncFromServer(accountId: string, isGuest: boolean): Promise<void> {
    if (!cleaned) {
      removeLegacyCache()
      cleaned = true
    }
    if (isGuest || !accountId) {
      reset()
      return
    }
    if (accountId !== activeAccountId) reset()
    activeAccountId = accountId
    const version = ++loadVersion
    const loaded = await personalPlaylistsApi.likedSongs()
    if (version === loadVersion) tracks.value = [...loaded].reverse()
  }

  function isLiked(music: Music | null | undefined): boolean {
    return Boolean(music && likedSongs.value.some(song => song.key === songKey(music)))
  }

  // Room controls change the same server playlist through a WebSocket mutation.
  function stageLike(music: Music, liked: boolean): ((accepted: boolean) => void) | undefined {
    const key = songKey(music)
    if (!activeAccountId || pending.has(key)) return
    const track = tracks.value.find(track => songKey(track.music) === key) ?? {
      id: key, playlistId: '', music, createdAt: Date.now(), sortOrder: 0,
    }
    const started = generation
    const token = Symbol(key)
    pending.set(key, { track, liked, token })
    return accepted => {
      if (started !== generation || pending.get(key)?.token !== token) return
      pending.delete(key)
      if (!accepted) return
      loadVersion++
      const remaining = tracks.value.filter(track => songKey(track.music) !== key)
      tracks.value = liked ? [track, ...remaining] : remaining
      void syncFromServer(activeAccountId, false).catch(() => undefined)
    }
  }

  async function toggle(music: Music): Promise<'liked' | 'unliked' | undefined> {
    const liked = !isLiked(music)
    const settle = stageLike(music, liked)
    if (!settle) return
    try {
      if (liked) await personalPlaylistsApi.likeSong(music)
      else await personalPlaylistsApi.unlikeSong(music.platform, music.id)
    } catch (error) {
      settle(false)
      throw error
    }
    settle(true)
    return liked ? 'liked' : 'unliked'
  }

  async function removeAndSync(key: string): Promise<void> {
    const song = likedSongs.value.find(song => song.key === key)
    if (song) await toggle(song)
  }

  return { likedSongs, isLiked, syncFromServer, removeAndSync, toggle, stageLike, reset }
})
