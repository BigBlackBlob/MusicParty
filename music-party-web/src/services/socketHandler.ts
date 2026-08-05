import type { ChatMessage, PlayerEvent, PresenceSnapshot, RoomSummary } from '../contracts/generated/models'
import type { RealtimeCallbacks, RealtimeHandlers } from '../transport/realtimeClient'
import { useToast } from '../composables/useToast'
import { WS_DEST } from '../constants/api'
import { realtimeClient } from '../transport/realtimeClient'

type PlayerStatePayload = Parameters<NonNullable<RealtimeHandlers['player.state']>>[0]
type QueuePatchPayload = Parameters<NonNullable<RealtimeHandlers['queue.patch']>>[0]
type SyncPongPayload = Parameters<NonNullable<RealtimeHandlers['sync.pong']>>[0]

interface PlayerHandlers {
  scheduleSyncState?: (state: PlayerStatePayload) => void
  syncState?: (state: PlayerStatePayload) => void
  handleSyncPong?: (pong: SyncPongPayload) => void
  scheduleQueuePatch?: (patch: QueuePatchPayload) => void
  applyQueuePatch?: (patch: QueuePatchPayload) => void
  settleQueueReorder?: (mutationId: string, accepted: boolean, reason?: string) => void
  settleQueueMutation?: (mutationId: string, accepted: boolean, reason?: string) => void
  settleEnqueue?: (mutationId: string, accepted: boolean, reason?: string) => void
  switchRoom?: (roomId: string) => void
}

interface UserHandlers {
  bindings?: Record<string, string>
  resolveName?: (publicId: string) => string
  resetAuthentication?: () => void
}

interface PresenceHandlers {
  apply: (snapshot: PresenceSnapshot, activeRoomId: string) => boolean
  resolveName?: (publicId: string, fallbackName?: string) => string
}

interface ChatHandlers {
  messages: ChatMessage[]
  publicMessages: ChatMessage[]
  addMessage?: (message: ChatMessage) => void
  addPublicMessage?: (message: ChatMessage) => void
  setHistory?: (messages: ChatMessage[]) => void
  prependHistory?: (messages: ChatMessage[]) => void
  setPublicHistory?: (messages: ChatMessage[]) => void
  prependPublicHistory?: (messages: ChatMessage[]) => void
  resetRoomMessages?: () => void
}

interface RoomHandlers {
  currentRoomId: string
  rooms: RoomSummary[]
  transition?: { status: string; roomId?: string }
  commitTransition?: (roomId: string) => void
  setRooms?: (rooms: RoomSummary[]) => void
  setCurrentRoom?: (roomId: string) => void
  clearRoomAccess?: (roomId: string) => void
  failTransition?: (roomId: string, error: Error) => void
}

interface SocketHandlerStores {
  player?: PlayerHandlers
  userStore?: UserHandlers
  presence?: PresenceHandlers
  chatStore?: ChatHandlers
  roomStore?: RoomHandlers
  loadRoomPlaylists?: () => void
  setShowNameModal?: (visible: boolean) => void
  resetRoomMessages?: () => void
  setCurrentRoom?: (roomId: string) => void
  reconnectToCurrentRoom?: () => void
  resolveName?: (publicId: string) => string
}

interface SocketCallbackStores {
  userStore?: UserHandlers
  roomStore?: RoomHandlers
  setConnected?: (connected: boolean) => void
  resetSyncGate?: () => void
  requestPing?: (reason: string, immediate: boolean) => void
  requestResync?: (reason: string, immediate: boolean) => void
  requestPresence?: () => void
  requestChatHistory?: (reset: boolean) => void
  requestPublicChatHistory?: (reset: boolean) => void
  bindAccount?: (platform: string, id: string) => void
  startHeartbeat?: () => void
  stopHeartbeat?: () => void
}

function handleGameEvent(event: PlayerEvent, stores: SocketHandlerStores): void {
  const { userStore, chatStore, loadRoomPlaylists, setShowNameModal, resetRoomMessages, setCurrentRoom, reconnectToCurrentRoom, resolveName } = stores
  const { show, error } = useToast()
  const actorName = event.actor?.name || (event.actor?.publicId
    ? (resolveName?.(event.actor.publicId) || userStore?.resolveName?.(event.actor.publicId))
    : '系统')

  switch (event.code) {
    case 'RESET': chatStore?.resetRoomMessages?.(); break
    case 'ROOM_PASSWORD_CHANGED':
      error('房间密码已更改，请重新验证')
      reconnectToCurrentRoom?.()
      return
    case 'ROOM_DELETED':
      error(event.message || '房间已被删除，已返回 Lounge')
      setCurrentRoom?.('lounge')
      reconnectToCurrentRoom?.()
      resetRoomMessages?.()
      return
    case 'RENAME_FAILED':
      error(event.message || '该名称已被占用，请更换。')
      setShowNameModal?.(true)
      return
    case 'ROOM_PLAYLISTS_UPDATED': loadRoomPlaylists?.(); return
    case 'CONTROL_DENIED':
      if (event.metadata?.requiresDisplayName === true) setShowNameModal?.(true)
      break
    case 'USER_JOIN':
    case 'USER_LEAVE':
    case 'PLAY_START': return
    default: break
  }

  const titleMap: Partial<Record<PlayerEvent['code'], string>> = {
    ERROR_LOAD: '播放错误', SYSTEM_MESSAGE: '系统消息', CONTROL_DENIED: '操作未生效',
    SEEK_DENIED: '无法调整进度', RESET: '房间已重置', REMOVE: '队列已更新', LIKE: '收到回应'
  }
  show({
    title: titleMap[event.code] || '房间通知',
    message: event.message || `${actorName} ${event.code}`,
    type: event.severity,
    duration: 3000
  })
}

export function createSocketHandlers(stores: SocketHandlerStores = {}): RealtimeHandlers {
  const { player, userStore, presence, chatStore, roomStore } = stores
  return {
    'player.state': (state, envelope) => {
      const activeRoomId = roomStore?.transition?.status === 'connecting' ? roomStore.transition.roomId : roomStore?.currentRoomId
      if (envelope.roomId && envelope.roomId !== activeRoomId) return
      if (roomStore?.transition?.status === 'connecting' && activeRoomId) roomStore.commitTransition?.(activeRoomId)
      ;(player?.scheduleSyncState || player?.syncState)?.(state)
    },
    'sync.pong': pong => player?.handleSyncPong?.(pong),
    'users.online': (snapshot, envelope) => {
      const activeRoomId = roomStore?.transition?.status === 'connecting' ? roomStore.transition.roomId : roomStore?.currentRoomId
      if (!activeRoomId || envelope.roomId !== activeRoomId || snapshot.roomId !== activeRoomId) return
      presence?.apply(snapshot, activeRoomId)
    },
    'queue.patch': (patch, envelope) => {
      const activeRoomId = roomStore?.transition?.status === 'connecting' ? roomStore.transition.roomId : roomStore?.currentRoomId
      if (envelope.roomId && envelope.roomId !== activeRoomId) return
      ;(player?.scheduleQueuePatch || player?.applyQueuePatch)?.(patch)
    },
    'queue.reorder.ack': data => player?.settleQueueReorder?.(data.mutationId, true),
    'queue.reorder.nack': data => player?.settleQueueReorder?.(data.mutationId, false, data.reason),
    'queue.mutation.ack': data => player?.settleQueueMutation?.(data.mutationId, true),
    'queue.mutation.nack': data => player?.settleQueueMutation?.(data.mutationId, false, data.reason),
    'enqueue.ack': data => player?.settleEnqueue?.(data.mutationId, true),
    'enqueue.nack': data => player?.settleEnqueue?.(data.mutationId, false, data.reason),
    'player.events': event => handleGameEvent(event, stores),
    'chat.message': message => chatStore?.addMessage?.(message),
    'public-chat.message': message => chatStore?.addPublicMessage?.(message),
    'rooms.list': rooms => roomStore?.setRooms?.(rooms),
    'chat.history': messages => {
      if (!chatStore) return
      if (chatStore.messages.length === 0) chatStore.setHistory?.(messages)
      else chatStore.prependHistory?.(messages)
    },
    'public-chat.history': messages => {
      if (!chatStore) return
      if (chatStore.publicMessages.length === 0) chatStore.setPublicHistory?.(messages)
      else chatStore.prependPublicHistory?.(messages)
    },
    'rooms.created': room => {
      if (!roomStore || !player?.switchRoom) return
      roomStore.setRooms?.([...roomStore.rooms.filter(item => item.roomId !== room.roomId), room])
      player.switchRoom(room.roomId)
    }
  }
}

export function createSocketCallbacks(stores: SocketCallbackStores = {}): RealtimeCallbacks {
  const { userStore, roomStore, setConnected, resetSyncGate, requestPing, requestResync, requestPresence, requestChatHistory, requestPublicChatHistory, bindAccount, startHeartbeat, stopHeartbeat } = stores
  return {
    onConnect: () => {
      setConnected?.(true)
      resetSyncGate?.()
      requestResync?.('connect', true)
      requestPresence?.()
      startHeartbeat?.()
      realtimeClient.send(WS_DEST.USER_ME)
      requestPing?.('connect', true)
      setTimeout(() => {
        requestChatHistory?.(true)
        requestPublicChatHistory?.(true)
      }, 300)
      Object.entries(userStore?.bindings || {}).forEach(([platform, id]) => {
        if (id) bindAccount?.(platform, id)
      })
    },
    onDisconnect: () => { setConnected?.(false); stopHeartbeat?.() },
    onRoomAccessRejected: () => {
      const roomId = roomStore?.currentRoomId
      if (!roomId) return
      roomStore.clearRoomAccess?.(roomId)
      roomStore.failTransition?.(roomId, new Error('Room access expired'))
    },
    onSessionRejected: () => userStore?.resetAuthentication?.()
  }
}
