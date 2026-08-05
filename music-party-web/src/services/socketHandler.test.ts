import { describe, expect, it, vi } from 'vitest'
import { createSocketCallbacks, createSocketHandlers } from './socketHandler'

describe('socket presence handler', () => {
  it('forwards only current-room versioned presence snapshots', () => {
    const apply = vi.fn()
    const handlers = createSocketHandlers({
      presence: { apply },
      roomStore: { currentRoomId: 'lounge', rooms: [] }
    })
    const snapshot = { roomId: 'lounge', revision: 2, users: [{ publicId: 'u_1', name: 'One', isGuest: true }] }

    handlers['users.online']?.(snapshot, { type: 'users.online', roomId: 'lounge', payload: snapshot })
    expect(apply).toHaveBeenCalledWith(snapshot, 'lounge')

    handlers['users.online']?.({ ...snapshot, roomId: 'studio', revision: 3 }, {
      type: 'users.online', roomId: 'studio', payload: { ...snapshot, roomId: 'studio', revision: 3 }
    })
    expect(apply).toHaveBeenCalledOnce()
  })
})

describe('socket connection callbacks', () => {
  it('requests a fresh presence snapshot on every new connection', () => {
    const requestPresence = vi.fn()
    const callbacks = createSocketCallbacks({ requestPresence })

    callbacks.onConnect?.(new Event('open'))

    expect(requestPresence).toHaveBeenCalledOnce()
  })
})
