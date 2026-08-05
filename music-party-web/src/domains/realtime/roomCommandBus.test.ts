import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useRoomCommandBus } from './roomCommandBus'

describe('roomCommandBus', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })

  it('tracks and settles a pending mutation', () => {
    const bus = useRoomCommandBus()
    bus.beginMutation({
      mutationId: 'mutation-1',
      kind: 'reorder',
      timeoutMs: 1500,
      onTimeout: vi.fn()
    })

    expect(bus.hasPendingMutations).toBe(true)
    expect(bus.settleMutation('mutation-1')).toBe('reorder')
    expect(bus.hasPendingMutations).toBe(false)
  })

  it('removes a mutation before invoking its timeout callback', () => {
    const bus = useRoomCommandBus()
    const onTimeout = vi.fn()
    bus.beginMutation({
      mutationId: 'mutation-2',
      kind: 'remove',
      timeoutMs: 1500,
      onTimeout
    })

    vi.advanceTimersByTime(1500)

    expect(bus.hasMutation('mutation-2')).toBe(false)
    expect(bus.hasPendingMutations).toBe(false)
    expect(onTimeout).toHaveBeenCalledWith('mutation-2')
  })

  it('cancels all mutation timeouts when reset', () => {
    const bus = useRoomCommandBus()
    const onTimeout = vi.fn()
    bus.beginMutation({
      mutationId: 'mutation-3',
      kind: 'batch-remove',
      timeoutMs: 1500,
      onTimeout
    })

    bus.reset()
    vi.runAllTimers()

    expect(bus.hasPendingMutations).toBe(false)
    expect(onTimeout).not.toHaveBeenCalled()
  })
})
