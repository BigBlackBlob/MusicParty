import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { musicApi } from '../../api/music'
import { useLyricsStore } from '../playback/lyrics'
import { useRoomRealtimeCoordinator } from './roomRealtimeCoordinator'
import { useRoomRuntimeStore } from './roomRuntimeStore'

vi.mock('../../api/music', () => ({
  musicApi: {
    getLyricDetail: vi.fn(),
    getLyric: vi.fn(),
  },
}))

describe('room realtime lyrics coordination', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('loads lyrics whenever the authoritative current track changes', async () => {
    vi.mocked(musicApi.getLyricDetail).mockResolvedValue({
      lyric: '[00:01.00]Original',
      translatedLyric: '[00:01.00]Translation',
    })
    useRoomRealtimeCoordinator()
    const runtime = useRoomRuntimeStore()
    const lyrics = useLyricsStore()

    runtime.nowPlaying = {
      music: {
        id: '28816031',
        name: 'Track',
        artists: ['Artist'],
        duration: 120_000,
        platform: 'netease',
        coverUrl: '',
        url: '/api/netease/stream/28816031',
        needsProxy: false,
      },
      currentPosition: 0,
      likedUserIds: [],
      likeMarkers: [],
      playEpoch: 1,
      positionUpdatedAt: 0,
    }

    await vi.waitFor(() => {
      expect(musicApi.getLyricDetail).toHaveBeenCalledWith('netease', '28816031')
      expect(lyrics.lyricText).toBe('[00:01.00]Original')
      expect(lyrics.lyricDetail.translatedLyric).toBe('[00:01.00]Translation')
    })
  })
})
