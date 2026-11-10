import { createPinia, setActivePinia } from 'pinia'
import { useRoomPresenceStore } from '../domains/realtime/roomPresenceStore'
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


it('accepts restarted server presence revisions after an automatic reconnect', () => {
  setActivePinia(createPinia())
  const presence = useRoomPresenceStore()
  presence.reset('lounge')
  presence.apply({ roomId: 'lounge', revision: 500, users: [] }, 'lounge')
  const callbacks = createSocketCallbacks({ resetPresence: () => presence.reset('lounge') })
  callbacks.onConnect?.(new Event('open'))
  expect(presence.apply({ roomId: 'lounge', revision: 1, users: [
    { publicId: 'new', name: 'New', isGuest: true },
  ] }, 'lounge')).toBe(true)
  expect(presence.users[0]?.publicId).toBe('new')
})
