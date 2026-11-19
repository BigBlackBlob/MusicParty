import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useLikedSongsStore } from './likedSongs'
import { personalPlaylistsApi, type LikedSongTrack } from '../../api/personalPlaylists'

vi.mock('../../api/personalPlaylists', () => ({
  personalPlaylistsApi: { likedSongs: vi.fn(), likeSong: vi.fn(), unlikeSong: vi.fn() },
}))

const song = (id: string) => ({ id, platform: 'netease', name: id, artists: [] })
const track = (id: string): LikedSongTrack => ({ id, playlistId: 'likes', music: song(id), createdAt: 1, sortOrder: 0 })

describe('server-authoritative liked songs', () => {
  beforeEach(() => {
    localStorage.clear()
    setActivePinia(createPinia())
    vi.resetAllMocks()
    vi.mocked(personalPlaylistsApi.likedSongs).mockResolvedValue([])
  })

  it('loads from the server and deletes legacy caches without uploading them', async () => {
    localStorage.setItem('mp_liked_songs:member', JSON.stringify([song('cached')]))
    vi.mocked(personalPlaylistsApi.likedSongs).mockResolvedValue([track('server')])
    const store = useLikedSongsStore()
    await store.syncFromServer('member', false)
    expect(store.likedSongs.map(item => item.id)).toEqual(['server'])
    expect(localStorage.getItem('mp_liked_songs:member')).toBeNull()
    expect(personalPlaylistsApi.likeSong).not.toHaveBeenCalled()
  })

  it('ignores a previous account response after switching to a guest', async () => {
    let finish!: (tracks: LikedSongTrack[]) => void
    vi.mocked(personalPlaylistsApi.likedSongs).mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const store = useLikedSongsStore()
    const request = store.syncFromServer('member', false)
    await store.syncFromServer('guest', true)
    finish([track('member-only')])
    await request
    await store.toggle(song('guest-only'))
    expect(store.likedSongs).toEqual([])
    expect(personalPlaylistsApi.likedSongs).toHaveBeenCalledOnce()
    expect(personalPlaylistsApi.likeSong).not.toHaveBeenCalled()
  })

  it('rolls back the failed song without discarding another optimistic write', async () => {
    const store = useLikedSongsStore()
    await store.syncFromServer('member', false)
    let rejectFirst!: (error: Error) => void
    vi.mocked(personalPlaylistsApi.likeSong).mockImplementationOnce(() => new Promise((_, reject) => { rejectFirst = reject }))
    vi.mocked(personalPlaylistsApi.likedSongs).mockResolvedValue([track('saved')])
    const failed = store.toggle(song('failed'))
    expect(store.isLiked(song('failed'))).toBe(true)
    await store.toggle(song('saved'))
    rejectFirst(new Error('offline'))
    await expect(failed).rejects.toThrow('offline')
    expect(store.likedSongs.map(item => item.id)).toEqual(['saved'])
  })
})
