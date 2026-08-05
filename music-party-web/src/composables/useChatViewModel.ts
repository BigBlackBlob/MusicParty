import dayjs from 'dayjs'
import { computed, nextTick, type Ref } from 'vue'
import type { ChatMessage } from '../contracts/generated/models'
import { realtimeClient } from '../transport/realtimeClient'
import { useChatStore } from '../stores/chat'
import { useUserStore } from '../stores/user'

export type ChatTab = 'CHAT' | 'PUBLIC' | 'SYSTEM'

const timeThreshold = 3 * 60 * 1000

export function useChatViewModel(activeTabRef: Ref<ChatTab>) {
  const chat = useChatStore()
  const user = useUserStore()

  const isSelf = (message: ChatMessage): boolean => message.userId === user.publicId
  const tabLabel = (tab: ChatTab): string => tab === 'CHAT' ? '聊天' : tab === 'PUBLIC' ? '公共' : '系统'
  const formatTime = (timestamp: number): string => dayjs(timestamp).format('MM-DD HH:mm')

  const filteredMessages = computed(() => {
    if (activeTabRef.value === 'PUBLIC') return chat.publicMessages
    return chat.messages.filter((message) => {
      if (activeTabRef.value === 'CHAT') return ['CHAT', 'LIKE', 'PLAY_START'].includes(message.type)
      return ['SYSTEM', 'LIKE', 'PLAY_START'].includes(message.type)
    })
  })

  const processedMessages = computed(() => {
    let lastTime = 0
    return filteredMessages.value.map((message) => {
      const timestamp = Number(message.timestamp || 0)
      const showTime = timestamp - lastTime > timeThreshold
      if (showTime) lastTime = timestamp
      return { msg: message, showTime }
    })
  })

  function markAsRead(): void {
    chat.markAsRead(activeTabRef.value === 'PUBLIC' ? 'public' : 'room')
  }

  function sendMessage(content: unknown): boolean {
    const text = typeof content === 'string' ? content.trim() : ''
    if (!text) return false
    if (!user.hasDisplayName) {
      user.showNameModal = true
      return false
    }
    const destination = activeTabRef.value === 'PUBLIC' ? 'public-chat.message' : 'chat.message'
    return realtimeClient.send(destination, { content: text })
  }

  async function scrollToBottom(listRef: Ref<HTMLElement | null>, force = false): Promise<void> {
    await nextTick()
    const element = listRef.value
    if (!element) return
    const isAtBottom = element.scrollHeight - element.scrollTop - element.clientHeight < 50
    if (isAtBottom || force) {
      element.scrollTop = element.scrollHeight
      if (force) requestAnimationFrame(() => {
        if (listRef.value) listRef.value.scrollTop = listRef.value.scrollHeight
      })
    }
  }

  return {
    chat,
    user,
    filteredMessages,
    processedMessages,
    isSelf,
    tabLabel,
    formatTime,
    markAsRead,
    sendMessage,
    scrollToBottom,
  }
}
