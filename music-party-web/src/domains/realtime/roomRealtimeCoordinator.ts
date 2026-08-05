import { storeToRefs } from 'pinia'
import { defineStore } from 'pinia'
import { watch } from 'vue'
import { useRoomPlaylistsStore } from '../../stores/roomPlaylists'
import { useChatStore } from '../../stores/chat'
import { useRoomStore } from '../../stores/room'
import { useUserStore } from '../../stores/user'
import { createSocketCallbacks, createSocketHandlers } from '../../services/socketHandler'
import { realtimeClient } from '../../transport/realtimeClient'
import { shouldForceSocketReconnect } from '../../utils/socketHealth'
import { useAudioPlaybackStore } from '../playback/audioPlaybackStore'
import { useLikedSongsStore } from '../playback/likedSongs'
import { useLyricsStore } from '../playback/lyrics'
import { useRealtimeConnectionStore } from './realtimeConnectionStore'
import { useRoomCommandStore } from './roomCommandStore'
import { useRoomRuntimeStore } from './roomRuntimeStore'
import { useRoomPresenceStore } from './roomPresenceStore'
import { useRoomStateSynchronizer } from './roomStateSynchronizer'

export const ROOM_SWITCH_PASSWORD_REQUIRED = 'PASSWORD_REQUIRED'

export class RoomPasswordRequiredError extends Error {
  readonly code = ROOM_SWITCH_PASSWORD_REQUIRED
  constructor(readonly roomId: string) {
    super(ROOM_SWITCH_PASSWORD_REQUIRED)
    this.name = 'RoomPasswordRequiredError'
  }
}

export const useRoomRealtimeCoordinator = defineStore('room-realtime-coordinator', () => {
  const runtime = useRoomRuntimeStore()
  const presence = useRoomPresenceStore()
  const connection = useRealtimeConnectionStore()
  const audio = useAudioPlaybackStore()
  const commands = useRoomCommandStore()
  const synchronizer = useRoomStateSynchronizer()
  const lyrics = useLyricsStore()
  const likedSongs = useLikedSongsStore()
  const chat = useChatStore()
  const user = useUserStore()
  const room = useRoomStore()
  const { connected, lastPingSentAt, lastPongAt, lastResyncSentAt } = storeToRefs(connection)
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let pingTimer: ReturnType<typeof setInterval> | null = null
  let switchOriginRoomId: string | null = null

  watch(
    () => {
      const music = runtime.nowPlaying?.music
      return music ? `${music.platform}:${music.id}` : ''
    },
    () => { void lyrics.load(runtime.nowPlaying?.music) },
    { immediate: true },
  )

  function requestPing(reason = 'manual', force = false): boolean {
    const now = Date.now()
    if (!force && now - lastPingSentAt.value < 1000) return false
    lastPingSentAt.value = now
    return realtimeClient.send('sync.ping', {
      pingId: `${reason}-${now}-${Math.random().toString(36).slice(2, 8)}`,
      clientSendTime: now,
    })
  }

  function requestResync(_reason = 'manual', force = false): boolean {
    const now = Date.now()
    if (!force && now - lastResyncSentAt.value < 500) return false
    lastResyncSentAt.value = now
    return realtimeClient.send('player.resync', {})
  }

  // Presence has its own revision stream. Refresh it alongside a player
  // resync so a reconnect never leaves an intentionally reset store empty.
  function requestPresence(): boolean {
    return realtimeClient.send('users.online', {})
  }

  function requestSyncRefresh(reason = 'refresh', force = false): void {
    if (!connected.value) return
    requestPing(reason, force)
    requestResync(reason, force)
    requestPresence()
  }

  function startHeartbeat(): void {
    if (pingTimer) return
    const interval = typeof document !== 'undefined' && document.hidden ? 60_000 : 10_000
    pingTimer = setInterval(() => requestPing('interval'), interval)
  }

  function stopHeartbeat(): void {
    if (!pingTimer) return
    clearInterval(pingTimer)
    pingTimer = null
  }

  function requestChatHistory(initial = false): void {
    realtimeClient.send('chat.history.fetch', { offset: initial ? 0 : chat.messages.length, limit: 50 })
  }

  function requestPublicChatHistory(initial = false): void {
    if (initial && chat.publicMessages.length > 0) return
    realtimeClient.send('public-chat.history.fetch', { offset: initial ? 0 : chat.publicMessages.length, limit: 50 })
  }

  function bindAccount(platform: string, accountId: string): void {
    realtimeClient.send('user.bind', { platform, accountId })
    user.updateBinding(platform, accountId)
  }

  function renameUser(newName: string): void {
    realtimeClient.send('user.rename', { newName })
  }

  function resetRoomState(): void {
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = null
    commands.reset()
    stopHeartbeat()
    runtime.reset()
    connection.reset()
    audio.reset()
    lyrics.reset()
    synchronizer.reset()
    chat.resetRoomMessages()
    presence.reset()
  }

  function connect(targetRoomId = room.currentRoomId): void {
    presence.reset(targetRoomId)
    synchronizer.resetSyncGate()
    const handlers = createSocketHandlers({
      player: {
        syncState: (state) => {
          synchronizer.syncState(state)
          switchOriginRoomId = null
        },
        scheduleSyncState: synchronizer.scheduleSyncState,
        handleSyncPong: synchronizer.handleSyncPong,
        switchRoom,
        applyQueuePatch: commands.applyQueuePatch,
        scheduleQueuePatch: synchronizer.scheduleQueuePatch,
        settleQueueReorder: commands.settleQueueReorder,
        settleQueueMutation: commands.settleQueueMutation,
        settleEnqueue: commands.settleEnqueue,
      },
      userStore: user,
      presence,
      chatStore: chat,
      roomStore: room,
      loadRoomPlaylists: () => useRoomPlaylistsStore().loadPlaylists(),
      setShowNameModal: value => { user.showNameModal = value },
      resetRoomMessages: chat.resetRoomMessages,
      setCurrentRoom: room.setCurrentRoom,
      reconnectToCurrentRoom,
      resolveName: presence.resolveName,
    })
    handlers['user.me'] = (me) => {
      user.initUser(me.publicId, me.name, me.isGuest, me.role, me.isAdmin)
      likedSongs.syncFromServer(user.isGuest).catch(error => console.warn('Failed to sync liked songs', error))
    }

    const callbacks = createSocketCallbacks({
      setConnected: value => {
        connected.value = value
        if (value || room.transition.status !== 'connecting') return
        const failedRoomId = room.transition.roomId
        const fallbackRoomId = switchOriginRoomId
        room.failTransition(failedRoomId, new Error('Room connection closed before the initial snapshot'))
        switchOriginRoomId = null
        if (fallbackRoomId && fallbackRoomId !== failedRoomId) setTimeout(() => connect(fallbackRoomId), 100)
      },
      resetSyncGate: synchronizer.resetSyncGate,
      requestPing,
      requestResync,
      requestPresence,
      requestChatHistory,
      requestPublicChatHistory,
      bindAccount,
      startHeartbeat,
      stopHeartbeat,
      userStore: user,
      roomStore: room,
    })
    realtimeClient.connect({ 'room-id': targetRoomId }, callbacks, handlers)
  }

  function reconnectToCurrentRoom(): void {
    realtimeClient.disconnect()
    resetRoomState()
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect(room.currentRoomId)
    }, 100)
  }

  function disconnectForLogout(): void {
    realtimeClient.disconnect()
    resetRoomState()
  }

  async function switchRoom(roomId: string, password?: string): Promise<void> {
    if (!roomId || roomId === room.currentRoomId) return
    const targetRoom = room.rooms.find(candidate => candidate.roomId === roomId)
    if (targetRoom?.privateRoom) {
      if (password !== undefined) await room.verifyRoomAccess(roomId, password)
      else if (!room.hasValidRoomAccess(roomId)) throw new RoomPasswordRequiredError(roomId)
    }
    switchOriginRoomId = room.currentRoomId
    room.beginConnection(roomId)
    realtimeClient.disconnect()
    resetRoomState()
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect(roomId)
    }, 100)
  }

  function tryReconnect(): void {
    if (shouldForceSocketReconnect({ connected: connected.value, lastPongAt: lastPongAt.value })) realtimeClient.reconnectNow()
    else requestSyncRefresh('reconnect-check', true)
  }

  function handleVisibilityChange(): void {
    document.documentElement.toggleAttribute('data-page-hidden', document.hidden)
    if (!connected.value) return
    stopHeartbeat()
    startHeartbeat()
    if (!document.hidden) requestSyncRefresh('visibility-restored', true)
  }

  if (typeof document !== 'undefined') {
    document.documentElement.toggleAttribute('data-page-hidden', document.hidden)
    document.addEventListener('visibilitychange', handleVisibilityChange)
  }

  return {
    connect, tryReconnect, reconnectToCurrentRoom, switchRoom, resetRoomState, disconnectForLogout,
    requestPing, requestResync, requestSyncRefresh,
    requestPresence,
    requestChatHistory, requestPublicChatHistory, bindAccount, renameUser,
  }
})
