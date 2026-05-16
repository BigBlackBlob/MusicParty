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
import java.util.concurrent.atomic.AtomicBoolean;
import java.util.concurrent.atomic.AtomicLong;
import java.util.function.Consumer;
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
    private final RoomSessionCoordinator roomSessionCoordinator;
    private final RoomStatePersistenceService roomStatePersistenceService;
    private final RoomStateMutationService roomStateMutationService;
    private final PlaybackTransitionService playbackTransitionService;
    private final Map<String, RoomPlayerSession> sessions = new ConcurrentHashMap<>();

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
                              PlaybackTransitionService playbackTransitionService) {
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
                roomSessionCoordinator.evictColdRoom(roomId, removed::flushPersistentState);
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

    public void reorderQueue(String queueId, String targetQueueId, String position, String sessionId) {
        sessionForUser(sessionId).reorderQueue(queueId, targetQueueId, position, sessionId);
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
        private final RoomPlaybackState playbackState = new RoomPlaybackState();
        private final AtomicBoolean isStreamActive = new AtomicBoolean(false);
        private final AtomicLong lastControlTimestamp = new AtomicLong(0);
        private final AtomicLong playHeadVersion = new AtomicLong(0);

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
            playbackState.touchHotActivity();
            playbackState.bumpStateVersion();
            applyPlaybackTransition(true, true, true, null);
            try {
                getApiService(nextItem.music().platform()).getPlayableMusic(nextItem.music().id())
                        .timeout(Duration.ofSeconds(10))
                        .subscribe(playable -> {
                            if (playHeadVersion.get() == version) applyNewSong(playable, nextItem);
                        }, error -> {
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
                        });
            } catch (Exception e) {
                playbackState.setLoading(false);
                playbackState.bumpStateVersion();
                applyPlaybackTransition(false, false, true, null);
            }
        }

        private void applyNewSong(PlayableMusic music, MusicQueueItem queueItem) {
            playbackState.startNewTrack(music, queueItem);
            applyPlaybackTransition(
                    false,
                    false,
                    true,
                    new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, PlayerAction.PLAY_START, queueItem.enqueuedBy().publicId(), music.name(), roomId)
            );
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
                    playbackState.touchHotActivity();
                    persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.ADD, enqueuer.getPublicId(), music.name(), roomId), false);
                    if (playbackState.currentMusic() == null) playNextInQueue();
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
                playbackState.touchHotActivity();
                persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.IMPORT_PLAYLIST, enqueuer.getPublicId(), String.valueOf(count), roomId), false);
                if (playbackState.currentMusic() == null) playNextInQueue();
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
                playbackState.touchHotActivity();
                persistQueueMutation(new SystemMessageEvent(this, SystemMessageEvent.Level.SUCCESS, PlayerAction.IMPORT_PLAYLIST, enqueuer.getPublicId(), String.valueOf(count), roomId), false);
                if (playbackState.currentMusic() == null) playNextInQueue();
            });
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
            queueManager.reorder(oldIndex, newIndex);
            playbackState.touchHotActivity();
            persistQueueMutation(null, true);
        }

        public synchronized void reorderQueue(String queueId, String targetQueueId, String position, String sessionId) {
            queueManager.reorderByQueueId(queueId, targetQueueId, position);
            playbackState.touchHotActivity();
            persistQueueMutation(null, true);
        }

        public void skipToNext(String sessionId) {
            if (isRateLimited(sessionId)) return;
            if (playbackState.isSkipLocked() && !"SYSTEM".equals(sessionId)) return;
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
        }

        public void togglePause(String sessionId) {
            if (playbackState.currentMusic() == null) {
                if (!queueManager.getQueueSnapshot().isEmpty()) playNextInQueue();
                return;
            }
            if (isRateLimited(sessionId)) return;
            if (!"SYSTEM".equals(sessionId) && playbackState.isPauseLocked() && !playbackState.isPaused()) return;
            long currentPos = playbackState.calculateCurrentPosition();
            boolean newState = !playbackState.isPaused();
            playbackState.setPaused(newState);
            playbackState.updatePlaybackAnchor(currentPos);
            playbackState.touchHotActivity();
            playbackState.bumpStateVersion();
            applyPlaybackOnlyTransition(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, newState ? PlayerAction.PAUSE : PlayerAction.PLAY, getUserPublicId(sessionId), null, roomId));
        }

        public synchronized Optional<String> seekTo(long positionMs, String sessionId) {
            PlayableMusic music = playbackState.currentMusic();
            if (music == null) return Optional.of("当前没有正在播放的歌曲");
            if (!Objects.equals(playbackState.currentEnqueuerId(), getUserPublicId(sessionId))) return Optional.of("只有点播者可以调整这首歌的进度");
            long clamped = music.duration() > 0 ? Math.max(0, Math.min(positionMs, music.duration())) : Math.max(0, positionMs);
            playbackState.updatePlaybackAnchor(clamped);
            playbackState.touchHotActivity();
            playbackState.bumpPlayEpochAndStateVersion();
            applyPlaybackOnlyTransition(null);
            return Optional.empty();
        }

        public void toggleShuffle(String sessionId) {
            if (isRateLimited(sessionId) || (playbackState.isShuffleLocked() && !"SYSTEM".equals(sessionId))) return;
            playbackState.setShuffle(!playbackState.isShuffle());
            playbackState.touchHotActivity();
            playbackState.bumpStateVersion();
            applyPlaybackOnlyTransition(new SystemMessageEvent(this, SystemMessageEvent.Level.INFO, playbackState.isShuffle() ? PlayerAction.SHUFFLE_ON : PlayerAction.SHUFFLE_OFF, getUserPublicId(sessionId), null, roomId));
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
            boolean exists = queueManager.getQueueSnapshot().stream().anyMatch(item -> item.music().id().equals(event.getMusicId()));
            if (exists) {
                broadcastQueueUpdate();
                if (playbackState.currentMusic() == null) playNextInQueue();
            }
        }

        public void onUserCountChanged(UserCountChangeEvent event) {
            if (event.getOnlineUserCount() > 0) {
                playbackState.touchHotActivity();
            } else if (!isStreamActive.get()) {
                enterIdleMode();
            }
            broadcastFullPlayerState();
        }

        public void onStreamStatusChanged(StreamStatusEvent event) {
            isStreamActive.set(event.isHasListeners());
            if (event.isHasListeners()) {
                playbackState.touchHotActivity();
                if (playbackState.currentMusic() == null) playNextInQueue();
            }
            else if (!event.isHasListeners() && userService.getOnlineUserSummaries(roomId).isEmpty()) enterIdleMode();
            playbackState.bumpStateVersion();
            broadcastFullPlayerState();
        }

        public void cleanupIdlePlayer() {
            if (playbackState.isPaused() && playbackState.currentMusic() != null) {
                long pausedDuration = System.currentTimeMillis() - currentPlaybackStateSnapshot().timestampAnchor();
                if (pausedDuration > IDLE_RESET_TIMEOUT_MS) {
                    playbackState.clearCurrentTrack();
                    playbackState.setPaused(false);
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
            if (playbackState.currentMusic() != null && !playbackState.isPaused()) {
                playbackState.updatePlaybackAnchor(playbackState.calculateCurrentPosition());
                playbackState.setPaused(true);
                playbackState.bumpStateVersion();
                applyPlaybackOnlyTransition(null);
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
            playbackState.applySnapshot(state);
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
