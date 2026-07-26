// src/composables/useAudio.js

import { ref, onMounted, onUnmounted, watch } from 'vue';
import { isUserGestureRequiredError } from '../utils/audioPlayback';

const SMALL_DRIFT_MS = 250;
const RATE_CORRECTION_DRIFT_MS = 2000;
const BACKGROUND_HARD_SEEK_DRIFT_MS = 10000;
const TRANSITION_FADE_MS = 1500;
// 分级软重试：L1 软等待 → L2 软重试(保留缓冲) → L3 硬重载
// L1: 卡住后只显示缓冲遮罩，不动 audio，给浏览器自我恢复的机会
const STALLED_L1_SOFT_WAIT_MS = 2500;
// L2: L1 没恢复，重新发同一个网络请求（浏览器会做 Range 续传补缺口），保留 currentTime 和已缓冲区间
const STALLED_L2_SOFT_RETRY_MS = 3000;
// L3: L2 也没恢复，才 load() 核弹重拉（会清空缓冲、currentTime 归零）
const STALLED_L3_HARD_RELOAD_MS = 5000;
const STALLED_RETRY_DELAY_MS = 800;
const STALLED_MAX_RETRIES = 2;
const SUPPORTED_TRANSITION_PLATFORMS = new Set(['netease', 'bilibili', 'youtube']);

export function useAudio(audioRef, playerStore, userVolumeRef) {
    const localProgress = ref(0);
    const isBuffering = ref(false);
    const bufferedMs = ref(0);
    const retryCount = ref(0);
    const isErrorState = ref(false);
    const needsUserGesture = ref(false);
    let syncTimer = null;
    let wakeLock = null;
    let smoothSeekInFlight = false;
    let fadeGain = 1;
    let activeFadeToken = 0;
    let transitionFadeInPending = false;
    let stalledTimer = null;
    let stalledRetryCount = 0;
    // 重载音轨后用于恢复进度的目标位置，避免从 0 开始播放
    let pendingResumePositionMs = null;
    // 跟踪所有重试相关的定时器，切歌/卸载时统一清理，避免在错误的音轨上 load()
    const retryTimers = new Set();
    const scheduleRetry = (fn, delay) => {
        const id = setTimeout(() => {
            retryTimers.delete(id);
            fn();
        }, delay);
        retryTimers.add(id);
        return id;
    };
    const clearRetryTimers = () => {
        for (const id of retryTimers) {
            clearTimeout(id);
        }
        retryTimers.clear();
    };

    // 读取媒体元素当前已缓冲到的时间（毫秒），用于在进度条上展示缓冲进度
    const updateBufferedMs = () => {
        const audio = audioRef.value;
        if (!audio) {
            bufferedMs.value = 0;
            return;
        }
        try {
            const ranges = audio.buffered;
            if (!ranges || ranges.length === 0) return;
            const seekPos = audio.currentTime;
            let end = 0;
            for (let i = 0; i < ranges.length; i++) {
                const start = ranges.start(i);
                const stop = ranges.end(i);
                if (start <= seekPos + 1 && stop > end) end = stop;
            }
            if (!end) end = ranges.end(ranges.length - 1);
            const durationMs = playerStore.nowPlaying?.music?.duration || audio.duration * 1000 || 0;
            const candidateMs = end * 1000;
            bufferedMs.value = durationMs ? Math.max(0, Math.min(candidateMs, durationMs)) : candidateMs;
        } catch {
            // buffered 访问偶尔会抛错，忽略
        }
    };

const clearStalledWatchdog = () => {
        clearTimeout(stalledTimer);
        stalledTimer = null;
    };

    const enterPlaybackErrorState = (reason) => {
        clearStalledWatchdog();
        isBuffering.value = false;
        isErrorState.value = true;
        if (audioRef.value) {
            audioRef.value.pause();
            audioRef.value.playbackRate = 1;
        }
        console.warn('[Audio] playback stalled, pausing current track', {
            reason,
            currentSrc: audioRef.value?.currentSrc,
            readyState: audioRef.value?.readyState,
            networkState: audioRef.value?.networkState,
            platform: playerStore.nowPlaying?.music?.platform
        });
    };

    // L3 硬重载：load() 清空缓冲、currentTime 归零，记下进度在 canplay 时恢复
    const hardReloadSource = (reason) => {
        const audio = audioRef.value;
        if (!audio || !playerStore.nowPlaying || playerStore.isPaused) return;
        if (stalledRetryCount >= STALLED_MAX_RETRIES) {
            enterPlaybackErrorState(reason);
            return;
        }
        stalledRetryCount++;
        clearStalledWatchdog();
        const scheduledTrackId = playerStore.nowPlaying?.music?.id;
        scheduleRetry(() => {
            const currentAudio = audioRef.value;
            const currentTrackId = playerStore.nowPlaying?.music?.id;
            if (!currentAudio || !playerStore.nowPlaying || playerStore.isPaused || isErrorState.value) return;
            if (scheduledTrackId !== currentTrackId) return;
            console.warn(`[Audio] L3 hard reload (${stalledRetryCount}/${STALLED_MAX_RETRIES})`, { reason });
            // load() 会重置 currentTime 到 0，先记下目标进度以便 canplay 时恢复
            pendingResumePositionMs = playerStore.getCurrentProgress();
            playerStore.forceNextSyncSeek = true;
            currentAudio.load();
            // canplay -> checkAutoPlay 会先恢复进度再播放，这里重新 arm 从 L1 开始
            armStalledWatchdog(reason, 1);
        }, STALLED_RETRY_DELAY_MS);
    };

    // L2 软重试：重新发同一个网络请求，保留 currentTime 和已缓冲区间
    // 浏览器实现上会做 Range 续传请求补齐缓冲缺口，不丢已下载字节
    const softRetrySource = (reason) => {
        const audio = audioRef.value;
        if (!audio || !playerStore.nowPlaying || playerStore.isPaused || isErrorState.value) return;
        console.warn(`[Audio] L2 soft retry (re-request, keep buffer)`, { reason });
        try {
            // 重新赋值 src 触发网络层重新请求，但浏览器会保留 buffered ranges
            // 并按需发 Range 请求补齐，不重置 currentTime
            const currentSrc = audio.currentSrc || audio.src;
            if (currentSrc) {
                audio.src = currentSrc;
            }
            safePlay();
        } catch (e) {
            // 软重试失败，直接进 L3
            console.warn('[Audio] L2 soft retry failed, escalating to L3', e);
        }
        // 软重试后 arm L3：若 STALLED_L3_HARD_RELOAD_MS 内没恢复，硬重载
        armStalledWatchdog(reason, 3);
    };

    // 判断事件是否真的代表卡顿，避免 suspend 误判
    // suspend 在浏览器"我下够了"时也会发，不是真卡
    const isActuallyStalled = (reason) => {
        const audio = audioRef.value;
        if (!audio) return false;
        // emptied 通常是切歌/src 变化导致，不是网络卡顿
        if (reason === 'emptied') return false;
        if (reason === 'suspend') {
            // suspend 只在"还在加载但断流"时才算卡：readyState 不足 + 网络还在 loading
            if (audio.readyState >= 4) return false; // 已 HAVE_ENOUGH_DATA，是正常 suspend
            if (audio.networkState !== HTMLMediaElement.NETWORK_LOADING) return false;
            return true;
        }
        return true;
    };

    // 三级 arm：retryLevel=1 → L1 软等待；=2 → L2 软重试；=3 → L3 硬重载
    // 外部入口（onPlaybackStalled）总是从 L1 开始
    const armStalledWatchdog = (reason, retryLevel = 1) => {
        clearStalledWatchdog();
        if (!audioRef.value || playerStore.isPaused || !playerStore.nowPlaying || isErrorState.value) return;
        isBuffering.value = true;
        let delay;
        let action;
        if (retryLevel <= 1) {
            // L1: 软等待 2.5s，超时升级到 L2
            delay = STALLED_L1_SOFT_WAIT_MS;
            action = () => softRetrySource(reason);
        } else if (retryLevel === 2) {
            // L2: 软重试前给 3s 缓冲（一般 softRetrySource 直接调，不走这里）
            delay = STALLED_L2_SOFT_RETRY_MS;
            action = () => softRetrySource(reason);
        } else {
            // L3: 硬重载前等 5s
            delay = STALLED_L3_HARD_RELOAD_MS;
            action = () => hardReloadSource(reason);
        }
        stalledTimer = setTimeout(action, delay);
    };

    const markPlaybackHealthy = () => {
        clearStalledWatchdog();
        stalledRetryCount = 0;
        isBuffering.value = false;
    };

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
            markPlaybackHealthy();
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
    // 重载音轨会清空 currentTime，所以重载后需要先恢复到目标进度再播放，
    // 否则会从歌曲开头短暂播放，导致"缓冲时回到开头"的现象
    const checkAutoPlay = () => {
        if (!playerStore.nowPlaying) return;
        markPlaybackHealthy();

        const audio = audioRef.value;
        const restoreMs = pendingResumePositionMs;
        pendingResumePositionMs = null;
        if (restoreMs !== null && audio) {
            // 重载后先尝试恢复到中断前进度，避免从 0 开始播放"回到开头"
            try {
                if (audio.readyState >= 2 && audio.seekable && audio.seekable.length > 0) {
                    audio.currentTime = restoreMs / 1000;
                }
            } catch {
                // 即便恢复失败，交给下面 forceNextSyncSeek 由 sync 循环纠偏
            }
            // 交给 sync 循环在 readyState 就绪后强制硬跳到服务端目标进度
            playerStore.forceNextSyncSeek = true;
        }

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
            stalledRetryCount = 0;
            clearStalledWatchdog();
            clearRetryTimers();
            isErrorState.value = false;
            needsUserGesture.value = false;
            bufferedMs.value = 0;
            pendingResumePositionMs = null;
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
        clearStalledWatchdog();
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
            audioRef.value?.pause();
            return;
        }

        retryCount.value++;
        console.log(`Retry audio (${retryCount.value})...`);
        const scheduledTrackId = playerStore.nowPlaying?.music?.id;
        scheduleRetry(() => {
            const currentAudio = audioRef.value;
            const currentTrackId = playerStore.nowPlaying?.music?.id;
            if (!currentAudio) return;
            if (scheduledTrackId !== currentTrackId) return;
            // 记录中断前进度，canplay 时恢复，避免从开头播放
            pendingResumePositionMs = playerStore.getCurrentProgress();
            playerStore.forceNextSyncSeek = true;
            currentAudio.load();
            // load 完会触发 canplay，进而触发 checkAutoPlay (其中会恢复进度并播放)
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
        syncTimer = setInterval(() => {
            if (!playerStore.nowPlaying) {
                localProgress.value = 0;
                bufferedMs.value = 0;
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
            updateBufferedMs();

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
        clearStalledWatchdog();
        clearRetryTimers();
        if (audioRef.value) audioRef.value.playbackRate = 1;
        setFadeGain(1);
        releaseWakeLock();
    });

return {
        localProgress,
        isBuffering,
        bufferedMs,
        isErrorState,
        retryCount,
        needsUserGesture,
        safePlay,
        handleError,
        checkAutoPlay,
        armStalledWatchdog,
        markPlaybackHealthy,
        isActuallyStalled
    };
}
