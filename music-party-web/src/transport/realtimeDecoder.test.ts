import { describe, expect, it } from 'vitest'
import { decodeServerEnvelope } from './realtimeDecoder'

describe('decodeServerEnvelope', () => {
  it('accepts versioned Go player state envelopes', () => {
    const envelope = decodeServerEnvelope(JSON.stringify({
      type: 'player.state',
      roomId: 'lounge',
      payload: { stateVersion: 4, queueVersion: 7, playEpoch: 2 }
    }))
    expect(envelope?.type).toBe('player.state')
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
})
