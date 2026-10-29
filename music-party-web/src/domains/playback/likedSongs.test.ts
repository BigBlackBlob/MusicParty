import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useLikedSongsStore } from './likedSongs'
import { personalPlaylistsApi } from '../../api/personalPlaylists'

vi.mock('../../api/personalPlaylists', () => ({
  personalPlaylistsApi: {
    likedSongs: vi.fn(),
    likeSong: vi.fn(),
    unlikeSong: vi.fn()
  }
}))

const song = (id: string) => ({ id, platform: 'netease', name: id, artists: [] })

describe('liked songs account isolation', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.mocked(personalPlaylistsApi.likedSongs).mockResolvedValue([])
  })

  it('does not carry cached songs or upload them across accounts', async () => {
    const store = useLikedSongsStore()
    store.add(song('from-a'))
    expect(localStorage.getItem('mp_liked_songs')).toBeNull()

    await store.syncFromServer('account-b', false)

    expect(store.likedSongs).toEqual([])
    expect(personalPlaylistsApi.likeSong).not.toHaveBeenCalled()
    expect(localStorage.getItem('mp_liked_songs:account-b')).toBe('[]')
  })

  it('restores the correct account cache after switching', async () => {
    const store = useLikedSongsStore()
    await store.syncFromServer('account-a', true)
    store.add(song('a'))
    await store.syncFromServer('account-b', true)
    store.add(song('b'))
    await store.syncFromServer('account-a', true)

    expect(store.likedSongs.map(item => item.id)).toEqual(['a'])
  })
})
