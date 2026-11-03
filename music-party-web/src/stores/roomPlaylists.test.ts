import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { queryClient } from '../app/providers'
import { playlistsApi } from '../api/playlists'
import { useRoomStore } from './room'
import { useRoomPlaylistsStore } from './roomPlaylists'

vi.mock('../api/playlists', () => ({
  playlistsApi: {
    list: vi.fn(), create: vi.fn(), rename: vi.fn(), remove: vi.fn(), tracks: vi.fn(),
    addTrack: vi.fn(), removeTrack: vi.fn(), reorder: vi.fn(), importExternal: vi.fn(), exportPlaylist: vi.fn(),
  }
}))

describe('room playlist query and commands', () => {
  beforeEach(() => {
    localStorage.clear()
    queryClient.clear()
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('reads list and selected tracks from the room-scoped Query Cache', async () => {
    vi.mocked(playlistsApi.list).mockResolvedValue([{ id: 'room-p1', name: 'Room mix' }])
    vi.mocked(playlistsApi.tracks).mockResolvedValue([{ id: 't1', music: { id: 'song', name: 'Song' } } as never])
    const store = useRoomPlaylistsStore()

    await store.loadPlaylists()

    expect(store.playlists.map(item => item.id)).toEqual(['room-p1'])
    expect(store.selectedTracks.map(track => track.id)).toEqual(['t1'])
    expect(playlistsApi.list).toHaveBeenCalledWith('lounge', expect.any(AbortSignal))
    expect(playlistsApi.tracks).toHaveBeenCalledWith('lounge', 'room-p1', 0, 200)
  })

  it('switches list scope with the active room and drops the old selection', async () => {
    vi.mocked(playlistsApi.list).mockImplementation(async roomId => [{ id: `${roomId}-playlist`, name: roomId }])
    const store = useRoomPlaylistsStore()
    const room = useRoomStore()
    await store.loadPlaylists()
    expect(store.selectedPlaylistId).toBe('lounge-playlist')

    room.setCurrentRoom('studio')
    await store.loadPlaylists()

    expect(store.playlists.map(item => item.id)).toEqual(['studio-playlist'])
    expect(store.selectedPlaylistId).toBe('studio-playlist')
    expect(playlistsApi.list).toHaveBeenCalledWith('studio')
  })
})
