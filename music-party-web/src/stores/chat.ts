import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import type { ChatMessage } from '../contracts/generated/models'
import { realtimeClient } from '../transport/realtimeClient'
import { useUserStore } from './user'

export interface ChatTimeline {
  messages: ChatMessage[]
  cursor: string | null
  hasMore: boolean
  loading: boolean
  unread: number
}

const pageSize = 50
type HistoryDestination = 'chat.history.fetch' | 'public-chat.history.fetch'
type ChatScope = 'room' | 'public'
const timeline = (): ChatTimeline => ({ messages: [], cursor: null, hasMore: true, loading: false, unread: 0 })

export const useChatStore = defineStore('chat', () => {
  const roomTimeline = reactive<ChatTimeline>(timeline())
  const publicTimeline = reactive<ChatTimeline>(timeline())
  const isOpen = ref(false)
  const userStore = useUserStore()

  const messages = computed(() => roomTimeline.messages)
  const publicMessages = computed(() => publicTimeline.messages)
  const unreadCount = computed(() => roomTimeline.unread)
  const publicUnreadCount = computed(() => publicTimeline.unread)
  const hasMore = computed(() => roomTimeline.hasMore)
  const publicHasMore = computed(() => publicTimeline.hasMore)
  const isLoadingMore = computed(() => roomTimeline.loading)
  const isLoadingPublicMore = computed(() => publicTimeline.loading)

  function add(t: ChatTimeline, message: ChatMessage): void {
    t.messages.push(message)
    if (t.messages.length > 2000) t.messages = t.messages.slice(-1000)
    if (!isOpen.value && message.userId !== userStore.publicId && message.type === 'CHAT') t.unread += 1
  }

  function setHistory(t: ChatTimeline, history: ChatMessage[]): void {
    t.messages = history
    t.unread = 0
    t.hasMore = history.length >= pageSize
    t.loading = false
  }

  function prepend(t: ChatTimeline, history: ChatMessage[]): void {
    if (history.length === 0) t.hasMore = false
    else {
      t.messages = [...history, ...t.messages]
      if (history.length < pageSize) t.hasMore = false
    }
    t.loading = false
  }

  function loadMore(t: ChatTimeline, destination: HistoryDestination): void {
    if (!t.hasMore || t.loading) return
    t.loading = true
    if (!realtimeClient.send(destination, { offset: t.messages.length, limit: pageSize })) t.loading = false
  }

  function resetRoomMessages(): void {
    Object.assign(roomTimeline, timeline())
  }

  function markAsRead(scope: ChatScope): void {
    if (scope === 'public') publicTimeline.unread = 0
    else roomTimeline.unread = 0
  }

  function toggleChat(): void {
    isOpen.value = !isOpen.value
    if (isOpen.value) {
      roomTimeline.unread = 0
      publicTimeline.unread = 0
    }
  }

  return {
    roomTimeline,
    publicTimeline,
    messages,
    publicMessages,
    unreadCount,
    publicUnreadCount,
    isOpen,
    hasMore,
    publicHasMore,
    isLoadingMore,
    isLoadingPublicMore,
    addMessage: (message: ChatMessage) => add(roomTimeline, message),
    addPublicMessage: (message: ChatMessage) => add(publicTimeline, message),
    setHistory: (history: ChatMessage[]) => setHistory(roomTimeline, history),
    setPublicHistory: (history: ChatMessage[]) => setHistory(publicTimeline, history),
    prependHistory: (history: ChatMessage[]) => prepend(roomTimeline, history),
    prependPublicHistory: (history: ChatMessage[]) => prepend(publicTimeline, history),
    loadMoreHistory: () => loadMore(roomTimeline, 'chat.history.fetch'),
    loadMorePublicHistory: () => loadMore(publicTimeline, 'public-chat.history.fetch'),
    markAsRead,
    resetRoomMessages,
    toggleChat
  }
})
