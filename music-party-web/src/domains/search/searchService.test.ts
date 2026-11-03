import { beforeEach, describe, expect, it, vi } from 'vitest'
import { queryClient } from '../../app/providers'
import { musicApi } from '../../api/music'
import { loadPlatformPlaylistPage, searchTracks } from './searchService'

vi.mock('../../api/music', () => ({
  musicApi: {
    search: vi.fn(),
    getPlaylistSongs: vi.fn(),
  },
}))

describe('search service', () => {
  beforeEach(() => {
    queryClient.clear()
    vi.clearAllMocks()
  })

  it('isolates track searches by room, platform, keyword, and page', async () => {
    vi.mocked(musicApi.search).mockResolvedValue([])

    await searchTracks('netease', 'blue', 20, 20, 'room-a')
    await searchTracks('netease', 'blue', 40, 20, 'room-a')
    await searchTracks('netease', 'blue', 20, 20, 'room-b')

    expect(musicApi.search).toHaveBeenCalledTimes(3)
    expect(musicApi.search).toHaveBeenNthCalledWith(1, 'netease', 'blue', 20, 20, 'room-a', expect.any(AbortSignal))
    expect(musicApi.search).toHaveBeenNthCalledWith(2, 'netease', 'blue', 40, 20, 'room-a', expect.any(AbortSignal))
    expect(musicApi.search).toHaveBeenNthCalledWith(3, 'netease', 'blue', 20, 20, 'room-b', expect.any(AbortSignal))
  })

  it('loads platform playlist pages through the cached search boundary', async () => {
    vi.mocked(musicApi.getPlaylistSongs).mockResolvedValue([])

    await loadPlatformPlaylistPage('bilibili', 'playlist-1', 0, 20)
    await loadPlatformPlaylistPage('bilibili', 'playlist-1', 0, 20)
    await loadPlatformPlaylistPage('bilibili', 'playlist-1', 20, 20)

    expect(musicApi.getPlaylistSongs).toHaveBeenCalledTimes(2)
    expect(musicApi.getPlaylistSongs).toHaveBeenNthCalledWith(1, 'bilibili', 'playlist-1', 0, 20, expect.any(AbortSignal))
    expect(musicApi.getPlaylistSongs).toHaveBeenNthCalledWith(2, 'bilibili', 'playlist-1', 20, 20, expect.any(AbortSignal))
  })
})
