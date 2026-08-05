import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

export type QueueMutationKind =
  | 'top'
  | 'remove'
  | 'batch-top'
  | 'batch-remove'
  | 'reorder'
  | 'enqueue'
  | 'enqueue-album'
  | 'enqueue-playlist'

interface PendingMutation {
  kind: QueueMutationKind
  timeoutId: ReturnType<typeof setTimeout>
}

interface BeginMutationOptions {
  mutationId: string
  kind: QueueMutationKind
  timeoutMs: number
  onTimeout: (mutationId: string) => void
}

export const useRoomCommandBus = defineStore('room-command-bus', () => {
  const pending = new Map<string, PendingMutation>()
  const pendingCount = ref(0)

  const hasPendingMutations = computed(() => pendingCount.value > 0)

  function createMutationId(kind: QueueMutationKind): string {
    return `${kind}-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`
  }

  function beginMutation(options: BeginMutationOptions): void {
    settleMutation(options.mutationId)
    const timeoutId = setTimeout(() => {
      if (!pending.delete(options.mutationId)) return
      pendingCount.value = pending.size
      options.onTimeout(options.mutationId)
    }, options.timeoutMs)
    pending.set(options.mutationId, { kind: options.kind, timeoutId })
    pendingCount.value = pending.size
  }

  function settleMutation(mutationId: string): QueueMutationKind | null {
    const mutation = pending.get(mutationId)
    if (!mutation) return null
    clearTimeout(mutation.timeoutId)
    pending.delete(mutationId)
    pendingCount.value = pending.size
    return mutation.kind
  }

  function hasMutation(mutationId: string): boolean {
    return pending.has(mutationId)
  }

  function hasMutationKind(kind: QueueMutationKind): boolean {
    return [...pending.values()].some(mutation => mutation.kind === kind)
  }

  function reset(): void {
    for (const mutation of pending.values()) clearTimeout(mutation.timeoutId)
    pending.clear()
    pendingCount.value = 0
  }

  return {
    pendingCount,
    hasPendingMutations,
    createMutationId,
    beginMutation,
    settleMutation,
    hasMutation,
    hasMutationKind,
    reset
  }
})
