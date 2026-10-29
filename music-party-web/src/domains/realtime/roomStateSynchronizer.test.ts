import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { PlayerStateServerEnvelope } from '../../contracts/generated/websocket'
import { realtimeClient } from '../../transport/realtimeClient'
import { useRoomCommandBus } from './roomCommandBus'
import { useRoomRuntimeStore } from './roomRuntimeStore'
import { useRoomStateSynchronizer } from './roomStateSynchronizer'

vi.mock('../../transport/realtimeClient', () => ({ realtimeClient: { send: vi.fn(() => true) } }))

const state = (queueEpoch: string, queueVersion: number): PlayerStateServerEnvelope['payload'] => ({
  queueEpoch, queueVersion, stateVersion: 0, playEpoch: 0, serverTimestamp: 1000,
  queue: [{ queueId: queueEpoch, status: 'PENDING',
    music: { id: 'track', name: 'Track', artists: [], duration: 0, platform: 'local', coverUrl: '' },
    enqueuedBy: { publicId: 'user', name: 'User', isGuest: false } }],
  isShuffle: false, isPaused: true, isPauseLocked: false, isSkipLocked: false,
  isShuffleLocked: false, isLoading: false, historyCursor: 0,
})

describe('runtime generation recovery', () => {
  beforeEach(() => { setActivePinia(createPinia()); vi.clearAllMocks() })

  it('replaces a high old version with a rebuilt snapshot and converges without resync', () => {
    const sync = useRoomStateSynchronizer()
    const runtime = useRoomRuntimeStore()
    const mutations = useRoomCommandBus()
    sync.syncState({ ...state('old', 90), stateVersion: 20 })
    mutations.beginMutation({ mutationId: 'old-drag', kind: 'reorder', timeoutMs: 1000, onTimeout: vi.fn() })
    sync.syncState(state('rebuilt', 1))
    expect(runtime.queueEpoch).toBe('rebuilt')
    expect(runtime.queue.map(item => item.queueId)).toEqual(['rebuilt'])
    expect(runtime.lastQueueVersion).toBe(1)
    expect(mutations.hasPendingMutations).toBe(false)
    sync.syncState({ ...state('old', 91), stateVersion: 21 })
    sync.syncState(state('rebuilt', 1))
    expect(runtime.queueEpoch).toBe('rebuilt')
    expect(runtime.queue.map(item => item.queueId)).toEqual(['rebuilt'])
    expect(realtimeClient.send).not.toHaveBeenCalled()
  })

  it('applies socket state while the document is hidden', () => {
    vi.useFakeTimers()
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'hidden' })
    try {
      const sync = useRoomStateSynchronizer()
      const runtime = useRoomRuntimeStore()
      sync.scheduleSyncState(state('hidden', 1))
      expect(runtime.queue).toEqual([])

      vi.runAllTimers()
      expect(runtime.queue.map(item => item.queueId)).toEqual(['hidden'])
    } finally {
      Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' })
      vi.useRealTimers()
    }
  })
})
