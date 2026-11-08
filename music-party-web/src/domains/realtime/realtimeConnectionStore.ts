import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { ConnectionState } from '../../transport/realtimeClient'

export const useRealtimeConnectionStore = defineStore('realtime-connection', () => {
  const connected = ref(false)
  const hasInitialSnapshot = ref(false)
  // True only after the current socket has applied an authoritative snapshot.
  const synced = ref(false)
  const state = ref<ConnectionState>('idle')
  const lastPingSentAt = ref(0)
  const lastPongAt = ref(0)
  const connectedAt = ref(0)
  const lastResyncSentAt = ref(0)
  const lastRttMs = ref<number | null>(null)
  // Users may only drag or skip while the room is live and synced; offline
  // commands are never queued for later.
  const canOperate = computed(() => connected.value && synced.value && state.value === 'connected')

  function setState(next: ConnectionState): void {
    state.value = next
    if (next === 'connected') return
    synced.value = false
    if (next === 'offline') connected.value = false
  }

  function reset(): void {
    connected.value = false
    hasInitialSnapshot.value = false
    synced.value = false
    state.value = 'idle'
    lastPingSentAt.value = 0
    lastPongAt.value = 0
    connectedAt.value = 0
    lastResyncSentAt.value = 0
    lastRttMs.value = null
  }

  return {
    connected, hasInitialSnapshot, synced, state, canOperate,
    lastPingSentAt, lastPongAt, connectedAt, lastResyncSentAt, lastRttMs,
    setState, reset
  }
})
