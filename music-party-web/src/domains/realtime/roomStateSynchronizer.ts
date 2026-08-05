import { storeToRefs } from 'pinia'
import { defineStore } from 'pinia'
import type { RealtimeHandlers } from '../../transport/realtimeClient'
import { useAudioPlaybackStore } from '../playback/audioPlaybackStore'
import { useRealtimeConnectionStore } from './realtimeConnectionStore'
import { useRoomCommandStore } from './roomCommandStore'
import { useRoomRuntimeStore } from './roomRuntimeStore'

type PlayerState = Parameters<NonNullable<RealtimeHandlers['player.state']>>[0]
type QueuePatch = Parameters<NonNullable<RealtimeHandlers['queue.patch']>>[0]
type SyncPong = Parameters<NonNullable<RealtimeHandlers['sync.pong']>>[0]

export const useRoomStateSynchronizer = defineStore('room-state-synchronizer', () => {
  const runtime = useRoomRuntimeStore()
  const connection = useRealtimeConnectionStore()
  const audio = useAudioPlaybackStore()
  const commands = useRoomCommandStore()
  const {
    nowPlaying, isPaused, isShuffle, isPauseLocked, isSkipLocked, isShuffleLocked, isLoading,
    lastStateVersion, lastPlayEpoch, lastServerTimestamp, remotePosition, lastSyncTime,
    serverClockOffset, hasClockSample,
  } = storeToRefs(runtime)
  const { hasInitialSnapshot, lastPongAt, lastRttMs } = storeToRefs(connection)
  const { forceNextSyncSeek } = storeToRefs(audio)
  let queuedState: PlayerState | null = null
  let queuedPatches: QueuePatch[] = []
  let stateFrame: number | ReturnType<typeof setTimeout> | null = null
  let patchFrame: number | ReturnType<typeof setTimeout> | null = null

  function getCurrentProgress(): number {
    if (!nowPlaying.value) return 0
    if (isPaused.value) return remotePosition.value
    return remotePosition.value + Date.now() + serverClockOffset.value - lastSyncTime.value
  }

  function resetSyncGate(): void {
    lastStateVersion.value = 0
    lastPlayEpoch.value = 0
    lastServerTimestamp.value = 0
    forceNextSyncSeek.value = true
    hasClockSample.value = false
  }

  function syncState(state: PlayerState | null): void {
    if (!state) return
    if (![state.stateVersion, state.queueVersion, state.playEpoch, state.serverTimestamp].every(Number.isFinite)) {
      commands.requestResync('player-state-invalid', true)
      return
    }
    if (state.stateVersion < lastStateVersion.value) return
    if (state.stateVersion === lastStateVersion.value && state.serverTimestamp < lastServerTimestamp.value) return
    lastStateVersion.value = state.stateVersion
    lastServerTimestamp.value = state.serverTimestamp
    if (state.playEpoch !== lastPlayEpoch.value) {
      lastPlayEpoch.value = state.playEpoch
      forceNextSyncSeek.value = true
    }

    nowPlaying.value = state.nowPlaying ?? null
    commands.setQueue(state.queue, state.queueVersion, { snapshot: true })
    isPaused.value = state.isPaused
    isShuffle.value = state.isShuffle
    isPauseLocked.value = state.isPauseLocked
    isSkipLocked.value = state.isSkipLocked
    isShuffleLocked.value = state.isShuffleLocked
    isLoading.value = state.isLoading

    const receivedAt = Date.now()
    if (!hasClockSample.value) {
      serverClockOffset.value = state.serverTimestamp - receivedAt
      hasClockSample.value = true
    }
    remotePosition.value = state.nowPlaying?.currentPosition ?? 0
    lastSyncTime.value = state.serverTimestamp
    if (!state.nowPlaying) audio.setPlaybackPosition(0)
    hasInitialSnapshot.value = true
  }

  const scheduleFrame = (callback: (timestamp: number) => void): number | ReturnType<typeof setTimeout> =>
    typeof requestAnimationFrame === 'function' ? requestAnimationFrame(callback) : setTimeout(() => callback(0), 0)

  function scheduleSyncState(state: PlayerState): void {
    queuedState = state
    if (stateFrame !== null) return
    stateFrame = scheduleFrame(() => {
      stateFrame = null
      const latest = queuedState
      queuedState = null
      syncState(latest)
    })
  }

  function scheduleQueuePatch(patch: QueuePatch): void {
    queuedPatches.push(patch)
    if (patchFrame !== null) return
    patchFrame = scheduleFrame(() => {
      patchFrame = null
      const patches = queuedPatches
      queuedPatches = []
      patches.forEach(commands.applyQueuePatch)
    })
  }

  function handleSyncPong(pong: SyncPong): void {
    const receivedAt = Date.now()
    const rtt = receivedAt - pong.clientSendTime
    if (rtt < 0 || rtt > 3000) return
    lastRttMs.value = rtt
    lastPongAt.value = receivedAt
    const sampleOffset = pong.serverSendTime + rtt / 2 - receivedAt
    if (hasClockSample.value && Math.abs(sampleOffset - serverClockOffset.value) > 10000) return
    serverClockOffset.value = hasClockSample.value ? serverClockOffset.value * 0.85 + sampleOffset * 0.15 : sampleOffset
    hasClockSample.value = true
  }

  function reset(): void {
    queuedState = null
    queuedPatches = []
    stateFrame = null
    patchFrame = null
    resetSyncGate()
  }

  return { getCurrentProgress, resetSyncGate, syncState, scheduleSyncState, scheduleQueuePatch, handleSyncPong, reset }
})
