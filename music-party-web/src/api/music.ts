import type { Music, PlatformDescriptor, PlaylistSummary } from '../contracts/generated/models'
import { httpClient as client } from '../transport/httpClient'
import type { RequestOptions } from '../transport/httpClient'

export interface AlbumSummary {
  id: string
  name: string
  artist?: string
  coverUrl?: string
  trackCount?: number
}

export interface ExternalUser {
  id: string
  name: string
  avatarUrl?: string
}

export interface LyricDetail {
  lyric?: string
  translatedLyric?: string
  romanizedLyric?: string
  [key: string]: string | undefined
}

export interface CoverColorResult {
  primary?: string
  secondary?: string
  accent?: string
}

const segment = (value: string) => encodeURIComponent(value)
const options = (config: RequestOptions, signal?: AbortSignal): RequestOptions =>
  signal ? { ...config, signal } : config

export const musicApi = {
  getPlatforms: (roomId: string, signal?: AbortSignal) =>
    client.get<PlatformDescriptor[]>('/api/platforms', options({ params: { roomId } }, signal)),
  search: (platform: string, keyword: string, offset = 0, limit = 20, roomId?: string, signal?: AbortSignal) =>
    client.get<Music[]>(`/api/search/${segment(platform)}/${segment(keyword)}`, options({
      params: { offset, limit, roomId }
    }, signal)),
  searchAlbums: (platform: string, keyword: string, roomId?: string, signal?: AbortSignal) =>
    client.get<AlbumSummary[]>(`/api/album/search/${segment(platform)}`, options({
      params: { keyword, roomId }
    }, signal)),
  getAlbumSongs: (platform: string, albumId: string, roomId?: string, signal?: AbortSignal) =>
    client.get<Music[]>(`/api/album/songs/${segment(platform)}/${segment(albumId)}`, options({
      params: { roomId }
    }, signal)),
  getLyric: (platform: string, songId: string, signal?: AbortSignal) =>
    client.get<string>(`/api/music/lyric/${segment(platform)}/${segment(songId)}`, options({}, signal)),
  getLyricDetail: (platform: string, songId: string, signal?: AbortSignal) =>
    client.get<LyricDetail>(`/api/music/lyric-detail/${segment(platform)}/${segment(songId)}`, options({}, signal)),
  getUserPlaylists: (platform: string, userId: string, signal?: AbortSignal) =>
    client.get<PlaylistSummary[]>(`/api/user/playlists/${segment(platform)}/${segment(userId)}`, options({}, signal)),
  getPlaylistSongs: (platform: string, playlistId: string, offset = 0, limit = 50, signal?: AbortSignal) =>
    client.get<Music[]>(`/api/playlist/songs/${segment(platform)}/${segment(playlistId)}`, options({
      params: { offset, limit }
    }, signal)),
  searchUser: (platform: string, keyword: string, signal?: AbortSignal) =>
    client.get<ExternalUser[]>(`/api/user/search/${segment(platform)}/${segment(keyword)}`, options({}, signal)),
  extractCoverColor: (url: string, signal?: AbortSignal) =>
    client.get<CoverColorResult>('/api/theme/extract-cover-color', options({ params: { url } }, signal))
}
