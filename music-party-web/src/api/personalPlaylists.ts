import type { Music, PlaylistSummary, PlaylistTrack } from '../contracts/generated/models'
import type { PlaylistMutationResult } from './playlists'
import { httpClient as client } from '../transport/httpClient'

export type LikedSongTrack = PlaylistTrack

const base = '/api/me/playlists'
const likedBase = '/api/me/liked-songs'
const id = (value: string) => encodeURIComponent(value)

export const personalPlaylistsApi = {
  list: () => client.get<PlaylistSummary[]>(base),
  create: (name: string) => client.post<PlaylistSummary, { name: string }>(base, { name }),
  rename: (playlistId: string, name: string) =>
    client.patch<PlaylistSummary, { name: string }>(`${base}/${id(playlistId)}`, { name }),
  remove: (playlistId: string) => client.delete<void>(`${base}/${id(playlistId)}`),
  tracks: (playlistId: string, offset = 0, limit = 100) =>
    client.get<PlaylistTrack[]>(`${base}/${id(playlistId)}/tracks`, { params: { offset, limit } }),
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
  exportPlaylist: (playlistId: string, format = 'txt') =>
    client.get<string>(`${base}/${id(playlistId)}/export`, { params: { format } }),
  enqueue: (playlistId: string) => client.post<PlaylistMutationResult>(`${base}/${id(playlistId)}/enqueue`),
  likedSongs: () => client.get<LikedSongTrack[]>(likedBase),
  likeSong: (music: Music) => client.put<Music, Music>(`${likedBase}/${id(music.platform)}/${id(music.id)}`, music),
  unlikeSong: (platform: string, musicId: string) => client.delete<void>(`${likedBase}/${id(platform)}/${id(musicId)}`)
}
