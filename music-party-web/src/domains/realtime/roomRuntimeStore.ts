import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { NowPlaying, QueueItem } from '../../contracts/generated/models'
import type { QueuePatchServerEnvelope } from '../../contracts/generated/websocket'

type QueuePatch = QueuePatchServerEnvelope['payload']
export type QueueUpdateResult = 'applied' | 'stale' | 'version-gap' | 'invalid'

function reorderQueue(queue: QueueItem[], patch: QueuePatch): QueueItem[] | null {
  if (!patch.queueId || !patch.targetQueueId) return null
  const snapshot = [...queue]
  const sourceIndex = snapshot.findIndex(item => item.queueId === patch.queueId)
  if (sourceIndex < 0) return null
  const [item] = snapshot.splice(sourceIndex, 1)
  if (!item) return null
  const targetIndex = snapshot.findIndex(candidate => candidate.queueId === patch.targetQueueId)
  if (targetIndex < 0) return null
  snapshot.splice(patch.position === 'after' ? targetIndex + 1 : targetIndex, 0, item)
  return snapshot
}

export const useRoomRuntimeStore = defineStore('room-runtime', () => {
  const nowPlaying = ref<NowPlaying | null>(null)
  const queue = ref<QueueItem[]>([])
  const isPaused = ref(false)
  const isShuffle = ref(false)
  const isPauseLocked = ref(false)
  const isSkipLocked = ref(false)
  const isShuffleLocked = ref(false)
  const isLoading = ref(false)
  const lastStateVersion = ref(0)
  const lastQueueVersion = ref(0)
  const lastPlayEpoch = ref(0)
  const lastServerTimestamp = ref(0)
  const remotePosition = ref(0)
  const lastSyncTime = ref(0)
  const serverClockOffset = ref(0)
  const hasClockSample = ref(false)

  function replaceQueue(nextQueue: QueueItem[], queueVersion: number, snapshot = false): QueueUpdateResult {
    if (!Number.isFinite(queueVersion) || queueVersion <= 0) return 'invalid'
    // A player.state snapshot is authoritative even when the preceding patch
    // advanced to the same version. This occurs when the first enqueue begins
    // playback: its append patch contains the new item while the state snapshot
    // correctly removes that item from the pending queue.
    if (queueVersion < lastQueueVersion.value || (queueVersion === lastQueueVersion.value && !snapshot)) return 'stale'
    if (!snapshot && lastQueueVersion.value > 0 && queueVersion !== lastQueueVersion.value + 1) {
      return 'version-gap'
    }
    queue.value = nextQueue
    lastQueueVersion.value = queueVersion
    return 'applied'
  }

  function applyQueuePatch(patch: QueuePatch): QueueUpdateResult {
    if (patch.operation === 'snapshot') {
      return replaceQueue(patch.queue ?? [], patch.queueVersion, true)
    }
    if (patch.queueVersion <= lastQueueVersion.value) return 'stale'
    if (patch.queueVersion !== lastQueueVersion.value + 1) return 'version-gap'

    let nextQueue: QueueItem[] | null = null
    if (patch.operation === 'append') {
      const existingIds = new Set(queue.value.map(item => item.queueId))
      nextQueue = [...queue.value, ...(patch.items ?? []).filter(item => !existingIds.has(item.queueId))]
    } else if (patch.operation === 'remove') {
      const removedIds = new Set(patch.queueIds ?? [])
      nextQueue = queue.value.filter(item => !removedIds.has(item.queueId))
    } else if (patch.operation === 'move') {
      nextQueue = reorderQueue(queue.value, patch)
    } else if (patch.operation === 'status') {
      if (!patch.queueId) return 'invalid'
      const itemIndex = queue.value.findIndex(item => item.queueId === patch.queueId)
      if (itemIndex < 0) return 'invalid'
      nextQueue = queue.value.map((item, index) => index === itemIndex
        ? { ...item, ...patch.item, status: patch.status ?? patch.item?.status ?? item.status }
        : item)
    } else if (patch.operation === 'clear') {
      nextQueue = []
    }
    if (!nextQueue) return 'invalid'
    queue.value = nextQueue
    lastQueueVersion.value = patch.queueVersion
    return 'applied'
  }

  function reset(): void {
    nowPlaying.value = null
    queue.value = []
    isPaused.value = false
    isShuffle.value = false
    isPauseLocked.value = false
    isSkipLocked.value = false
    isShuffleLocked.value = false
    isLoading.value = false
    lastStateVersion.value = 0
    lastQueueVersion.value = 0
    lastPlayEpoch.value = 0
    lastServerTimestamp.value = 0
    remotePosition.value = 0
    lastSyncTime.value = 0
    serverClockOffset.value = 0
    hasClockSample.value = false
  }

  return {
    nowPlaying, queue, isPaused, isShuffle,
    isPauseLocked, isSkipLocked, isShuffleLocked, isLoading,
    lastStateVersion, lastQueueVersion, lastPlayEpoch, lastServerTimestamp,
    remotePosition, lastSyncTime, serverClockOffset, hasClockSample,
    replaceQueue, applyQueuePatch,
    reset
  }
})
