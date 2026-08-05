import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { RoomSummary, RoomUpdateInput } from '../contracts/generated/models'
import { roomApi } from '../api/rooms'
import { STORAGE_KEYS } from '../constants/keys'
import { realtimeClient } from '../transport/realtimeClient'
import { isAPIError } from '../transport/errors'
import { queryClient } from '../app/providers'
import { queryKeys } from '../domains/queryKeys'

const DEFAULT_ROOM_ID = 'lounge'

export interface RoomTransitionError {
  status: number
  code: string
  message: string
}

export type RoomTransition =
  | { status: 'idle' }
  | { status: 'verifying'; roomId: string }
  | { status: 'connecting'; roomId: string }
  | { status: 'failed'; roomId: string; error: RoomTransitionError }

const loungeFallback: RoomSummary = {
  roomId: DEFAULT_ROOM_ID,
  name: 'Lounge',
  creatorPublicId: '',
  createdAt: 0,
  privateRoom: false,
  system: true,
  onlineCount: 0,
  accessGranted: true
}

export const useRoomStore = defineStore('room', () => {
  const cacheRevision = ref(0)
  queryClient.getQueryCache().subscribe(() => { cacheRevision.value += 1 })
  const rooms = computed(() => {
    cacheRevision.value
    const cached = queryClient.getQueryData<RoomSummary[]>(queryKeys.rooms.all)
    return cached?.length ? cached : [loungeFallback]
  })
  const currentRoomId = ref(localStorage.getItem(STORAGE_KEYS.ROOM_ID) || DEFAULT_ROOM_ID)
  const transition = ref<RoomTransition>({ status: 'idle' })
  const isLoading = ref(false)
  const createError = ref('')

  const currentRoom = computed(() => rooms.value.find((room) => room.roomId === currentRoomId.value) || rooms.value[0] || loungeFallback)

  async function fetchRooms(signal?: AbortSignal): Promise<RoomSummary[]> {
    isLoading.value = true
    try {
      const data = await queryClient.fetchQuery({
        queryKey: queryKeys.rooms.all,
        queryFn: ({ signal: querySignal }) => roomApi.list(signal ?? querySignal),
        staleTime: 0
      })
      ensureCurrentRoom(data)
      return data
    } finally {
      isLoading.value = false
    }
  }

  function setRooms(nextRooms: RoomSummary[]): void {
    const normalized = nextRooms.length ? nextRooms : [loungeFallback]
    queryClient.setQueryData(queryKeys.rooms.all, normalized)
    ensureCurrentRoom(normalized)
  }

  function ensureCurrentRoom(nextRooms: RoomSummary[]): void {
    if (!nextRooms.some((room) => room.roomId === currentRoomId.value)) commitTransition(DEFAULT_ROOM_ID)
  }

  function setCurrentRoom(roomId: string): void { commitTransition(roomId || DEFAULT_ROOM_ID) }

  function commitTransition(roomId: string): void {
    currentRoomId.value = roomId || DEFAULT_ROOM_ID
    transition.value = { status: 'idle' }
    localStorage.setItem(STORAGE_KEYS.ROOM_ID, currentRoomId.value)
  }

  function beginConnection(roomId: string): void {
    transition.value = { status: 'connecting', roomId }
  }

  function failTransition(roomId: string, error: unknown): RoomTransitionError {
    const normalized: RoomTransitionError = isAPIError(error)
      ? { status: error.status, code: error.code, message: error.message }
      : { status: 0, code: 'ROOM_TRANSITION_FAILED', message: error instanceof Error ? error.message : 'Room transition failed' }
    transition.value = { status: 'failed', roomId, error: normalized }
    return normalized
  }

  const findRoom = (roomId: string) => rooms.value.find((room) => room.roomId === (roomId || DEFAULT_ROOM_ID))
  const isPrivateRoom = (roomId: string) => Boolean(findRoom(roomId)?.privateRoom)

  function setRoomAccessGranted(roomId: string, granted: boolean): void {
    queryClient.setQueryData<RoomSummary[]>(queryKeys.rooms.all, (current = [loungeFallback]) => current.map((room) =>
      room.roomId === (roomId || DEFAULT_ROOM_ID) ? { ...room, accessGranted: granted } : room))
  }

  function clearRoomAccess(roomId: string): void { setRoomAccessGranted(roomId, false) }
  const hasValidRoomAccess = (roomId: string) => !isPrivateRoom(roomId) || Boolean(findRoom(roomId)?.accessGranted)

  async function verifyRoomAccess(roomId: string, password: string, signal?: AbortSignal) {
    const targetRoomId = roomId || DEFAULT_ROOM_ID
    transition.value = { status: 'verifying', roomId: targetRoomId }
    try {
      const response = signal
        ? await roomApi.verify(targetRoomId, password, signal)
        : await roomApi.verify(targetRoomId, password)
      setRoomAccessGranted(targetRoomId, response.accessGranted)
      transition.value = { status: 'idle' }
      return response
    } catch (error) {
      failTransition(targetRoomId, error)
      throw error
    }
  }

  function createRoom(name: string, options: { isPrivate?: boolean; password?: string } = {}): boolean {
    createError.value = ''
    return realtimeClient.send('rooms.create', { name, ...options })
  }

  async function updateRoom(roomId: string, payload: RoomUpdateInput): Promise<RoomSummary> {
    const updated = await roomApi.update(roomId, payload)
    await fetchRooms()
    return updated
  }

  async function deleteRoom(roomId: string): Promise<void> {
    await roomApi.remove(roomId)
    if (currentRoomId.value === roomId) commitTransition(DEFAULT_ROOM_ID)
    await fetchRooms()
  }

  return {
    rooms, currentRoomId, currentRoom, transition, isLoading, createError,
    fetchRooms, setRooms, setCurrentRoom, commitTransition, beginConnection, failTransition,
    setRoomAccessGranted, clearRoomAccess, hasValidRoomAccess, verifyRoomAccess,
    isPrivateRoom, createRoom, updateRoom, deleteRoom
  }
})
