import { defineStore } from 'pinia';
import { ref, watch } from 'vue';
import { musicApi } from '../api/music';
import { useUserStore } from './user';

const defaultPlatforms = [
    { id: 'netease', label: 'netease', supportsAlbumSearch: true },
    { id: 'bilibili', label: 'bilibili', supportsAlbumSearch: false }
];

export const useMusicStore = defineStore('music', () => {
    const userStore = useUserStore();
    const platforms = ref([...defaultPlatforms]);
    const isLoading = ref(false);
    let loadInFlight = null;

    const loadPlatforms = async () => {
        if (loadInFlight) {
            return await loadInFlight;
        }

        isLoading.value = true;
        loadInFlight = (async () => {
            try {
                const data = await musicApi.getPlatforms(userStore.sessionToken);
                platforms.value = Array.isArray(data) && data.length > 0 ? data : [...defaultPlatforms];
            } catch (e) {
                console.warn('Load platforms failed:', e);
                platforms.value = [...defaultPlatforms];
            } finally {
                loadInFlight = null;
                isLoading.value = false;
            }
        })();

        return await loadInFlight;
    };

    const refreshPlatforms = async () => {
        await loadPlatforms();
    };

    // 监听用户身份变化，自动刷新可用平台
    watch(
        () => [userStore.currentUser.name, userStore.isGuest, userStore.sessionToken],
        () => {
            refreshPlatforms();
        }
    );

    return {
        platforms,
        isLoading,
        loadPlatforms,
        refreshPlatforms
    };
});

