import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useRoomPresenceStore } from './roomPresenceStore'

describe('room presence store', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('uses only newer snapshots for the active room', () => {
    const presence = useRoomPresenceStore()
    presence.reset('lounge')

    expect(presence.apply({ roomId: 'lounge', revision: 1, users: [{ publicId: 'u_one', name: 'One', isGuest: true }] }, 'lounge')).toBe(true)
    expect(presence.apply({ roomId: 'lounge', revision: 1, users: [] }, 'lounge')).toBe(false)
    expect(presence.apply({ roomId: 'other', revision: 2, users: [] }, 'lounge')).toBe(false)
    expect(presence.users).toEqual([{ publicId: 'u_one', name: 'One', isGuest: true }])
  })

  it('resets revisions for a room transition and deduplicates account connections', () => {
    const presence = useRoomPresenceStore()
    presence.reset('lounge')
    presence.apply({ roomId: 'lounge', revision: 9, users: [{ publicId: 'u_one', name: 'One', isGuest: false }] }, 'lounge')

    presence.reset('studio')
    expect(presence.apply({ roomId: 'studio', revision: 1, users: [
      { publicId: 'u_two', name: 'Two', isGuest: true },
      { publicId: 'u_two', name: 'Duplicate', isGuest: true }
    ] }, 'studio')).toBe(true)
    expect(presence.revision).toBe(1)
    expect(presence.users).toEqual([{ publicId: 'u_two', name: 'Two', isGuest: true }])
  })
})
