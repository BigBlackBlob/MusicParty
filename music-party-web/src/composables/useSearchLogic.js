import { ref, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { useToast } from './useToast';
import { musicApi } from '../api/music.js';
import { extractErrorMessage } from '../utils/errors.js';
import { useUserStore } from '../stores/user.js';
import { usePlayerStore } from '../stores/player.js';
import { useRoomStore } from '../stores/room.js';
import { usePlatforms } from './usePlatforms.js';
import { useExternalPlaylist, parseNeteasePlaylistId } from './useExternalPlaylist.js';

export function useSearchLogic() {
    const { success, error } = useToast();
    const { t } = useI18n();
    const userStore = useUserStore();
    const playerStore = usePlayerStore();
    const roomStore = useRoomStore();

    const SONGS_CACHE_KEY = 'mp_search_songs';
    const ALBUMS_CACHE_KEY = 'mp_search_albums';

    const platform = ref('netease');
    const { platforms, supportsAlbumSearch, loadPlatforms } = usePlatforms(platform);
    const keyword = ref('');
    const songs = ref(JSON.parse(localStorage.getItem(SONGS_CACHE_KEY) || '[]'));
    const albums = ref(JSON.parse(localStorage.getItem(ALBUMS_CACHE_KEY) || '[]'));
    const {
        playlistSongs,
        playlistId,
        currentPlaylistPage,
        canGoPlaylistNext,
        loadNeteasePlaylistPage,
        clearExternalPlaylist
    } = useExternalPlaylist();

    const loading = ref(false);
    const hasSubmittedSearch = ref(false);
    const currentPage = ref(1);
    const canGoNext = ref(true);
    const SEARCH_LIMIT = 20;

    const listMode = ref('search'); // 'search' | 'playlist' | 'albumSearch' | 'album'
    const searchType = ref('song'); // 'song' | 'album' | 'playlist'

    const albumSongs = ref({}); // albumId -> songs[]
    const loadingAlbumIds = ref(new Set());
    const expandedAlbumIds = ref(new Set());
    let searchRequestSeq = 0;

    watch(platform, (newPlatform) => {
        if (newPlatform === 'local') {
            keyword.value = '';
            doSearch(1);
        }
    });

const doSearch = async (page = 1) => {
        const requestSeq = ++searchRequestSeq;
        const val = keyword.value.trim();
        const isLocal = platform.value === 'local';
        if (!val && searchType.value !== 'playlist' && !isLocal) return;

        const effectiveKeyword = (!val && isLocal) ? ' ' : val;

        // 1. 歌单解析逻辑
        if (searchType.value === 'playlist') {
            const id = parseNeteasePlaylistId(val) || playlistId.value;
            if (!id) {
                error(t('search.invalidPlaylistId'));
                return;
            }

            platform.value = 'netease';

            try {
                loading.value = true;
                await loadNeteasePlaylistPage(id, page);
                if (requestSeq !== searchRequestSeq) return;
                listMode.value = 'playlist';
                hasSubmittedSearch.value = true;
                return;
            } catch (e) {
                error(extractErrorMessage(e, t('search.playlistFailed')));
            } finally {
                if (requestSeq === searchRequestSeq) loading.value = false;
            }
            return;
        }


        // 2. 普通搜索
        currentPage.value = page;
        const offset = (page - 1) * SEARCH_LIMIT;

        listMode.value = searchType.value === 'album' && supportsAlbumSearch.value ? 'albumSearch' : 'search';
        hasSubmittedSearch.value = true;
        loading.value = true;
        expandedAlbumIds.value = new Set();
        loadingAlbumIds.value = new Set();
        
        try {
            if (searchType.value === 'album' && supportsAlbumSearch.value) {
                const data = await musicApi.searchAlbums(platform.value, val, userStore.sessionToken, roomStore.currentRoomId);
                if (requestSeq !== searchRequestSeq) return;
                albums.value = data;
                localStorage.setItem(ALBUMS_CACHE_KEY, JSON.stringify(data));
                canGoNext.value = false;
            } else {
                const data = await musicApi.search(platform.value, effectiveKeyword, userStore.sessionToken, offset, SEARCH_LIMIT, roomStore.currentRoomId);
                if (requestSeq !== searchRequestSeq) return;
                songs.value = data;
                localStorage.setItem(SONGS_CACHE_KEY, JSON.stringify(data));
                
                // 如果返回的数量达到 Limit，假设还有下一页
                canGoNext.value = data.length === SEARCH_LIMIT;

                const missingCoverCount = Array.isArray(data)
                    ? data.filter(song => !song?.coverUrl || !String(song.coverUrl).trim()).length
                    : 0;
                if (missingCoverCount > 0) {
                    console.warn(`[search] ${platform.value} results missing coverUrl: ${missingCoverCount}/${data.length}`);
                }
            }
        } catch (e) {
            if (requestSeq !== searchRequestSeq) return;
            console.error('Search failed:', e);
            error(extractErrorMessage(e, t('search.failed')));
        } finally {
            if (requestSeq === searchRequestSeq) loading.value = false;
        }
    };

    const clearSearchState = () => {
        songs.value = [];
        albums.value = [];
        clearExternalPlaylist();
        albumSongs.value = {};
        loadingAlbumIds.value = new Set();
        expandedAlbumIds.value = new Set();
        hasSubmittedSearch.value = false;
        currentPage.value = 1;
        localStorage.removeItem(SONGS_CACHE_KEY);
        localStorage.removeItem(ALBUMS_CACHE_KEY);
    };

    const refreshPlatformsAndClear = async () => {
        await loadPlatforms(true);
        clearSearchState();
    };

    const nextPage = () => {
        if (loading.value) return;
        if (searchType.value === 'playlist') {
            if (!canGoPlaylistNext.value) return;
            doSearch(currentPlaylistPage.value + 1);
        } else {
            if (!canGoNext.value) return;
            doSearch(currentPage.value + 1);
        }
    };

    const prevPage = () => {
        if (loading.value) return;
        if (searchType.value === 'playlist') {
            if (currentPlaylistPage.value <= 1) return;
            doSearch(currentPlaylistPage.value - 1);
        } else {
            if (currentPage.value <= 1) return;
            doSearch(currentPage.value - 1);
        }
    };

    const addAllPlaylistSongs = () => {
        if (!playlistId.value) return;
        playerStore.enqueuePlaylist('netease', playlistId.value);
        success(t('search.addingPlaylist'));
    };

    const toggleAlbum = async (albumId) => {
        if (expandedAlbumIds.value.has(albumId)) {
            expandedAlbumIds.value.delete(albumId);
            return;
        }

        expandedAlbumIds.value.add(albumId);
        if (albumSongs.value[albumId]) return;

        try {
            loadingAlbumIds.value.add(albumId);
            const data = await musicApi.getAlbumSongs(platform.value, albumId, userStore.sessionToken, roomStore.currentRoomId);
            albumSongs.value[albumId] = data;
        } catch (e) {
            console.error('Failed to load album songs:', e);
            error(extractErrorMessage(e, t('search.failed')));
            expandedAlbumIds.value.delete(albumId);
        } finally {
            loadingAlbumIds.value.delete(albumId);
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
        playlistSongs,
        playlistId,
        loading,
        listMode,
        searchType,
        hasSubmittedSearch,
        currentPage,
        currentPlaylistPage,
        canGoNext,
        canGoPlaylistNext,
        albumSongs,
        loadingAlbumIds,
        expandedAlbumIds,
        doSearch,
        refreshPlatformsAndClear,
        nextPage,
        prevPage,
        addAllPlaylistSongs,
        toggleAlbum
    };
}

