import { computed, onMounted, ref, watch } from 'vue';
import { musicApi } from '../api/music.js';
import { useUserStore } from '../stores/user.js';

const defaultPlatforms = [
  { id: 'netease', label: 'netease', supportsAlbumSearch: true },
  { id: 'bilibili', label: 'bilibili', supportsAlbumSearch: false }
];

const platforms = ref([...defaultPlatforms]);
let loadInFlight = null;

export function usePlatforms(selectedPlatform) {
  const userStore = useUserStore();
  const currentPlatform = computed(() => platforms.value.find(item => item.id === selectedPlatform.value));
  const supportsAlbumSearch = computed(() => !!currentPlatform.value?.supportsAlbumSearch);

  const loadPlatforms = async () => {
    if (loadInFlight) {
      await loadInFlight;
    } else {
      loadInFlight = (async () => {
        try {
          const data = await musicApi.getPlatforms(userStore.userToken);
          platforms.value = Array.isArray(data) && data.length > 0 ? data : [...defaultPlatforms];
        } catch (e) {
          console.warn('Load platforms failed:', e);
          platforms.value = [...defaultPlatforms];
        } finally {
          loadInFlight = null;
        }
      })();
      await loadInFlight;
    }

    if (!platforms.value.some(item => item.id === selectedPlatform.value)) {
      selectedPlatform.value = platforms.value[0]?.id || 'netease';
    }
  };

  const refreshPlatforms = async () => {
    try {
      const data = await musicApi.getPlatforms(userStore.userToken);
      platforms.value = Array.isArray(data) && data.length > 0 ? data : [...defaultPlatforms];
    } catch (e) {
      console.warn('Load platforms failed:', e);
      platforms.value = [...defaultPlatforms];
    }

    if (!platforms.value.some(item => item.id === selectedPlatform.value)) {
      selectedPlatform.value = platforms.value[0]?.id || 'netease';
    }
  };

  onMounted(loadPlatforms);
  watch(
    () => [userStore.currentUser.name, userStore.isGuest, userStore.userToken],
    () => refreshPlatforms()
  );

  return {
    platforms,
    supportsAlbumSearch,
    loadPlatforms
  };
}
