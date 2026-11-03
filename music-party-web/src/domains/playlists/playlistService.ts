import type { Music, PlaylistSummary, PlaylistTrack } from '../../contracts/generated/models'
import { personalPlaylistsApi } from '../../api/personalPlaylists'
import { playlistsApi, type PlaylistMutationResult } from '../../api/playlists'
import { queryKeys } from '../queryKeys'
import { queryClient } from '../../app/providers'
import type { QueryFunctionContext } from '@tanstack/vue-query'
import type { QueryKey } from '@tanstack/vue-query'

export type PlaylistScope = { type: 'personal'; publicId: string } | { type: 'room'; roomId: string }

export const playlistKeys = {
  list: (scope: PlaylistScope) => scope.type === 'personal'
    ? queryKeys.playlists.personal(scope.publicId)
    : queryKeys.playlists.room(scope.roomId),
  tracks: (scope: PlaylistScope, playlistId: string) => scope.type === 'personal'
    ? queryKeys.playlists.personalTracks(scope.publicId, playlistId)
    : queryKeys.playlists.roomTracks(scope.roomId, playlistId)
}

export function playlistListQueryOptions(scope: Extract<PlaylistScope, { type: 'personal' }>) {
  return { queryKey: playlistKeys.list(scope) as QueryKey, queryFn: () => personalPlaylistsApi.list() }
}

export function roomPlaylistListQueryOptions(scope: Extract<PlaylistScope, { type: 'room' }>) {
  return {
    queryKey: playlistKeys.list(scope) as QueryKey,
    queryFn: ({ signal }: QueryFunctionContext) => playlistsApi.list(scope.roomId, signal),
  }
}

export function playlistTracksQueryOptions(scope: Extract<PlaylistScope, { type: 'personal' }>, playlistId: string, limit?: number) {
  return {
    queryKey: playlistKeys.tracks(scope, playlistId) as QueryKey,
    queryFn: () => personalPlaylistsApi.tracks(playlistId, 0, limit ?? 500),
  }
}

export function roomPlaylistTracksQueryOptions(scope: Extract<PlaylistScope, { type: 'room' }>, playlistId: string, limit = 500) {
  return {
    queryKey: playlistKeys.tracks(scope, playlistId) as QueryKey,
    queryFn: ({ signal }: QueryFunctionContext) => playlistsApi.tracks(scope.roomId, playlistId, 0, limit, signal),
  }
}

export async function listPlaylists(scope: PlaylistScope): Promise<PlaylistSummary[]> {
  return queryClient.fetchQuery({
    queryKey: playlistKeys.list(scope),
    queryFn: () => scope.type === 'personal' ? personalPlaylistsApi.list() : playlistsApi.list(scope.roomId),
    staleTime: 0,
  })
}

export async function listPlaylistTracks(scope: PlaylistScope, playlistId: string, limit = 500): Promise<PlaylistTrack[]> {
  return queryClient.fetchQuery({
    queryKey: playlistKeys.tracks(scope, playlistId),
    queryFn: () => scope.type === 'personal'
      ? personalPlaylistsApi.tracks(playlistId, 0, limit)
      : playlistsApi.tracks(scope.roomId, playlistId, 0, limit),
    staleTime: 0,
  })
}

async function refresh(scope: PlaylistScope, playlistId?: string): Promise<void> {
  await queryClient.invalidateQueries({ queryKey: playlistKeys.list(scope) })
  if (playlistId) await queryClient.invalidateQueries({ queryKey: playlistKeys.tracks(scope, playlistId) })
}

export async function createPlaylist(scope: PlaylistScope, name: string): Promise<PlaylistSummary> {
  const playlist = scope.type === 'personal'
    ? await personalPlaylistsApi.create(name)
    : await playlistsApi.create(scope.roomId, name)
  await refresh(scope)
  return playlist
}

export async function renamePlaylist(scope: PlaylistScope, playlistId: string, name: string): Promise<void> {
  if (scope.type === 'personal') await personalPlaylistsApi.rename(playlistId, name)
  else await playlistsApi.rename(scope.roomId, playlistId, name)
  await refresh(scope)
}

export async function removePlaylist(scope: PlaylistScope, playlistId: string): Promise<void> {
  if (scope.type === 'personal') await personalPlaylistsApi.remove(playlistId)
  else await playlistsApi.remove(scope.roomId, playlistId)
  queryClient.removeQueries({ queryKey: playlistKeys.tracks(scope, playlistId) })
  await refresh(scope)
}

export async function removePlaylistTrack(scope: PlaylistScope, playlistId: string, trackId: string): Promise<void> {
  if (scope.type === 'personal') await personalPlaylistsApi.removeTrack(playlistId, trackId)
  else await playlistsApi.removeTrack(scope.roomId, playlistId, trackId)
  await refresh(scope, playlistId)
}

export async function reorderPlaylistTracks(scope: PlaylistScope, playlistId: string, trackIds: string[]): Promise<void> {
  if (scope.type === 'personal') await personalPlaylistsApi.reorder(playlistId, trackIds)
  else await playlistsApi.reorder(scope.roomId, playlistId, trackIds)
  await refresh(scope, playlistId)
}

export async function importPlaylist(scope: PlaylistScope, playlistId: string, platform: string, externalPlaylistId: string): Promise<PlaylistMutationResult> {
  const result = scope.type === 'personal'
    ? await personalPlaylistsApi.importPlaylist(playlistId, platform, externalPlaylistId)
    : await playlistsApi.importExternal(scope.roomId, playlistId, platform, externalPlaylistId)
  await refresh(scope, playlistId)
  return result
}

export async function addPersonalTracks(publicId: string, playlistId: string, musics: Music[]): Promise<PlaylistMutationResult> {
  const result = await personalPlaylistsApi.addTracks(playlistId, musics)
  const scope: PlaylistScope = { type: 'personal', publicId }
  await refresh(scope, playlistId)
  return result
}
