import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useChatStore } from './chat'

vi.mock('../transport/realtimeClient', () => ({ realtimeClient: { send: vi.fn(() => true) } }))

const message = (id: string) => ({
  id,
  userId: 'other',
  userName: 'Other',
  content: id,
  timestamp: 1,
  type: 'CHAT'
})

describe('chat timelines', () => {
  beforeEach(() => setActivePinia(createPinia()))

  it('keeps room and public histories isolated', () => {
    const chat = useChatStore()
    chat.setHistory([message('room')])
    chat.setPublicHistory([message('public')])
    chat.addMessage(message('room-2'))

    expect(chat.messages.map(item => item.id)).toEqual(['room', 'room-2'])
    expect(chat.publicMessages.map(item => item.id)).toEqual(['public'])
  })

  it('resets only the room timeline during a room transition', () => {
    const chat = useChatStore()
    chat.setHistory([message('room')])
    chat.setPublicHistory([message('public')])

    chat.resetRoomMessages()

    expect(chat.messages).toEqual([])
    expect(chat.hasMore).toBe(true)
    expect(chat.publicMessages.map(item => item.id)).toEqual(['public'])
  })
})
