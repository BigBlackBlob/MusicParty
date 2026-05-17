// src/composables/useAudio.js

import { ref, onMounted, onUnmounted, watch } from 'vue';
import { isUserGestureRequiredError } from '../utils/audioPlayback';

const SMALL_DRIFT_MS = 250;
const RATE_CORRECTION_DRIFT_MS = 2000;
const BACKGROUND_HARD_SEEK_DRIFT_MS = 10000;
const TRANSITION_FADE_MS = 1500;
const SUPPORTED_TRANSITION_PLATFORMS = new Set(['netease', 'bilibili']);

export function useAudio(audioRef, playerStore, userVolumeRef) {
    const localProgress = ref(0);
    const isBuffering = ref(false);
    const retryCount = ref(0);
    const isErrorState = ref(false);
    const needsUserGesture = ref(false);
    let syncTimer = null;
    let wakeLock = null;
    let smoothSeekInFlight = false;
    let pingTimer = null;
    let fadeGain = 1;
    let activeFadeToken = 0;
    let transitionFadeInPending = false;

    const getUserVolume = () => {
        const value = userVolumeRef?.value;
        return Number.isFinite(value) ? Math.max(0, Math.min(1, value)) : 1;
    };

    const applyEffectiveVolume = () => {
        if (!audioRef.value) return;
        audioRef.value.volume = Math.max(0, Math.min(1, getUserVolume() * fadeGain));
    };

    // 请求唤醒锁 (防止 WebSocket 断连)
    const requestWakeLock = async () => {
        if ('wakeLock' in navigator) {
            try {
                wakeLock = await navigator.wakeLock.request('screen');
                console.log('Wake Lock active');
            } catch (err) {
                console.warn('Wake Lock request failed:', err);
            }
        }
    };

    // 释放唤醒锁
    const releaseWakeLock = async () => {
        if (wakeLock !== null) {
            await wakeLock.release();
            wakeLock = null;
        }
    };

    const waitForAudioReady = (audio, timeoutMs = 800) => new Promise((resolve) => {
        let done = false;
        const finish = () => {
            if (done) return;
            done = true;
            audio.removeEventListener('seeked', finish);
            audio.removeEventListener('canplay', finish);
            clearTimeout(timer);
            resolve();
        };
        const timer = setTimeout(finish, timeoutMs);
        audio.addEventListener('seeked', finish, { once: true });
        audio.addEventListener('canplay', finish, { once: true });
    });

    const fadeToGain = (targetGain, durationMs) => new Promise((resolve) => {
        const token = ++activeFadeToken;
        const startGain = fadeGain;
        const startedAt = performance.now();
        const step = (now) => {
            if (token !== activeFadeToken) {
                resolve(false);
                return;
            }
            const progress = Math.min(1, (now - startedAt) / durationMs);
            fadeGain = startGain + (targetGain - startGain) * progress;
            applyEffectiveVolume();
            if (progress < 1) {
                requestAnimationFrame(step);
            } else {
                resolve(true);
            }
        };
        requestAnimationFrame(step);
    });

    const setFadeGain = (nextGain) => {
        activeFadeToken++;
        fadeGain = Math.max(0, Math.min(1, nextGain));
        applyEffectiveVolume();
    };

    const smoothSeekTo = async (targetSeconds) => {
        const audio = audioRef.value;
        if (!audio || smoothSeekInFlight) return;
        smoothSeekInFlight = true;

        const restoreGain = fadeGain;
        try {
            await fadeToGain(0, 120);
            audio.currentTime = targetSeconds;
            await waitForAudioReady(audio);
            await fadeToGain(restoreGain, 160);
        } catch {
            if (audioRef.value) {
                audioRef.value.currentTime = targetSeconds;
                setFadeGain(restoreGain);
            }
        } finally {
            setFadeGain(restoreGain);
            smoothSeekInFlight = false;
        }
    };

    const supportsTransitionFade = () => {
        const platform = playerStore.nowPlaying?.music?.platform;
        return SUPPORTED_TRANSITION_PLATFORMS.has(platform);
    };

    const startTrackFadeIn = async () => {
        if (!supportsTransitionFade()) {
            setFadeGain(1);
            return;
        }
        setFadeGain(0);
        await fadeToGain(1, TRANSITION_FADE_MS);
    };

    const updateTransitionFadeOut = (targetTime) => {
        if (!audioRef.value || playerStore.isPaused || smoothSeekInFlight || !supportsTransitionFade()) return;
        const duration = playerStore.nowPlaying?.music?.duration || 0;
        if (!duration || duration <= TRANSITION_FADE_MS) return;
        const remainingMs = duration - targetTime;
        if (remainingMs <= TRANSITION_FADE_MS && remainingMs >= 0) {
            const nextGain = Math.max(0, Math.min(1, remainingMs / TRANSITION_FADE_MS));
            if (nextGain < fadeGain) {
                fadeGain = nextGain;
                applyEffectiveVolume();
            }
        } else if (!transitionFadeInPending && fadeGain < 1) {
            setFadeGain(1);
        }
    };

    // 更新系统媒体中心 (锁屏控制)
    const updateMediaSession = () => {
        if (!('mediaSession' in navigator) || !playerStore.nowPlaying) return;

        const music = playerStore.nowPlaying.music;

        // 1. 设置元数据
        navigator.mediaSession.metadata = new MediaMetadata({
            title: music.name,
            artist: music.artists.join(' / '),
            artwork: [
                { src: music.coverUrl, sizes: '512x512', type: 'image/png' }
            ]
        });

        // 2. 注册控制事件 (关键：告诉系统我们支持后台控制)
        // 这样点击锁屏的下一首/暂停，会通过 WebSocket 发送给服务器
        try {
            navigator.mediaSession.setActionHandler('play', () => playerStore.togglePause());
            navigator.mediaSession.setActionHandler('pause', () => playerStore.togglePause());
            navigator.mediaSession.setActionHandler('previoustrack', null); // 暂不支持上一首
            navigator.mediaSession.setActionHandler('nexttrack', () => playerStore.playNext());
        } catch (e) {
            console.warn('Media Session actions warning:', e);
        }
    };

    // 尝试播放并处理浏览器拦截
    const safePlay = async () => {
        if (!audioRef.value || !playerStore.nowPlaying) return;

        try {
            await audioRef.value.play();
            needsUserGesture.value = false;
            isErrorState.value = false;
            updateMediaSession();
            requestWakeLock();
            if (transitionFadeInPending) {
                transitionFadeInPending = false;
                startTrackFadeIn();
            }
        } catch (e) {
            // NotAllowedError 是浏览器由于缺乏用户交互而拦截
            if (isUserGestureRequiredError(e)) {
                needsUserGesture.value = true;
                console.warn("Autoplay blocked. User interaction required.");
            } else if (e.name !== 'AbortError') {
                console.warn("Play failed:", e);
            }
        }
    };

    // === 1. 监听资源加载 (canplay) ===
    // 这是修复你问题的关键：音频加载就绪后，主动判断是否需要播放
    const checkAutoPlay = () => {
        if (!playerStore.nowPlaying) return;
        isBuffering.value = false;

        if (playerStore.isPaused) {
            audioRef.value.pause();
        } else {
            safePlay();
        }
    };

    // === 2. 监听后端状态变化 ===
    watch(() => playerStore.isPaused, (newPaused) => {
        if (!audioRef.value) return;
        if (newPaused) {
            audioRef.value.pause();
            navigator.mediaSession.playbackState = 'paused';
            setFadeGain(1);
            releaseWakeLock();
        } else {
            safePlay();
            navigator.mediaSession.playbackState = 'playing';
        }
    });

    // === 3. 监听切歌 ===
    watch(() => playerStore.nowPlaying?.music?.id, () => {
        // 更新媒体中心信息 (锁屏显示)
        if ('mediaSession' in navigator && playerStore.nowPlaying) {
            const music = playerStore.nowPlaying.music;
            navigator.mediaSession.metadata = new MediaMetadata({
                title: music.name,
                artist: music.artists.join(' / '),
                artwork: [{ src: music.coverUrl, sizes: '512x512', type: 'image/png' }]
            });
        }

            retryCount.value = 0;
            isErrorState.value = false;
            needsUserGesture.value = false;
        transitionFadeInPending = supportsTransitionFade();
        setFadeGain(transitionFadeInPending ? 0 : 1);
        // 切歌会导致 src 变化，自动触发 load -> canplay -> checkAutoPlay
        // 所以这里不需要手动 call play
        updateMediaSession();
    }, { immediate: true });

    if (userVolumeRef) {
        watch(userVolumeRef, applyEffectiveVolume, { immediate: true });
    }

    // === 4. 错误重试机制 ===
    const handleError = () => {
        if (!playerStore.nowPlaying?.music?.url) return;

        // 忽略由切换 src 导致的中断错误
        if (audioRef.value && audioRef.value.error && audioRef.value.error.code === 20) return;

        isBuffering.value = false;
        if (audioRef.value) {
            console.warn('[Audio] media error', {
                code: audioRef.value.error?.code,
                message: audioRef.value.error?.message,
                networkState: audioRef.value.networkState,
                readyState: audioRef.value.readyState,
                currentSrc: audioRef.value.currentSrc,
                platform: playerStore.nowPlaying?.music?.platform
            });
        }
        if (retryCount.value >= 3) {
            isErrorState.value = true;
            return;
        }

        retryCount.value++;
        console.log(`Retry audio (${retryCount.value})...`);
        setTimeout(() => {
            if (audioRef.value) {
                audioRef.value.load();
                // load 完会触发 canplay，进而触发 checkAutoPlay
            }
        }, 1500);
    };

    // 页面可见性变化监听
    // 当重新回到前台时，如果发现 WebSocket 断了，应该自动重连
    // 这里主要处理 Wake Lock 的重新获取以及 Socket 的快速恢复
    const handleVisibilityChange = async () => {
        if (document.visibilityState === 'visible') {
            // 1. 尝试恢复 Wake Lock
            if (!playerStore.isPaused) {
                await requestWakeLock();
            }
            // 2. 检查连接状态，必要时重连
            playerStore.tryReconnect();
            playerStore.requestSyncRefresh('visible', true);
        }
    };
    
    // 网络状态监听
    const handleNetworkChange = () => {
        if (navigator.onLine) {
            console.log('[Network] Back online, checking socket...');
            playerStore.tryReconnect();
            playerStore.requestSyncRefresh('online', true);
        }
    };

    // === 5. 进度条同步 ===
    onMounted(() => {
        document.addEventListener('visibilitychange', handleVisibilityChange);
        window.addEventListener('online', handleNetworkChange);
        pingTimer = setInterval(() => playerStore.requestPing('interval'), 10000);

        syncTimer = setInterval(() => {
            if (!playerStore.nowPlaying) {
                localProgress.value = 0;
                if (audioRef.value) audioRef.value.playbackRate = 1;
                setFadeGain(1);
                return;
            }

            // 1. 获取理论上的正确进度
            const targetTime = playerStore.getCurrentProgress();

            // UI 进度统一使用服务端锚点推导出的房间时钟。
            // audio.currentTime 只用于纠偏，不再作为歌词/进度条主时间源，避免缓冲、
            // smooth seek 或媒体元素状态抖动时把歌词时间带回旧位置。
            localProgress.value = targetTime;
            playerStore.setPlaybackPosition(targetTime);
            updateTransitionFadeOut(targetTime);

            // 3. 强行同步逻辑 (纠偏)
            if (audioRef.value && !isBuffering.value && !isErrorState.value && !playerStore.isSeekingPreview) {
                // 如果是暂停状态，强制对齐
                if (playerStore.isPaused) {
                    audioRef.value.playbackRate = 1;
                    // 避免重复赋值导致杂音
                    if (Math.abs(audioRef.value.currentTime * 1000 - targetTime) > 200) {
                        audioRef.value.currentTime = targetTime / 1000;
                    }
                }
                // 如果是播放状态，只有偏差过大才对齐
                else {
                    const domTime = audioRef.value.currentTime * 1000;
                    const drift = targetTime - domTime;
                    const absDrift = Math.abs(drift);

                    if (playerStore.forceNextSyncSeek && audioRef.value.readyState >= 2) {
                        audioRef.value.playbackRate = 1;
                        audioRef.value.currentTime = targetTime / 1000;
                        playerStore.forceNextSyncSeek = false;
                    } else if (absDrift <= SMALL_DRIFT_MS) {
                        audioRef.value.playbackRate = 1;
                    } else if (absDrift <= RATE_CORRECTION_DRIFT_MS) {
                        audioRef.value.playbackRate = drift > 0 ? 1.03 : 0.97;
                    } else if (document.hidden) {
                        audioRef.value.playbackRate = 1;
                        if (absDrift > BACKGROUND_HARD_SEEK_DRIFT_MS && audioRef.value.readyState >= 2) {
                            console.log(`[Sync] Background hard seek: ${domTime} -> ${targetTime}`);
                            audioRef.value.currentTime = targetTime / 1000;
                        }
                    } else if (audioRef.value.readyState >= 2) {
                        audioRef.value.playbackRate = 1;
                        if (!smoothSeekInFlight) {
                            console.log(`[Sync] Smooth seek: ${domTime} -> ${targetTime}`);
                            smoothSeekTo(targetTime / 1000);
                        }
                    }
                }
            } else if (audioRef.value) {
                audioRef.value.playbackRate = 1;
            }
        }, 200);
    });

    onUnmounted(() => {
        document.removeEventListener('visibilitychange', handleVisibilityChange);
        window.removeEventListener('online', handleNetworkChange);
        clearInterval(syncTimer);
        clearInterval(pingTimer);
        if (audioRef.value) audioRef.value.playbackRate = 1;
        setFadeGain(1);
        releaseWakeLock();
    });

    return {
        localProgress,
        isBuffering,
        isErrorState,
        retryCount,
        needsUserGesture,
        safePlay,
        handleError,
        checkAutoPlay
    };
}
