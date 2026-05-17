import { computed, onMounted } from 'vue';
import { useMusicStore } from '../stores/music.js';

export function usePlatforms(selectedPlatform) {
  const musicStore = useMusicStore();

  const platforms = computed(() => musicStore.platforms);
  const currentPlatform = computed(() => platforms.value.find(item => item.id === selectedPlatform.value));
  const supportsAlbumSearch = computed(() => !!currentPlatform.value?.supportsAlbumSearch);

  const loadPlatforms = async (force = false) => {
    await musicStore.loadPlatforms(force);

    if (!platforms.value.some(item => item.id === selectedPlatform.value)) {
      selectedPlatform.value = platforms.value[0]?.id || 'netease';
    }
  };

  onMounted(() => {
    if (musicStore.platforms.length <= 2) { // 只有默认平台时才主动加载
      loadPlatforms();
    }
  });

  return {
    platforms,
    supportsAlbumSearch,
    loadPlatforms
  };
}
