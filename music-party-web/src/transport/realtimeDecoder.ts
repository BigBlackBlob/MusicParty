import type { ServerEnvelope } from '../contracts/generated/websocket'

const SERVER_TYPES = new Set<ServerEnvelope['type']>([
  'chat.history', 'chat.message', 'enqueue.ack', 'enqueue.nack', 'player.events',
  'player.state', 'public-chat.history', 'public-chat.message', 'queue.mutation.ack',
  'queue.mutation.nack', 'queue.patch', 'queue.reorder.ack', 'queue.reorder.nack',
  'rooms.created', 'rooms.list', 'sync.pong', 'user.me', 'users.online'
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

function validPayload(type: ServerEnvelope['type'], payload: unknown): boolean {
  if (['chat.history', 'public-chat.history', 'rooms.list'].includes(type)) {
    return Array.isArray(payload)
  }
  if (!isRecord(payload)) return false
  if (type === 'player.state') {
    return typeof payload.stateVersion === 'number'
      && typeof payload.queueVersion === 'number'
      && typeof payload.playEpoch === 'number'
  }
  if (type === 'player.events') {
    return typeof payload.code === 'string'
      && ['info', 'success', 'warning', 'error'].includes(String(payload.severity))
      && typeof payload.message === 'string'
  }
  if (type === 'queue.patch') return typeof payload.queueVersion === 'number' && typeof payload.operation === 'string'
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
