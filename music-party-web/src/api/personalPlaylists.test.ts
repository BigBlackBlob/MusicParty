import { beforeEach, describe, expect, it, vi } from 'vitest'

const put = vi.fn()

vi.mock('../transport/httpClient', () => ({
  httpClient: { put }
}))

const { personalPlaylistsApi } = await import('./personalPlaylists')

describe('personalPlaylistsApi likes', () => {
  beforeEach(() => put.mockReset())

  it('sends only Music fields when the source is PlayableMusic', () => {
    const playable = {
      id: 'track-1',
      name: 'Track',
      artists: ['Artist'],
      duration: 120_000,
      platform: 'subsonic-local-navi@lounge',
      coverUrl: '',
      url: '/api/play/track-1',
      needsProxy: true
    }

    personalPlaylistsApi.likeSong(playable)

    expect(put).toHaveBeenCalledWith(
      '/api/me/liked-songs/subsonic-local-navi%40lounge/track-1',
      {
        id: 'track-1',
        name: 'Track',
        artists: ['Artist'],
        duration: 120_000,
        platform: 'subsonic-local-navi@lounge',
        coverUrl: ''
      }
    )
  })
})
