import { storeToRefs } from 'pinia'
import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Music, QueueItem } from '../../contracts/generated/models'
import type { ClientEnvelope, ControlAckServerEnvelope, QueuePatchServerEnvelope } from '../../contracts/generated/websocket'
import { useToast } from '../../composables/useToast'
import { applyQueueReorder } from '../../utils/queueReorder'
import { realtimeClient } from '../../transport/realtimeClient'
import { useUserStore } from '../../stores/user'
import { useLikedSongsStore } from '../playback/likedSongs'
import { useRealtimeConnectionStore } from './realtimeConnectionStore'
import { useRoomCommandBus, type QueueMutationKind } from './roomCommandBus'
import { useRoomRuntimeStore } from './roomRuntimeStore'

type ClientType = ClientEnvelope['type']
type ClientPayload<T extends ClientType> = Extract<ClientEnvelope, { type: T }>['payload']
type QueuePatch = QueuePatchServerEnvelope['payload']
type DeferredQueueUpdate = { queue: QueueItem[]; version: number; snapshot: boolean; epoch: string }
type EnqueueEnvelope = Extract<ClientEnvelope, { type: 'enqueue' | 'enqueue.album' | 'enqueue.playlist' | 'enqueue.room-playlist' }>
type QueueMutationEnvelope = Extract<ClientEnvelope, { type: 'queue.top' | 'queue.remove' | 'queue.batch-top' | 'queue.batch-remove' }>

const localCooldownMs = 500

export const useRoomCommandStore = defineStore('room-commands', () => {
  const runtime = useRoomRuntimeStore()
  const connection = useRealtimeConnectionStore()
  const mutationBus = useRoomCommandBus()
  const user = useUserStore()
  const likedSongs = useLikedSongsStore()
  const { queue } = storeToRefs(runtime)
  const { connected, lastResyncSentAt } = storeToRefs(connection)
  const lastControlTime = ref(0)
  let lastAuthPromptAt = 0
  let deferredQueueUpdate: DeferredQueueUpdate | null = null
  let likeToggleInProgress = false
  const pendingLikes = new Map<string, { music: Music; result: 'liked' | 'unliked'; actorId: string; previousLiked: boolean; previousPersonalLiked: boolean }>()
  const retryableCommands = new Map<string, { envelope: ClientEnvelope; scopeId: string; actorId: string; socketGeneration: number; expiresAt: number; kind: 'pause' | 'like' | 'unlike' | 'reorder' }>()

  function notifyFailure(message: string, type: 'warning' | 'error' = 'warning'): void {
    useToast().show({ title: '操作未生效', message, type, duration: 2200 })
  }

  function requireAuth(): boolean {
    if (user.hasDisplayName) return true
    user.showNameModal = true
    const now = Date.now()
    if (now - lastAuthPromptAt > 1200) {
      lastAuthPromptAt = now
      notifyFailure('请先设置昵称再控制播放')
    }
    return false
  }

  function requestResync(reason = 'manual', force = false): boolean {
    const now = Date.now()
    if (!force && now - lastResyncSentAt.value < 500) return false
    lastResyncSentAt.value = now
    return realtimeClient.send('player.resync', {})
  }

  function setQueue(nextQueue: QueueItem[], queueVersion: number, options: { snapshot?: boolean; epoch?: string } = {}): boolean {
    if (!Number.isFinite(queueVersion)) {
      requestResync('queue-version-missing', true)
      return false
    }
    const update = { queue: nextQueue, version: queueVersion, snapshot: options.snapshot ?? false, epoch: options.epoch ?? runtime.queueEpoch }
    if (mutationBus.hasMutationKind('reorder')) {
      deferredQueueUpdate = update
      return false
    }
    const result = runtime.replaceQueue(update.queue, update.version, update.snapshot)
    if (result === 'version-gap' || result === 'invalid') requestResync('queue-version-gap', true)
    return result === 'applied'
  }

  function resetQueueGeneration(): void {
    deferredQueueUpdate = null
    mutationBus.reset()
    for (const pending of pendingLikes.values()) {
      rollbackLike(pending.music, pending.actorId, pending.previousLiked, pending.previousPersonalLiked)
    }
    pendingLikes.clear()
    retryableCommands.clear()
  }

  function sendRetryableCommand(
    mutationId: string,
    kind: 'pause' | 'like' | 'unlike' | 'reorder',
    envelope: ClientEnvelope,
    timeoutMs: number,
    onTimeout: () => void,
  ): boolean {
    const scopeId = runtime.idempotencyScopeId
    if (!scopeId || !realtimeClient.sendEnvelope(envelope)) return false
    const ttlMs = runtime.idempotencyTtlMs > 0 ? Math.min(timeoutMs, runtime.idempotencyTtlMs) : timeoutMs
    const retryWindowMs = Math.max(0, ttlMs - 1000)
    retryableCommands.set(mutationId, {
      envelope, scopeId, actorId: user.publicId, socketGeneration: realtimeClient.generation,
      expiresAt: Date.now() + retryWindowMs, kind,
    })
    mutationBus.beginMutation({
      mutationId, kind, timeoutMs: retryWindowMs,
      onTimeout: () => {
        retryableCommands.delete(mutationId)
        onTimeout()
      },
    })
    return true
  }

  function retryPendingCommands(scopeId: string): void {
    if (!scopeId || !connected.value || !realtimeClient.connected) return
    for (const [mutationId, pending] of retryableCommands) {
      if (pending.scopeId !== scopeId || pending.actorId !== user.publicId) {
        mutationBus.settleMutation(mutationId)
        retryableCommands.delete(mutationId)
        pendingLikes.delete(mutationId)
        continue
      }
      if (pending.socketGeneration === realtimeClient.generation || Date.now() >= pending.expiresAt) continue
      if (realtimeClient.sendEnvelope(pending.envelope)) pending.socketGeneration = realtimeClient.generation
    }
  }

  function settleControl(ack: ControlAckServerEnvelope['payload']): void {
    if (!ack.mutationId) return
    const pending = retryableCommands.get(ack.mutationId)
    if (!pending || pending.kind !== 'pause') return
    retryableCommands.delete(ack.mutationId)
    mutationBus.settleMutation(ack.mutationId)
    if (ack.outcome === 'rejected') {
      notifyFailure(ack.code || '播放控制未生效，请重试', 'error')
      requestResync('pause-rejected', true)
    }
  }

  function applyQueuePatch(patch: QueuePatch): boolean {
    if (!Number.isFinite(patch.queueVersion)) {
      requestResync('queue-patch-invalid', true)
      return false
    }
    if (mutationBus.hasMutationKind('reorder')) {
      if (patch.operation !== 'snapshot') {
        requestResync('queue-patch-during-mutation', true)
        return false
      }
      deferredQueueUpdate = { queue: patch.queue ?? [], version: patch.queueVersion, snapshot: true, epoch: patch.queueEpoch ?? runtime.queueEpoch }
      return false
    }
    const result = runtime.applyQueuePatch(patch)
    if (result === 'version-gap' || result === 'invalid') requestResync('queue-version-gap', true)
    return result === 'applied'
  }

  function applyDeferredQueueUpdate(): void {
    if (!deferredQueueUpdate) return
    if (deferredQueueUpdate.epoch === runtime.queueEpoch) runtime.replaceQueue(deferredQueueUpdate.queue, deferredQueueUpdate.version, deferredQueueUpdate.snapshot)
    deferredQueueUpdate = null
  }

  function settleQueueReorder(mutationId: string, accepted: boolean, reason = ''): void {
    if (!mutationBus.settleMutation(mutationId)) return
    retryableCommands.delete(mutationId)
    if (!accepted) {
      deferredQueueUpdate = null
      notifyFailure(reason === 'RATE_LIMITED' ? '队列操作太快了，请稍后重试' : '队列已更新，请重试排序')
      requestResync('queue-reorder-nack', true)
      return
    }
    if (!mutationBus.hasMutationKind('reorder')) applyDeferredQueueUpdate()
  }

  function settleQueueMutation(mutationId: string, accepted: boolean, reason = ''): void {
    if (!mutationBus.settleMutation(mutationId) || accepted) return
    notifyFailure(reason === 'RATE_LIMITED' ? '队列操作太快了，请稍后重试' : '队列操作未生效，请重试')
    requestResync('queue-mutation-nack', true)
  }

  function settleEnqueue(mutationId: string, accepted: boolean, reason = ''): void {
    if (!mutationBus.settleMutation(mutationId) || accepted) return
    notifyFailure(reason || '点歌失败，请重试', 'error')
    requestResync('enqueue-nack', true)
  }

  function checkCooldown(): boolean {
    const now = Date.now()
    if (now - lastControlTime.value < localCooldownMs) {
      notifyFailure('操作太快了，稍等一下再试')
      return false
    }
    lastControlTime.value = now
    return true
  }

  function sendControl<T extends 'control.next' | 'control.previous' | 'control.toggle-pause' | 'control.toggle-shuffle' | 'control.seek'>(
    destination: T,
    payload: ClientPayload<T>,
    cooldown = true,
  ): boolean {
    if (!requireAuth()) return false
    if (!connected.value || !realtimeClient.connected) {
      notifyFailure('播放服务尚未连接，请稍后重试', 'error')
      requestResync('control-disconnected', true)
      return false
    }
    if (cooldown && !checkCooldown()) return false
    if (realtimeClient.sendEnvelope({ type: destination, payload } as ClientEnvelope)) return true
    notifyFailure('指令没有发出，请等待连接恢复后再试', 'error')
    requestResync('control-send-failed', true)
    return false
  }

  const playNext = (): boolean => sendControl('control.next', {})
  const playPrevious = (): boolean => sendControl('control.previous', {})
  function togglePause(): boolean {
    if (mutationBus.hasMutationKind('pause')) return false
    if (!requireAuth()) return false
    if (!connected.value || !realtimeClient.connected) {
      notifyFailure('播放服务尚未连接，请稍后重试', 'error')
      return false
    }
    if (!runtime.idempotencyScopeId) {
      notifyFailure('播放服务尚未同步，请稍后重试', 'error')
      return false
    }
    if (!checkCooldown()) return false
    const mutationId = mutationBus.createMutationId('pause')
    const envelope: ClientEnvelope = {
      type: 'control.toggle-pause',
      payload: { mutationId, idempotencyScopeId: runtime.idempotencyScopeId },
    }
    if (sendRetryableCommand(mutationId, 'pause', envelope, 60_000, () => {
      notifyFailure('播放控制确认超时，正在同步状态', 'error')
      requestResync('pause-timeout', true)
    })) return true
    notifyFailure('播放控制没有发出，请等待连接恢复后再试', 'error')
    return false
  }
  const toggleShuffle = (): boolean => sendControl('control.toggle-shuffle', {})
  const seek = (positionMs: number): boolean => sendControl('control.seek', { positionMs }, false)

  async function toggleLike(music: Music | null | undefined): Promise<void> {
    if (!music || !requireAuth()) return
    if (likeToggleInProgress || mutationBus.hasMutationKind('like') || mutationBus.hasMutationKind('unlike')) return
    if (!connected.value || !realtimeClient.connected || !runtime.idempotencyScopeId) {
      notifyFailure('播放服务尚未连接，请稍后重试', 'error')
      return
    }
    const scopeId = runtime.idempotencyScopeId
    const actorId = user.publicId
    likeToggleInProgress = true
    try {
      const previousLiked = runtime.nowPlaying?.music.id === music.id
        && runtime.nowPlaying.music.platform === music.platform
        && runtime.nowPlaying.likedUserIds.includes(actorId)
      const previousPersonalLiked = likedSongs.includes(music)
      const result = previousLiked ? 'unliked' : 'liked'
      const epoch = runtime.nowPlaying?.playEpoch ?? runtime.lastPlayEpoch
      if (!runtime.nowPlaying || epoch <= 0) {
        notifyFailure('当前曲目尚未同步，请稍后重试', 'error')
        return
      }
      if (result === 'liked') {
        likedSongs.add(music)
        runtime.nowPlaying.likedUserIds = [...new Set([...runtime.nowPlaying.likedUserIds, actorId])]
      } else {
        likedSongs.remove(`${music.platform}:${music.id}`)
        runtime.nowPlaying.likedUserIds = runtime.nowPlaying.likedUserIds.filter(id => id !== actorId)
      }
      const mutationId = mutationBus.createMutationId(result === 'liked' ? 'like' : 'unlike')
      const envelope: ClientEnvelope = {
        type: result === 'liked' ? 'control.like' : 'control.unlike',
        payload: { mutationId, idempotencyScopeId: scopeId, expectedPlayEpoch: epoch }
      }
      const sent = sendRetryableCommand(mutationId, result === 'liked' ? 'like' : 'unlike', envelope, 60_000, () => {
        const pending = pendingLikes.get(mutationId)
        pendingLikes.delete(mutationId)
        if (!pending) return
        rollbackLike(pending.music, pending.actorId, pending.previousLiked, pending.previousPersonalLiked)
        notifyFailure('红心确认超时，请稍后重试', 'error')
        requestResync('like-timeout', true)
      })
      if (!sent) {
        rollbackLike(music, actorId, previousLiked, previousPersonalLiked)
        notifyFailure('红心操作未发送，请稍后重试', 'error')
        return
      }
      pendingLikes.set(mutationId, { music, result, actorId, previousLiked, previousPersonalLiked })
    } finally {
      likeToggleInProgress = false
    }
  }

  function rollbackLike(music: Music, actorId: string, previousRoomLiked: boolean, previousPersonalLiked: boolean): void {
    if (user.publicId !== actorId) return
    if (previousPersonalLiked) likedSongs.add(music)
    else likedSongs.remove(`${music.platform}:${music.id}`)
    if (runtime.nowPlaying?.music.id === music.id && runtime.nowPlaying.music.platform === music.platform) {
      runtime.nowPlaying.likedUserIds = previousRoomLiked
        ? [...new Set([...runtime.nowPlaying.likedUserIds, actorId])]
        : runtime.nowPlaying.likedUserIds.filter(id => id !== actorId)
    }
  }

  function settleLike(ack: ControlAckServerEnvelope['payload']): void {
    if (!ack.mutationId) return
    const pending = pendingLikes.get(ack.mutationId)
    if (!pending) return
    pendingLikes.delete(ack.mutationId)
    retryableCommands.delete(ack.mutationId)
    mutationBus.settleMutation(ack.mutationId)
    if (ack.outcome === 'rejected') {
      rollbackLike(pending.music, pending.actorId, pending.previousLiked, pending.previousPersonalLiked)
      notifyFailure(ack.code || '红心操作未生效，请重试', 'error')
    }
  }

  function sendEnqueue(
    kind: Extract<QueueMutationKind, 'enqueue' | 'enqueue-album' | 'enqueue-playlist' | 'enqueue-room-playlist'>,
    buildEnvelope: (mutationId: string) => EnqueueEnvelope,
  ): boolean {
    if (!requireAuth()) return false
    if (!connected.value) {
      notifyFailure('连接已断开，请等待连接恢复后再点歌', 'error')
      return false
    }
    const mutationId = mutationBus.createMutationId(kind)
    if (!realtimeClient.sendEnvelope(buildEnvelope(mutationId))) {
      notifyFailure('点歌指令未发送，请等待连接恢复后再试', 'error')
      return false
    }
    mutationBus.beginMutation({
      mutationId, kind, timeoutMs: 3000,
      onTimeout: () => {
        notifyFailure('点歌确认超时，正在同步队列', 'error')
        requestResync('enqueue-timeout', true)
      },
    })
    return true
  }

  const enqueue = (platform: string, musicId: string): boolean => sendEnqueue('enqueue', mutationId => ({ type: 'enqueue', payload: { platform, musicId, mutationId } }))
  const enqueuePlaylist = (platform: string, playlistId: string): boolean => sendEnqueue('enqueue-playlist', mutationId => ({ type: 'enqueue.playlist', payload: { platform, playlistId, mutationId } }))
  const enqueueAlbum = (platform: string, albumId: string): boolean => sendEnqueue('enqueue-album', mutationId => ({ type: 'enqueue.album', payload: { platform, albumId, mutationId } }))
  const enqueueRoomPlaylist = (playlistId: string): boolean => sendEnqueue('enqueue-room-playlist', mutationId => ({ type: 'enqueue.room-playlist', payload: { playlistId, mutationId } }))

  function sendQueueMutation(
    kind: Extract<QueueMutationKind, 'top' | 'remove' | 'batch-top' | 'batch-remove'>,
    buildEnvelope: (mutationId: string) => QueueMutationEnvelope,
  ): boolean {
    const mutationId = mutationBus.createMutationId(kind)
    if (!realtimeClient.sendEnvelope(buildEnvelope(mutationId))) {
      notifyFailure('队列操作没有发出，请等待连接恢复后再试', 'error')
      requestResync('queue-mutation-send-failed', true)
      return false
    }
    mutationBus.beginMutation({ mutationId, kind, timeoutMs: 1500, onTimeout: () => requestResync('queue-mutation-timeout', true) })
    return true
  }

  const topSong = (queueId: string): boolean => requireAuth() && sendQueueMutation('top', mutationId => ({ type: 'queue.top', payload: { queueId, mutationId } }))
  const removeSong = (queueId: string): boolean => requireAuth() && sendQueueMutation('remove', mutationId => ({ type: 'queue.remove', payload: { queueId, mutationId } }))
  const topSongs = (queueIds: string[]): boolean => queueIds.length > 0 && requireAuth() && sendQueueMutation('batch-top', mutationId => ({ type: 'queue.batch-top', payload: { queueIds, mutationId } }))
  const removeSongs = (queueIds: string[]): boolean => queueIds.length > 0 && requireAuth() && sendQueueMutation('batch-remove', mutationId => ({ type: 'queue.batch-remove', payload: { queueIds, mutationId } }))

  function reorderQueue(oldIndex: number, newIndex: number, queueId?: string | null, targetQueueId?: string | null, position = 'before'): boolean {
    if (!requireAuth()) return false
    const mutationId = mutationBus.createMutationId('reorder')
    const payload = {
      oldIndex, newIndex, position, mutationId,
      ...(queueId ? { queueId } : {}),
      ...(targetQueueId ? { targetQueueId } : {}),
    }
    const envelope: ClientEnvelope = {
      type: 'queue.reorder',
      payload: { ...payload, mutationId },
    }
    if (!sendRetryableCommand(mutationId, 'reorder', envelope, 60_000, () => {
      applyDeferredQueueUpdate()
      requestResync('queue-reorder-timeout', true)
    })) {
      notifyFailure('队列排序没有发出，请等待连接恢复后再试', 'error')
      requestResync('queue-reorder-send-failed', true)
      return false
    }
    queue.value = applyQueueReorder(queue.value, { ...payload, queueId: queueId ?? null, targetQueueId: targetQueueId ?? null })
    return true
  }

  function reset(): void {
    resetQueueGeneration()
    lastControlTime.value = 0
  }

  return {
    requireAuth,
    resetQueueGeneration, retryPendingCommands, setQueue, applyQueuePatch, settleQueueReorder, settleQueueMutation, settleEnqueue,
    settleControl,
    requestResync, playNext, playPrevious, togglePause, toggleShuffle, seek, toggleLike, settleLike,
    enqueue, enqueuePlaylist, enqueueAlbum, enqueueRoomPlaylist, topSong, removeSong, topSongs, removeSongs, reorderQueue,
    reset,
  }
})
