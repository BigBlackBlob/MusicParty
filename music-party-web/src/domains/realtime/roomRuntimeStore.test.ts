import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { QueueItem } from '../../contracts/generated/models'
import { useRoomRuntimeStore } from './roomRuntimeStore'

const queueItem = (queueId: string, status = 'PENDING'): QueueItem => ({
  queueId,
  status,
  music: { id: queueId, name: queueId, artists: [], duration: 1, platform: 'test', coverUrl: '' },
  enqueuedBy: { publicId: 'user', name: 'User', isGuest: false }
})

describe('roomRuntimeStore queue reducer', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('rejects missing versions and version gaps', () => {
    const runtime = useRoomRuntimeStore()
    expect(runtime.replaceQueue([queueItem('a')], 0)).toBe('invalid')
    expect(runtime.replaceQueue([queueItem('a')], 1)).toBe('applied')
    expect(runtime.replaceQueue([queueItem('b')], 3)).toBe('version-gap')
    expect(runtime.queue.map(item => item.queueId)).toEqual(['a'])
  })

  it('applies ordered patches and preserves unchanged item identity', () => {
    const runtime = useRoomRuntimeStore()
    runtime.replaceQueue([queueItem('a'), queueItem('b')], 1)
    const unchanged = runtime.queue[1]

    expect(runtime.applyQueuePatch({ operation: 'status', queueVersion: 2, queueId: 'a', status: 'DOWNLOADING' })).toBe('applied')
    expect(runtime.queue[0]?.status).toBe('DOWNLOADING')
    expect(runtime.queue[1]).toBe(unchanged)
    expect(runtime.applyQueuePatch({ operation: 'clear', queueVersion: 3 })).toBe('applied')
    expect(runtime.queue).toEqual([])
  })

  it('applies authoritative snapshots across a version gap', () => {
    const runtime = useRoomRuntimeStore()
    runtime.replaceQueue([queueItem('a')], 1)
    expect(runtime.applyQueuePatch({ operation: 'snapshot', queueVersion: 9, queue: [queueItem('z')] })).toBe('applied')
    expect(runtime.lastQueueVersion).toBe(9)
    expect(runtime.queue.map(item => item.queueId)).toEqual(['z'])
  })

  it('lets an authoritative snapshot correct a patch at the same version', () => {
    const runtime = useRoomRuntimeStore()
    runtime.replaceQueue([], 1, true)
    expect(runtime.applyQueuePatch({ operation: 'append', queueVersion: 2, items: [queueItem('started')] })).toBe('applied')
    expect(runtime.replaceQueue([], 2, true)).toBe('applied')
    expect(runtime.queue).toEqual([])
  })

  it('requires a full baseline and rejects patches from another runtime generation', () => {
    const runtime = useRoomRuntimeStore()
    expect(runtime.applyQueuePatch({ operation: 'append', queueVersion: 1, items: [queueItem('partial')] })).toBe('version-gap')
    expect(runtime.acceptQueueEpoch('old')).toBe(true)
    runtime.replaceQueue([queueItem('old')], 90, true)
    expect(runtime.applyQueuePatch({ operation: 'snapshot', queueEpoch: 'new', queueVersion: 1, queue: [] })).toBe('version-gap')
    expect(runtime.acceptQueueEpoch('new')).toBe(true)
    expect(runtime.replaceQueue([queueItem('restored')], 1, true)).toBe('applied')
    expect(runtime.applyQueuePatch({ operation: 'append', queueEpoch: 'old', queueVersion: 91, items: [queueItem('obsolete')] })).toBe('stale')
    expect(runtime.acceptQueueEpoch('old')).toBe(false)
    expect(runtime.applyQueuePatch({ operation: 'append', queueEpoch: 'new', queueVersion: 2, items: [queueItem('next')] })).toBe('applied')
    expect(runtime.queue.map(item => item.queueId)).toEqual(['restored', 'next'])
  })
})
