import { afterEach, describe, expect, it, vi } from 'vitest'
import { createSocketHandlers } from '../services/socketHandler'
import { RealtimeClient } from './realtimeClient'

class FakeWebSocket {
  static readonly CONNECTING = 0
  static readonly OPEN = 1
  static readonly CLOSING = 2
  static readonly CLOSED = 3

  readyState = FakeWebSocket.OPEN
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null

  constructor(readonly url: string) {}
  send(): void {}
  close(): void { this.readyState = FakeWebSocket.CLOSED }
}

afterEach(() => vi.unstubAllGlobals())

describe('RealtimeClient decoded message dispatch', () => {
  it('routes a decoded ACK through socket handlers to the domain settle callback', () => {
    vi.stubGlobal('WebSocket', FakeWebSocket)
    const settleLike = vi.fn()
    const client = new RealtimeClient()
    const handlers = createSocketHandlers({ player: { settleLike } })

    client.connect({ 'room-id': 'lounge' }, {}, handlers)
    const socket = client.client as unknown as FakeWebSocket
    socket.onmessage?.(new MessageEvent('message', {
      data: JSON.stringify({
        type: 'control.ack',
        roomId: 'lounge',
        payload: { outcome: 'applied', mutationId: 'like-1', replayed: true }
      })
    }))

    expect(settleLike).toHaveBeenCalledWith({ outcome: 'applied', mutationId: 'like-1', replayed: true })
    client.disconnect()
  })

  it('delivers an empty-room snapshot to the room handler', () => {
    vi.stubGlobal('WebSocket', FakeWebSocket)
    const syncState = vi.fn()
    const client = new RealtimeClient()
    const handlers = createSocketHandlers({
      player: { syncState },
      roomStore: { currentRoomId: 'lounge', rooms: [] },
    })
    const state = {
      stateVersion: 0, queueVersion: 1, playEpoch: 0, serverTimestamp: 100, historyCursor: 0,
      queue: [], nowPlaying: null, isLoading: false, isPauseLocked: false, isPaused: false,
      isShuffle: false, isShuffleLocked: false, isSkipLocked: false,
    }

    client.connect({ 'room-id': 'lounge' }, {}, handlers)
    const socket = client.client as unknown as FakeWebSocket
    socket.onmessage?.(new MessageEvent('message', {
      data: JSON.stringify({ type: 'player.state', roomId: 'lounge', payload: state }),
    }))

    expect(syncState).toHaveBeenCalledWith(state)
    client.disconnect()
  })
})
