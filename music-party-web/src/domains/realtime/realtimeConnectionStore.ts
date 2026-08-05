import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ConnectionState } from '../../transport/realtimeClient'

export const useRealtimeConnectionStore = defineStore('realtime-connection', () => {
  const connected = ref(false)
  const hasInitialSnapshot = ref(false)
  const state = ref<ConnectionState>('idle')
  const lastPingSentAt = ref(0)
  const lastPongAt = ref(0)
  const lastResyncSentAt = ref(0)
  const lastRttMs = ref<number | null>(null)

  function reset(): void {
    connected.value = false
    hasInitialSnapshot.value = false
    state.value = 'idle'
    lastPingSentAt.value = 0
    lastPongAt.value = 0
    lastResyncSentAt.value = 0
    lastRttMs.value = null
  }

  return {
    connected, hasInitialSnapshot, state,
    lastPingSentAt, lastPongAt, lastResyncSentAt, lastRttMs,
    reset
  }
})
