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
import org.thornex.musicparty.service.api.IMusicApiService;
import org.thornex.musicparty.service.api.NeteaseMusicApiService;
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
    private final LiveStreamService liveStreamService;
    private final ApplicationEventPublisher eventPublisher;
    private final AppProperties appProperties;
    private final NavidromeAccessService navidromeAccessService;
    private final RoomService roomService;
    private final RoomStatePersistenceService roomStatePersistenceService;
    private final Map<String, RoomPlayerSession> sessions = new ConcurrentHashMap<>();

    public MusicPlayerService(List<IMusicApiService> apiServices,
                              UserService userService,
                              LocalCacheService localCacheService,
                              LiveStreamService liveStreamService,
                              ApplicationEventPublisher eventPublisher,
                              AppProperties appProperties,
                              NavidromeAccessService navidromeAccessService,
                              RoomService roomService,
                              RoomStatePersistenceService roomStatePersistenceService) {
        this.apiServiceMap = apiServices.stream().collect(Collectors.toMap(IMusicApiService::getPlatformName, Function.identity()));
        this.userService = userService;
        this.localCacheService = localCacheService;
        this.liveStreamService = liveStreamService;
        this.eventPublisher = eventPublisher;
        this.appProperties = appProperties;
        this.navidromeAccessService = navidromeAccessService;
        this.roomService = roomService;
        this.roomStatePersistenceService = roomStatePersistenceService;
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
        getActiveRoomIds().forEach(roomId -> session(roomId).playerLoop());
    }

    @Scheduled(fixedRate = 600000)
    public void cleanupIdlePlayer() {
        sessions.values().forEach(RoomPlayerSession::cleanupIdlePlayer);
    }

    @Scheduled(fixedRate = 300000)
    public void evictColdRooms() {
        long now = System.currentTimeMillis();
        long idleThresholdMs = appProperties.getPlayer().getRoomEvictionIdleMs();
        List<String> coldRoomIds = sessions.values().stream()
                .filter(session -> !RoomService.DEFAULT_ROOM_ID.equals(session.getRoomId()))
                .filter(session -> session.isEvictable(now, idleThresholdMs))
                .map(RoomPlayerSession::getRoomId)
                .toList();

        for (String roomId : coldRoomIds) {
            RoomPlayerSession removed = sessions.remove(roomId);
            if (removed != null) {
                removed.flushPersistentState();
                eventPublisher.publishEvent(new RoomSessionEvictedEvent(this, roomId));
                log.info("Evicted cold room session {}", roomId);
            }
        }
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

    public void skipToNext(String sessionId) {
        sessionForUser(sessionId).skipToNext(sessionId);
    }

    public void togglePause(String sessionId) {
        sessionForUser(sessionId).togglePause(sessionId);
    }

    public Optional<String> seekTo(long positionMs, String sessionId) {
        return sessionForUser(sessionId).seekTo(positionMs, sessionId);
    }

    public void toggleShuffle(String sessionId) {
        sessionForUser(sessionId).toggleShuffle(sessionId);
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
        return sessions.computeIfAbsent(normalized, key -> {
            RoomPlayerSession session = new RoomPlayerSession(key);
            session.restorePersistentState();
            return session;
        });
    }

    public class RoomPlayerSession {
        private final String roomId;
        private final MusicQueueManager queueManager;
        private final AtomicReference<PlayableMusic> currentMusic = new AtomicReference<>(null);
        private final AtomicReference<String> currentEnqueuerId = new AtomicReference<>(null);
        private final AtomicReference<String> currentEnqueuerName = new AtomicReference<>(null);
        private final AtomicLong positionAnchor = new AtomicLong(0);
        private final AtomicLong timestampAnchor = new AtomicLong(0);
        private final AtomicLong positionUpdatedAt = new AtomicLong(0);
        private final AtomicBoolean isShuffle = new AtomicBoolean(false);
        private final AtomicBoolean isPaused = new AtomicBoolean(false);
        private final AtomicBoolean isPauseLocked = new AtomicBoolean(false);
        private final AtomicBoolean isSkipLocked = new AtomicBoolean(false);
        private final AtomicBoolean isShuffleLocked = new AtomicBoolean(false);
        private final AtomicBoolean isLoading = new AtomicBoolean(false);
        private final AtomicBoolean isStreamActive = new AtomicBoolean(false);
        private final Set<String> currentLikedUserIds = ConcurrentHashMap.newKeySet();
        private final List<Long> currentLikeMarkers = new CopyOnWriteArrayList<>();
        private final AtomicLong lastControlTimestamp = new AtomicLong(0);
        private final AtomicLong playHeadVersion = new AtomicLong(0);
        private final AtomicLong stateVersion = new AtomicLong(0);
        private final AtomicLong playEpoch = new AtomicLong(0);
        private final AtomicLong lastHotActivityAt = new AtomicLong(System.currentTimeMillis());

        private static final long GLOBAL_COOLDOWN_MS = 1000;
        private static final long IDLE_RESET_TIMEOUT_MS = 2 * 60 * 60 * 1000L;

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

        public void playerLoop() {
            if (isPaused.get()) return;
            PlayableMusic music = currentMusic.get();
            if (music != null) {
                long currentPos = calculateCurrentPosition();
                if (currentPos >= music.duration() && music.duration() > 0) {
                    Music finishedMusic = new Music(music.id(), music.name(), music.artists(), music.duration(), music.platform(), music.coverUrl());
                    queueManager.addToHistory(finishedMusic);
                    roomStatePersistenceService.appendHistoryEntry(roomId, finishedMusic, currentEnqueuerId.get());
                    roomStatePersistenceService.persistHistorySnapshot(roomId, queueManager.getHistorySnapshot());
                    currentMusic.set(null);
                    currentEnqueuerId.set(null);
                    currentEnqueuerName.set(null);
                    persistPlaybackStateSnapshot();
                    bumpPlayEpochAndStateVersion();
                    playNextInQueue();
                }
                return;
            }
            if (userService.getOnlineUserSummaries(roomId).isEmpty() && !isStreamActive.get()) return;
            if (!queueManager.getQueueSnapshot().isEmpty()) playNextInQueue();
        }

        private synchronized void playNextInQueue() {
            if (currentMusic.get() != null || isLoading.get()) return;
            MusicQueueItem nextItem = queueManager.pollNext(isShuffle.get(), buildStatusMap(), userService.getRecentlyActivePublicIds(roomId));
            if (nextItem == null) {
                isLoading.set(false);
                persistPlaybackStateSnapshot();
                broadcastFullPlayerState();
                return;
            }
            roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
            if (nextItem.status() == QueueItemStatus.FAILED) {
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, "SYSTEM", "加载失败: " + nextItem.music().name(), roomId));
                playNextInQueue();
                return;
            }
            long version = playHeadVersion.incrementAndGet();
            isLoading.set(true);
            touchHotActivity();
            persistPlaybackStateSnapshot();
            bumpStateVersion();
            broadcastFullPlayerState();
            isPaused.set(false);
            try {
                getApiService(nextItem.music().platform()).getPlayableMusic(nextItem.music().id())
                        .timeout(Duration.ofSeconds(10))
                        .subscribe(playable -> {
                            if (playHeadVersion.get() == version) applyNewSong(playable, nextItem);
                        }, error -> {
                            log.error("Play failed for {} in room {}", nextItem.music().name(), roomId, error);
                            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, "SYSTEM", nextItem.music().name(), roomId));
                            isLoading.set(false);
                            persistPlaybackStateSnapshot();
                            bumpStateVersion();
                            broadcastFullPlayerState();
                            playNextInQueue();
                        });
            } catch (Exception e) {
                isLoading.set(false);
                persistPlaybackStateSnapshot();
                bumpStateVersion();
                broadcastFullPlayerState();
            }
        }

        private void applyNewSong(PlayableMusic music, MusicQueueItem queueItem) {
            currentLikedUserIds.clear();
            currentLikeMarkers.clear();
            currentMusic.set(music);
            currentEnqueuerId.set(queueItem.enqueuedBy().publicId());
            currentEnqueuerName.set(queueItem.enqueuedBy().name());
            updatePlaybackAnchor(0);
            isPaused.set(false);
            isLoading.set(false);
            touchHotActivity();
            persistPlaybackStateSnapshot();
            bumpPlayEpochAndStateVersion();
            broadcastFullPlayerState();
            broadcastQueueUpdate();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.PLAY_START, queueItem.enqueuedBy().publicId(), music.name(), roomId));
        }

        public PlayerState getCurrentPlayerState() {
            PlayableMusic music = currentMusic.get();
            NowPlayingInfo info = music == null ? null : new NowPlayingInfo(
                    music,
                    calculateCurrentPosition(),
                    currentEnqueuerId.get(),
                    currentEnqueuerName.get(),
                    currentLikedUserIds,
                    currentLikeMarkers,
                    playEpoch.get(),
                    positionUpdatedAt.get()
            );
            return new PlayerState(
                    info,
                    getQueueWithUpdatedStatus(),
                    isShuffle.get(),
                    userService.getOnlineUserSummaries(roomId),
                    isPaused.get(),
                    isPauseLocked.get(),
                    isSkipLocked.get(),
                    isShuffleLocked.get(),
                    isLoading.get(),
                    RoomService.DEFAULT_ROOM_ID.equals(roomId) ? liveStreamService.getStreamListenerCount() : 0,
                    System.currentTimeMillis(),
                    stateVersion.get(),
                    playEpoch.get()
            );
        }

        public void enqueue(EnqueueRequest request, String sessionId) {
            Optional<User> userOpt = userService.getUser(sessionId);
            if (userOpt.isEmpty()) return;
            User enqueuer = userOpt.get();
            if ("navidrome".equals(request.platform()) && !navidromeAccessService.canUseBySession(sessionId)) {
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: 无权使用 Navidrome", roomId));
                return;
            }
            long count = queueManager.getQueueSnapshot().stream().filter(i -> i.enqueuedBy().publicId().equals(enqueuer.getPublicId())).count();
            if (count >= appProperties.getQueue().getMaxUserSongs()) return;
            IMusicApiService service = getApiService(request.platform());
            service.getPlayableMusic(request.musicId()).subscribe(playable -> {
                Music music = new Music(playable.id(), playable.name(), playable.artists(), playable.duration(), playable.platform(), playable.coverUrl());
                QueueItemStatus initialStatus = "bilibili".equals(request.platform()) ? QueueItemStatus.PENDING : QueueItemStatus.READY;
                if ("bilibili".equals(request.platform())) service.prefetchMusic(music.id());
                MusicQueueItem item = queueManager.add(music, new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest()), initialStatus);
                if (item != null) {
                    touchHotActivity();
                    roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                    broadcastQueueUpdate();
                    eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.ADD, enqueuer.getPublicId(), music.name(), roomId));
                    if (currentMusic.get() == null) playNextInQueue();
                }
            }, error -> eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.ERROR, PlayerAction.ERROR_LOAD, enqueuer.getPublicId(), "添加失败: " + error.getMessage(), roomId)));
        }

        public void enqueuePlaylist(EnqueuePlaylistRequest request, String sessionId) {
            Optional<User> userOpt = userService.getUser(sessionId);
            if (userOpt.isEmpty()) return;
            User enqueuer = userOpt.get();
            if ("navidrome".equals(request.platform())) return;
            int importLimit = appProperties.getPlayer().getMaxPlaylistImportSize();
            IMusicApiService service = getApiService(request.platform());
            service.getPlaylistMusics(request.playlistId(), 0, importLimit).subscribe(musics -> {
                int count = 0;
                QueueItemStatus initialStatus = "bilibili".equals(request.platform()) ? QueueItemStatus.PENDING : QueueItemStatus.READY;
                for (Music music : musics) {
                    if ("bilibili".equals(request.platform())) service.prefetchMusic(music.id());
                    if (queueManager.add(music, new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest()), initialStatus) != null) count++;
                }
                touchHotActivity();
                roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                broadcastQueueUpdate();
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.IMPORT_PLAYLIST, enqueuer.getPublicId(), String.valueOf(count), roomId));
                if (currentMusic.get() == null) playNextInQueue();
            });
        }

        public void enqueueAlbum(EnqueueAlbumRequest request, String sessionId) {
            Optional<User> userOpt = userService.getUser(sessionId);
            if (userOpt.isEmpty() || !"netease".equals(request.platform())) return;
            User enqueuer = userOpt.get();
            IMusicApiService service = getApiService(request.platform());
            if (!(service instanceof NeteaseMusicApiService neteaseService)) return;
            neteaseService.getAlbumMusics(request.albumId()).subscribe(musics -> {
                int count = 0;
                for (Music music : musics.stream().limit(appProperties.getPlayer().getMaxPlaylistImportSize()).toList()) {
                    if (queueManager.add(music, new UserSummary(enqueuer.getPublicId(), enqueuer.getName(), enqueuer.isGuest()), QueueItemStatus.READY) != null) count++;
                }
                touchHotActivity();
                roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                broadcastQueueUpdate();
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.IMPORT_PLAYLIST, enqueuer.getPublicId(), String.valueOf(count), roomId));
                if (currentMusic.get() == null) playNextInQueue();
            });
        }

        public synchronized void topSong(String queueId, String sessionId) {
            TopResult result = queueManager.top(queueId, isShuffle.get());
            if (result != TopResult.NONE) {
                touchHotActivity();
                roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                broadcastQueueUpdate();
                if (result == TopResult.GLOBAL) eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.TOP, getUserPublicId(sessionId), "置顶成功", roomId));
                if (currentMusic.get() == null) playNextInQueue();
            }
        }

        public synchronized void topSongs(List<String> queueIds, String sessionId) {
            List<MusicQueueItem> topped = queueManager.topManyGlobal(queueIds);
            if (!topped.isEmpty()) {
                touchHotActivity();
                roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                broadcastQueueUpdate();
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.TOP, getUserPublicId(sessionId), "置顶 " + topped.size() + " 首歌曲", roomId));
            }
        }

        public void removeSongFromQueue(String queueId, String sessionId) {
            queueManager.remove(queueId).ifPresent(item -> {
                touchHotActivity();
                roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                broadcastQueueUpdate();
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.REMOVE, getUserPublicId(sessionId), item.music().name(), roomId));
            });
        }

        public void removeSongsFromQueue(List<String> queueIds, String sessionId) {
            List<MusicQueueItem> removed = queueManager.removeMany(queueIds);
            if (!removed.isEmpty()) {
                touchHotActivity();
                roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                broadcastQueueUpdate();
                broadcastFullPlayerState();
                eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.REMOVE, getUserPublicId(sessionId), "移除 " + removed.size() + " 首歌曲", roomId));
            }
        }

        public synchronized void reorderQueue(int oldIndex, int newIndex, String sessionId) {
            queueManager.reorder(oldIndex, newIndex);
            touchHotActivity();
            roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
            broadcastQueueUpdate();
            broadcastFullPlayerState();
        }

        public void skipToNext(String sessionId) {
            if (isRateLimited(sessionId)) return;
            if (isSkipLocked.get() && !"SYSTEM".equals(sessionId)) return;
            playHeadVersion.incrementAndGet();
            isLoading.set(false);
            currentMusic.set(null);
            currentEnqueuerId.set(null);
            currentEnqueuerName.set(null);
            updatePlaybackAnchor(0);
            touchHotActivity();
            persistPlaybackStateSnapshot();
            bumpPlayEpochAndStateVersion();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.SKIP, getUserPublicId(sessionId), null, roomId));
            playNextInQueue();
        }

        public void togglePause(String sessionId) {
            if (currentMusic.get() == null) {
                if (!queueManager.getQueueSnapshot().isEmpty()) playNextInQueue();
                return;
            }
            if (isRateLimited(sessionId)) return;
            if (!"SYSTEM".equals(sessionId) && isPauseLocked.get() && !isPaused.get()) return;
            long currentPos = calculateCurrentPosition();
            boolean newState = !isPaused.get();
            isPaused.set(newState);
            updatePlaybackAnchor(currentPos);
            touchHotActivity();
            persistPlaybackStateSnapshot();
            bumpStateVersion();
            broadcastFullPlayerState();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, newState ? PlayerAction.PAUSE : PlayerAction.PLAY, getUserPublicId(sessionId), null, roomId));
        }

        public synchronized Optional<String> seekTo(long positionMs, String sessionId) {
            PlayableMusic music = currentMusic.get();
            if (music == null) return Optional.of("当前没有正在播放的歌曲");
            if (!Objects.equals(currentEnqueuerId.get(), getUserPublicId(sessionId))) return Optional.of("只有点播者可以调整这首歌的进度");
            long clamped = music.duration() > 0 ? Math.max(0, Math.min(positionMs, music.duration())) : Math.max(0, positionMs);
            updatePlaybackAnchor(clamped);
            touchHotActivity();
            persistPlaybackStateSnapshot();
            bumpPlayEpochAndStateVersion();
            broadcastFullPlayerState();
            return Optional.empty();
        }

        public void toggleShuffle(String sessionId) {
            if (isRateLimited(sessionId) || (isShuffleLocked.get() && !"SYSTEM".equals(sessionId))) return;
            isShuffle.set(!isShuffle.get());
            touchHotActivity();
            persistPlaybackStateSnapshot();
            bumpStateVersion();
            broadcastFullPlayerState();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, isShuffle.get() ? PlayerAction.SHUFFLE_ON : PlayerAction.SHUFFLE_OFF, getUserPublicId(sessionId), null, roomId));
        }

        public void likeSong(String sessionId) {
            PlayableMusic music = currentMusic.get();
            if (music == null) return;
            String publicId = getUserPublicId(sessionId);
            if (!currentLikedUserIds.add(publicId)) return;
            currentLikeMarkers.add(calculateCurrentPosition());
            touchHotActivity();
            bumpStateVersion();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.LIKE, publicId, music.name(), roomId));
            broadcastFullPlayerState();
        }

        public void setLock(String type, boolean locked) {
            AtomicBoolean target = switch (type.toUpperCase()) {
                case "PAUSE" -> isPauseLocked;
                case "SKIP" -> isSkipLocked;
                case "SHUFFLE" -> isShuffleLocked;
                default -> throw new IllegalArgumentException("Unknown lock type");
            };
            if (target.getAndSet(locked) != locked) {
                persistPlaybackStateSnapshot();
                bumpStateVersion();
                broadcastFullPlayerState();
            }
        }

        public void setAllLocks(boolean locked) {
            isPauseLocked.set(locked);
            isSkipLocked.set(locked);
            isShuffleLocked.set(locked);
            persistPlaybackStateSnapshot();
            bumpStateVersion();
            broadcastFullPlayerState();
        }

        public void resetSystem(boolean notify) {
            resetSystem(notify, true);
        }

        public void resetSystem(boolean notify, boolean persist) {
            currentMusic.set(null);
            currentEnqueuerId.set(null);
            currentEnqueuerName.set(null);
            updatePlaybackAnchor(0);
            queueManager.clearAll();
            if (persist) {
                roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
                roomStatePersistenceService.persistHistorySnapshot(roomId, queueManager.getHistorySnapshot());
            }
            isPaused.set(false);
            isShuffle.set(false);
            isLoading.set(false);
            if (persist) {
                persistPlaybackStateSnapshot();
            }
            bumpPlayEpochAndStateVersion();
            broadcastFullPlayerState();
            broadcastQueueUpdate();
            if (notify) eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, PlayerAction.RESET, "SYSTEM", null, roomId));
        }

        public void clearQueue() {
            queueManager.clearPendingQueue();
            touchHotActivity();
            roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
            broadcastQueueUpdate();
            eventPublisher.publishEvent(new SystemMessageEvent(this, SystemMessageEvent.Level.WARN, PlayerAction.REMOVE, "SYSTEM", "播放列表已由管理员清空", roomId));
        }

        public void handleDownloadEvent(DownloadStatusEvent event) {
            boolean exists = queueManager.getQueueSnapshot().stream().anyMatch(item -> item.music().id().equals(event.getMusicId()));
            if (exists) {
                broadcastQueueUpdate();
                if (currentMusic.get() == null) playNextInQueue();
            }
        }

        public void onUserCountChanged(UserCountChangeEvent event) {
            if (event.getOnlineUserCount() > 0) {
                touchHotActivity();
            } else if (!isStreamActive.get()) {
                enterIdleMode();
            }
            broadcastFullPlayerState();
        }

        public void onStreamStatusChanged(StreamStatusEvent event) {
            isStreamActive.set(event.isHasListeners());
            if (event.isHasListeners()) {
                touchHotActivity();
                if (currentMusic.get() == null) playNextInQueue();
            }
            else if (!event.isHasListeners() && userService.getOnlineUserSummaries(roomId).isEmpty()) enterIdleMode();
            bumpStateVersion();
            broadcastFullPlayerState();
        }

        public void cleanupIdlePlayer() {
            if (isPaused.get() && currentMusic.get() != null) {
                long pausedDuration = System.currentTimeMillis() - timestampAnchor.get();
                if (pausedDuration > IDLE_RESET_TIMEOUT_MS) {
                    currentMusic.set(null);
                    currentEnqueuerId.set(null);
                    currentEnqueuerName.set(null);
                    updatePlaybackAnchor(0);
                    isPaused.set(false);
                    persistPlaybackStateSnapshot();
                    bumpPlayEpochAndStateVersion();
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
            isLoading.set(false);
            if (currentMusic.get() != null && !isPaused.get()) {
                updatePlaybackAnchor(calculateCurrentPosition());
                isPaused.set(true);
                persistPlaybackStateSnapshot();
                bumpStateVersion();
                broadcastFullPlayerState();
            }
        }

        private void restorePersistentState() {
            queueManager.restore(
                    roomStatePersistenceService.loadQueue(roomId),
                    roomStatePersistenceService.loadHistory(roomId, appProperties.getQueue().getHistorySize())
            );
            roomStatePersistenceService.loadPlaybackState(roomId).ifPresent(this::applyPersistedPlaybackState);
        }

        private void applyPersistedPlaybackState(PersistedPlaybackState state) {
            currentMusic.set(state.currentMusic());
            currentEnqueuerId.set(state.currentEnqueuerId());
            currentEnqueuerName.set(state.currentEnqueuerName());
            positionAnchor.set(state.positionAnchor());
            timestampAnchor.set(state.timestampAnchor());
            positionUpdatedAt.set(state.positionUpdatedAt());
            isShuffle.set(state.shuffle());
            isPaused.set(state.paused());
            isPauseLocked.set(state.pauseLocked());
            isSkipLocked.set(state.skipLocked());
            isShuffleLocked.set(state.shuffleLocked());
            isLoading.set(state.loading() && state.currentMusic() != null);
            playEpoch.set(state.playEpoch());
            stateVersion.set(state.stateVersion());
            lastHotActivityAt.set(Math.max(state.lastPersistedAt(), System.currentTimeMillis()));
        }

        private void persistPlaybackStateSnapshot() {
            roomStatePersistenceService.persistPlaybackState(new PersistedPlaybackState(
                    roomId,
                    currentMusic.get(),
                    currentEnqueuerId.get(),
                    currentEnqueuerName.get(),
                    positionAnchor.get(),
                    timestampAnchor.get(),
                    positionUpdatedAt.get(),
                    isShuffle.get(),
                    isPaused.get(),
                    isPauseLocked.get(),
                    isSkipLocked.get(),
                    isShuffleLocked.get(),
                    isLoading.get(),
                    playEpoch.get(),
                    stateVersion.get(),
                    System.currentTimeMillis()
            ));
        }

        public void flushPersistentState() {
            roomStatePersistenceService.persistQueueSnapshot(roomId, queueManager.getQueueSnapshot());
            roomStatePersistenceService.persistHistorySnapshot(roomId, queueManager.getHistorySnapshot());
            persistPlaybackStateSnapshot();
        }

        private void touchHotActivity() {
            lastHotActivityAt.set(System.currentTimeMillis());
        }

        public boolean isEvictable(long now, long idleThresholdMs) {
            return userService.getOnlineCount(roomId) == 0
                    && !isStreamActive.get()
                    && !isLoading.get()
                    && (now - lastHotActivityAt.get()) >= idleThresholdMs;
        }

        private long calculateCurrentPosition() {
            if (currentMusic.get() == null) return 0;
            if (isPaused.get()) return positionAnchor.get();
            return positionAnchor.get() + (System.currentTimeMillis() - timestampAnchor.get());
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

        private List<MusicQueueItem> getQueueWithUpdatedStatus() {
            return queueManager.getQueueSnapshot().stream().map(item -> {
                if ("netease".equals(item.music().platform())) {
                    return item.status() == QueueItemStatus.READY ? item : item.withStatus(QueueItemStatus.READY);
                }
                if ("bilibili".equals(item.music().platform())) {
                    QueueItemStatus newStatus = mapCacheStatusToEnum(localCacheService.getStatus(item.music().id()));
                    if (item.status() != newStatus) return item.withStatus(newStatus);
                }
                return item;
            }).collect(Collectors.toList());
        }

        private Map<String, QueueItemStatus> buildStatusMap() {
            Map<String, QueueItemStatus> statusMap = new HashMap<>();
            for (MusicQueueItem item : queueManager.getQueueSnapshot()) {
                statusMap.put(MusicQueueManager.musicKey(item.music()), "bilibili".equals(item.music().platform())
                        ? mapCacheStatusToEnum(localCacheService.getStatus(item.music().id()))
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
            if (now - lastControlTimestamp.get() < GLOBAL_COOLDOWN_MS) return true;
            lastControlTimestamp.set(now);
            return false;
        }
    }

    private IMusicApiService getApiService(String platform) {
        IMusicApiService service = apiServiceMap.get(platform);
        if (service == null) throw new ApiRequestException("Unsupported platform: " + platform);
        return service;
    }

    private String getUserPublicId(String sessionId) {
        return userService.getUser(sessionId).map(User::getPublicId).orElse("UNKNOWN_USER");
    }
}
