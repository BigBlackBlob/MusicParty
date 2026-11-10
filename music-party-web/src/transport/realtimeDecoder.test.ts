import { describe, expect, it } from 'vitest'
import type { PlayerStateServerEnvelope } from '../contracts/generated/websocket'
import { decodeServerEnvelope } from './realtimeDecoder'

describe('decodeServerEnvelope', () => {
  it('accepts versioned Go player state envelopes', () => {
    const envelope = decodeServerEnvelope(JSON.stringify({
      type: 'player.state',
      roomId: 'lounge',
      payload: {
        stateVersion: 4, queueVersion: 7, playEpoch: 2, serverTimestamp: 100, historyCursor: 0,
        queue: [], isLoading: false, isPauseLocked: false, isPaused: false,
        isShuffle: false, isShuffleLocked: false, isSkipLocked: false,
      }
    }))
    expect(envelope?.type).toBe('player.state')
  })

  it('accepts compact playback updates but rejects an invalid explicit queue', () => {
    const payload = {
      stateVersion: 4, queueVersion: 7, queueEpoch: 'room', playEpoch: 2, serverTimestamp: 100, historyCursor: 0,
      isLoading: false, isPauseLocked: false, isPaused: false,
      isShuffle: false, isShuffleLocked: false, isSkipLocked: false,
    }
    expect(decodeServerEnvelope(JSON.stringify({ type: 'player.state', payload }))?.type).toBe('player.state')
    expect(decodeServerEnvelope(JSON.stringify({ type: 'player.state', payload: { ...payload, queue: null } }))).toBeNull()
  })

  it('accepts Go player states with no track or a null history enqueuer', () => {
    const base = {
      stateVersion: 4, queueVersion: 7, playEpoch: 2, serverTimestamp: 100, historyCursor: 0,
      queue: [], isLoading: false, isPauseLocked: false, isPaused: false,
      isShuffle: false, isShuffleLocked: false, isSkipLocked: false,
    }
    const music = {
      id: 'song', name: 'Song', artists: [], duration: 1000, platform: 'local',
      coverUrl: '', url: '/song', needsProxy: false,
    }
    const nowPlaying = {
      music, currentPosition: 0, enqueuedById: 'user', enqueuedByName: null,
      likedUserIds: [], likeMarkers: [], playEpoch: 2, positionUpdatedAt: 100,
    }

    const emptyPayload = { ...base, nowPlaying: null } satisfies PlayerStateServerEnvelope['payload']
    const historyPayload = { ...base, nowPlaying } satisfies PlayerStateServerEnvelope['payload']

    for (const payload of [
      emptyPayload,
      historyPayload,
      { ...base, nowPlaying: { ...nowPlaying, enqueuedById: null } },
    ]) {
      expect(decodeServerEnvelope(JSON.stringify({ type: 'player.state', payload }))).not.toBeNull()
    }
    expect(decodeServerEnvelope(JSON.stringify({
      type: 'player.state', payload: { ...base, nowPlaying: { ...nowPlaying, enqueuedByName: 42 } }
    }))).toBeNull()
  })

  it('accepts nullable queue versions in Go reorder nacks', () => {
    expect(decodeServerEnvelope(JSON.stringify({
      type: 'queue.reorder.nack', payload: { mutationId: 'reorder-1', reason: 'STALE_QUEUE', queueVersion: null }
    }))).not.toBeNull()
  })

  it('rejects malformed nested player state and queue patch items', () => {
    const base = {
      stateVersion: 4, queueVersion: 7, playEpoch: 2, serverTimestamp: 100, historyCursor: 0,
      queue: [], isLoading: false, isPauseLocked: false, isPaused: false,
      isShuffle: false, isShuffleLocked: false, isSkipLocked: false,
    }
    expect(decodeServerEnvelope(JSON.stringify({ type: 'player.state', payload: { ...base, nowPlaying: { music: {} } } }))).toBeNull()
    expect(decodeServerEnvelope(JSON.stringify({ type: 'player.state', payload: { ...base, queue: [{}] } }))).toBeNull()
    expect(decodeServerEnvelope(JSON.stringify({ type: 'queue.patch', payload: { operation: 'append', queueVersion: 2, items: [{}] } }))).toBeNull()
  })

  it('rejects malformed, unknown, and legacy unversioned messages', () => {
    expect(decodeServerEnvelope('{')).toBeNull()
    expect(decodeServerEnvelope(JSON.stringify({ type: 'legacy.state', payload: {} }))).toBeNull()
    expect(decodeServerEnvelope(JSON.stringify({ type: 'player.state', payload: { isPaused: true } }))).toBeNull()
  })

  it('requires stable fields for player events', () => {
    expect(decodeServerEnvelope(JSON.stringify({
      type: 'player.events',
      payload: { code: 'CONTROL_DENIED', severity: 'error', message: 'Denied' }
    }))).not.toBeNull()
    expect(decodeServerEnvelope(JSON.stringify({
      type: 'player.events',
      payload: { action: 'CONTROL_DENIED', type: 'ERROR', message: 'Denied' }
    }))).toBeNull()
  })

  it('requires a versioned presence snapshot instead of legacy user arrays', () => {
    expect(decodeServerEnvelope(JSON.stringify({
      type: 'users.online',
      roomId: 'lounge',
      payload: { roomId: 'lounge', revision: 3, users: [{ publicId: 'u_1', name: 'One', isGuest: true }] }
    }))).not.toBeNull()
    expect(decodeServerEnvelope(JSON.stringify({
      type: 'users.online',
      roomId: 'lounge',
      payload: [{ publicId: 'u_1', name: 'One', isGuest: true }]
    }))).toBeNull()
    expect(decodeServerEnvelope(JSON.stringify({
      type: 'users.online',
      roomId: 'lounge',
      payload: { roomId: 'lounge', revision: 4, users: [{ publicId: 'u_1', name: 'One' }] }
    }))).toBeNull()
  })

  it('accepts control acknowledgements and rejects malformed acknowledgement payloads', () => {
    const ack = decodeServerEnvelope(JSON.stringify({
      type: 'control.ack',
      roomId: 'lounge',
      payload: {
        outcome: 'applied',
        mutationId: 'like-1',
        committed: { stateVersion: 4, playEpoch: 2, queueVersion: 7 }
      }
    }))
    expect(ack?.type).toBe('control.ack')

    for (const payload of [
      {},
      { outcome: 'unknown' },
      { outcome: 'applied', mutationId: 12 },
      { outcome: 'applied', committed: { stateVersion: 4, playEpoch: 2 } },
      { outcome: 'applied', replayed: 'yes' }
    ]) {
      expect(decodeServerEnvelope(JSON.stringify({ type: 'control.ack', payload }))).toBeNull()
    }
  })

  it('validates payloads for every newly accepted envelope family', () => {
    const valid = [
      { type: 'server.hello', payload: { apiVersion: '1', minimumClientVersion: '1' } },
      { type: 'history.page', payload: { items: [], offset: 0, roomId: 'lounge', total: 0 } },
      { type: 'playlist.ack', payload: { mutationId: 'm1', operation: 'create', scope: 'room:lounge' } },
      { type: 'playlist.data', payload: { operation: 'list', scope: 'room:lounge', playlists: [] } },
      { type: 'playlist.nack', payload: { operation: 'create', reason: 'denied' } },
    ]
    const invalid = [
      { type: 'server.hello', payload: {} },
      { type: 'history.page', payload: { items: [{}], offset: 0, roomId: 'lounge', total: 0 } },
      { type: 'playlist.ack', payload: {} },
      { type: 'playlist.data', payload: { operation: 'list', scope: 'room:lounge', tracks: [{}] } },
      { type: 'playlist.nack', payload: { operation: 'create' } },
    ]

    for (const envelope of valid) expect(decodeServerEnvelope(JSON.stringify(envelope))).not.toBeNull()
    for (const envelope of invalid) expect(decodeServerEnvelope(JSON.stringify(envelope))).toBeNull()
  })
})
