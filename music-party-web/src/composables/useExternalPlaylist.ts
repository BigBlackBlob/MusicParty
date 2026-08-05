import { computed, ref } from 'vue'
import type { Music } from '../contracts/generated/models'
import { musicApi } from '../api/music'
import { queryKeys } from '../domains/queryKeys'
import { queryClient } from '../app/providers'

const legacyCacheKeys = [
  'mp_search_playlist_songs',
  'mp_search_playlist_id',
  'mp_search_playlist_page'
] as const
const playlistLimit = 50

export function parseNeteasePlaylistId(input: unknown): string | null {
  const value = String(input ?? '').trim()
  if (!value) return null
  for (const pattern of [/playlist\?id=(\d+)/, /playlist\/(\d+)/, /music\.163\.com\/.*id=(\d+)/, /^(\d+)$/]) {
    const match = value.match(pattern)
    if (match?.[1]) return match[1]
  }
  return null
}

export function useExternalPlaylist() {
  for (const key of legacyCacheKeys) localStorage.removeItem(key)
  const playlistSongs = ref<Music[]>([])
  const playlistId = ref('')
  const currentPlaylistPage = ref(1)
  const canGoPlaylistNext = computed(() => playlistSongs.value.length === playlistLimit)

  async function loadNeteasePlaylistPage(input: unknown, page = 1): Promise<Music[] | null> {
    const id = parseNeteasePlaylistId(input) ?? playlistId.value
    if (!id) return null
    const offset = (page - 1) * playlistLimit
    const data = await queryClient.fetchQuery({
      queryKey: queryKeys.search.externalPlaylist('netease', id, page),
      queryFn: ({ signal }) => musicApi.getPlaylistSongs('netease', id, offset, playlistLimit, signal)
    })
    playlistSongs.value = data
    playlistId.value = id
    currentPlaylistPage.value = page
    return data
  }

  function clearExternalPlaylist(): void {
    playlistSongs.value = []
    playlistId.value = ''
    currentPlaylistPage.value = 1
  }

  return {
    playlistSongs,
    playlistId,
    currentPlaylistPage,
    canGoPlaylistNext,
    loadNeteasePlaylistPage,
    clearExternalPlaylist
  }
}
