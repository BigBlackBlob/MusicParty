package org.thornex.musicparty.service;

import jakarta.annotation.PostConstruct;
import lombok.extern.slf4j.Slf4j;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.context.event.EventListener;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.*;
import org.thornex.musicparty.enums.CacheStatus;
import org.thornex.musicparty.enums.PlayerAction;
import org.thornex.musicparty.enums.QueueItemStatus;
import org.thornex.musicparty.enums.TopResult;
import org.thornex.musicparty.event.*;
import org.thornex.musicparty.exception.ApiRequestException;
import org.thornex.musicparty.service.api.IMusicApiService;
import org.thornex.musicparty.service.api.NeteaseMusicApiService;
import org.thornex.musicparty.service.NavidromeAccessService;
import org.thornex.musicparty.service.stream.LiveStreamService;

import java.time.Duration;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.CopyOnWriteArrayList;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.Function;
import java.util.stream.Collectors;

@Service
@Slf4j
public class MusicPlayerService {

    private final Map<String, IMusicApiService> apiServiceMap;
    private final UserService userService;
    private final LocalCacheService localCacheService;
    // ChatService dependency removed to break circular reference
    private final LiveStreamService liveStreamService;
    private final NavidromeAccessService navidromeAccessService;

    // --- Refactored Dependencies ---
    private final MusicQueueManager queueManager;
    private final ApplicationEventPublisher eventPublisher;
    private final AppProperties appProperties;

    // --- Player State ---
    private final AtomicReference<PlayableMusic> currentMusic = new AtomicReference<>(null);
    private final AtomicReference<String> currentEnqueuerId = new AtomicReference<>(null);
    private final AtomicReference<String> currentEnqueuerName = new AtomicReference<>(null);

    // 核心计时逻辑
    private final AtomicLong positionAnchor = new AtomicLong(0); // 上一次更新状态时的进度(ms)
    private final AtomicLong timestampAnchor = new AtomicLong(0); // 上一次更新状态时的系统时间(ms)
    private final AtomicLong positionUpdatedAt = new AtomicLong(0);


    private final AtomicBoolean isShuffle = new AtomicBoolean(false);
    private final AtomicBoolean isPaused = new AtomicBoolean(false);
    private final AtomicBoolean isPauseLocked = new AtomicBoolean(false);
    private final AtomicBoolean isSkipLocked = new AtomicBoolean(false);
    private final AtomicBoolean isShuffleLocked = new AtomicBoolean(false);
    private final AtomicBoolean isLoading = new AtomicBoolean(false);
    private final AtomicBoolean isStreamActive = new AtomicBoolean(false);

    private final Map<String, Object> likeLock = new HashMap<>();
    private Set<String> currentLikedUserIds;
    private List<Long> currentLikeMarkers;

    private final AtomicLong lastControlTimestamp = new AtomicLong(0);
    private static final long GLOBAL_COOLDOWN_MS = 1000;
    private static final long IDLE_RESET_TIMEOUT_MS = Duration.ofHours(2).toMillis();

    private final AtomicLong playHeadVersion = new AtomicLong(0);
    private final AtomicLong stateVersion = new AtomicLong(0);
    private final AtomicLong playEpoch = new AtomicLong(0);

    public MusicPlayerService(List<IMusicApiService> apiServices, UserService userService,
                              LocalCacheService localCacheService,
                              LiveStreamService liveStreamService,
                              MusicQueueManager queueManager,
                              ApplicationEventPublisher eventPublisher,
                              AppProperties appProperties,
                              NavidromeAccessService navidromeAccessService) {
        this.apiServiceMap = apiServices.stream()
                .collect(Collectors.toMap(IMusicApiService::getPlatformName, Function.identity()));
        this.userService = userService;
        this.localCacheService = localCacheService;
        this.liveStreamService = liveStreamService;
        this.queueManager = queueManager;
        this.eventPublisher = eventPublisher;
        this.appProperties = appProperties;
        this.navidromeAccessService = navidromeAccessService;
        this.currentLikedUserIds = ConcurrentHashMap.newKeySet();
        this.currentLikeMarkers = new CopyOnWriteArrayList<>();
    }

    @PostConstruct
    public void init() {
        log.info("MusicPlayerService initialized with {} API services: {}", apiServiceMap.size(), apiServiceMap.keySet());
    }

    @Scheduled(fixedRate = 1000)
    public void playerLoop() {
        if (isPaused.get()) {
            return;
        }

        PlayableMusic music = currentMusic.get();

        if (music != null) {
            long currentPos = calculateCurrentPosition();

            // 检查是否播放结束
            if (currentPos >= music.duration() && music.duration() > 0) {
                log.info("Song finished: {}", music.name());

                Music finishedMusic = new Music(
                        music.id(),
                        music.name(),
                        music.artists(),
                        music.duration(),
                        music.platform(),
                        music.coverUrl()
                );
                queueManager.addToHistory(finishedMusic);

                // 清空当前，触发下一首
                currentMusic.set(null);
                bumpPlayEpochAndStateVersion();
                playNextInQueue();
            }
        } else {
            if (userService.getOnlineUserSummaries().isEmpty() && !isStreamActive.get()) {
                return;
            }
            if (!queueManager.getQueueSnapshot().isEmpty()) {
                playNextInQueue();
            }
        }
    }

    private synchronized void playNextInQueue() {
        if (currentMusic.get() != null || isLoading.get()) {
            return;
        }

        Map<String, QueueItemStatus> statusMap = buildStatusMap();

        Set<String> recentlyActivePublicIds = userService.getRecentlyActivePublicIds();

        MusicQueueItem nextItem = queueManager.pollNext(isShuffle.get(), statusMap, recentlyActivePublicIds);

        if (nextItem == null) {
            if (isLoading.get()) {
                isLoading.set(false);
                bumpStateVersion();
            }
            broadcastFullPlayerState();
            return;
        }

        // Handle failed items
        if (nextItem.status() == QueueItemStatus.FAILED ||
                (statusMap.get(MusicQueueManager.musicKey(nextItem.music())) == QueueItemStatus.FAILED)) {
            log.warn("Skipping failed song: {}", nextItem.music().name());
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, "SYSTEM", "加载失败: " + nextItem.music().name()));
            playNextInQueue(); // Recursively try next
            return;
        }

        // 增加版本号，这表示"开始一次新的播放尝试"
        long currentVersion = playHeadVersion.incrementAndGet();
        isLoading.set(true);
        bumpStateVersion();
        broadcastFullPlayerState();
        isPaused.set(false);

        log.info("Playing next: {}", nextItem.music().name());

        try {
            IMusicApiService service = getApiService(nextItem.music().platform());
            service.getPlayableMusic(nextItem.music().id())
                    .timeout(Duration.ofSeconds(10))
                    .subscribe(
                            playableMusic -> {
                                // 检查版本号是否匹配
                                // 如果在请求期间执行了 skip/stop，版本号会变，这里就应该丢弃结果
                                if (playHeadVersion.get() == currentVersion) {
                                    applyNewSong(playableMusic, nextItem);
                                } else {
                                    log.info("Discarded stale play result for {}", nextItem.music().name());
                                }
                            },
                        error -> {
                        log.error("Play failed for {}: {}", nextItem.music().name(), error.getMessage());
                        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, "SYSTEM", nextItem.music().name()));
                        isLoading.set(false);
                        bumpStateVersion();
                        broadcastFullPlayerState();
                        playNextInQueue();
                    });
        } catch (Exception e) {
            log.error("Unexpected error in playNextInQueue", e);
            isLoading.set(false);
            bumpStateVersion();
            broadcastFullPlayerState();
        }
    }

    private long calculateCurrentPosition() {
        if (currentMusic.get() == null) return 0;
        if (isPaused.get()) {
            return positionAnchor.get();
        } else {
            long now = System.currentTimeMillis();
            long elapsed = now - timestampAnchor.get();
            return positionAnchor.get() + elapsed;
        }
    }

    private void updatePlaybackAnchor(long positionMs) {
        long now = System.currentTimeMillis();
        positionAnchor.set(Math.max(0, positionMs));
        timestampAnchor.set(now);
        positionUpdatedAt.set(now);
    }

    private void bumpStateVersion() {
        stateVersion.incrementAndGet();
    }

    private void bumpPlayEpochAndStateVersion() {
        playEpoch.incrementAndGet();
        bumpStateVersion();
    }

    private void applyNewSong(PlayableMusic music, MusicQueueItem queueItem) {
        currentLikedUserIds.clear();
        currentLikeMarkers.clear();

        // 重置计时器
        currentMusic.set(music);
        currentEnqueuerId.set(queueItem.enqueuedBy().publicId());
        currentEnqueuerName.set(queueItem.enqueuedBy().name());

        updatePlaybackAnchor(0);
        isPaused.set(false);

        log.info("Now playing: {}", music.name());
        isLoading.set(false);
        bumpPlayEpochAndStateVersion();
        broadcastFullPlayerState();
        broadcastQueueUpdate();

        // 发布开始播放事件 (用于聊天栏展示)
        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.PLAY_START, queueItem.enqueuedBy().publicId(), music.name()));
    }

    public PlayerState getCurrentPlayerState() {
        PlayableMusic music = currentMusic.get();
        NowPlayingInfo infoToSend = null;

        if (music != null) {
            infoToSend = new NowPlayingInfo(
                    music,
                    calculateCurrentPosition(), // 直接返回计算好的进度
                    currentEnqueuerId.get(),
                    currentEnqueuerName.get(),
                    currentLikedUserIds,
                    currentLikeMarkers,
                    playEpoch.get(),
                    positionUpdatedAt.get()
            );
        }

        return new PlayerState(
                infoToSend,
                getQueueWithUpdatedStatus(),
                isShuffle.get(),
                userService.getOnlineUserSummaries(),
                isPaused.get(),
                isPauseLocked.get(),
                isSkipLocked.get(),
                isShuffleLocked.get(),
                isLoading.get(),
                liveStreamService.getStreamListenerCount(),
                System.currentTimeMillis(),
                stateVersion.get(),
                playEpoch.get()
        );
    }

    public void setLock(String type, boolean locked) {
        AtomicBoolean targetLock;
        String desc;
        switch (type.toUpperCase()) {
            case "PAUSE" -> { targetLock = isPauseLocked; desc = "暂停"; }
            case "SKIP" -> { targetLock = isSkipLocked; desc = "切歌"; }
            case "SHUFFLE" -> { targetLock = isShuffleLocked; desc = "随机播放"; }
            default -> throw new IllegalArgumentException("Unknown lock type");
        }

        boolean old = targetLock.getAndSet(locked);
        if (old != locked) {
            log.info("{} lock set to: {}", desc, locked);
            bumpStateVersion();
            broadcastFullPlayerState();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, PlayerAction.SYSTEM_MESSAGE, "SYSTEM",
                    locked ? "管理员锁定了" + desc : "管理员解锁了" + desc));
        }
    }

    public void setAllLocks(boolean locked) {
        boolean changed = isPauseLocked.getAndSet(locked) != locked;
        changed = (isSkipLocked.getAndSet(locked) != locked) || changed;
        changed = (isShuffleLocked.getAndSet(locked) != locked) || changed;
        if (changed) {
            bumpStateVersion();
        }
        broadcastFullPlayerState();
        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, PlayerAction.SYSTEM_MESSAGE, "SYSTEM",
                locked ? "管理员锁定了所有控制" : "管理员解锁了所有控制"));
    }

    public void enqueue(EnqueueRequest request, String sessionId) {
        Optional<User> userOpt = userService.getUser(sessionId);
        if (userOpt.isEmpty()) return;
        User enqueuer = userOpt.get();

        if ("navidrome".equals(request.platform()) && !navidromeAccessService.canUseBySession(sessionId)) {
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: 无权使用 Navidrome"));
            return;
        }

        // Check user song limit
        long userSongCount = queueManager.getQueueSnapshot().stream()
                .filter(item -> item.enqueuedBy().publicId().equals(enqueuer.getPublicId()))
                .count();

        if (userSongCount >= appProperties.getQueue().getMaxUserSongs()) {
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: 您的点歌数量已达上限 (" + appProperties.getQueue().getMaxUserSongs() + "首)"));
            return;
        }

        IMusicApiService service = getApiService(request.platform());
        service.getPlayableMusic(request.musicId())
                .subscribe(playableMusic -> {
                            Music music = new Music(playableMusic.id(), playableMusic.name(), playableMusic.artists(), playableMusic.duration(), playableMusic.platform(), playableMusic.coverUrl());

                            QueueItemStatus initialStatus = "bilibili".equals(request.platform()) ? QueueItemStatus.PENDING : QueueItemStatus.READY;
                            if ("bilibili".equals(request.platform())) {
                                service.prefetchMusic(music.id());
                            }

                            MusicQueueItem newItem = queueManager.add(music, new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest()), initialStatus);

                            if (newItem != null) {
                                log.info("{} enqueued: {}", enqueuer.getName(), music.name());
                                broadcastQueueUpdate();
                                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.ADD, enqueuer.getPublicId(), music.name()));
                            }
                        },
                        error -> {
                            log.error("Enqueue failed for musicId: {}", request.musicId(), error);
                            String msg = error.getMessage().contains("Could not get Bilibili video info") ? "无效资源或API受限" : error.getMessage();
                            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: " + msg));
                        });
    }

    // 点赞逻辑
    public void likeSong(String sessionId) {
        PlayableMusic music = currentMusic.get();
        if (music == null) return;

        String publicId = getUserPublicId(sessionId);

        // 1. 检查去重 (单人单曲一次)
        if (currentLikedUserIds.contains(publicId)) return;

        // 2. 更新数据
        currentLikedUserIds.add(publicId);

        // 使用计算出的当前进度作为 Marker
        long progress = calculateCurrentPosition();
        currentLikeMarkers.add(progress);

        log.info("Like received from {}", getUserName(sessionId));
        bumpStateVersion();

        // 3. 广播
        // 广播事件用于触发特效
        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.LIKE, publicId, music.name()));
        // 广播状态更新进度条打点和用户列表
        broadcastFullPlayerState();
    }

    public void enqueuePlaylist(EnqueuePlaylistRequest request, String sessionId) {
        Optional<User> userOpt = userService.getUser(sessionId);
        if (userOpt.isEmpty()) return;
        User enqueuer = userOpt.get();

        if ("navidrome".equals(request.platform())) {
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "Navidrome 暂不支持歌单导入"));
            return;
        }

        // Check user song limit
        long currentCount = queueManager.getQueueSnapshot().stream()
                .filter(item -> item.enqueuedBy().publicId().equals(enqueuer.getPublicId()))
                .count();
        int maxUserSongs = appProperties.getQueue().getMaxUserSongs();

        if (currentCount >= maxUserSongs) {
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "导入失败: 您的点歌数量已达上限"));
            return;
        }

        // Calculate remaining quota
        int remainingQuota = (int) (maxUserSongs - currentCount);
        int importLimit = Math.min(appProperties.getPlayer().getMaxPlaylistImportSize(), remainingQuota);

        IMusicApiService service = getApiService(request.platform());
        service.getPlaylistMusics(request.playlistId(), 0, importLimit)
                .subscribe(musics -> {
                    int count = 0;
                    QueueItemStatus initialStatus = "bilibili".equals(request.platform()) ? QueueItemStatus.PENDING : QueueItemStatus.READY;

                    for (Music music : musics) {
                        if ("bilibili".equals(request.platform())) {
                            service.prefetchMusic(music.id());
                        }
                        MusicQueueItem newItem = queueManager.add(music, new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest()), initialStatus);
                        if (newItem != null) {
                            count++;
                        }
                    }

                    log.info("{} enqueued {} songs from playlist", enqueuer.getName(), count);
                    broadcastQueueUpdate();
                    eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.IMPORT_PLAYLIST, enqueuer.getPublicId(), String.valueOf(count)));
                });
    }

    public void enqueueAlbum(EnqueueAlbumRequest request, String sessionId) {
        Optional<User> userOpt = userService.getUser(sessionId);
        if (userOpt.isEmpty()) return;
        User enqueuer = userOpt.get();

        if (!"netease".equals(request.platform())) {
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "当前仅支持导入网易云专辑"));
            return;
        }

        IMusicApiService service = getApiService(request.platform());
        if (!(service instanceof NeteaseMusicApiService neteaseService)) {
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "网易云专辑服务不可用"));
            return;
        }

        long currentCount = queueManager.getQueueSnapshot().stream()
                .filter(item -> item.enqueuedBy().publicId().equals(enqueuer.getPublicId()))
                .count();
        int maxUserSongs = appProperties.getQueue().getMaxUserSongs();

        if (currentCount >= maxUserSongs) {
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "导入失败: 您的点歌数量已达上限"));
            return;
        }

        int remainingQuota = (int) (maxUserSongs - currentCount);
        int importLimit = Math.min(appProperties.getPlayer().getMaxPlaylistImportSize(), remainingQuota);

        neteaseService.getAlbumMusics(request.albumId())
                .subscribe(musics -> {
                    int count = 0;
                    for (Music music : musics.stream().limit(importLimit).toList()) {
                        MusicQueueItem newItem = queueManager.add(music, new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest()), QueueItemStatus.READY);
                        if (newItem != null) {
                            count++;
                        }
                    }

                    log.info("{} enqueued {} songs from album {}", enqueuer.getName(), count, request.albumId());
                    broadcastQueueUpdate();
                    eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.IMPORT_PLAYLIST, enqueuer.getPublicId(), String.valueOf(count)));
                }, error -> {
                    log.error("Album import failed for albumId: {}", request.albumId(), error);
                    eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "专辑导入失败: " + error.getMessage()));
                });
    }

    public synchronized void topSong(String queueId, String sessionId) {
        // 先调用 top 执行置顶操作
        TopResult result = queueManager.top(queueId, isShuffle.get());
        
        if (result != TopResult.NONE) {
            log.info("Song topped ({}) request by {}", result, getUserName(sessionId));
            broadcastQueueUpdate();

            // 只有全局置顶才发送系统消息广播
            if (result == TopResult.GLOBAL) {
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.TOP, getUserPublicId(sessionId), "置顶成功"));
            }
            
            if (currentMusic.get() == null) {
                playNextInQueue();
            }
        }
    }

    public synchronized void topSongs(List<String> queueIds, String sessionId) {
        if (queueIds == null || queueIds.isEmpty()) return;

        List<MusicQueueItem> toppedItems = queueManager.topManyGlobal(queueIds);
        if (!toppedItems.isEmpty()) {
            log.info("{} songs topped by {}", toppedItems.size(), getUserName(sessionId));
            broadcastQueueUpdate();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.TOP, getUserPublicId(sessionId), "置顶 " + toppedItems.size() + " 首歌曲"));

            if (currentMusic.get() == null) {
                playNextInQueue();
            }
        }
    }

    public void removeSongFromQueue(String queueId, String sessionId) {
        Optional<MusicQueueItem> removedItem = queueManager.remove(queueId);
        if (removedItem.isPresent()) {
            log.info("Removed song from queue by {}", getUserName(sessionId));
            broadcastQueueUpdate();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.REMOVE, getUserPublicId(sessionId), removedItem.get().music().name()));
        }
    }

    public void removeSongsFromQueue(List<String> queueIds, String sessionId) {
        if (queueIds == null || queueIds.isEmpty()) return;

        List<MusicQueueItem> removedItems = queueManager.removeMany(queueIds);
        if (!removedItems.isEmpty()) {
            log.info("{} songs removed from queue by {}", removedItems.size(), getUserName(sessionId));
            broadcastQueueUpdate();
            broadcastFullPlayerState();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.REMOVE, getUserPublicId(sessionId), "移除 " + removedItems.size() + " 首歌曲"));
        }
    }

    public void skipToNext(String sessionId) {
        if (isRateLimited(sessionId)) return;
        if (isSkipLocked.get() && !"SYSTEM".equals(sessionId)) {
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, getUserPublicId(sessionId), "切歌功能已被锁定"));
            return;
        }

        // 切歌时版本号自增，废弃之前的任何 pending 请求
        playHeadVersion.incrementAndGet();
        isLoading.set(false);

        currentMusic.set(null);
        updatePlaybackAnchor(0);
        bumpPlayEpochAndStateVersion();

        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.SKIP, getUserPublicId(sessionId), null));
        playNextInQueue();
    }

    public void togglePause(String sessionId) {
        if (currentMusic.get() == null) {
            if (!queueManager.getQueueSnapshot().isEmpty()) {
                playNextInQueue();
            }
            return;
        }
        if (isRateLimited(sessionId)) return;

        // 锁定检查：如果是系统操作，放行。如果是用户操作，检查锁。
        // 规则：如果不控制播放权限（允许从暂停->播放），则只有当当前是播放状态(即试图暂停)且锁定时才拦截。
        if (!"SYSTEM".equals(sessionId)) {
            if (isPauseLocked.get() && !isPaused.get()) {
                // eventPublisher.publishEvent(...);
                return;
            }
        }

        // 核心：在切换状态的一瞬间，更新 Anchor
        // 1. 先计算出当前的进度
        long currentPos = calculateCurrentPosition();

        // 2. 更新状态
        boolean newState = !isPaused.get();
        isPaused.set(newState);

        // 3. 重置锚点：无论是暂停还是播放，当前进度都变成新的基准进度
        updatePlaybackAnchor(currentPos);

        log.info("Player {} by {}", newState ? "PAUSED" : "RESUMED", getUserName(sessionId));
        bumpStateVersion();
        broadcastFullPlayerState();
        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, newState ? PlayerAction.PAUSE : PlayerAction.PLAY, getUserPublicId(sessionId), null));
    }

    public synchronized Optional<String> seekTo(long positionMs, String sessionId) {
        PlayableMusic music = currentMusic.get();
        if (music == null) {
            return Optional.of("当前没有正在播放的歌曲");
        }

        String requesterPublicId = getUserPublicId(sessionId);
        String enqueuerPublicId = currentEnqueuerId.get();
        if (!Objects.equals(enqueuerPublicId, requesterPublicId)) {
            log.warn("Seek denied for user {} on song queued by {}", requesterPublicId, enqueuerPublicId);
            return Optional.of("只有点播者可以调整这首歌的进度");
        }

        long duration = Math.max(0, music.duration());
        long clampedPosition = duration > 0
                ? Math.max(0, Math.min(positionMs, duration))
                : Math.max(0, positionMs);

        updatePlaybackAnchor(clampedPosition);
        log.info("Player seek to {}ms by {}", clampedPosition, getUserName(sessionId));
        bumpPlayEpochAndStateVersion();
        broadcastFullPlayerState();
        return Optional.empty();
    }

    public void toggleShuffle(String sessionId) {
        if (isRateLimited(sessionId)) return;
        if (isShuffleLocked.get() && !"SYSTEM".equals(sessionId)) return;

        // 使用标准的 CAS 循环来原子性地翻转布尔值
        boolean current;
        boolean newState;
        do {
            current = isShuffle.get();
            newState = !current;
        } while (!isShuffle.compareAndSet(current, newState));

        log.info("Shuffle mode set to {} by {}", newState, getUserName(sessionId));
        bumpStateVersion();
        broadcastFullPlayerState();
        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO,
                newState ? PlayerAction.SHUFFLE_ON : PlayerAction.SHUFFLE_OFF, getUserPublicId(sessionId), null));
    }

    public void resetSystem() {
        log.warn("!!!SYSTEM RESET INITIATED!!!");
        currentMusic.set(null);
        updatePlaybackAnchor(0);

        queueManager.clearAll();
        isPaused.set(false);
        isShuffle.set(false);
        isLoading.set(false);

        bumpPlayEpochAndStateVersion();
        broadcastFullPlayerState();
        broadcastQueueUpdate();
        log.warn("System reset complete.");
        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, PlayerAction.RESET, "SYSTEM", null));
    }

    public void clearQueue() {
        queueManager.clearPendingQueue();
        log.info("Queue cleared by Admin.");
        // 广播队列更新
        broadcastQueueUpdate();
        // 发送全员通知
        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, PlayerAction.REMOVE, "SYSTEM", "播放列表已由管理员清空"));
    }

    @EventListener
    public void handleDownloadEvent(DownloadStatusEvent event) {
        boolean existsInQueue = queueManager.getQueueSnapshot().stream()
                .anyMatch(item -> item.music().id().equals(event.getMusicId()));

        if (existsInQueue) {
            log.debug("Download status changed for {}, updating queue UI.", event.getMusicId());
            broadcastQueueUpdate();
            if (currentMusic.get() == null) {
                playNextInQueue();
            }
        }
    }

    /**
     * 监听用户数量变化事件
     */
    @EventListener
    public void onUserCountChanged(UserCountChangeEvent event) {
        if (event.getOnlineUserCount() == 0 && !isStreamActive.get()) {
            enterIdleMode();
        }
    }

    /**
     * 监听直播流状态变化
     */
    @EventListener
    public void onStreamStatusChanged(StreamStatusEvent event) {
        boolean hasListeners = event.isHasListeners();
        this.isStreamActive.set(hasListeners);
        log.info("System: Stream active status changed to: {}, Count: {}", hasListeners, event.getListenerCount());

        if (hasListeners) {
            // 场景 A: 列表为空，有人连入流 -> 尝试开始播放下一首
            if (currentMusic.get() == null) {
                playNextInQueue();
            } 
            // 场景 B: 正在暂停中，且网页端没人，有人连入流 -> 自动恢复播放
            else if (isPaused.get() && userService.getOnlineUserSummaries().isEmpty()) {
                log.info("System: Auto-resuming player for new stream listener.");
                togglePause("SYSTEM");
            }
        } else {
            // 场景 C: 流用户离开，且网页端也没人 -> 进入休眠
            if (userService.getOnlineUserSummaries().isEmpty()) {
                enterIdleMode();
            }
        }
        bumpStateVersion();
        broadcastFullPlayerState();
    }

    /**
     * 进入空闲模式，停止播放
     */
    private void enterIdleMode() {
        log.info("Last user disconnected. Entering idle mode.");
        isLoading.set(false);

        // 如果正在播放，自动暂停，记录当前进度
        if (currentMusic.get() != null && !isPaused.get()) {
            long currentPos = calculateCurrentPosition();

            if (isPaused.compareAndSet(false, true)) {
                // 暂停时，更新锚点为刚才计算出的准确进度
                updatePlaybackAnchor(currentPos);

                log.info("Player paused as all users have disconnected. Position saved at: {}", currentPos);
                bumpStateVersion();
                broadcastFullPlayerState();
            }
        }
    }

    /**
     * 定时清理长时间暂停的播放器状态
     */
    @Scheduled(fixedRate = 600000) // 每10分钟检查一次
    public void cleanupIdlePlayer() {
        if (isPaused.get() && currentMusic.get() != null) {
            // 在暂停状态下，timestampAnchor 记录的是暂停开始的时间
            long pausedDuration = System.currentTimeMillis() - timestampAnchor.get();
            if (pausedDuration > IDLE_RESET_TIMEOUT_MS) {
                log.info("Idle player timeout reached. Resetting now playing.");
                currentMusic.set(null);
                updatePlaybackAnchor(0);
                isPaused.set(false);
                bumpPlayEpochAndStateVersion();
                broadcastFullPlayerState();
            }
        }
    }

    // --- Broadcasting and Helper Methods ---

    public void broadcastQueueUpdate() {
        eventPublisher.publishEvent(new QueueUpdateEvent(this, getQueueWithUpdatedStatus()));
    }

    public void broadcastFullPlayerState() {
        eventPublisher.publishEvent(new PlayerStateEvent(this, getCurrentPlayerState()));
    }

    public void broadcastOnlineUsers() {
        // This is triggered by UserService, so we can keep it simple or create another event type
        broadcastFullPlayerState();
    }

    public void broadcastPasswordChanged() {
        // Can create a specific event or use SystemMessageEvent
        // For now, let's keep it simple
        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, null, "SYSTEM", "PASSWORD_CHANGED"));
    }

    private List<MusicQueueItem> getQueueWithUpdatedStatus() {
        return queueManager.getQueueSnapshot().stream().map(item -> {
            if ("netease".equals(item.music().platform())) {
                return item.status() == QueueItemStatus.READY ? item : item.withStatus(QueueItemStatus.READY);
            }
            if ("bilibili".equals(item.music().platform())) {
                CacheStatus cacheStatus = localCacheService.getStatus(item.music().id());
                QueueItemStatus newStatus = mapCacheStatusToEnum(cacheStatus);
                if (item.status() != newStatus) {
                    return item.withStatus(newStatus);
                }
            }
            return item;
        }).collect(Collectors.toList());
    }

    private Map<String, QueueItemStatus> buildStatusMap() {
        Map<String, QueueItemStatus> statusMap = new HashMap<>();
        for (MusicQueueItem item : queueManager.getQueueSnapshot()) {
            if ("bilibili".equals(item.music().platform())) {
                statusMap.put(MusicQueueManager.musicKey(item.music()), mapCacheStatusToEnum(localCacheService.getStatus(item.music().id())));
            } else {
                statusMap.put(MusicQueueManager.musicKey(item.music()), QueueItemStatus.READY);
            }
        }
        return statusMap;
    }

    private QueueItemStatus mapCacheStatusToEnum(CacheStatus status) {
        if (status == null) return QueueItemStatus.PENDING;
        return switch (status) {
            case COMPLETED -> QueueItemStatus.READY;
            case DOWNLOADING -> QueueItemStatus.DOWNLOADING;
            case FAILED -> QueueItemStatus.FAILED;
            default -> QueueItemStatus.PENDING;
        };
    }

    /*private void resetPauseState() {
        isPaused.set(false);
        pauseStateChangeTime.set(0);
        totalPausedTimeMillis.set(0);
    }*/

    private boolean isRateLimited(String userId) {
        long now = System.currentTimeMillis();
        if (now - lastControlTimestamp.get() < GLOBAL_COOLDOWN_MS) {
            log.warn("Action rate limited for user: {}", userId);
            // eventPublisher.publishEvent(...); // Optional: notify user about rate limit
            return true;
        }
        lastControlTimestamp.set(now);
        return false;
    }

    private IMusicApiService getApiService(String platform) {
        IMusicApiService service = apiServiceMap.get(platform);
        if (service == null) throw new ApiRequestException("Unsupported platform: " + platform);
        return service;
    }

    private String getUserPublicId(String sessionId) {
        return userService.getUser(sessionId).map(User::getPublicId).orElse("UNKNOWN_USER");
    }

    private String getUserName(String sessionId) {
        return userService.getUser(sessionId).map(User::getName).orElse("Unknown User");
    }
}

