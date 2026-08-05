import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAudioPlaybackStore = defineStore('audio-playback', () => {
  const localProgress = ref(0)
  const playbackPositionMs = ref(0)
  const isBuffering = ref(false)
  const bufferedMs = ref(0)
  const isErrorState = ref(false)
  const isSeekingPreview = ref(false)
  const forceNextSyncSeek = ref(false)

  function setPlaybackPosition(positionMs: number): void {
    const nextPosition = Number.isFinite(positionMs) ? Math.max(0, positionMs) : 0
    playbackPositionMs.value = nextPosition
    localProgress.value = nextPosition
  }

  function reset(): void {
    localProgress.value = 0
    playbackPositionMs.value = 0
    isBuffering.value = false
    bufferedMs.value = 0
    isErrorState.value = false
    isSeekingPreview.value = false
    forceNextSyncSeek.value = false
  }

  return {
    localProgress, playbackPositionMs, isBuffering, bufferedMs,
    isErrorState, isSeekingPreview, forceNextSyncSeek, setPlaybackPosition, reset
  }
})
