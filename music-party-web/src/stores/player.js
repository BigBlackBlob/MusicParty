// src/stores/player.js

import { defineStore } from 'pinia';
import { ref, watch } from 'vue';
import { useUserStore } from './user';
import { socketService } from '../services/socket';
import { createSocketSubscriptions, createSocketCallbacks } from '../services/socketHandler'; // 引入新文件
import { musicApi } from '../api/music';
import { WS_DEST } from '../constants/api';
import { STORAGE_KEYS } from '../constants/keys';

export const usePlayerStore = defineStore('player', () => {
    // === 1. State ===
    const nowPlaying = ref(null);
    const queue = ref([]);
    const isPaused = ref(false);
    const isShuffle = ref(false);
    const isPauseLocked = ref(false);
    const isSkipLocked = ref(false);
    const isShuffleLocked = ref(false);
    const lyricText = ref('');
    const lyricDetail = ref({
        lyric: '',
        translatedLyric: '',
        romanizedLyric: ''
    });
    const likedSongs = ref([]);
    const connected = ref(false);
    const isLoading = ref(false);
    const streamListenerCount = ref(0);
    const lastControlTime = ref(0);
    const remotePosition = ref(0);
    const lastSyncTime = ref(0);
    const serverClockOffset = ref(0);
    const localProgress = ref(0);
    const isBuffering = ref(false);
    const isErrorState = ref(false);
    const isSeekingPreview = ref(false);

    const userStore = useUserStore();
    const LOCAL_COOLDOWN = 500; // 稍微调低一点冷却时间提升手感

    try {
        const savedLikedSongs = JSON.parse(localStorage.getItem(STORAGE_KEYS.LIKED_SONGS) || '[]');
        likedSongs.value = Array.isArray(savedLikedSongs) ? savedLikedSongs : [];
    } catch (e) {
        likedSongs.value = [];
    }

    // === 2. Logic ===
    const getCurrentProgress = () => {
        if (!nowPlaying.value) return 0;
        if (isPaused.value) {
            return remotePosition.value;
        } else {
            return remotePosition.value + ((Date.now() + serverClockOffset.value) - lastSyncTime.value);
        }
    };

    const requireAuth = () => {
        if (userStore.isGuest) {
            userStore.showNameModal = true;
            return false;
        }
        return true;
    };

    const checkCooldown = () => {
        const now = Date.now();
        if (now - lastControlTime.value < LOCAL_COOLDOWN) {
            // 这里可以不再弹 Toast 报错，而是静默失败，避免刷屏
            return false;
        }
        lastControlTime.value = now;
        return true;
    };

    // === 3. Actions ===

    const syncState = (state) => {
        nowPlaying.value = state.nowPlaying;
        queue.value = state.queue;
        isPaused.value = state.isPaused;
        isShuffle.value = state.isShuffle;
        isPauseLocked.value = state.isPauseLocked || false;
        isSkipLocked.value = state.isSkipLocked || false;
        isShuffleLocked.value = state.isShuffleLocked || false;
        isLoading.value = state.isLoading || false;
        streamListenerCount.value = state.streamListenerCount || 0;

        const clientReceiveTime = Date.now();
        const serverTimestamp = state.serverTimestamp || clientReceiveTime;

        // 记录服务器发来的进度和状态包对应的服务端时间
        if (state.nowPlaying) {
            remotePosition.value = state.nowPlaying.currentPosition;
            lastSyncTime.value = serverTimestamp;
            serverClockOffset.value = serverTimestamp - clientReceiveTime;
        } else {
            remotePosition.value = 0;
            lastSyncTime.value = serverTimestamp;
            serverClockOffset.value = serverTimestamp - clientReceiveTime;
        }

        if (state.onlineUsers) {
            userStore.setOnlineUsers(state.onlineUsers);
        }
    };

    const connect = () => {
        const authHeaders = {
            'user-name': localStorage.getItem(STORAGE_KEYS.USERNAME) || '游客',
            'user-token': userStore.userToken,
            'room-password': localStorage.getItem(STORAGE_KEYS.ROOM_PASSWORD) || ''
        };

        // 使用抽离出的订阅配置
        const subscriptions = createSocketSubscriptions();

        // 补充 UserMe 的特殊处理 (因为它需要用到 renameUser，如果放在 socketHandler 会导致循环依赖)
        subscriptions[WS_DEST.USER_ME] = (me) => {
            // me: { token, sessionId, name, isGuest }
            userStore.initUser(me.sessionId, me.name, me.isGuest);
        };

        subscriptions[WS_DEST.USER_ME_UPDATE] = (me) => {
            userStore.initUser(me.sessionId, me.name, me.isGuest);
        };

        const callbacks = createSocketCallbacks();

        socketService.connect(authHeaders, callbacks, subscriptions);
    };

    const tryReconnect = () => {
        if (!connected.value) {
            socketService.forceReconnect();
        }
    };

    // --- 指令发送 ---
    const playNext = () => requireAuth() && checkCooldown() && socketService.send(WS_DEST.PLAYER_NEXT);
    const togglePause = () => requireAuth() && checkCooldown() && socketService.send(WS_DEST.PLAYER_PAUSE);
    const toggleShuffle = () => requireAuth() && checkCooldown() && socketService.send(WS_DEST.PLAYER_SHUFFLE);
    const seek = (positionMs) => requireAuth() && socketService.send(WS_DEST.PLAYER_SEEK, { positionMs });
    const setSeekingPreview = (val) => {
        isSeekingPreview.value = val;
    };

    const enqueue = (platform, musicId) => requireAuth() && socketService.send(WS_DEST.ENQUEUE, { platform, musicId });
    const enqueuePlaylist = (platform, playlistId) => requireAuth() && socketService.send(WS_DEST.ENQUEUE_PLAYLIST, { platform, playlistId });
    const enqueueAlbum = (platform, albumId) => requireAuth() && socketService.send(WS_DEST.ENQUEUE_ALBUM, { platform, albumId });
    const topSong = (queueId) => requireAuth() && socketService.send(WS_DEST.QUEUE_TOP, { queueId });
    const removeSong = (queueId) => requireAuth() && socketService.send(WS_DEST.QUEUE_REMOVE, { queueId });
    const topSongs = (queueIds) => {
        if (!Array.isArray(queueIds) || queueIds.length === 0) return;
        if (!requireAuth()) return false;
        socketService.send(WS_DEST.QUEUE_BATCH_TOP, { queueIds });
        return true;
    };
    const removeSongs = (queueIds) => {
        if (!Array.isArray(queueIds) || queueIds.length === 0) return;
        if (!requireAuth()) return false;
        socketService.send(WS_DEST.QUEUE_BATCH_REMOVE, { queueIds });
        setTimeout(() => socketService.send(WS_DEST.RESYNC), 250);
        return true;
    };
    const topSongsCompat = (queueIds) => {
        if (!Array.isArray(queueIds) || queueIds.length === 0) return;
        if (!requireAuth()) return false;
        queueIds.forEach(queueId => socketService.send(WS_DEST.QUEUE_TOP, { queueId }));
        return true;
    };
    const removeSongsCompat = (queueIds) => {
        if (!Array.isArray(queueIds) || queueIds.length === 0) return;
        if (!requireAuth()) return false;
        queueIds.forEach(queueId => socketService.send(WS_DEST.QUEUE_REMOVE, { queueId }));
        return true;
    };

    const bindAccount = (platform, accountId) => {
        socketService.send(WS_DEST.USER_BIND, { platform, accountId });
        userStore.updateBinding(platform, accountId);
    };

    const renameUser = (newName) => {
        socketService.send(WS_DEST.USER_RENAME, { newName });
        // userStore.saveName(newName) removed; relying on backend sync
    };

    const getSongKey = (music) => music ? `${music.platform}:${music.id}` : '';

    const saveLikedSongs = () => {
        localStorage.setItem(STORAGE_KEYS.LIKED_SONGS, JSON.stringify(likedSongs.value));
    };

    const addLikedSong = (music = nowPlaying.value?.music) => {
        if (!music) return;
        const key = getSongKey(music);
        const nextSong = {
            key,
            id: music.id,
            platform: music.platform,
            name: music.name,
            artists: music.artists || [],
            duration: music.duration || 0,
            coverUrl: music.coverUrl || '',
            likedAt: Date.now()
        };
        likedSongs.value = [nextSong, ...likedSongs.value.filter(song => song.key !== key)].slice(0, 500);
        saveLikedSongs();
    };

    const removeLikedSong = (key) => {
        likedSongs.value = likedSongs.value.filter(song => song.key !== key);
        saveLikedSongs();
    };

    const isSongLiked = (music = nowPlaying.value?.music) => {
        const key = getSongKey(music);
        return !!key && likedSongs.value.some(song => song.key === key);
    };

    const sendLike = () => {
        if (requireAuth()) {
            addLikedSong();
            socketService.send(WS_DEST.PLAYER_LIKE);
        }
    };

    const sendChatMessage = (content) => {
        if (requireAuth()) socketService.send(WS_DEST.CHAT_SEND, { content });
    };

    // 歌词监听
    watch(() => nowPlaying.value?.music?.id, async (newId) => {
        lyricText.value = '';
        lyricDetail.value = { lyric: '', translatedLyric: '', romanizedLyric: '' };
        if (!newId) return;
        try {
            const platform = nowPlaying.value.music.platform;
            const data = await musicApi.getLyricDetail(platform, newId);
            lyricDetail.value = {
                lyric: data?.lyric || '',
                translatedLyric: data?.translatedLyric || '',
                romanizedLyric: data?.romanizedLyric || ''
            };
            lyricText.value = lyricDetail.value.lyric;
        } catch (e) {
            console.error("Lyrics Error", e);
            try {
                const platform = nowPlaying.value.music.platform;
                const fallback = await musicApi.getLyric(platform, newId);
                lyricText.value = fallback || '';
                lyricDetail.value = { lyric: lyricText.value, translatedLyric: '', romanizedLyric: '' };
            } catch (fallbackError) {
                console.error("Lyrics Fallback Error", fallbackError);
            }
        }
    });

    return {
        nowPlaying, queue, isPaused, isShuffle, isPauseLocked, isSkipLocked, isShuffleLocked, connected, isLoading, lyricText, lyricDetail, likedSongs,
        localProgress, isBuffering, isErrorState, streamListenerCount, lastSyncTime,
        isSeekingPreview, setSeekingPreview,
        connect, tryReconnect, getCurrentProgress, syncState, // 导出 syncState
        playNext, togglePause, toggleShuffle,
        seek,
        enqueue, enqueuePlaylist, enqueueAlbum, topSong, removeSong, topSongs, removeSongs, topSongsCompat, removeSongsCompat,
        bindAccount, renameUser, sendChatMessage, sendLike, addLikedSong, removeLikedSong, isSongLiked
    };
});
