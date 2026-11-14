import { createPinia, setActivePinia } from 'pinia'
import { useRoomPresenceStore } from '../domains/realtime/roomPresenceStore'
import { describe, expect, it, vi } from 'vitest'
import { useUiStore } from '../stores/ui'
import { useToastStore } from '../stores/toast'
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

it('shows localized playback failures only for the active room', () => {
 setActivePinia(createPinia())
 const ui = useUiStore()
 const toast = useToastStore()
 const handlers = createSocketHandlers({ roomStore: { currentRoomId: 'lounge', rooms: [] } })
 const failure = { queueId:'q', musicId:'m', name:'Track', platform:'local', reason:'preview_only' as const, message:'Only a preview is available for this track' }
 ui.locale = 'en'
 handlers['player.failed']?.(failure, { type:'player.failed', roomId:'other', payload:failure })
 expect(toast.toasts).toHaveLength(0)
 handlers['player.failed']?.(failure, { type:'player.failed', roomId:'lounge', payload:failure })
 expect(toast.toasts).toMatchObject([{ title:'Playback error', message:expect.stringContaining('preview') }])
 ui.locale = 'zh'
 handlers['player.failed']?.(failure, { type:'player.failed', roomId:'lounge', payload:failure })
 expect(toast.toasts[1]).toMatchObject({ title:'播放错误', message:expect.stringContaining('试听') })
})
