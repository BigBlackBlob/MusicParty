import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import type { PresenceSnapshot, UserSummary } from '../../contracts/generated/models'

export type PresenceStatus = 'idle' | 'synchronized'

function normalizeUsers(users: UserSummary[]): UserSummary[] {
  const seen = new Set<string>()
  return users.filter((user) => {
    if (!user.publicId || seen.has(user.publicId)) return false
    seen.add(user.publicId)
    return true
  })
}

export const useRoomPresenceStore = defineStore('room-presence', () => {
  const roomId = ref('')
  const revision = ref(0)
  const users = ref<UserSummary[]>([])
  const status = ref<PresenceStatus>('idle')

  const count = computed(() => users.value.length)

  function reset(nextRoomId = ''): void {
    roomId.value = nextRoomId
    revision.value = 0
    users.value = []
    status.value = 'idle'
  }

  function apply(snapshot: PresenceSnapshot, activeRoomId: string): boolean {
    if (snapshot.roomId !== activeRoomId || (roomId.value && roomId.value !== snapshot.roomId)) return false
    if (!Number.isSafeInteger(snapshot.revision) || snapshot.revision <= revision.value) return false

    roomId.value = snapshot.roomId
    revision.value = snapshot.revision
    users.value = normalizeUsers(snapshot.users)
    status.value = 'synchronized'
    return true
  }

  function resolveName(publicId: string, fallbackName?: string): string {
    return users.value.find(user => user.publicId === publicId)?.name || fallbackName || 'Unknown Agent'
  }

  return { roomId, revision, users, status, count, reset, apply, resolveName }
})
