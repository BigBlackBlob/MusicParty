import type { AccountStatus, LocalTrack, RoomInvite, RoomMember, Session } from '../contracts/generated/models'
import { httpClient as client } from '../transport/httpClient'

const accountRequestOptions = { timeout: 60_000 }
const id = (value: string) => encodeURIComponent(value)

export interface InviteMetadata {
  roomId: string
  roomName: string
  expiresAt: number
  valid: boolean
}

export interface SubsonicSource {
  id: string
  name: string
  url: string
  username?: string
  enabled: boolean
  sortOrder: number
  passwordConfigured?: boolean
}

export interface SubsonicSourceInput {
  id?: string
  name: string
  url: string
  username: string
  password?: string
  enabled?: boolean
  sortOrder?: number
}

export interface LocalTrackMetadata {
  name?: string
  artists?: string
  album?: string
  coverUrl?: string
  [key: string]: string | number | boolean | null | undefined
}

export const authApi = {
  getAccountStatus: () => client.get<AccountStatus>('/api/account/status'),
  createGuestSession: (displayName: string) =>
    client.post<Session, { displayName: string }>('/api/account/guest', { displayName }, accountRequestOptions),
  upgradeGuestToUser: (secret: string) =>
    client.post<Session, { secret: string }>('/api/account/upgrade', { secret }, accountRequestOptions),
  loginAccount: (username: string, password: string) =>
    client.post<Session, { username: string; password: string }>('/api/account/login', { username, password }, accountRequestOptions),
  getAccountMe: () => client.get<Session>('/api/account/me', accountRequestOptions),
  updateAccountProfile: (displayName: string) =>
    client.put<Session, { displayName: string }>('/api/account/profile', { displayName }, accountRequestOptions),
  logoutAccount: () => client.post<void>('/api/account/logout', undefined, accountRequestOptions),
  inviteMetadata: (secret: string) => client.get<InviteMetadata>(`/api/join/${id(secret)}/metadata`),
  redeemInvite: (secret: string, displayName: string) =>
    client.post<Session, { secret: string; displayName: string }>('/api/invites/redeem', { secret, displayName }, accountRequestOptions),
  createInvite: (roomId: string, label = '') =>
    client.post<RoomInvite, { label: string }>(`/api/rooms/${id(roomId)}/invites`, { label }),
  listInvites: (roomId: string) => client.get<RoomInvite[]>(`/api/rooms/${id(roomId)}/invites`),
  revokeInvite: (roomId: string, inviteId: string) => client.delete<void>(`/api/rooms/${id(roomId)}/invites/${id(inviteId)}`),
  updatePlatformCredential: (platform: string, credential: string) =>
    client.post<void, { platform: string; credential: string }>('/api/admin/platform-credentials', { platform, credential }),
  listRoomMembers: (roomId: string) => client.get<RoomMember[]>(`/api/rooms/${id(roomId)}/members`),
  removeRoomMember: (roomId: string, publicId: string) => client.delete<void>(`/api/rooms/${id(roomId)}/members/${id(publicId)}`),
  grantNavidrome: (userName: string, roomId: string) =>
    client.post<void, { userName: string; roomId: string }>('/api/admin/navidrome-access/grant', { userName, roomId }),
  revokeNavidrome: (userName: string, roomId: string) =>
    client.post<void, { userName: string; roomId: string }>('/api/admin/navidrome-access/revoke', { userName, roomId }),
  listSubsonicSources: (roomId: string) => client.get<SubsonicSource[]>('/api/admin/subsonic-sources', { params: { roomId } }),
  saveSubsonicSource: (roomId: string, source: SubsonicSourceInput) =>
    client.post<SubsonicSource, SubsonicSourceInput & { roomId: string }>('/api/admin/subsonic-source', { roomId, ...source }),
  removeSubsonicSource: (roomId: string, sourceId: string) =>
    client.post<void, { roomId: string; id: string }>('/api/admin/subsonic-source/remove', { roomId, id: sourceId }),
  testSubsonicSource: (roomId: string, sourceId: string) =>
    client.post<{ ok: boolean; message?: string }, { roomId: string; id: string }>('/api/admin/subsonic-source/test', { roomId, id: sourceId }),
  reorderSubsonicSource: (roomId: string, sourceId: string, sortOrder: number) =>
    client.post<void, { roomId: string; id: string; sortOrder: number }>('/api/admin/subsonic-source/order', { roomId, id: sourceId, sortOrder }),
  listLocalTracks: () => client.get<LocalTrack[]>('/api/local/tracks'),
  uploadLocalTrack: (file: File, metadata: LocalTrackMetadata = {}) => {
    const formData = new FormData()
    formData.append('file', file)
    Object.entries(metadata).forEach(([key, value]) => {
      if (value !== undefined && value !== null && String(value).trim()) formData.append(key, String(value))
    })
    return client.post<LocalTrack, FormData>('/api/local/tracks/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120_000
    })
  },
  updateLocalTrack: (trackId: string, track: Partial<LocalTrack>) => client.patch<LocalTrack, Partial<LocalTrack>>(`/api/local/tracks/${id(trackId)}`, track),
  deleteLocalTrack: (trackId: string) => client.delete<void>(`/api/local/tracks/${id(trackId)}`),
  listLocalUploadAccess: () => client.get<string[]>('/api/local/upload-access'),
  grantLocalUploadAccess: (userName: string) => client.post<void, { userName: string }>('/api/local/upload-access/grant', { userName }),
  revokeLocalUploadAccess: (userName: string) => client.post<void, { userName: string }>('/api/local/upload-access/revoke', { userName })
}
