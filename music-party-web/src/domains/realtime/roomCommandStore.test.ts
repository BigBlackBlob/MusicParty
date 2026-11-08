import { createPinia, setActivePinia } from 'pinia'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { personalPlaylistsApi } from '../../api/personalPlaylists'
import type { Music } from '../../contracts/generated/models'
import { useLikedSongsStore } from '../playback/likedSongs'
import { useUserStore } from '../../stores/user'
import { realtimeClient } from '../../transport/realtimeClient'
import { useRoomCommandStore } from './roomCommandStore'
import { useRoomCommandBus } from './roomCommandBus'
import { useRealtimeConnectionStore } from './realtimeConnectionStore'
import { useRoomStateSynchronizer } from './roomStateSynchronizer'
import { useRoomRuntimeStore } from './roomRuntimeStore'

vi.mock('../../transport/realtimeClient', () => ({
  realtimeClient: { connected: true, generation: 1, send: vi.fn(() => true), sendEnvelope: vi.fn(() => true) },
}))
vi.mock('../../api/personalPlaylists', () => ({
  personalPlaylistsApi: { likeSong: vi.fn(), unlikeSong: vi.fn() },
}))

const music: Music = { id: 'track', name: 'Track', artists: [], duration: 1000, platform: 'local', coverUrl: '' }

describe('liked song control confirmation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    vi.useFakeTimers()
    realtimeClient.generation = 1
    useUserStore().initUser('member', 'Member', false)
    useRealtimeConnectionStore().connected = true
    useRealtimeConnectionStore().state = 'connected'
    useRoomStateSynchronizer().syncState({
      idempotencyScopeId: 'room-scope', queueEpoch: 'room-generation',
      queueVersion: 1, stateVersion: 1, playEpoch: 1, serverTimestamp: Date.now(), queue: [],
      isShuffle: false, isPaused: true, isPauseLocked: false, isSkipLocked: false,
      isShuffleLocked: false, isLoading: false, historyCursor: 0,
      nowPlaying: { music: { ...music, url: '', needsProxy: false }, currentPosition: 0, likedUserIds: [], likeMarkers: [], playEpoch: 1, positionUpdatedAt: Date.now() },
    })
  })
  afterEach(() => vi.useRealTimers())

  it('uses room likes and the current play epoch for like and unlike', async () => {
    const commands = useRoomCommandStore()
    for (const type of ['control.like', 'control.unlike'] as const) {
      await commands.toggleLike(music)
      const envelope = vi.mocked(realtimeClient.sendEnvelope).mock.lastCall?.[0]
      expect(envelope).toMatchObject({ type, payload: { idempotencyScopeId: 'room-scope', expectedPlayEpoch: 1, mutationId: expect.any(String) } })
      if (!envelope || !('mutationId' in envelope.payload)) throw new Error('Missing mutation ID')
      commands.settleLike({ outcome: 'applied', mutationId: envelope.payload.mutationId })
      await vi.advanceTimersByTimeAsync(3001)
    }
    expect(personalPlaylistsApi.likeSong).not.toHaveBeenCalled()
    expect(personalPlaylistsApi.unlikeSong).not.toHaveBeenCalled()
    expect(realtimeClient.send).not.toHaveBeenCalled()
  })

  it('does not infer room likes from the personal liked songs cache', async () => {
    useLikedSongsStore().add(music)
    await useRoomCommandStore().toggleLike(music)
    expect(vi.mocked(realtimeClient.sendEnvelope).mock.lastCall?.[0]?.type).toBe('control.like')
  })

  it('restores the previous room state after a rejected unlike', async () => {
    const commands = useRoomCommandStore()
    const runtime = useRoomRuntimeStore()
    runtime.nowPlaying = { music: { ...music, url: '', needsProxy: false }, currentPosition: 0, likedUserIds: ['member'], likeMarkers: [], playEpoch: 1, positionUpdatedAt: Date.now() }
    await commands.toggleLike(music)
    const envelope = vi.mocked(realtimeClient.sendEnvelope).mock.lastCall?.[0]
    if (!envelope || !('mutationId' in envelope.payload)) throw new Error('Missing like mutation ID')
    expect(envelope.type).toBe('control.unlike')
    expect(runtime.nowPlaying?.likedUserIds).not.toContain('member')

    commands.settleLike({ outcome: 'rejected', mutationId: envelope.payload.mutationId })

    expect(runtime.nowPlaying?.likedUserIds).toContain('member')
  })

  it('rolls back only rejected room likes without making a second HTTP write', async () => {
    const commands = useRoomCommandStore()
    const liked = useLikedSongsStore()
    await commands.toggleLike(music)
    const envelope = vi.mocked(realtimeClient.sendEnvelope).mock.lastCall?.[0]
    if (!envelope || !('mutationId' in envelope.payload)) throw new Error('Missing like mutation ID')
    expect(personalPlaylistsApi.likeSong).not.toHaveBeenCalled()

    commands.settleLike({ outcome: 'rejected', mutationId: envelope.payload.mutationId, code: 'FORBIDDEN' })

    expect(liked.includes(music)).toBe(false)
    expect(personalPlaylistsApi.likeSong).not.toHaveBeenCalled()
    expect(personalPlaylistsApi.unlikeSong).not.toHaveBeenCalled()
  })

  it('does not persist a like before a server scope is available', async () => {
    useRoomRuntimeStore().reset()
    await useRoomCommandStore().toggleLike(music)
    expect(personalPlaylistsApi.likeSong).not.toHaveBeenCalled()
    expect(realtimeClient.sendEnvelope).not.toHaveBeenCalled()
  })

  it('replays the same pause mutation once after reconnecting in the same scope', () => {
    const commands = useRoomCommandStore()
    const sync = useRoomStateSynchronizer()
    expect(commands.togglePause()).toBe(true)
    expect(commands.togglePause()).toBe(false)
    const original = vi.mocked(realtimeClient.sendEnvelope).mock.calls[0]?.[0]
    expect(original).toMatchObject({
      type: 'control.toggle-pause',
      payload: { idempotencyScopeId: 'room-scope', mutationId: expect.any(String) }
    })

    realtimeClient.generation = 2
    sync.syncState({
      idempotencyScopeId: 'room-scope', queueEpoch: 'room-generation',
      queueVersion: 1, stateVersion: 2, playEpoch: 1, serverTimestamp: Date.now() + 1, queue: [],
      isShuffle: false, isPaused: true, isPauseLocked: false, isSkipLocked: false,
      isShuffleLocked: false, isLoading: false, historyCursor: 0,
    })
    expect(realtimeClient.sendEnvelope).toHaveBeenCalledTimes(2)
    expect(vi.mocked(realtimeClient.sendEnvelope).mock.calls[1]?.[0]).toEqual(original)

    sync.syncState({
      idempotencyScopeId: 'room-scope', queueEpoch: 'room-generation',
      queueVersion: 1, stateVersion: 3, playEpoch: 1, serverTimestamp: Date.now() + 2, queue: [],
      isShuffle: false, isPaused: true, isPauseLocked: false, isSkipLocked: false,
      isShuffleLocked: false, isLoading: false, historyCursor: 0,
    })
    expect(realtimeClient.sendEnvelope).toHaveBeenCalledTimes(2)
  })

  it('discards an old pause mutation when the authoritative scope changes', () => {
    const commands = useRoomCommandStore()
    const bus = useRoomCommandBus()
    const sync = useRoomStateSynchronizer()
    expect(commands.togglePause()).toBe(true)
    realtimeClient.generation = 2

    sync.syncState({
      idempotencyScopeId: 'replacement-scope', queueEpoch: 'replacement-generation',
      queueVersion: 1, stateVersion: 1, playEpoch: 1, serverTimestamp: Date.now() + 1, queue: [],
      isShuffle: false, isPaused: true, isPauseLocked: false, isSkipLocked: false,
      isShuffleLocked: false, isLoading: false, historyCursor: 0,
    })

    expect(realtimeClient.sendEnvelope).toHaveBeenCalledOnce()
    expect(bus.hasPendingMutations).toBe(false)
  })

  it('rolls back an unconfirmed optimistic like when the authoritative scope changes', async () => {
    const commands = useRoomCommandStore()
    const liked = useLikedSongsStore()
    const sync = useRoomStateSynchronizer()
    await commands.toggleLike(music)
    expect(liked.includes(music)).toBe(true)

    sync.syncState({
      idempotencyScopeId: 'replacement-scope', queueEpoch: 'replacement-generation',
      queueVersion: 1, stateVersion: 1, playEpoch: 1, serverTimestamp: Date.now() + 1, queue: [],
      isShuffle: false, isPaused: true, isPauseLocked: false, isSkipLocked: false,
      isShuffleLocked: false, isLoading: false, historyCursor: 0,
    })

    expect(liked.includes(music)).toBe(false)
    expect(useRoomCommandBus().hasPendingMutations).toBe(false)
    await expect(commands.toggleLike(music)).resolves.toBeUndefined()
    expect(liked.includes(music)).toBe(false)
    expect(realtimeClient.sendEnvelope).toHaveBeenCalledOnce()
  })

  it('rolls back an unconfirmed optimistic like when the room command state resets', async () => {
    const commands = useRoomCommandStore()
    const liked = useLikedSongsStore()
    await commands.toggleLike(music)
    expect(liked.includes(music)).toBe(true)

    commands.reset()

    expect(liked.includes(music)).toBe(false)
    expect(useRoomCommandBus().hasPendingMutations).toBe(false)
  })

  it('replays the exact queue reorder once after a same-scope reconnect', () => {
    const commands = useRoomCommandStore()
    const runtime = useRoomRuntimeStore()
    runtime.queue = [
      { queueId: 'a', status: 'PENDING', music, enqueuedBy: { publicId: 'member', name: 'Member', isGuest: false } },
      { queueId: 'b', status: 'PENDING', music: { ...music, id: 'track-2' }, enqueuedBy: { publicId: 'member', name: 'Member', isGuest: false } }
    ]
    expect(commands.reorderQueue(0, 1, 'a', 'b', 'after')).toBe(true)
    const original = vi.mocked(realtimeClient.sendEnvelope).mock.calls[0]?.[0]

    realtimeClient.generation = 2
    useRoomStateSynchronizer().syncState({
      idempotencyScopeId: 'room-scope', queueEpoch: 'room-generation',
      queueVersion: 2, stateVersion: 2, playEpoch: 1, serverTimestamp: Date.now() + 1, queue: [],
      isShuffle: false, isPaused: true, isPauseLocked: false, isSkipLocked: false,
      isShuffleLocked: false, isLoading: false, historyCursor: 0,
    })

    expect(realtimeClient.sendEnvelope).toHaveBeenCalledTimes(2)
    expect(vi.mocked(realtimeClient.sendEnvelope).mock.calls[1]?.[0]).toEqual(original)
  })

  it('replays a pending like only within its scope and server idempotency window', async () => {
    const commands = useRoomCommandStore()
    await commands.toggleLike(music)
    const original = vi.mocked(realtimeClient.sendEnvelope).mock.calls[0]?.[0]
    if (!original || !('mutationId' in original.payload)) throw new Error('Missing like mutation')

    realtimeClient.generation = 2
    useRoomStateSynchronizer().syncState({
      idempotencyScopeId: 'room-scope', idempotencyTtlMs: 60_000, queueEpoch: 'room-generation',
      queueVersion: 1, stateVersion: 2, playEpoch: 1, serverTimestamp: Date.now() + 1, queue: [],
      isShuffle: false, isPaused: true, isPauseLocked: false, isSkipLocked: false,
      isShuffleLocked: false, isLoading: false, historyCursor: 0,
    })

    expect(realtimeClient.sendEnvelope).toHaveBeenCalledTimes(2)
    expect(vi.mocked(realtimeClient.sendEnvelope).mock.calls[1]?.[0]).toEqual(original)
    commands.settleLike({ outcome: 'applied', mutationId: original.payload.mutationId })
  })
})

describe('offline command gating', () => {
  const items = () => [
    { queueId: 'a', status: 'PENDING' as const, music, enqueuedBy: { publicId: 'member', name: 'Member', isGuest: false } },
    { queueId: 'b', status: 'PENDING' as const, music: { ...music, id: 'track-2' }, enqueuedBy: { publicId: 'member', name: 'Member', isGuest: false } },
  ]

  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    useUserStore().initUser('member', 'Member', false)
  })

  it('refuses reorder, queue mutations and skips while offline without queueing or optimistic moves', () => {
    const connection = useRealtimeConnectionStore()
    connection.connected = true
    connection.state = 'connected'
    connection.synced = true
    connection.setState('offline')
    const commands = useRoomCommandStore()
    const runtime = useRoomRuntimeStore()
    runtime.queue = items()

    expect(connection.connected).toBe(false)
    expect(commands.canOperate).toBe(false)
    expect(commands.reorderQueue(0, 1, 'a', 'b', 'after')).toBe(false)
    expect(commands.topSong('b')).toBe(false)
    expect(commands.removeSong('a')).toBe(false)
    expect(commands.playNext()).toBe(false)
    expect(commands.playPrevious()).toBe(false)
    expect(realtimeClient.sendEnvelope).not.toHaveBeenCalled()
    expect(runtime.queue.map(item => item.queueId)).toEqual(['a', 'b'])
    expect(useRoomCommandBus().hasPendingMutations).toBe(false)
  })

  it('stays refused after reconnect until the new socket applies a snapshot', () => {
    const connection = useRealtimeConnectionStore()
    connection.setState('recovering')
    connection.connected = true
    connection.setState('connected')
    const commands = useRoomCommandStore()
    useRoomRuntimeStore().queue = items()

    expect(commands.reorderQueue(0, 1, 'a', 'b', 'after')).toBe(false)
    useRoomStateSynchronizer().syncState({
      idempotencyScopeId: 'room-scope', queueEpoch: 'room-generation',
      queueVersion: 1, stateVersion: 1, playEpoch: 1, serverTimestamp: Date.now(), queue: items(),
      isShuffle: false, isPaused: true, isPauseLocked: false, isSkipLocked: false,
      isShuffleLocked: false, isLoading: false, historyCursor: 0,
    })
    expect(commands.canOperate).toBe(true)
    expect(commands.reorderQueue(0, 1, 'a', 'b', 'after')).toBe(true)
  })
})
