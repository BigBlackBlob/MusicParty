import { reactive } from 'vue'
import { useAudioPlaybackStore } from '../domains/playback/audioPlaybackStore'
import { useRoomCommandStore } from '../domains/realtime/roomCommandStore'
import { useRoomRealtimeCoordinator } from '../domains/realtime/roomRealtimeCoordinator'
import { useRoomRuntimeStore } from '../domains/realtime/roomRuntimeStore'
import { useRoomStateSynchronizer } from '../domains/realtime/roomStateSynchronizer'

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
    togglePause: commands.togglePause,
    playNext: commands.playNext,
    tryReconnect: coordinator.tryReconnect,
    requestSyncRefresh: coordinator.requestSyncRefresh,
  })
}
