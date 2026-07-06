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
import org.thornex.musicparty.persistence.PersistedPlaybackState;
import org.thornex.musicparty.service.api.CachedMusicApiService;
import org.thornex.musicparty.service.api.IMusicApiService;
import org.thornex.musicparty.service.api.SubsonicMusicApiService;
import org.thornex.musicparty.service.stream.LiveStreamService;
import reactor.core.scheduler.Schedulers;

import java.time.Duration;
import java.util.*;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.atomic.AtomicLong;
import java.util.function.Consumer;
import java.util.function.Function;
import java.util.stream.Collectors;

@Service
@Slf4j
public class MusicPlayerService {
    public enum ControlResult {
        OK,
        COOLDOWN,
        LOCKED,
        EMPTY_QUEUE
    }

    // B站/YouTube 未下完时 getPlayableMusic 返回的占位 URL，绝不能发到前端成为 audio src
    static final String PENDING_DOWNLOAD_URL = "PENDING_DOWNLOAD";

    private final Map<String, IMusicApiService> apiServiceMap;
    private final UserService userService;
    private final LocalCacheService localCacheService;
    private final LiveStreamService liveStreamService;
    private final ApplicationEventPublisher eventPublisher;
    private final AppProperties appProperties;
    private final NavidromeAccessService navidromeAccessService;
    private final RoomService roomService;
    private final RoomSessionCoordinator roomSessionCoordinator;
    private final RoomStatePersistenceService roomStatePersistenceService;
    private final RoomStateMutationService roomStateMutationService;
    private final PlaybackTransitionService playbackTransitionService;
    private final SubsonicMusicApiService subsonicMusicApiService;
    private final SubsonicSourceRegistry subsonicSourceRegistry;
    private final Map<String, RoomPlayerSession> sessions = new ConcurrentHashMap<>();
    private final AtomicBoolean playerLoopRunning = new AtomicBoolean(false);
    private final AtomicBoolean idleCleanupRunning = new AtomicBoolean(false);
    private final AtomicBoolean coldEvictionRunning = new AtomicBoolean(false);

    // 用于 pending download 轮询的共享调度器
    private static final java.util.concurrent.ScheduledExecutorService PENDING_POLL_SCHEDULER =
            java.util.concurrent.Executors.newScheduledThreadPool(2, r -> {
                Thread t = new Thread(r, "mp-pending-download-poller");
                t.setDaemon(true);
                return t;
            });

    public MusicPlayerService(List<IMusicApiService> apiServices,
                              UserService userService,
                              LocalCacheService localCacheService,
                              LiveStreamService liveStreamService,
                              ApplicationEventPublisher eventPublisher,
                              AppProperties appProperties,
                              NavidromeAccessService navidromeAccessService,
                              RoomService roomService,
                              RoomSessionCoordinator roomSessionCoordinator,
                              RoomStatePersistenceService roomStatePersistenceService,
                              RoomStateMutationService roomStateMutationService,
                              PlaybackTransitionService playbackTransitionService,
                              SubsonicMusicApiService subsonicMusicApiService,
                              SubsonicSourceRegistry subsonicSourceRegistry) {
        this.apiServiceMap = apiServices.stream().collect(Collectors.toMap(IMusicApiService::getPlatformName, Function.identity()));
        this.userService = userService;
        this.localCacheService = localCacheService;
        this.liveStreamService = liveStreamService;
        this.eventPublisher = eventPublisher;
        this.appProperties = appProperties;
        this.navidromeAccessService = navidromeAccessService;
        this.roomService = roomService;
        this.roomSessionCoordinator = roomSessionCoordinator;
        this.roomStatePersistenceService = roomStatePersistenceService;
        this.roomStateMutationService = roomStateMutationService;
        this.playbackTransitionService = playbackTransitionService;
        this.subsonicMusicApiService = subsonicMusicApiService;
        this.subsonicSourceRegistry = subsonicSourceRegistry;
    }

    @PostConstruct
    public void init() {
        log.info("MusicPlayerService initialized with {} API services: {}", apiServiceMap.size(), apiServiceMap.keySet());
        session(RoomService.DEFAULT_ROOM_ID);
    }

    public Set<String> getActiveRoomIds() {
        Set<String> ids = new HashSet<>(sessions.keySet());
        roomService.listRooms().forEach(room -> ids.add(room.roomId()));
        return ids;
    }

    public Set<String> getLoadedRoomIds() {
        return new HashSet<>(sessions.keySet());
    }

    public RoomPlayerSession getSession(String roomId) {
        return session(roomId);
    }

    public void removeRoom(String roomId) {
        removeRoom(roomId, false);
    }

    public void removeRoom(String roomId, boolean skipPersistenceCleanup) {
        String normalized = roomId == null || roomId.isBlank() ? RoomService.DEFAULT_ROOM_ID : roomId;
        RoomPlayerSession removed = sessions.remove(normalized);
        if (removed != null) {
            removed.cancelPendingDownloadPoller();
            if (!skipPersistenceCleanup) {
                removed.flushPersistentState();
            }
            removed.resetSystem(false, !skipPersistenceCleanup);
        }
        if (!skipPersistenceCleanup) {
            roomStatePersistenceService.deletePlaybackState(normalized);
        }
    }

    @Scheduled(fixedRate = 1000)
    public void playerLoop() {
        runScheduledOnce(playerLoopRunning, () ->
                new ArrayList<>(sessions.values()).forEach(RoomPlayerSession::playerLoop)
        );
    }

    @Scheduled(fixedRate = 600000)
    public void cleanupIdlePlayer() {
        runScheduledOnce(idleCleanupRunning, () ->
                new ArrayList<>(sessions.values()).forEach(RoomPlayerSession::cleanupIdlePlayer)
        );
    }

    @Scheduled(fixedRate = 300000)
    public void scheduledEvictColdRooms() {
        runScheduledOnce(coldEvictionRunning, this::evictColdRoomsOnWorker);
    }

    public void evictColdRooms() {
        evictColdRoomsOnWorker();
    }

    private void evictColdRoomsOnWorker() {
        long now = System.currentTimeMillis();
        long idleThresholdMs = appProperties.getPlayer().getRoomEvictionIdleMs();
        List<String> coldRoomIds = sessions.values().stream()
                .filter(session -> !RoomService.DEFAULT_ROOM_ID.equals(session.getRoomId()))
                .filter(session -> session.isEvictable(now, idleThresholdMs))
                .map(RoomPlayerSession::getRoomId)
                .toList();

        for (String roomId : coldRoomIds) {
            RoomPlayerSession removed = sessions.get(roomId);
            if (removed != null
                    && removed.isEvictable(System.currentTimeMillis(), idleThresholdMs)
                    && sessions.remove(roomId, removed)) {
                removed.cancelPendingDownloadPoller();
                roomSessionCoordinator.evictColdRoom(roomId, removed::flushPersistentState);
            }
        }
    }

    private void runScheduledOnce(AtomicBoolean running, Runnable work) {
        if (!running.compareAndSet(false, true)) {
            return;
        }
        Schedulers.boundedElastic().schedule(() -> {
            try {
                work.run();
            } catch (Exception e) {
                log.error("Scheduled music player task failed", e);
            } finally {
                running.set(false);
            }
        });
    }

    public PlayerState getCurrentPlayerState() {
        return session(RoomService.DEFAULT_ROOM_ID).getCurrentPlayerState();
    }

    public PlayerState getCurrentPlayerState(String roomId) {
        return session(roomId).getCurrentPlayerState();
    }

    public PlayerState getCurrentPlayerStateForSession(String sessionId) {
        return session(userService.getRoomIdForSession(sessionId)).getCurrentPlayerState();
    }

    public void setLock(String type, boolean locked) {
        sessions.values().forEach(s -> s.setLock(type, locked));
    }

    public void setAllLocks(boolean locked) {
        sessions.values().forEach(s -> s.setAllLocks(locked));
    }

    public void enqueue(EnqueueRequest request, String sessionId) {
        sessionForUser(sessionId).enqueue(request, sessionId);
    }

    public void enqueuePlaylist(EnqueuePlaylistRequest request, String sessionId) {
        sessionForUser(sessionId).enqueuePlaylist(request, sessionId);
    }

    public void enqueueAlbum(EnqueueAlbumRequest request, String sessionId) {
        sessionForUser(sessionId).enqueueAlbum(request, sessionId);
    }

    public void enqueueSavedPlaylist(List<Music> musics, String sessionId) {
        sessionForUser(sessionId).enqueueSavedPlaylist(musics, sessionId);
    }

    public void topSong(String queueId, String sessionId) {
        sessionForUser(sessionId).topSong(queueId, sessionId);
    }

    public void topSongs(List<String> queueIds, String sessionId) {
        sessionForUser(sessionId).topSongs(queueIds, sessionId);
    }

    public void removeSongFromQueue(String queueId, String sessionId) {
        sessionForUser(sessionId).removeSongFromQueue(queueId, sessionId);
    }

    public void removeSongsFromQueue(List<String> queueIds, String sessionId) {
        sessionForUser(sessionId).removeSongsFromQueue(queueIds, sessionId);
    }

    public void reorderQueue(int oldIndex, int newIndex, String sessionId) {
        sessionForUser(sessionId).reorderQueue(oldIndex, newIndex, sessionId);
    }

    public void reorderQueue(String queueId, String targetQueueId, String position, String sessionId) {
        sessionForUser(sessionId).reorderQueue(queueId, targetQueueId, position, sessionId);
    }

    public ControlResult skipToNext(String sessionId) {
        return sessionForUser(sessionId).skipToNext(sessionId);
    }

    public ControlResult togglePause(String sessionId) {
        return sessionForUser(sessionId).togglePause(sessionId);
    }

    public Optional<String> seekTo(long positionMs, String sessionId, boolean adminOverride) {
        return sessionForUser(sessionId).seekTo(positionMs, sessionId, adminOverride);
    }

    public ControlResult toggleShuffle(String sessionId) {
        return sessionForUser(sessionId).toggleShuffle(sessionId);
    }

    public void likeSong(String sessionId) {
        sessionForUser(sessionId).likeSong(sessionId);
    }

    public void resetSystem() {
        sessions.values().forEach(s -> s.resetSystem(true));
    }

    public void clearQueue() {
        sessions.values().forEach(RoomPlayerSession::clearQueue);
    }

    public void broadcastQueueUpdate() {
        sessions.values().forEach(RoomPlayerSession::broadcastQueueUpdate);
    }

    public void broadcastFullPlayerState() {
        sessions.values().forEach(RoomPlayerSession::broadcastFullPlayerState);
    }

    public void broadcastOnlineUsers() {
        getActiveRoomIds().forEach(roomId -> session(roomId).broadcastFullPlayerState());
    }

    public void broadcastPasswordChanged() {
        eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, null, "SYSTEM", "PASSWORD_CHANGED", RoomService.DEFAULT_ROOM_ID));
    }

    @EventListener
    public void handleDownloadEvent(DownloadStatusEvent event) {
        sessions.values().forEach(session -> session.handleDownloadEvent(event));
    }

    @EventListener
    public void onUserCountChanged(UserCountChangeEvent event) {
        session(event.getRoomId()).onUserCountChanged(event);
    }

    @EventListener
    public void onStreamStatusChanged(StreamStatusEvent event) {
        session(RoomService.DEFAULT_ROOM_ID).onStreamStatusChanged(event);
    }

    private RoomPlayerSession sessionForUser(String sessionId) {
        return session(userService.getRoomIdForSession(sessionId));
    }

    private RoomPlayerSession session(String roomId) {
        String normalized = roomService.normalizeRoomId(roomId);
        RoomPlayerSession existing = sessions.get(normalized);
        if (existing != null) {
            return existing;
        }
        RoomPlayerSession candidate = new RoomPlayerSession(normalized);
        candidate.restorePersistentState();
        RoomPlayerSession raced = sessions.putIfAbsent(normalized, candidate);
        return raced == null ? candidate : raced;
    }

    public class RoomPlayerSession {
        private final String roomId;
        private final MusicQueueManager queueManager;
        private final RoomPlaybackState playbackState = new RoomPlaybackState();
        private final AtomicBoolean isStreamActive = new AtomicBoolean(false);
        private final AtomicBoolean idlePaused = new AtomicBoolean(false);
        private final AtomicInteger onlineUserCount = new AtomicInteger(0);
        private final Map<String, AtomicLong> lastControlTimestamps = new ConcurrentHashMap<>();
        private final AtomicLong playHeadVersion = new AtomicLong(0);
        // 当 playNextInQueue 选中的歌 url=PENDING_DOWNLOAD 时，轮询下载状态的定时任务
        private java.util.concurrent.ScheduledFuture<?> pendingDownloadPoller;

        private static final long GLOBAL_COOLDOWN_MS = 1000;
        private static final long IDLE_RESET_TIMEOUT_MS = 2 * 60 * 60 * 1000L;
        // 轮询下载状态的间隔
        private static final long PENDING_DOWNLOAD_POLL_MS = 2000;
        // 轮询超时：超过这个时间还没下完，放弃这首歌跳下一首
        private static final long PENDING_DOWNLOAD_TIMEOUT_MS = 120_000;

        private RoomPlayerSession(String roomId) {
            this.roomId = roomId;
            this.queueManager = new MusicQueueManager(appProperties);
        }

        public MusicQueueManager getQueueManager() {
            return queueManager;
        }

        public String getRoomId() {
            return roomId;
        }

        public synchronized void playerLoop() {
            if (playbackState.isPaused()) return;
            PlayableMusic music = playbackState.currentMusic();
            if (music != null) {
                long currentPos = playbackState.calculateCurrentPosition();
                if (currentPos >= music.duration() && music.duration() > 0) {
                    Music finishedMusic = new Music(music.id(), music.name(), music.artists(), music.duration(), music.platform(), music.coverUrl());
                    roomStateMutationService.runInTransaction(() -> {
                        appendFinishedTrackToHistory(finishedMusic);
                        playbackState.clearCurrentTrack();
                        persistPlaybackStateSnapshot();
                        playbackState.bumpPlayEpochAndStateVersion();
                    });
                    playNextInQueue();
                }
                return;
            }
            if (userService.getOnlineUserSummaries(roomId).isEmpty() && !isStreamActive.get()) return;
            if (!queueManager.getQueueSnapshot().isEmpty()) playNextInQueue();
        }

        private synchronized void playNextInQueue() {
            if (playbackState.currentMusic() != null || playbackState.isLoading()) return;
            MusicQueueItem nextItem = queueManager.pollNext(playbackState.isShuffle(), buildStatusMap(), userService.getRecentlyActivePublicIds(roomId));
            if (nextItem == null) {
                playbackState.setLoading(false);
                applyPlaybackTransition(false, false, true, null);
                return;
            }
            if (nextItem.status() == QueueItemStatus.FAILED) {
                applyPlaybackTransition(
                        true,
                        true,
                        false,
                        new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, "SYSTEM", "加载失败: " + nextItem.music().name(), roomId)
                );
                playNextInQueue();
                return;
            }
            long version = playHeadVersion.incrementAndGet();
            playbackState.setLoading(true);
            playbackState.setPaused(false);
            idlePaused.set(false);
            playbackState.touchHotActivity();
            playbackState.bumpStateVersion();
            applyPlaybackTransition(true, true, true, null);
            try {
                getApiService(nextItem.music().platform()).getPlayableMusic(nextItem.music().id())
                        .timeout(Duration.ofSeconds(10))
                        .publishOn(Schedulers.boundedElastic())
                        .subscribe(playable -> {
                            synchronized (RoomPlayerSession.this) {
                                if (playHeadVersion.get() != version) return;
                                // 拦截 PENDING_DOWNLOAD：不让它成为 currentMusic 发到前端
                                // 而是保持 loading=true，轮询下载状态，完成后重新 resolve
                                if (PENDING_DOWNLOAD_URL.equals(playable.url())) {
                                    waitForPendingDownload(nextItem, version);
                                } else {
                                    applyNewSong(playable, nextItem);
                                }
                            }
                        }, error -> {
                            synchronized (RoomPlayerSession.this) {
                                log.error("Play failed for {} in room {}", nextItem.music().name(), roomId, error);
                                playbackState.setLoading(false);
                                playbackState.bumpStateVersion();
                                applyPlaybackTransition(
                                        false,
                                        false,
                                        true,
                                        new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, "SYSTEM", nextItem.music().name(), roomId)
                                );
                                playNextInQueue();
                            }
                        });
            } catch (Exception e) {
                playbackState.setLoading(false);
                playbackState.bumpStateVersion();
                applyPlaybackTransition(false, false, true, null);
            }
        }

        private synchronized void applyNewSong(PlayableMusic music, MusicQueueItem queueItem) {
            idlePaused.set(false);
            playbackState.startNewTrack(music, queueItem);
            applyPlaybackTransition(
                    false,
                    false,
                    true,
                    new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.PLAY_START, queueItem.enqueuedBy().publicId(), music.name(), roomId)
            );
        }

        // 当 getPlayableMusic 返回 PENDING_DOWNLOAD 时，保持 loading=true，轮询下载状态
        // 下载完成(COMPLETED)后重新 getPlayableMusic → applyNewSong；失败/超时则跳下一首
        private synchronized void waitForPendingDownload(MusicQueueItem pendingItem, long version) {
            cancelPendingDownloadPoller();
            final long startedAt = System.currentTimeMillis();
            log.info("Pending download wait started for {} in room {}", pendingItem.music().name(), roomId);
            pendingDownloadPoller = PENDING_POLL_SCHEDULER.scheduleAtFixedRate(() -> {
                try {
                    if (playHeadVersion.get() != version) {
                        cancelPendingDownloadPoller();
                        return;
                    }
                    // 超时检查
                    if (System.currentTimeMillis() - startedAt > PENDING_DOWNLOAD_TIMEOUT_MS) {
                        log.warn("Pending download timed out for {} in room {}", pendingItem.music().name(), roomId);
                        cancelPendingDownloadPoller();
                        playbackState.setLoading(false);
                        playbackState.bumpStateVersion();
                        applyPlaybackTransition(
                                false,
                                false,
                                true,
                                new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, "SYSTEM", "下载超时: " + pendingItem.music().name(), roomId)
                        );
                        playNextInQueue();
                        return;
                    }
                    CacheStatus status = localCacheService.getStatus(cacheKey(pendingItem.music()));
                    if (status == CacheStatus.COMPLETED) {
                        cancelPendingDownloadPoller();
                        // 下载完成，重新 resolve url
                        getApiService(pendingItem.music().platform())
                                .getPlayableMusic(pendingItem.music().id())
                                .timeout(Duration.ofSeconds(10))
                                .publishOn(Schedulers.boundedElastic())
                                .subscribe(playable -> {
                                    synchronized (RoomPlayerSession.this) {
                                        if (playHeadVersion.get() != version) return;
                                        if (PENDING_DOWNLOAD_URL.equals(playable.url())) {
                                            // 极端情况：下载完成但仍返回 PENDING（理论不应发生），再等一轮
                                            log.warn("Pending download completed but URL still PENDING for {}", pendingItem.music().name());
                                            waitForPendingDownload(pendingItem, version);
                                            return;
                                        }
                                        applyNewSong(playable, pendingItem);
                                    }
                                }, error -> {
                                    synchronized (RoomPlayerSession.this) {
                                        if (playHeadVersion.get() != version) return;
                                        log.error("Re-resolve after download failed for {}", pendingItem.music().name(), error);
                                        playbackState.setLoading(false);
                                        playbackState.bumpStateVersion();
                                        playNextInQueue();
                                    }
                                });
                    } else if (status == CacheStatus.FAILED) {
                        log.warn("Pending download failed for {} in room {}", pendingItem.music().name(), roomId);
                        cancelPendingDownloadPoller();
                        playbackState.setLoading(false);
                        playbackState.bumpStateVersion();
                        applyPlaybackTransition(
                                false,
                                false,
                                true,
                                new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, "SYSTEM", "下载失败: " + pendingItem.music().name(), roomId)
                        );
                        playNextInQueue();
                    }
                    // PENDING / DOWNLOADING / null → 继续轮询
                } catch (Exception e) {
                    log.error("Error in pending download poller for {}", pendingItem.music().name(), e);
                }
            }, PENDING_DOWNLOAD_POLL_MS, PENDING_DOWNLOAD_POLL_MS, java.util.concurrent.TimeUnit.MILLISECONDS);
        }

        private synchronized void cancelPendingDownloadPoller() {
            if (pendingDownloadPoller != null) {
                pendingDownloadPoller.cancel(false);
                pendingDownloadPoller = null;
            }
        }

        public PlayerState getCurrentPlayerState() {
            long streamListenerCount = liveStreamService == null ? 0 : liveStreamService.getStreamListenerCount();
            return playbackState.toPlayerState(
                    roomId,
                    getQueueWithUpdatedStatus(),
                    userService.getOnlineUserSummaries(roomId),
                    streamListenerCount
            );
        }

        public synchronized void enqueue(EnqueueRequest request, String sessionId) {
            Optional<User> userOpt = userService.getUser(sessionId);
            if (userOpt.isEmpty()) return;
            User enqueuer = userOpt.get();
            if ("navidrome".equals(request.platform()) && !navidromeAccessService.canUseBySession(sessionId)) {
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: 无权使用 Navidrome", roomId));
                return;
            }
            if (isSubsonicPlatform(request.platform()) && !canUseSubsonicInRoom(request.platform(), sessionId)) {
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: 无权使用 Subsonic", roomId));
                return;
            }
            IMusicApiService service = getApiService(request.platform());
            service.getPlayableMusic(request.musicId()).publishOn(Schedulers.boundedElastic()).subscribe(playable -> {
                synchronized (RoomPlayerSession.this) {
                    long count = queueManager.getQueueSnapshot().stream()
                            .filter(i -> i.enqueuedBy().publicId().equals(enqueuer.getPublicId()))
                            .count();
                    if (count >= appProperties.getQueue().getMaxUserSongs()) return;
                    Music music = new Music(playable.id(), playable.name(), playable.artists(), playable.duration(), playable.platform(), playable.coverUrl());
                    QueueItemStatus initialStatus = isCachedPlatform(request.platform()) ? QueueItemStatus.PENDING : QueueItemStatus.READY;
                    if (isCachedPlatform(request.platform())) service.prefetchMusic(music.id());
                    MusicQueueItem item = queueManager.add(music, new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest()), initialStatus);
                    if (item != null) {
                        playbackState.touchHotActivity();
                        persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.ADD, enqueuer.getPublicId(), music.name(), roomId), false);
                        if (playbackState.currentMusic() == null) playNextInQueue();
                    }
                }
            }, error -> eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: " + error.getMessage(), roomId)));
        }

        public synchronized void enqueuePlaylist(EnqueuePlaylistRequest request, String sessionId) {
            Optional<User> userOpt = userService.getUser(sessionId);
            if (userOpt.isEmpty()) return;
            User enqueuer = userOpt.get();
            if ("navidrome".equals(request.platform())) return;
            int importLimit = appProperties.getPlayer().getMaxPlaylistImportSize();
            IMusicApiService service = getApiService(request.platform());
            service.getPlaylistMusics(request.playlistId(), 0, importLimit).publishOn(Schedulers.boundedElastic()).subscribe(musics -> {
                synchronized (RoomPlayerSession.this) {
                    int count = 0;
                    long existingUserCount = queueManager.getQueueSnapshot().stream()
                            .filter(i -> i.enqueuedBy().publicId().equals(enqueuer.getPublicId()))
                            .count();
                    int remaining = Math.max(0, appProperties.getQueue().getMaxUserSongs() - (int) existingUserCount);
                    int limit = Math.min(remaining, importLimit);
                    QueueItemStatus initialStatus = isCachedPlatform(request.platform()) ? QueueItemStatus.PENDING : QueueItemStatus.READY;
                    for (Music music : musics.stream().limit(limit).toList()) {
                        if (isCachedPlatform(request.platform())) service.prefetchMusic(music.id());
                        if (queueManager.add(music, new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest()), initialStatus) != null) count++;
                    }
                    if (count == 0) return;
                    playbackState.touchHotActivity();
                    persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.IMPORT_PLAYLIST, enqueuer.getPublicId(), String.valueOf(count), roomId), false);
                    if (playbackState.currentMusic() == null) playNextInQueue();
                }
            }, error -> eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "导入歌单失败: " + error.getMessage(), roomId)));
        }

        public synchronized void enqueueAlbum(EnqueueAlbumRequest request, String sessionId) {
            Optional<User> userOpt = userService.getUser(sessionId);
            if (userOpt.isEmpty()) return;
            User enqueuer = userOpt.get();
            if ("navidrome".equals(request.platform()) && !navidromeAccessService.canUseBySession(sessionId)) {
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: 无权使用 Navidrome", roomId));
                return;
            }
            if (isSubsonicPlatform(request.platform()) && !canUseSubsonicInRoom(request.platform(), sessionId)) {
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: 无权使用 Subsonic", roomId));
                return;
            }
            IMusicApiService service = getApiService(request.platform());
            service.getAlbumMusics(request.albumId()).publishOn(Schedulers.boundedElastic()).subscribe(musics -> {
                synchronized (RoomPlayerSession.this) {
                    int count = 0;
                    long existingUserCount = queueManager.getQueueSnapshot().stream()
                            .filter(i -> i.enqueuedBy().publicId().equals(enqueuer.getPublicId()))
                            .count();
                    int remaining = Math.max(0, appProperties.getQueue().getMaxUserSongs() - (int) existingUserCount);
                    int limit = Math.min(remaining, appProperties.getPlayer().getMaxPlaylistImportSize());
                    for (Music music : musics.stream().limit(limit).toList()) {
                        if (queueManager.add(music, new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest()), QueueItemStatus.READY) != null) count++;
                    }
                    playbackState.touchHotActivity();
                    persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.IMPORT_PLAYLIST, enqueuer.getPublicId(), String.valueOf(count), roomId), false);
                    if (playbackState.currentMusic() == null) playNextInQueue();
                }
            }, error -> eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "导入专辑失败: " + error.getMessage(), roomId)));
        }

        public void enqueueSavedPlaylist(List<Music> musics, String sessionId) {
            Optional<User> userOpt = userService.getUser(sessionId);
            if (userOpt.isEmpty() || musics == null || musics.isEmpty()) return;
            User enqueuer = userOpt.get();
            long existingUserCount = queueManager.getQueueSnapshot().stream()
                    .filter(i -> i.enqueuedBy().publicId().equals(enqueuer.getPublicId()))
                    .count();
            int remaining = Math.max(0, appProperties.getQueue().getMaxUserSongs() - (int) existingUserCount);
            int limit = Math.min(remaining, appProperties.getPlayer().getMaxPlaylistImportSize());
            if (limit <= 0) return;

            int count = 0;
            UserSummary summary = new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest());
            for (Music music : musics.stream().limit(limit).toList()) {
                QueueItemStatus status = isCachedPlatform(music.platform()) ? QueueItemStatus.PENDING : QueueItemStatus.READY;
                if (isCachedPlatform(music.platform())) getApiService(music.platform()).prefetchMusic(music.id());
                if (queueManager.add(music, summary, status) != null) count++;
            }
            if (count == 0) return;
            playbackState.touchHotActivity();
            persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.IMPORT_PLAYLIST, enqueuer.getPublicId(), String.valueOf(count), roomId), false);
            if (playbackState.currentMusic() == null) playNextInQueue();
        }

        public synchronized void topSong(String queueId, String sessionId) {
            TopResult result = queueManager.top(queueId, playbackState.isShuffle());
            if (result != TopResult.NONE) {
                playbackState.touchHotActivity();
                SystemMessageEvent systemMessage = result == TopResult.GLOBAL
                        ? new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.TOP, getUserPublicId(sessionId), "置顶成功", roomId)
                        : null;
                persistQueueMutation(systemMessage, false);
                if (playbackState.currentMusic() == null) playNextInQueue();
            }
        }

        public synchronized void topSongs(List<String> queueIds, String sessionId) {
            List<MusicQueueItem> topped = queueManager.topManyGlobal(queueIds);
            if (!topped.isEmpty()) {
                playbackState.touchHotActivity();
                persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.TOP, getUserPublicId(sessionId), "置顶 " + topped.size() + " 首歌曲", roomId), false);
            }
        }

        public void removeSongFromQueue(String queueId, String sessionId) {
            queueManager.remove(queueId).ifPresent(item -> {
                playbackState.touchHotActivity();
                persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.REMOVE, getUserPublicId(sessionId), item.music().name(), roomId), false);
            });
        }

        public void removeSongsFromQueue(List<String> queueIds, String sessionId) {
            List<MusicQueueItem> removed = queueManager.removeMany(queueIds);
            if (!removed.isEmpty()) {
                playbackState.touchHotActivity();
                persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.REMOVE, getUserPublicId(sessionId), "移除 " + removed.size() + " 首歌曲", roomId), true);
            }
        }

        public synchronized void reorderQueue(int oldIndex, int newIndex, String sessionId) {
            boolean changed = queueManager.reorder(oldIndex, newIndex);
            if (!changed) return;
            playbackState.touchHotActivity();
            persistQueueMutation(null, true);
        }

        public synchronized void reorderQueue(String queueId, String targetQueueId, String position, String sessionId) {
            boolean changed = queueManager.reorderByQueueId(queueId, targetQueueId, position);
            if (!changed) return;
            playbackState.touchHotActivity();
            persistQueueMutation(null, true);
        }

        public synchronized ControlResult skipToNext(String sessionId) {
            if (isRateLimited(sessionId)) return ControlResult.COOLDOWN;
            if (playbackState.isSkipLocked() && !"SYSTEM".equals(sessionId)) return ControlResult.LOCKED;
            playHeadVersion.incrementAndGet();
            playbackState.setLoading(false);
            playbackState.clearCurrentTrack();
            playbackState.touchHotActivity();
            roomStateMutationService.runInTransaction(() -> {
                persistPlaybackStateSnapshot();
                playbackState.bumpPlayEpochAndStateVersion();
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.SKIP, getUserPublicId(sessionId), null, roomId));
            });
            playNextInQueue();
            return ControlResult.OK;
        }

        public synchronized ControlResult togglePause(String sessionId) {
            if (playbackState.currentMusic() == null) {
                if (!queueManager.getQueueSnapshot().isEmpty()) {
                    playNextInQueue();
                    return ControlResult.OK;
                }
                return ControlResult.EMPTY_QUEUE;
            }
            if (isRateLimited(sessionId)) return ControlResult.COOLDOWN;
            if (!"SYSTEM".equals(sessionId) && playbackState.isPauseLocked() && !playbackState.isPaused()) return ControlResult.LOCKED;
            long currentPos = playbackState.calculateCurrentPosition();
            boolean newState = !playbackState.isPaused();
            idlePaused.set(false);
            playbackState.setPaused(newState);
            playbackState.updatePlaybackAnchor(currentPos);
            playbackState.touchHotActivity();
            playbackState.bumpStateVersion();
            applyPlaybackOnlyTransition(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, newState ? PlayerAction.PAUSE : PlayerAction.PLAY, getUserPublicId(sessionId), null, roomId));
            return ControlResult.OK;
        }

        public synchronized Optional<String> seekTo(long positionMs, String sessionId, boolean adminOverride) {
            PlayableMusic music = playbackState.currentMusic();
            if (music == null) return Optional.of("当前没有正在播放的歌曲");
            if (!adminOverride && !Objects.equals(playbackState.currentEnqueuerId(), getUserPublicId(sessionId))) return Optional.of("只有点播者可以调整这首歌的进度");
            long clamped = music.duration() > 0 ? Math.max(0, Math.min(positionMs, music.duration())) : Math.max(0, positionMs);
            playbackState.updatePlaybackAnchor(clamped);
            playbackState.touchHotActivity();
            playbackState.bumpPlayEpochAndStateVersion();
            applyPlaybackOnlyTransition(null);
            return Optional.empty();
        }

        public synchronized ControlResult toggleShuffle(String sessionId) {
            if (isRateLimited(sessionId)) return ControlResult.COOLDOWN;
            if (playbackState.isShuffleLocked() && !"SYSTEM".equals(sessionId)) return ControlResult.LOCKED;
            playbackState.setShuffle(!playbackState.isShuffle());
            playbackState.touchHotActivity();
            playbackState.bumpStateVersion();
            applyPlaybackOnlyTransition(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, playbackState.isShuffle() ? PlayerAction.SHUFFLE_ON : PlayerAction.SHUFFLE_OFF, getUserPublicId(sessionId), null, roomId));
            return ControlResult.OK;
        }

        public void likeSong(String sessionId) {
            PlayableMusic music = playbackState.currentMusic();
            if (music == null) return;
            String publicId = getUserPublicId(sessionId);
            if (!playbackState.likedUserIds().add(publicId)) return;
            playbackState.likeMarkers().add(playbackState.calculateCurrentPosition());
            playbackState.touchHotActivity();
            playbackState.bumpStateVersion();
            applyPlaybackOnlyTransition(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.LIKE, publicId, music.name(), roomId));
        }

        public void setLock(String type, boolean locked) {
            boolean changed = switch (type.toUpperCase()) {
                case "PAUSE" -> setBooleanIfChanged(playbackState.isPauseLocked(), locked, playbackState::setPauseLocked);
                case "SKIP" -> setBooleanIfChanged(playbackState.isSkipLocked(), locked, playbackState::setSkipLocked);
                case "SHUFFLE" -> setBooleanIfChanged(playbackState.isShuffleLocked(), locked, playbackState::setShuffleLocked);
                default -> throw new IllegalArgumentException("Unknown lock type");
            };
            if (changed) {
                playbackState.bumpStateVersion();
                applyPlaybackOnlyTransition(null);
            }
        }

        public void setAllLocks(boolean locked) {
            playbackState.setPauseLocked(locked);
            playbackState.setSkipLocked(locked);
            playbackState.setShuffleLocked(locked);
            playbackState.bumpStateVersion();
            applyPlaybackOnlyTransition(null);
        }

        public void resetSystem(boolean notify) {
            resetSystem(notify, true);
        }

        public void resetSystem(boolean notify, boolean persist) {
            playbackState.clearCurrentTrack();
            queueManager.clearAll();
            playbackState.setPaused(false);
            idlePaused.set(false);
            playbackState.setShuffle(false);
            playbackState.setLoading(false);
            roomStateMutationService.runInTransaction(() -> {
                if (persist) {
                    roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                    replacePersistentHistorySnapshot();
                    persistPlaybackStateSnapshot();
                }
                playbackState.bumpPlayEpochAndStateVersion();
                broadcastFullPlayerState();
                broadcastQueueUpdate();
                if (notify) {
                    eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, PlayerAction.RESET, "SYSTEM", null, roomId));
                }
            });
        }

        public void clearQueue() {
            queueManager.clearPendingQueue();
            playbackState.touchHotActivity();
            persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, PlayerAction.REMOVE, "SYSTEM", "播放列表已由管理员清空", roomId), false);
        }

        public void handleDownloadEvent(DownloadStatusEvent event) {
            boolean exists = queueManager.getQueueSnapshot().stream().anyMatch(item -> downloadEventMatches(item.music(), event.getMusicId()));
            if (exists) {
                broadcastQueueUpdate();
                if (playbackState.currentMusic() == null) playNextInQueue();
            }
            // 如果当前正在等下载的歌就是这首，reresolve url 并更新 currentMusic
            // 这处理"currentMusic 已经是 PENDING_DOWNLOAD 但事件后来才到"的存量情况
            // （主要来自持久化恢复路径，playNextInQueue 路径已有自己的轮询）
            PlayableMusic current = playbackState.currentMusic();
            if (current != null && PENDING_DOWNLOAD_URL.equals(current.url()) && downloadEventMatches(current, event.getMusicId())) {
                reresolveCurrentPlayableUrl(current);
            }
        }

        public void onUserCountChanged(UserCountChangeEvent event) {
            int previousOnlineCount = onlineUserCount.getAndSet(event.getOnlineUserCount());
            if (event.getOnlineUserCount() > 0) {
                playbackState.touchHotActivity();
                resumeFromIdlePauseIfNeeded(previousOnlineCount == 0);
            } else if (!isStreamActive.get()) {
                if (userService.hasPendingReconnectUsers(roomId)) {
                    broadcastFullPlayerState();
                    return;
                }
                enterIdleMode();
            }
            broadcastFullPlayerState();
        }

        public void onStreamStatusChanged(StreamStatusEvent event) {
            isStreamActive.set(event.isHasListeners());
            if (event.isHasListeners()) {
                playbackState.touchHotActivity();
                resumeFromIdlePauseIfNeeded(false);
                if (playbackState.currentMusic() == null) playNextInQueue();
            }
            else if (!event.isHasListeners() && userService.getOnlineUserSummaries(roomId).isEmpty()) enterIdleMode();
            playbackState.bumpStateVersion();
            broadcastFullPlayerState();
        }

        public void cleanupIdlePlayer() {
            if (idlePaused.get() && playbackState.isPaused() && playbackState.currentMusic() != null) {
                long pausedDuration = System.currentTimeMillis() - currentPlaybackStateSnapshot().timestampAnchor();
                if (pausedDuration > IDLE_RESET_TIMEOUT_MS) {
                    playbackState.clearCurrentTrack();
                    playbackState.setPaused(false);
                    idlePaused.set(false);
                    persistPlaybackStateSnapshot();
                    playbackState.bumpPlayEpochAndStateVersion();
                    broadcastFullPlayerState();
                }
            }
        }

        public void broadcastQueueUpdate() {
            eventPublisher.publishEvent(new QueueUpdateEvent(this, roomId, getQueueWithUpdatedStatus()));
        }

        public void broadcastFullPlayerState() {
            eventPublisher.publishEvent(new PlayerStateEvent(this, roomId, getCurrentPlayerState()));
        }

        private void enterIdleMode() {
            playbackState.setLoading(false);
            if (playbackState.currentMusic() != null) {
                idlePaused.set(true);
                if (!playbackState.isPaused()) {
                    playbackState.updatePlaybackAnchor(playbackState.calculateCurrentPosition());
                    playbackState.setPaused(true);
                    playbackState.bumpStateVersion();
                    applyPlaybackOnlyTransition(null);
                }
            }
        }

        private void resumeFromIdlePauseIfNeeded(boolean resumeStaleEmptyRoomPause) {
            boolean shouldResume = idlePaused.compareAndSet(true, false) || resumeStaleEmptyRoomPause;
            if (!shouldResume) return;
            if (playbackState.currentMusic() == null || !playbackState.isPaused()) return;
            playbackState.updatePlaybackAnchor(playbackState.calculateCurrentPosition());
            playbackState.setPaused(false);
            playbackState.bumpStateVersion();
            applyPlaybackOnlyTransition(null);
        }

        private void restorePersistentState() {
            queueManager.restore(
                    roomStatePersistenceService.loadQueue(roomId),
                    roomStatePersistenceService.loadHistory(roomId, appProperties.getQueue().getHistorySize())
            );
            roomStatePersistenceService.loadPlaybackState(roomId).ifPresent(this::applyPersistedPlaybackState);
        }

        private void applyPersistedPlaybackState(PersistedPlaybackState state) {
            playbackState.applySnapshot(state);
            idlePaused.set(state.currentMusic() != null
                    && state.paused()
                    && userService.getOnlineUserSummaries(roomId).isEmpty()
                    && !isStreamActive.get());
            refreshRestoredPlayableUrl(state.currentMusic());
        }

        private void refreshRestoredPlayableUrl(PlayableMusic restoredMusic) {
            if (restoredMusic == null) {
                return;
            }
            // 持久化恢复路径：如果当前歌 url 是 PENDING_DOWNLOAD，交给统一的重 resolve 方法
            reresolveCurrentPlayableUrl(restoredMusic);
        }

        // 统一的"重新解析当前歌 url"逻辑：用于持久化恢复和下载完成事件两条路径
        // 重新调 getPlayableMusic，拿到有效 url 后 setCurrentTrack（不 bump playEpoch，保持进度连续）
        private void reresolveCurrentPlayableUrl(PlayableMusic targetMusic) {
            if (targetMusic == null) return;
            long version = playHeadVersion.incrementAndGet();
            try {
                getApiService(targetMusic.platform()).getPlayableMusic(targetMusic.id())
                        .timeout(Duration.ofSeconds(10))
                        .subscribe(refreshed -> {
                            PlayableMusic current = playbackState.currentMusic();
                            if (playHeadVersion.get() != version
                                    || current == null
                                    || !Objects.equals(current.id(), targetMusic.id())
                                    || !Objects.equals(current.platform(), targetMusic.platform())) {
                                return;
                            }
                            // 如果仍是 PENDING_DOWNLOAD，说明下载还没好，不做任何事（轮询或下次事件会再触发）
                            if (PENDING_DOWNLOAD_URL.equals(refreshed.url())) {
                                log.debug("Re-resolve still PENDING for {}:{}", targetMusic.platform(), targetMusic.id());
                                return;
                            }
                            playbackState.setCurrentTrack(
                                    refreshed,
                                    playbackState.currentEnqueuerId(),
                                    playbackState.currentEnqueuerName()
                            );
                            playbackState.bumpStateVersion();
                            applyPlaybackOnlyTransition(null);
                        }, error -> log.warn(
                                "Failed to re-resolve playable URL for {}:{} in room {}",
                                targetMusic.platform(),
                                targetMusic.id(),
                                roomId,
                                error
                        ));
            } catch (Exception e) {
                log.warn(
                        "Failed to start re-resolve playable URL for {}:{} in room {}",
                        targetMusic.platform(),
                        targetMusic.id(),
                        roomId,
                        e
                );
            }
        }

        private void persistPlaybackStateSnapshot() {
            roomStatePersistenceService.persistPlaybackState(currentPlaybackStateSnapshot());
        }

        private void appendFinishedTrackToHistory(Music finishedMusic) {
            queueManager.addToHistory(finishedMusic);
            roomStatePersistenceService.appendHistoryEntry(roomId, finishedMusic, playbackState.currentEnqueuerId());
        }

        private void replacePersistentHistorySnapshot() {
            roomStatePersistenceService.persistHistorySnapshot(roomId, currentHistorySnapshot());
        }

        private List<Music> currentHistorySnapshot() {
            return queueManager.getHistorySnapshot();
        }

        public void flushPersistentState() {
            roomStatePersistenceService.flushPlayerState(
                    roomId,
                    queueManager.getQueueSnapshot(),
                    currentHistorySnapshot(),
                    currentPlaybackStateSnapshot()
            );
        }

        private void persistQueueMutation(SystemMessageEvent systemMessageEvent, boolean broadcastFullState) {
            roomSessionCoordinator.markRoomActive(roomId);
            roomStateMutationService.runInTransaction(() -> {
                roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                if (broadcastFullState) {
                    broadcastFullPlayerState();
                }
                broadcastQueueUpdate();
                if (systemMessageEvent != null) {
                    eventPublisher.publishEvent(systemMessageEvent);
                }
            });
        }

        private void applyPlaybackTransition(boolean persistQueue, boolean broadcastQueue, boolean broadcastPlayerState, SystemMessageEvent systemMessageEvent) {
            roomSessionCoordinator.markRoomActive(roomId);
            playbackTransitionService.apply(new PlaybackTransitionService.PlaybackTransition(
                    roomId,
                    persistQueue ? queueManager.getQueueSnapshot() : null,
                    currentPlaybackStateSnapshot(),
                    broadcastQueue ? new QueueUpdateEvent(this, roomId, getQueueWithUpdatedStatus()) : null,
                    broadcastPlayerState ? new PlayerStateEvent(this, roomId, getCurrentPlayerState()) : null,
                    systemMessageEvent
            ));
        }

        private PersistedPlaybackState currentPlaybackStateSnapshot() {
            return playbackState.snapshot(roomId);
        }

        private void applyPlaybackOnlyTransition(SystemMessageEvent systemMessageEvent) {
            roomSessionCoordinator.markRoomActive(roomId);
            playbackTransitionService.apply(new PlaybackTransitionService.PlaybackTransition(
                    roomId,
                    null,
                    currentPlaybackStateSnapshot(),
                    null,
                    new PlayerStateEvent(this, roomId, getCurrentPlayerState()),
                    systemMessageEvent
            ));
        }

        public boolean isEvictable(long now, long idleThresholdMs) {
            return userService.getOnlineCount(roomId) == 0
                    && !isStreamActive.get()
                    && !playbackState.isLoading()
                    && (now - playbackState.lastHotActivityAt()) >= idleThresholdMs;
        }

        private boolean setBooleanIfChanged(boolean currentValue, boolean nextValue, Consumer<Boolean> setter) {
            if (currentValue == nextValue) {
                return false;
            }
            setter.accept(nextValue);
            return true;
        }

        private List<MusicQueueItem> getQueueWithUpdatedStatus() {
            return queueManager.getQueueSnapshot().stream().map(item -> {
                if ("netease".equals(item.music().platform())) {
                    return item.status() == QueueItemStatus.READY ? item : item.withStatus(QueueItemStatus.READY);
                }
                if (isCachedPlatform(item.music().platform())) {
                    QueueItemStatus newStatus = mapCacheStatusToEnum(localCacheService.getStatus(cacheKey(item.music())));
                    if (item.status() != newStatus) return item.withStatus(newStatus);
                }
                return item;
            }).collect(Collectors.toList());
        }

        private Map<String, QueueItemStatus> buildStatusMap() {
            Map<String, QueueItemStatus> statusMap = new HashMap<>();
            for (MusicQueueItem item : queueManager.getQueueSnapshot()) {
                statusMap.put(MusicQueueManager.musicKey(item.music()), isCachedPlatform(item.music().platform())
                        ? mapCacheStatusToEnum(localCacheService.getStatus(cacheKey(item.music())))
                        : QueueItemStatus.READY);
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

        private boolean isRateLimited(String userId) {
            long now = System.currentTimeMillis();
            AtomicLong lastControlTimestamp = lastControlTimestamps.computeIfAbsent(userId, key -> new AtomicLong(0));
            while (true) {
                long last = lastControlTimestamp.get();
                if (now - last < GLOBAL_COOLDOWN_MS) return true;
                if (lastControlTimestamp.compareAndSet(last, now)) return false;
            }
        }

        private boolean isCachedPlatform(String platform) {
            return apiServiceMap.get(platform) instanceof CachedMusicApiService;
        }

        private String cacheKey(Music music) {
            IMusicApiService service = apiServiceMap.get(music.platform());
            if (service instanceof CachedMusicApiService cached) {
                return cached.cacheKey(music.id());
            }
            return music.id();
        }

        private boolean downloadEventMatches(Music music, String eventMusicId) {
            return Objects.equals(music.id(), eventMusicId) || Objects.equals(cacheKey(music), eventMusicId);
        }

        // PlayableMusic 重载：currentMusic 是 PlayableMusic 类型
        private boolean downloadEventMatches(PlayableMusic music, String eventMusicId) {
            if (Objects.equals(music.id(), eventMusicId)) return true;
            // cacheKey 接受 Music，PlayableMusic 与 Music 的 platform/id 同构，构造临时 Music 取 key
            return Objects.equals(cacheKey(new Music(music.id(), music.name(), music.artists(), music.duration(), music.platform(), music.coverUrl())), eventMusicId);
        }
    }

    private boolean isSubsonicPlatform(String platform) {
        return subsonicMusicApiService != null && subsonicMusicApiService.supports(platform);
    }

    private boolean canUseSubsonicInRoom(String platform, String sessionId) {
        if (subsonicSourceRegistry == null) return false;
        String roomId = roomIdFromPlatform(platform);
        Optional<RoomSubsonicSource> source = subsonicSourceRegistry.findRoomSourceByPlatformId(roomId, platform);
        if (source.isEmpty() || !source.get().active()) return false;
        String allowedUsers = source.get().allowedUsers();
        if (allowedUsers != null && allowedUsers.contains("*")) {
            return userService.getUser(sessionId).filter(user -> !user.isGuest()).isPresent();
        }
        return navidromeAccessService.canUseBySession(sessionId, allowedUsers);
    }

    private String roomIdFromPlatform(String platform) {
        int marker = platform == null ? -1 : platform.indexOf('@');
        return marker >= 0 ? platform.substring(marker + 1) : RoomService.DEFAULT_ROOM_ID;
    }

    private IMusicApiService getApiService(String platform) {
        if (subsonicMusicApiService != null && subsonicMusicApiService.supports(platform)) {
            return new IMusicApiService() {
                @Override
                public String getPlatformName() {
                    return platform;
                }

                @Override
                public reactor.core.publisher.Mono<List<Music>> searchMusic(String keyword) {
                    return subsonicMusicApiService.searchMusic(roomIdFromPlatform(platform), platform, keyword, 0, 20);
                }

                @Override
                public reactor.core.publisher.Mono<List<Music>> searchMusic(String keyword, int offset, int limit) {
                    return subsonicMusicApiService.searchMusic(roomIdFromPlatform(platform), platform, keyword, offset, limit);
                }

                @Override
                public reactor.core.publisher.Mono<PlayableMusic> getPlayableMusic(String musicId) {
                    return subsonicMusicApiService.getPlayableMusic(platform, musicId);
                }

                @Override
                public reactor.core.publisher.Mono<List<Playlist>> getUserPlaylists(String userId) {
                    return reactor.core.publisher.Mono.just(List.of());
                }

                @Override
                public reactor.core.publisher.Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) {
                    return reactor.core.publisher.Mono.just(List.of());
                }

                @Override
                public reactor.core.publisher.Mono<List<UserSearchResult>> searchUsers(String keyword) {
                    return reactor.core.publisher.Mono.just(List.of());
                }

                @Override
                public reactor.core.publisher.Mono<String> getLyric(String musicId) {
                    return reactor.core.publisher.Mono.just("");
                }

                @Override
                public reactor.core.publisher.Mono<LyricResponse> getLyricDetail(String musicId) {
                    return reactor.core.publisher.Mono.just(new LyricResponse("", "", ""));
                }

                @Override
                public reactor.core.publisher.Mono<List<Album>> searchAlbums(String keyword) {
                    return subsonicMusicApiService.searchAlbums(roomIdFromPlatform(platform), platform, keyword);
                }

                @Override
                public reactor.core.publisher.Mono<List<Music>> getAlbumMusics(String albumId) {
                    return subsonicMusicApiService.getAlbumMusics(roomIdFromPlatform(platform), platform, albumId);
                }
            };
        }
        IMusicApiService service = apiServiceMap.get(platform);
        if (service == null) throw new ApiRequestException("Unsupported platform: " + platform);
        return service;
    }

    private String getUserPublicId(String sessionId) {
        return userService.getUser(sessionId).map(User::getPublicId).orElse("UNKNOWN_USER");
    }
}
