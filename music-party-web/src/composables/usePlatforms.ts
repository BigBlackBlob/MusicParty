import { computed, watch, type Ref } from 'vue'
import { useQuery } from '@tanstack/vue-query'
import type { PlatformDescriptor } from '../contracts/generated/models'
import { musicApi } from '../api/music'
import { queryKeys } from '../domains/queryKeys'
import { useRoomStore } from '../stores/room'

const defaultPlatforms: PlatformDescriptor[] = [
  { id: 'netease', label: 'netease', supportsAlbumSearch: true, subsonic: false },
  { id: 'bilibili', label: 'bilibili', supportsAlbumSearch: false, subsonic: false }
]

export function usePlatforms(selectedPlatform: Ref<string>) {
  const roomStore = useRoomStore()
  const roomId = computed(() => roomStore.currentRoomId)
  const query = useQuery({
    queryKey: computed(() => queryKeys.platforms.byRoom(roomId.value)),
    queryFn: ({ signal }) => musicApi.getPlatforms(roomId.value, signal)
  })
  const platforms = computed(() => query.data.value?.length ? query.data.value : defaultPlatforms)
  const currentPlatform = computed(() => platforms.value.find(item => item.id === selectedPlatform.value))
  const supportsAlbumSearch = computed(() => currentPlatform.value?.supportsAlbumSearch ?? false)

  watch(platforms, (available) => {
    if (!available.some(item => item.id === selectedPlatform.value)) {
      selectedPlatform.value = available[0]?.id ?? 'netease'
    }
  }, { immediate: true })

  async function loadPlatforms(force = false): Promise<PlatformDescriptor[]> {
    if (force) await query.refetch()
    else if (!query.data.value) await query.refetch()
    return platforms.value
  }

  return { platforms, supportsAlbumSearch, loadPlatforms, isLoading: query.isLoading, error: query.error }
}
