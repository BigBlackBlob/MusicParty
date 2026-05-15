import { ref } from 'vue';
import { useI18n } from 'vue-i18n';
import { useToast } from './useToast';
import { musicApi } from '../api/music.js';
import { authApi } from '../api/auth.js';
import { extractErrorMessage } from '../utils/errors.js';
import { useUserStore } from '../stores/user.js';
import { usePlatforms } from './usePlatforms.js';

export function useSearchLogic(emit = () => {}) {
    const { success, error } = useToast();
    const { t } = useI18n();
    const userStore = useUserStore();

    const platform = ref('netease');
    const { platforms, supportsAlbumSearch, loadPlatforms } = usePlatforms(platform);
    const keyword = ref('');
    const songs = ref([]);
    const albums = ref([]);
    const loading = ref(false);
    const hasSubmittedSearch = ref(false);
    const listMode = ref('search'); // 'search' | 'playlist' | 'albumSearch' | 'album'
    const searchType = ref('song'); // 'song' | 'album'
    const isAdminMode = ref(false);

    // 存储原始的管理员指令
    const adminCommand = ref('');

    const handleAdminCommand = async (pwd) => {
        try {
            await authApi.adminCommand(pwd, adminCommand.value);
            success(t('search.adminExecuted'));
            emit('close');
        } catch (e) {
            console.error('Admin command failed:', e);
            error(extractErrorMessage(e, t('search.adminFailed')));
        } finally {
            isAdminMode.value = false;
            keyword.value = '';
            adminCommand.value = '';
        }
    };

    const doSearch = async () => {
        const val = keyword.value.trim();
        if (!val) return;

        // 1. 管理员密码输入模式
        if (isAdminMode.value) {
            hasSubmittedSearch.value = true;
            await handleAdminCommand(val);
            return;
        }

        if (val.startsWith('//')) {
            isAdminMode.value = true;
            adminCommand.value = val; // 保存完整指令
            keyword.value = ''; // 清空输入框，准备输入密码
            return;
        }


        // 3. 普通搜索
        listMode.value = searchType.value === 'album' && supportsAlbumSearch.value ? 'albumSearch' : 'search';
        hasSubmittedSearch.value = true;
        loading.value = true;
        songs.value = [];
        albums.value = [];
        try {
            if (searchType.value === 'album' && supportsAlbumSearch.value) {
                albums.value = await musicApi.searchNeteaseAlbums(val);
            } else {
                const data = await musicApi.search(platform.value, val, userStore.sessionToken);
                songs.value = data;
                const missingCoverCount = Array.isArray(data)
                    ? data.filter(song => !song?.coverUrl || !String(song.coverUrl).trim()).length
                    : 0;
                if (missingCoverCount > 0) {
                    console.warn(`[search] ${platform.value} results missing coverUrl: ${missingCoverCount}/${data.length}`);
                }
            }
        } catch (e) {
            console.error('Search failed:', e);
            error(extractErrorMessage(e, t('search.failed')));
        } finally {
            loading.value = false;
        }
    };

    return {
        platform,
        platforms,
        supportsAlbumSearch,
        loadPlatforms,
        keyword,
        songs,
        albums,
        loading,
        listMode,
        searchType,
        isAdminMode,
        hasSubmittedSearch,
        doSearch
    };
}

