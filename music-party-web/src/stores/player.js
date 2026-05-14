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
    // 歌词缓存，防止重复请求导致的 UI 闪烁和滚动重置
    const lyricCache = new Map();

    const likedSongs = ref([]);
    const connected = ref(false);
    const isLoading = ref(false);
    const streamListenerCount = ref(0);
    const lastControlTime = ref(0);
    const remotePosition = ref(0);
    const lastSyncTime = ref(0);
    const serverClockOffset = ref(0);
    const hasClockSample = ref(false);
    const lastStateVersion = ref(0);
    const lastPlayEpoch = ref(0);
    const lastServerTimestamp = ref(0);
    const forceNextSyncSeek = ref(false);
    const lastPingSentAt = ref(0);
    const lastResyncSentAt = ref(0);
    const lastRttMs = ref(null);
    const localProgress = ref(0);
    const playbackPositionMs = ref(0);
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
        if (isPaused.value) return remotePosition.value;
        return remotePosition.value + ((Date.now() + serverClockOffset.value) - lastSyncTime.value);
    };

    const setPlaybackPosition = (positionMs) => {
        const nextPosition = Number.isFinite(positionMs) ? Math.max(0, positionMs) : 0;
        playbackPositionMs.value = nextPosition;
        localProgress.value = nextPosition;
    };

    const requestPing = (reason = 'manual', force = false) => {
        const now = Date.now();
        if (!force && now - lastPingSentAt.value < 1000) return false;
        lastPingSentAt.value = now;
        socketService.send(WS_DEST.SYNC_PING, {
            pingId: `${reason}-${now}-${Math.random().toString(36).slice(2, 8)}`,
            clientSendTime: now
        });
        return true;
    };

    const requestResync = (reason = 'manual', force = false) => {
        const now = Date.now();
        if (!force && now - lastResyncSentAt.value < 500) return false;
        lastResyncSentAt.value = now;
        socketService.send(WS_DEST.RESYNC, { reason });
        return true;
    };

    const requestSyncRefresh = (reason = 'refresh', force = false) => {
        if (!connected.value) return;
        requestPing(reason, force);
        requestResync(reason, force);
    };

    const handleSyncPong = (pong) => {
        if (!pong || typeof pong.clientSendTime !== 'number' || typeof pong.serverSendTime !== 'number') return;
        const clientReceiveTime = Date.now();
        const rtt = clientReceiveTime - pong.clientSendTime;
        if (rtt < 0 || rtt > 3000) return;
        lastRttMs.value = rtt;

        const sampleOffset = (pong.serverSendTime + rtt / 2) - clientReceiveTime;
        if (hasClockSample.value && Math.abs(sampleOffset - serverClockOffset.value) > 10000) return;

        serverClockOffset.value = hasClockSample.value
            ? serverClockOffset.value * 0.85 + sampleOffset * 0.15
            : sampleOffset;
        hasClockSample.value = true;
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
        if (!state) return;
        const incomingVersion = Number.isFinite(state.stateVersion) ? state.stateVersion : null;
        const serverTimestamp = state.serverTimestamp || Date.now();

        if (incomingVersion !== null) {
            if (incomingVersion < lastStateVersion.value) return;
            if (incomingVersion === lastStateVersion.value && serverTimestamp < lastServerTimestamp.value) return;
            lastStateVersion.value = incomingVersion;
            lastServerTimestamp.value = serverTimestamp;
        }

        const incomingEpoch = state.nowPlaying?.playEpoch ?? state.playEpoch ?? 0;
        if (incomingEpoch !== lastPlayEpoch.value) {
            lastPlayEpoch.value = incomingEpoch;
            forceNextSyncSeek.value = true;
        }

        nowPlaying.value = state.nowPlaying;
        queue.value = state.queue || [];
        isPaused.value = state.isPaused;
        isShuffle.value = state.isShuffle;
        isPauseLocked.value = state.isPauseLocked || false;
        isSkipLocked.value = state.isSkipLocked || false;
        isShuffleLocked.value = state.isShuffleLocked || false;
        isLoading.value = state.isLoading || false;
        streamListenerCount.value = state.streamListenerCount || 0;

        const clientReceiveTime = Date.now();
        if (!hasClockSample.value) {
            serverClockOffset.value = serverTimestamp - clientReceiveTime;
            hasClockSample.value = true;
        }

        // 记录服务器发来的进度和状态包对应的服务端时间
        if (state.nowPlaying) {
            remotePosition.value = state.nowPlaying.currentPosition;
            lastSyncTime.value = serverTimestamp;
        } else {
            remotePosition.value = 0;
            lastSyncTime.value = serverTimestamp;
            setPlaybackPosition(0);
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
        } else {
            requestSyncRefresh('reconnect-check', true);
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
        setTimeout(() => requestResync('batch-remove', true), 250);
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
            const music = nowPlaying.value?.music;
            const key = getSongKey(music);
            if (key && isSongLiked(music)) {
                removeLikedSong(key);
                return;
            }
            addLikedSong(music);
            socketService.send(WS_DEST.PLAYER_LIKE);
        }
    };

    const sendChatMessage = (content) => {
        if (requireAuth()) socketService.send(WS_DEST.CHAT_SEND, { content });
    };

    // 歌词监听
    let lyricRequestId = 0;
    watch(() => `${nowPlaying.value?.music?.platform || ''}:${nowPlaying.value?.music?.id || ''}`, async (newKey, oldKey) => {
        const requestId = ++lyricRequestId;
        const music = nowPlaying.value?.music;
        const newId = music?.id;
        const platform = music?.platform;
        // 如果 ID 没变 (仅仅是进度或其它状态同步)，且已有歌词，绝对不要操作歌词状态
        if (newKey === oldKey && (lyricDetail.value.lyric || lyricCache.has(newKey))) return;

        if (!newId || !platform) {
            lyricText.value = '';
            lyricDetail.value = { lyric: '', translatedLyric: '', romanizedLyric: '' };
            return;
        }

        // 1. 检查缓存
        if (lyricCache.has(newKey)) {
            const cached = lyricCache.get(newKey);
            lyricDetail.value = cached;
            lyricText.value = cached.lyric;
            return;
        }

        // 2. 只有确实换了新歌且没缓存时，才清空旧状态并请求
        lyricText.value = '';
        lyricDetail.value = { lyric: '', translatedLyric: '', romanizedLyric: '' };
        
        try {
            const data = await musicApi.getLyricDetail(platform, newId);
            if (requestId !== lyricRequestId || `${nowPlaying.value?.music?.platform || ''}:${nowPlaying.value?.music?.id || ''}` !== newKey) return;
            const result = {
                lyric: data?.lyric || '',
                translatedLyric: data?.translatedLyric || '',
                romanizedLyric: data?.romanizedLyric || ''
            };
            
            // 写入缓存并更新状态
            lyricCache.set(newKey, result);
            // 限制缓存大小防止内存泄露 (保留最近10首)
            if (lyricCache.size > 10) {
                const firstKey = lyricCache.keys().next().value;
                lyricCache.delete(firstKey);
            }

            lyricDetail.value = result;
            lyricText.value = result.lyric;
        } catch (e) {
            console.error("Lyrics Error", e);
            try {
                const fallback = await musicApi.getLyric(platform, newId);
                if (requestId !== lyricRequestId || `${nowPlaying.value?.music?.platform || ''}:${nowPlaying.value?.music?.id || ''}` !== newKey) return;
                const fallbackResult = { lyric: fallback || '', translatedLyric: '', romanizedLyric: '' };
                lyricCache.set(newKey, fallbackResult);
                lyricDetail.value = fallbackResult;
                lyricText.value = fallbackResult.lyric;
            } catch (fallbackError) {
                console.error("Lyrics Fallback Error", fallbackError);
            }
        }
    });

    return {
        nowPlaying, queue, isPaused, isShuffle, isPauseLocked, isSkipLocked, isShuffleLocked, connected, isLoading, lyricText, lyricDetail, likedSongs,
        localProgress, playbackPositionMs, isBuffering, isErrorState, streamListenerCount, lastSyncTime, lastRttMs,
        isSeekingPreview, forceNextSyncSeek, setSeekingPreview,
        setPlaybackPosition,
        connect, tryReconnect, getCurrentProgress, syncState, handleSyncPong, requestPing, requestResync, requestSyncRefresh,
        playNext, togglePause, toggleShuffle,
        seek,
        enqueue, enqueuePlaylist, enqueueAlbum, topSong, removeSong, topSongs, removeSongs, topSongsCompat, removeSongsCompat,
        bindAccount, renameUser, sendChatMessage, sendLike, addLikedSong, removeLikedSong, isSongLiked
    };
});
