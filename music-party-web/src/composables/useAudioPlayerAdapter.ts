import { reactive } from 'vue'
import { useAudioPlaybackStore } from '../domains/playback/audioPlaybackStore'
import { useRoomCommandStore } from '../domains/realtime/roomCommandStore'
import { useRoomRealtimeCoordinator } from '../domains/realtime/roomRealtimeCoordinator'
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore'
import { useRoomStateSynchronizer } from '../domains/realtime/roomStateSynchronizer'
import { realtimeClient } from '../transport/realtimeClient'

// Narrow adapter for the existing audio recovery engine. It contains no state;
// every property delegates to its authoritative domain store.
export function useAudioPlayerAdapter() {
  const runtime = useRoomRuntimeStore()
  const audio = useAudioPlaybackStore()
  const commands = useRoomCommandStore()
  const coordinator = useRoomRealtimeCoordinator()
  const synchronizer = useRoomStateSynchronizer()

  return reactive({
    get nowPlaying() { return runtime.nowPlaying },
    get queue() { return runtime.queue },
    get isPaused() { return runtime.isPaused },
    get isBuffering() { return audio.isBuffering },
    set isBuffering(value: boolean) { audio.isBuffering = value },
    get bufferedMs() { return audio.bufferedMs },
    set bufferedMs(value: number) { audio.bufferedMs = value },
    get isErrorState() { return audio.isErrorState },
    set isErrorState(value: boolean) { audio.isErrorState = value },
    get isSeekingPreview() { return audio.isSeekingPreview },
    get forceNextSyncSeek() { return audio.forceNextSyncSeek },
    set forceNextSyncSeek(value: boolean) { audio.forceNextSyncSeek = value },
    setPlaybackPosition: audio.setPlaybackPosition,
    getCurrentProgress: synchronizer.getCurrentProgress,
    serverNow: () => Date.now() + runtime.serverClockOffset,
    togglePause: commands.togglePause,
    playNext: commands.playNext,
    playPrevious: commands.playPrevious,
    tryReconnect: coordinator.tryReconnect,
    requestSyncRefresh: coordinator.requestSyncRefresh,
    reportPlayback: (type: 'playback.started' | 'playback.stalled' | 'playback.ended', payload: { playEpoch: number; queueId: string; positionMs?: number; clientTime?: number; durationMs?: number }) => {
      if (type === 'playback.started') return realtimeClient.send(type, { playEpoch: payload.playEpoch, queueId: payload.queueId, positionMs: payload.positionMs ?? 0, clientTime: payload.clientTime ?? Date.now(), durationMs: payload.durationMs ?? 0 })
      if (type === 'playback.stalled') return realtimeClient.send(type, { playEpoch: payload.playEpoch, queueId: payload.queueId, positionMs: payload.positionMs ?? 0 })
      return realtimeClient.send(type, { playEpoch: payload.playEpoch, queueId: payload.queueId })
    },
  })
}
