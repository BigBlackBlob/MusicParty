import type { ClientEnvelope, ServerEnvelope } from '../contracts/generated/websocket'
import { decodeServerEnvelope } from './realtimeDecoder'

export type ConnectionState =
  | 'idle'
  | 'connecting'
  | 'connected'
  | 'recovering'
  | 'offline'
  | 'session-rejected'
  | 'room-access-rejected'

type ClientType = ClientEnvelope['type']
type ClientPayload<T extends ClientType> = Extract<ClientEnvelope, { type: T }>['payload']
type ServerType = ServerEnvelope['type']

export type RealtimeHandlers = {
  [Type in ServerType]?: (
    payload: Extract<ServerEnvelope, { type: Type }>['payload'],
    envelope: Extract<ServerEnvelope, { type: Type }>
  ) => void
}

export interface RealtimeCallbacks {
  onConnect?: (event: Event) => void
  onDisconnect?: (event: CloseEvent) => void
  onError?: (event: Event) => void
  onStateChange?: (state: ConnectionState) => void
  onSessionRejected?: (event: CloseEvent) => void
  onRoomAccessRejected?: (event: CloseEvent) => void
  onAuthError?: (event: CloseEvent) => void
}

interface SocketConfig {
  authParams: Record<string, string>
  callbacks: RealtimeCallbacks
  handlers: RealtimeHandlers
}

export class RealtimeClient {
  client: WebSocket | null = null
  connected = false
  state: ConnectionState = 'idle'
  socketConfig: SocketConfig | null = null
  reconnectTimer: ReturnType<typeof setTimeout> | null = null
  reconnectNowTimer: ReturnType<typeof setTimeout> | null = null
  reconnectDelay = 1_000
  readonly maxReconnectDelay = 10_000
  intentionalClose = false
  requestSeq = 0
  generation = 0

  private readonly onOnline = () => {
    if (!this.intentionalClose && this.socketConfig && !this.connected) this.reconnectNow()
  }
  private readonly onOffline = () => {
    this.clearReconnectTimer()
    this.setState('offline')
  }
  private readonly onVisibility = () => {
    if (document.visibilityState !== 'visible' || !this.connected) return
    this.send('sync.ping', { pingId: this.nextRequestId('visibility-ping'), clientSendTime: Date.now() })
    this.send('player.resync', {})
  }

  constructor() {
    if (typeof window !== 'undefined') {
      window.addEventListener('online', this.onOnline)
      window.addEventListener('offline', this.onOffline)
      document.addEventListener('visibilitychange', this.onVisibility)
    }
  }

  connect(authParams: Record<string, string> = {}, callbacks: RealtimeCallbacks = {}, handlers: RealtimeHandlers = {}): void {
    const nextConfig = { authParams, callbacks, handlers }
    if (this.client && this.isSocketActive(this.client)) {
      if (!this.shouldReconnectForConfig(nextConfig)) return
      this.disconnect()
    }
    this.socketConfig = nextConfig
    this.intentionalClose = false
    this.clearReconnectTimer()
    this.clearReconnectNowTimer()
    const socketGeneration = ++this.generation
    const socket = new WebSocket(this.buildUrl(authParams))
    this.client = socket
    this.setState(this.connected ? 'recovering' : 'connecting')

    socket.onopen = (event) => {
      if (!this.isCurrent(socket, socketGeneration)) return
      this.connected = true
      this.reconnectDelay = 1_000
      this.setState('connected')
      callbacks.onConnect?.(event)
    }
    socket.onmessage = (event) => {
      if (!this.isCurrent(socket, socketGeneration) || typeof event.data !== 'string') return
      const envelope = decodeServerEnvelope(event.data)
      if (!envelope) {
        console.warn('Ignoring malformed socket message')
        return
      }
      const currentRoomId = this.socketConfig?.authParams['room-id']
      if (envelope.roomId && currentRoomId && envelope.roomId !== currentRoomId) return
      this.dispatch(envelope, handlers)
    }
    socket.onerror = (event) => {
      if (this.isCurrent(socket, socketGeneration)) callbacks.onError?.(event)
    }
    socket.onclose = (event) => {
      if (!this.isCurrent(socket, socketGeneration)) return
      this.connected = false
      callbacks.onDisconnect?.(event)
      if (event.code === 1008) {
        const roomRejected = /forbidden|room/i.test(event.reason || '')
        this.setState(roomRejected ? 'room-access-rejected' : 'session-rejected')
        if (roomRejected) {
          if (callbacks.onRoomAccessRejected) callbacks.onRoomAccessRejected(event)
          else callbacks.onAuthError?.(event)
        } else if (callbacks.onSessionRejected) {
          callbacks.onSessionRejected(event)
        } else {
          callbacks.onAuthError?.(event)
        }
        return
      }
      if (!this.intentionalClose) {
        this.setState(navigator.onLine ? 'recovering' : 'offline')
        this.scheduleReconnect()
      }
    }
  }

  send<T extends ClientType>(type: T, payload?: ClientPayload<T>): boolean {
    if (!this.client || !this.connected || this.client.readyState !== WebSocket.OPEN) return false
    const envelope = {
      type,
      requestId: this.nextRequestId(type),
      payload: payload ?? {}
    }
    this.client.send(JSON.stringify(envelope))
    return true
  }

  sendEnvelope(envelope: ClientEnvelope): boolean {
    if (!this.client || !this.connected || this.client.readyState !== WebSocket.OPEN) return false
    this.client.send(JSON.stringify({
      ...envelope,
      requestId: envelope.requestId ?? this.nextRequestId(envelope.type),
    }))
    return true
  }

  buildUrl(authParams: Record<string, string> = {}): string {
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = new URL(`${protocol}//${window.location.host}/ws`)
    Object.entries(authParams).forEach(([key, value]) => {
      if (value) url.searchParams.set(key, value)
    })
    return url.toString()
  }

  forceReconnect(): void { this.reconnectNow() }

  reconnectNow(): void {
    const config = this.socketConfig
    this.disconnect()
    if (!config) return
    this.reconnectNowTimer = setTimeout(() => {
      this.reconnectNowTimer = null
      this.connect(config.authParams, config.callbacks, config.handlers)
    }, 100)
  }

  disconnect(): void {
    this.intentionalClose = true
    this.clearReconnectTimer()
    this.clearReconnectNowTimer()
    const socket = this.client
    this.client = null
    this.generation += 1
    if (socket) {
      socket.onopen = null
      socket.onmessage = null
      socket.onerror = null
      socket.onclose = null
      if (this.isSocketActive(socket)) socket.close()
    }
    this.connected = false
    this.setState('idle')
  }

  scheduleReconnect(): void {
    if (this.reconnectTimer || !this.socketConfig || !navigator.onLine) return
    const config = this.socketConfig
    const delay = Math.round(this.reconnectDelay * (0.8 + Math.random() * 0.4))
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.connect(config.authParams, config.callbacks, config.handlers)
    }, delay)
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay)
  }

  clearReconnectTimer(): void {
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.reconnectTimer = null
  }

  clearReconnectNowTimer(): void {
    if (this.reconnectNowTimer) clearTimeout(this.reconnectNowTimer)
    this.reconnectNowTimer = null
  }

  nextRequestId(type: string): string {
    this.requestSeq += 1
    return `${type}-${Date.now()}-${this.requestSeq}`
  }

  isSocketActive(socket: WebSocket): boolean {
    return socket.readyState === WebSocket.CONNECTING || socket.readyState === WebSocket.OPEN
  }

  shouldReconnectForConfig(next: SocketConfig): boolean {
    if (!this.socketConfig) return true
    return !sameRecord(this.socketConfig.authParams, next.authParams)
      || !sameKeys(this.socketConfig.handlers, next.handlers)
  }

  private isCurrent(socket: WebSocket, generation: number): boolean {
    return this.client === socket && this.generation === generation
  }

  private dispatch(envelope: ServerEnvelope, handlers: RealtimeHandlers): void {
    switch (envelope.type) {
      case 'chat.history': handlers['chat.history']?.(envelope.payload, envelope); return
      case 'chat.message': handlers['chat.message']?.(envelope.payload, envelope); return
      case 'enqueue.ack': handlers['enqueue.ack']?.(envelope.payload, envelope); return
      case 'enqueue.nack': handlers['enqueue.nack']?.(envelope.payload, envelope); return
      case 'player.events': handlers['player.events']?.(envelope.payload, envelope); return
      case 'player.state': handlers['player.state']?.(envelope.payload, envelope); return
      case 'public-chat.history': handlers['public-chat.history']?.(envelope.payload, envelope); return
      case 'public-chat.message': handlers['public-chat.message']?.(envelope.payload, envelope); return
      case 'queue.mutation.ack': handlers['queue.mutation.ack']?.(envelope.payload, envelope); return
      case 'queue.mutation.nack': handlers['queue.mutation.nack']?.(envelope.payload, envelope); return
      case 'queue.patch': handlers['queue.patch']?.(envelope.payload, envelope); return
      case 'queue.reorder.ack': handlers['queue.reorder.ack']?.(envelope.payload, envelope); return
      case 'queue.reorder.nack': handlers['queue.reorder.nack']?.(envelope.payload, envelope); return
      case 'rooms.created': handlers['rooms.created']?.(envelope.payload, envelope); return
      case 'rooms.list': handlers['rooms.list']?.(envelope.payload, envelope); return
      case 'sync.pong': handlers['sync.pong']?.(envelope.payload, envelope); return
      case 'user.me': handlers['user.me']?.(envelope.payload, envelope); return
      case 'users.online': handlers['users.online']?.(envelope.payload, envelope); return
      default: assertNever(envelope)
    }
  }

  private setState(state: ConnectionState): void {
    this.state = state
    this.socketConfig?.callbacks.onStateChange?.(state)
  }
}

function assertNever(value: never): never {
  throw new Error(`Unhandled realtime envelope: ${JSON.stringify(value)}`)
}

function sameRecord(current: Record<string, string>, next: Record<string, string>): boolean {
  const currentKeys = Object.keys(current).sort()
  const nextKeys = Object.keys(next).sort()
  return currentKeys.length === nextKeys.length
    && currentKeys.every((key, index) => key === nextKeys[index] && current[key] === next[key])
}

function sameKeys(current: RealtimeHandlers, next: RealtimeHandlers): boolean {
  const currentKeys = Object.keys(current).sort()
  const nextKeys = Object.keys(next).sort()
  return currentKeys.length === nextKeys.length && currentKeys.every((key, index) => key === nextKeys[index])
}

export const realtimeClient = new RealtimeClient()
