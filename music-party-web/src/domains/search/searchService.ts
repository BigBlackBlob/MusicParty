import type { Music } from '../../contracts/generated/models'
import { musicApi } from '../../api/music'
import { queryKeys } from '../queryKeys'
import { queryClient } from '../../app/providers'

export const searchTracks = (platform: string, keyword: string, offset: number, limit: number, roomId: string) =>
  queryClient.fetchQuery({
    queryKey: queryKeys.search.tracks(roomId, platform, keyword, Math.floor(offset / limit) + 1),
    queryFn: ({ signal }) => musicApi.search(platform, keyword, offset, limit, roomId, signal),
  })

export const searchAlbums = (platform: string, keyword: string, roomId: string) =>
  queryClient.fetchQuery({
    queryKey: queryKeys.search.albums(roomId, platform, keyword),
    queryFn: ({ signal }) => musicApi.searchAlbums(platform, keyword, roomId, signal),
  })

export const loadAlbumTracks = (platform: string, albumId: string, roomId: string) =>
  queryClient.fetchQuery({
    queryKey: queryKeys.search.albumTracks(roomId, platform, albumId),
    queryFn: ({ signal }) => musicApi.getAlbumSongs(platform, albumId, roomId, signal),
  })

export const loadExternalPlaylistPage = (playlistId: string, page: number, limit: number) =>
  queryClient.fetchQuery({
    queryKey: queryKeys.search.externalPlaylist('netease', playlistId, page),
    queryFn: ({ signal }) => musicApi.getPlaylistSongs('netease', playlistId, (page - 1) * limit, limit, signal),
  })

export const listPlatformUserPlaylists = (platform: string, userId: string) =>
  queryClient.fetchQuery({
    queryKey: queryKeys.search.userPlaylists(platform, userId),
    queryFn: ({ signal }) => musicApi.getUserPlaylists(platform, userId, signal),
  })

export const findPlatformUsers = (platform: string, keyword: string) =>
  queryClient.fetchQuery({
    queryKey: queryKeys.search.userSearch(platform, keyword),
    queryFn: ({ signal }) => musicApi.searchUser(platform, keyword, signal),
  })

export const loadPlatformPlaylistPage = (platform: string, playlistId: string, offset: number, limit: number) =>
  queryClient.fetchQuery({
    queryKey: queryKeys.search.userPlaylistTracks(platform, playlistId, offset, limit),
    queryFn: ({ signal }) => musicApi.getPlaylistSongs(platform, playlistId, offset, limit, signal),
  })

export function clearSearchCache(): Promise<void> {
  return queryClient.cancelQueries({ queryKey: ['search'] })
}

export function hasMissingCovers(songs: Music[]): number {
  return songs.filter(song => !song.coverUrl || !String(song.coverUrl).trim()).length
}
