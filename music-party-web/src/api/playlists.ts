import type { Music, PlaylistSummary, PlaylistTrack } from '../contracts/generated/models'
import { httpClient as client } from '../transport/httpClient'

export interface PlaylistMutationResult {
  accepted?: boolean
  count?: number
  addedCount?: number
  skippedCount?: number
  tracks?: PlaylistTrack[]
}

const base = (roomId: string) => `/api/rooms/${encodeURIComponent(roomId || 'lounge')}/playlists`
const id = (value: string) => encodeURIComponent(value)

export const playlistsApi = {
  list: (roomId: string) => client.get<PlaylistSummary[]>(base(roomId)),
  create: (roomId: string, name: string) => client.post<PlaylistSummary, { name: string }>(base(roomId), { name }),
  rename: (roomId: string, playlistId: string, name: string) =>
    client.patch<PlaylistSummary, { name: string }>(`${base(roomId)}/${id(playlistId)}`, { name }),
  remove: (roomId: string, playlistId: string) => client.delete<void>(`${base(roomId)}/${id(playlistId)}`),
  tracks: (roomId: string, playlistId: string, offset = 0, limit = 100) =>
    client.get<PlaylistTrack[]>(`${base(roomId)}/${id(playlistId)}/tracks`, { params: { offset, limit } }),
  addTrack: (roomId: string, playlistId: string, music: Music) =>
    client.post<PlaylistTrack, { music: Music }>(`${base(roomId)}/${id(playlistId)}/tracks`, { music }),
  removeTrack: (roomId: string, playlistId: string, trackId: string) =>
    client.delete<void>(`${base(roomId)}/${id(playlistId)}/tracks/${id(trackId)}`),
  reorder: (roomId: string, playlistId: string, trackIds: string[]) =>
    client.post<void, { trackIds: string[] }>(`${base(roomId)}/${id(playlistId)}/tracks/reorder`, { trackIds }),
  importExternal: (roomId: string, playlistId: string, platform: string, externalPlaylistId: string) =>
    client.post<PlaylistMutationResult, { platform: string; playlistId: string }>(
      `${base(roomId)}/${id(playlistId)}/import`,
      { platform, playlistId: externalPlaylistId }
    ),
  exportPlaylist: (roomId: string, playlistId: string, format = 'txt') =>
    client.get<string>(`${base(roomId)}/${id(playlistId)}/export`, { params: { format } })
}
