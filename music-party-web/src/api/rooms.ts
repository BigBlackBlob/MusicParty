import type { RoomAccessResult, RoomSummary, RoomUpdateInput } from '../contracts/generated/models'
import type { RequestOptions } from '../transport/httpClient'
import { httpClient as client } from '../transport/httpClient'

const roomPath = (roomId: string) => `/api/rooms/${encodeURIComponent(roomId)}`
const options = (signal?: AbortSignal): RequestOptions => signal ? { signal } : {}

export const roomApi = {
  list: (signal?: AbortSignal) => client.get<RoomSummary[]>('/api/rooms', options(signal)),
  verify: (roomId: string, password: string, signal?: AbortSignal) =>
    client.post<RoomAccessResult, { password: string }>(`${roomPath(roomId)}/verify`, { password }, options(signal)),
  update: (roomId: string, payload: RoomUpdateInput, signal?: AbortSignal) =>
    client.put<RoomSummary, RoomUpdateInput>(roomPath(roomId), payload, options(signal)),
  remove: (roomId: string, signal?: AbortSignal) => client.delete<void>(roomPath(roomId), options(signal))
}
