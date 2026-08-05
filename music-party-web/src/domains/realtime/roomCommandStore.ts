import { storeToRefs } from 'pinia'
import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Music, QueueItem } from '../../contracts/generated/models'
import type { ClientEnvelope, QueuePatchServerEnvelope } from '../../contracts/generated/websocket'
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
type DeferredQueueUpdate = { queue: QueueItem[]; version: number; snapshot: boolean }
type EnqueueEnvelope = Extract<ClientEnvelope, { type: 'enqueue' | 'enqueue.album' | 'enqueue.playlist' }>
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

  function setQueue(nextQueue: QueueItem[], queueVersion: number, options: { snapshot?: boolean } = {}): boolean {
    if (!Number.isFinite(queueVersion)) {
      requestResync('queue-version-missing', true)
      return false
    }
    const update = { queue: nextQueue, version: queueVersion, snapshot: options.snapshot ?? false }
    if (mutationBus.hasMutationKind('reorder')) {
      deferredQueueUpdate = update
      return false
    }
    const result = runtime.replaceQueue(update.queue, update.version, update.snapshot)
    if (result === 'version-gap' || result === 'invalid') requestResync('queue-version-gap', true)
    return result === 'applied'
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
      deferredQueueUpdate = { queue: patch.queue ?? [], version: patch.queueVersion, snapshot: true }
      return false
    }
    const result = runtime.applyQueuePatch(patch)
    if (result === 'version-gap' || result === 'invalid') requestResync('queue-version-gap', true)
    return result === 'applied'
  }

  function applyDeferredQueueUpdate(): void {
    if (!deferredQueueUpdate) return
    runtime.replaceQueue(deferredQueueUpdate.queue, deferredQueueUpdate.version, deferredQueueUpdate.snapshot)
    deferredQueueUpdate = null
  }

  function settleQueueReorder(mutationId: string, accepted: boolean, reason = ''): void {
    if (!mutationBus.settleMutation(mutationId)) return
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

  function sendControl<T extends 'control.next' | 'control.toggle-pause' | 'control.toggle-shuffle' | 'control.seek'>(
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
  const togglePause = (): boolean => sendControl('control.toggle-pause', {})
  const toggleShuffle = (): boolean => sendControl('control.toggle-shuffle', {})
  const seek = (positionMs: number): boolean => sendControl('control.seek', { positionMs }, false)

  async function toggleLike(music: Music | null | undefined): Promise<void> {
    if (!music || !requireAuth()) return
    if (await likedSongs.toggle(music) === 'liked') realtimeClient.send('control.like', {})
  }

  function sendEnqueue(
    kind: Extract<QueueMutationKind, 'enqueue' | 'enqueue-album' | 'enqueue-playlist'>,
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

  function sendQueueMutation(
    kind: Extract<QueueMutationKind, 'top' | 'remove' | 'batch-top' | 'batch-remove'>,
    buildEnvelope: (mutationId: string) => QueueMutationEnvelope,
  ): boolean {
    const mutationId = mutationBus.createMutationId(kind)
    if (!realtimeClient.sendEnvelope(buildEnvelope(mutationId))) return false
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
    if (!realtimeClient.send('queue.reorder', payload)) {
      notifyFailure('队列排序没有发出，请等待连接恢复后再试', 'error')
      requestResync('queue-reorder-send-failed', true)
      return false
    }
    queue.value = applyQueueReorder(queue.value, { ...payload, queueId: queueId ?? null, targetQueueId: targetQueueId ?? null })
    mutationBus.beginMutation({
      mutationId, kind: 'reorder', timeoutMs: 1500,
      onTimeout: () => {
        applyDeferredQueueUpdate()
        requestResync('queue-reorder-timeout', true)
      },
    })
    return true
  }

  function reset(): void {
    mutationBus.reset()
    deferredQueueUpdate = null
    lastControlTime.value = 0
  }

  return {
    requireAuth,
    setQueue, applyQueuePatch, settleQueueReorder, settleQueueMutation, settleEnqueue,
    requestResync, playNext, togglePause, toggleShuffle, seek, toggleLike,
    enqueue, enqueuePlaylist, enqueueAlbum, topSong, removeSong, topSongs, removeSongs, reorderQueue,
    reset,
  }
})
