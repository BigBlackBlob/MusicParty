import type { ServerEnvelope } from '../contracts/generated/websocket'
import type { ChatMessage, Music, PlaybackHistoryItem, PlaylistSummary, PlaylistTrack, RoomSummary } from '../contracts/generated/models'

const SERVER_TYPES = new Set<ServerEnvelope['type']>([
  'chat.history', 'chat.message', 'control.ack', 'enqueue.ack', 'enqueue.nack',
  'history.page', 'player.events', 'player.failed', 'player.state', 'playlist.ack', 'playlist.data',
  'playlist.nack', 'public-chat.history', 'public-chat.message', 'queue.mutation.ack',
  'queue.mutation.nack', 'queue.patch', 'queue.reorder.ack', 'queue.reorder.nack',
  'rooms.created', 'rooms.list', 'server.hello', 'sync.pong', 'user.me', 'users.online'
])

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

function isUserSummary(value: unknown): boolean {
  return isRecord(value)
    && typeof value.publicId === 'string'
    && typeof value.name === 'string'
    && typeof value.isGuest === 'boolean'
}

function isMusic(value: unknown): value is Music {
  return isRecord(value)
    && typeof value.id === 'string'
    && typeof value.name === 'string'
    && Array.isArray(value.artists)
    && value.artists.every(artist => typeof artist === 'string')
    && typeof value.duration === 'number'
    && typeof value.platform === 'string'
    && typeof value.coverUrl === 'string'
}

function isPlayableMusic(value: unknown): boolean {
  return isMusic(value) && isRecord(value) && typeof value.url === 'string' && typeof value.needsProxy === 'boolean'
}

function isQueueItem(value: unknown): boolean {
  return isRecord(value)
    && typeof value.queueId === 'string'
    && isMusic(value.music)
    && isUserSummary(value.enqueuedBy)
    && typeof value.status === 'string'
    && (value.playbackPriority === undefined || typeof value.playbackPriority === 'number')
}

function isPartialQueueItem(value: unknown): boolean {
  return isRecord(value)
    && (value.queueId === undefined || typeof value.queueId === 'string')
    && (value.music === undefined || isMusic(value.music))
    && (value.enqueuedBy === undefined || isUserSummary(value.enqueuedBy))
    && (value.status === undefined || typeof value.status === 'string')
    && (value.playbackPriority === undefined || typeof value.playbackPriority === 'number')
}

function isNowPlaying(value: unknown): boolean {
  return isRecord(value)
    && isPlayableMusic(value.music)
    && typeof value.currentPosition === 'number'
    && isOptionalNullableString(value.enqueuedById)
    && isOptionalNullableString(value.enqueuedByName)
    && Array.isArray(value.likedUserIds)
    && value.likedUserIds.every(id => typeof id === 'string')
    && Array.isArray(value.likeMarkers)
    && value.likeMarkers.every(marker => typeof marker === 'number')
    && typeof value.playEpoch === 'number'
    && typeof value.positionUpdatedAt === 'number'
}

function isChatMessage(value: unknown): value is ChatMessage {
  return isRecord(value)
    && typeof value.id === 'string'
    && typeof value.userId === 'string'
    && typeof value.userName === 'string'
    && typeof value.content === 'string'
    && typeof value.timestamp === 'number'
    && typeof value.type === 'string'
}

function isRoomSummary(value: unknown): value is RoomSummary {
  return isRecord(value)
    && typeof value.roomId === 'string'
    && typeof value.name === 'string'
    && typeof value.creatorPublicId === 'string'
    && typeof value.createdAt === 'number'
    && typeof value.privateRoom === 'boolean'
    && typeof value.system === 'boolean'
    && typeof value.onlineCount === 'number'
    && typeof value.accessGranted === 'boolean'
}

function isPlaylistSummary(value: unknown): value is PlaylistSummary {
  return isRecord(value)
    && typeof value.id === 'string'
    && (value.ownerPublicId === undefined || typeof value.ownerPublicId === 'string')
    && (value.roomId === undefined || typeof value.roomId === 'string')
    && typeof value.name === 'string'
    && (value.systemKey === null || typeof value.systemKey === 'string')
    && typeof value.trackCount === 'number'
    && typeof value.createdAt === 'number'
    && typeof value.updatedAt === 'number'
}

function isPlaylistTrack(value: unknown): value is PlaylistTrack {
  return isRecord(value)
    && typeof value.id === 'string'
    && typeof value.playlistId === 'string'
    && isMusic(value.music)
    && typeof value.sortOrder === 'number'
    && typeof value.createdAt === 'number'
}

function isPlaybackHistoryItem(value: unknown): value is PlaybackHistoryItem {
  return isRecord(value)
    && typeof value.id === 'string'
    && isMusic(value.music)
    && (typeof value.enqueuerPublicId === 'string' || value.enqueuerPublicId === null)
    && (typeof value.enqueuerName === 'string' || value.enqueuerName === null)
    && typeof value.playedAt === 'number'
}

function isOptionalArray<T>(value: unknown, validate: (item: unknown) => item is T): boolean {
  return value === undefined || (Array.isArray(value) && value.every(validate))
}

function isOptionalString(value: unknown): boolean {
  return value === undefined || typeof value === 'string'
}

function isOptionalNullableString(value: unknown): boolean {
  return value === undefined || value === null || typeof value === 'string'
}

function validPayload(type: ServerEnvelope['type'], payload: unknown): boolean {
  if (type === 'chat.history' || type === 'public-chat.history') {
    return Array.isArray(payload) && payload.every(isChatMessage)
  }
  if (type === 'rooms.list') {
    return Array.isArray(payload) && payload.every(isRoomSummary)
  }
  if (!isRecord(payload)) return false
  if (type === 'control.ack') {
    const committed = payload.committed
    return ['applied', 'noop', 'rejected'].includes(String(payload.outcome))
      && (payload.code === undefined || typeof payload.code === 'string')
      && (payload.mutationId === undefined || typeof payload.mutationId === 'string')
      && (payload.replayed === undefined || typeof payload.replayed === 'boolean')
      && (committed === undefined || (isRecord(committed)
        && typeof committed.stateVersion === 'number'
        && typeof committed.playEpoch === 'number'
        && typeof committed.queueVersion === 'number'))
  }
  if (type === 'player.state') {
    return typeof payload.stateVersion === 'number'
      && typeof payload.queueVersion === 'number'
      && typeof payload.playEpoch === 'number'
      && typeof payload.serverTimestamp === 'number'
      && typeof payload.historyCursor === 'number'
      && (payload.queue === undefined || (Array.isArray(payload.queue) && payload.queue.every(isQueueItem)))
      && (payload.nowPlaying === undefined || payload.nowPlaying === null || isNowPlaying(payload.nowPlaying))
      && ['isLoading', 'isPauseLocked', 'isPaused', 'isShuffle', 'isShuffleLocked', 'isSkipLocked'].every(key => typeof payload[key] === 'boolean')
      && isOptionalString(payload.queueEpoch)
      && isOptionalString(payload.idempotencyScopeId)
      && (payload.idempotencyTtlMs === undefined || typeof payload.idempotencyTtlMs === 'number')
  }
  if (type === 'chat.message' || type === 'public-chat.message') return isChatMessage(payload)
  if (type === 'server.hello') return typeof payload.apiVersion === 'string' && typeof payload.minimumClientVersion === 'string'
  if (type === 'history.page') {
    return Array.isArray(payload.items)
      && payload.items.every(isPlaybackHistoryItem)
      && typeof payload.offset === 'number'
      && typeof payload.roomId === 'string'
      && typeof payload.total === 'number'
  }
  if (type === 'playlist.data') {
    return typeof payload.operation === 'string'
      && typeof payload.scope === 'string'
      && isOptionalString(payload.playlistId)
      && isOptionalArray(payload.playlists, isPlaylistSummary)
      && isOptionalArray(payload.tracks, isPlaylistTrack)
  }
  if (type === 'playlist.ack') {
    return typeof payload.mutationId === 'string'
      && typeof payload.operation === 'string'
      && typeof payload.scope === 'string'
      && isOptionalString(payload.playlistId)
      && isOptionalArray(payload.playlists, isPlaylistSummary)
      && isOptionalArray(payload.tracks, isPlaylistTrack)
      && ['addedCount', 'count', 'removedCount', 'skippedCount'].every(key => payload[key] === undefined || typeof payload[key] === 'number')
      && (payload.playlist === undefined || isPlaylistSummary(payload.playlist))
  }
  if (type === 'playlist.nack') {
    return typeof payload.operation === 'string'
      && typeof payload.reason === 'string'
      && isOptionalString(payload.mutationId)
      && isOptionalString(payload.scope)
  }
  if (type === 'enqueue.ack') {
    return typeof payload.accepted === 'boolean'
      && typeof payload.mutationId === 'string'
      && (payload.count === undefined || typeof payload.count === 'number')
      && isOptionalString(payload.queueId)
  }
  if (type === 'enqueue.nack') {
    return typeof payload.mutationId === 'string'
      && typeof payload.reason === 'string'
      && (payload.accepted === undefined || typeof payload.accepted === 'boolean')
      && (payload.queueVersion === undefined || typeof payload.queueVersion === 'number')
  }
  if (type === 'queue.mutation.ack' || type === 'queue.reorder.ack') return typeof payload.mutationId === 'string'
  if (type === 'queue.mutation.nack' || type === 'queue.reorder.nack') {
    return typeof payload.mutationId === 'string'
      && typeof payload.reason === 'string'
      && (payload.queueVersion === undefined || payload.queueVersion === null || typeof payload.queueVersion === 'number')
  }
  if (type === 'rooms.created') return isRoomSummary(payload)
  if (type === 'user.me') {
    return typeof payload.publicId === 'string'
      && typeof payload.name === 'string'
      && typeof payload.isGuest === 'boolean'
      && typeof payload.role === 'string'
      && typeof payload.isAdmin === 'boolean'
  }
  if (type === 'sync.pong') {
    return ['clientSendTime', 'serverReceiveTime', 'serverSendTime'].every(key => typeof payload[key] === 'number')
      && typeof payload.pingId === 'string'
  }
  if (type === 'player.events') {
    return typeof payload.code === 'string'
      && ['info', 'success', 'warning', 'error'].includes(String(payload.severity))
      && typeof payload.message === 'string'
  }
  if (type === 'player.failed') {
    return ['queueId', 'musicId', 'name', 'platform', 'message'].every(key => typeof payload[key] === 'string')
      && ['preview_only', 'unavailable', 'resolve_failed', 'unknown'].includes(String(payload.reason))
  }
  if (type === 'queue.patch') {
    if (typeof payload.queueVersion !== 'number' || typeof payload.operation !== 'string') return false
    if (!['append', 'remove', 'move', 'snapshot', 'status', 'clear'].includes(payload.operation)) return false
    if (payload.queueEpoch !== undefined && typeof payload.queueEpoch !== 'string') return false
    if (payload.items !== undefined && (!Array.isArray(payload.items) || !payload.items.every(isQueueItem))) return false
    if (payload.queue !== undefined && (!Array.isArray(payload.queue) || !payload.queue.every(isQueueItem))) return false
    if (payload.queueIds !== undefined && (!Array.isArray(payload.queueIds) || !payload.queueIds.every(id => typeof id === 'string'))) return false
    if (payload.queueId !== undefined && typeof payload.queueId !== 'string') return false
    if (payload.targetQueueId !== undefined && typeof payload.targetQueueId !== 'string') return false
    if (payload.position !== undefined && !['before', 'after'].includes(String(payload.position))) return false
    if (payload.status !== undefined && typeof payload.status !== 'string') return false
    if (payload.item !== undefined && !isPartialQueueItem(payload.item)) return false
    return true
  }
  if (type === 'users.online') {
    return typeof payload.roomId === 'string'
      && typeof payload.revision === 'number'
      && Number.isSafeInteger(payload.revision)
      && payload.revision > 0
      && Array.isArray(payload.users)
      && payload.users.every(isUserSummary)
  }
  return true
}

export function decodeServerEnvelope(raw: string): ServerEnvelope | null {
  let parsed: unknown
  try {
    parsed = JSON.parse(raw) as unknown
  } catch {
    return null
  }
  if (!isRecord(parsed) || typeof parsed.type !== 'string' || !SERVER_TYPES.has(parsed.type as ServerEnvelope['type'])) {
    return null
  }
  const type = parsed.type as ServerEnvelope['type']
  if (!('payload' in parsed) || !validPayload(type, parsed.payload)) return null
  if (parsed.requestId !== undefined && parsed.requestId !== null && typeof parsed.requestId !== 'string') return null
  if (parsed.roomId !== undefined && parsed.roomId !== null && typeof parsed.roomId !== 'string') return null
  return parsed as unknown as ServerEnvelope
}
