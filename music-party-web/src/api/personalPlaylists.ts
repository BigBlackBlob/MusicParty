import type { Music, PlaylistSummary, PlaylistTrack } from '../contracts/generated/models'
import type { PlaylistMutationResult } from './playlists'
import { httpClient as client } from '../transport/httpClient'

export type LikedSongTrack = PlaylistTrack

const base = '/api/me/playlists'
const likedBase = '/api/me/liked-songs'
const id = (value: string) => encodeURIComponent(value)

export const personalPlaylistsApi = {
  list: (signal?: AbortSignal) => client.get<PlaylistSummary[]>(base, signal ? { signal } : {}),
  create: (name: string) => client.post<PlaylistSummary, { name: string }>(base, { name }),
  rename: (playlistId: string, name: string) =>
    client.patch<PlaylistSummary, { name: string }>(`${base}/${id(playlistId)}`, { name }),
  remove: (playlistId: string) => client.delete<void>(`${base}/${id(playlistId)}`),
  tracks: (playlistId: string, offset = 0, limit = 100, signal?: AbortSignal) =>
    client.get<PlaylistTrack[]>(`${base}/${id(playlistId)}/tracks`, { params: { offset, limit }, ...(signal ? { signal } : {}) }),
  addTracks: (playlistId: string, musics: Music[]) =>
    client.post<PlaylistMutationResult, { musics: Music[] }>(`${base}/${id(playlistId)}/tracks/batch`, { musics }),
  removeTrack: (playlistId: string, trackId: string) =>
    client.delete<void>(`${base}/${id(playlistId)}/tracks/${id(trackId)}`),
  reorder: (playlistId: string, trackIds: string[]) =>
    client.post<void, { trackIds: string[] }>(`${base}/${id(playlistId)}/tracks/reorder`, { trackIds }),
  importNetease: (playlistId: string, externalPlaylistId: string) =>
    client.post<PlaylistMutationResult, { playlistId: string }>(`${base}/${id(playlistId)}/import/netease`, { playlistId: externalPlaylistId }),
  importPlaylist: (playlistId: string, platform: string, externalPlaylistId: string) =>
    client.post<PlaylistMutationResult, { platform: string; playlistId: string }>(
      `${base}/${id(playlistId)}/import`,
      { platform, playlistId: externalPlaylistId }
    ),
  uploadCover: (playlistId: string, file: File) => {
    const form = new FormData()
    form.append('file', file)
    return client.put<void, FormData>(`${base}/${id(playlistId)}/cover`, form)
  },
  updateDescription: (playlistId: string, description: string) =>
    client.patch<void, { description: string }>(`${base}/${id(playlistId)}/description`, { description }),
  updateTrackNotes: (playlistId: string, trackId: string, notes: string) =>
    client.patch<void, { notes: string }>(`${base}/${id(playlistId)}/tracks/${id(trackId)}/notes`, { notes }),
  move: (playlistId: string, position: number) =>
    client.put<void, { position: number }>(`${base}/${id(playlistId)}/order`, { position }),
  exportPlaylist: (playlistId: string, format = 'txt') =>
    client.get<string>(`${base}/${id(playlistId)}/export`, { params: { format }, responseType: 'text' }),
  enqueue: (playlistId: string, roomId: string) =>
    client.post<PlaylistMutationResult>(`${base}/${id(playlistId)}/enqueue`, undefined, { params: { roomId } }),
  likedSongs: () => client.get<LikedSongTrack[]>(likedBase),
  likeSong: (music: Music) => {
    const { id: musicId, name, artists, duration, platform, coverUrl } = music
    return client.put<PlaylistMutationResult, Music>(`${likedBase}/${id(platform)}/${id(musicId)}`, {
      id: musicId,
      name,
      artists,
      duration,
      platform,
      coverUrl
    })
  },
  unlikeSong: (platform: string, musicId: string) => client.delete<void>(`${likedBase}/${id(platform)}/${id(musicId)}`)
}
