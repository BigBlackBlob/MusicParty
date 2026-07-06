package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.EnqueueRequest;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicQueueItem;
import org.thornex.musicparty.dto.PlayableMusic;
import org.thornex.musicparty.dto.Playlist;
import org.thornex.musicparty.dto.UserSearchResult;
import org.thornex.musicparty.dto.UserSummary;
import org.thornex.musicparty.enums.CacheStatus;
import org.thornex.musicparty.enums.QueueItemStatus;
import org.thornex.musicparty.event.DownloadStatusEvent;
import org.thornex.musicparty.event.QueueUpdateEvent;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.PersistedHistoryEntry;
import org.thornex.musicparty.persistence.RoomRepository;
import org.thornex.musicparty.service.api.CachedMusicApiService;
import org.thornex.musicparty.service.api.IMusicApiService;
import reactor.core.publisher.Mono;

import java.lang.reflect.Method;
import java.time.Duration;
import java.util.ArrayList;
import java.util.List;
import java.util.concurrent.atomic.AtomicInteger;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

class MusicPlayerServiceHistoryTests {

    @Test
    void playbackCompletionAppendsHistoryAndFlushRehydratesWithoutCollapsingEntries() throws Exception {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());
        Music song = new Music("music-1", "Song A", List.of("Artist A"), 180_000L, "netease", "cover-a");

        invokeMethod(session, "appendFinishedTrackToHistory", new Class[]{Music.class}, song);
        invokeMethod(session, "appendFinishedTrackToHistory", new Class[]{Music.class}, song);
        session.flushPersistentState();

        TestContext restarted = context.restart(queueRepository);

        assertThat(queueRepository.appendHistoryCalls).isEqualTo(2);
        assertThat(queueRepository.replaceHistoryCalls).isEqualTo(1);
        assertThat(session.getQueueManager().getHistorySnapshot()).containsExactly(song, song);
        assertThat(restarted.musicPlayerService().getSession(restarted.roomId()).getQueueManager().getHistorySnapshot())
                .containsExactly(song, song);
    }

    @Test
    void resetSystemClearsPersistedHistoryWithoutLeavingResidualEntriesAfterRehydrate() {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());
        Music first = new Music("music-1", "Song A", List.of("Artist A"), 180_000L, "netease", "cover-a");
        Music second = new Music("music-2", "Song B", List.of("Artist B"), 200_000L, "netease", "cover-b");

        session.getQueueManager().restore(List.of(), List.of(first, second));
        session.flushPersistentState();
        session.resetSystem(false, true);

        TestContext restarted = context.restart(queueRepository);

        assertThat(queueRepository.appendHistoryCalls).isZero();
        assertThat(queueRepository.replaceHistoryCalls).isEqualTo(2);
        assertThat(queueRepository.replacedHistorySnapshots.getLast()).isEmpty();
        assertThat(restarted.musicPlayerService().getSession(restarted.roomId()).getQueueManager().getHistorySnapshot()).isEmpty();
    }

    @Test
    void flushUsesReplaceOnlyForCurrentHistorySnapshot() {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());
        Music first = new Music("music-1", "Song A", List.of("Artist A"), 180_000L, "netease", "cover-a");
        Music second = new Music("music-2", "Song B", List.of("Artist B"), 200_000L, "netease", "cover-b");

        session.getQueueManager().restore(List.of(), List.of(first, second));
        session.flushPersistentState();

        assertThat(queueRepository.appendHistoryCalls).isZero();
        assertThat(queueRepository.replaceHistoryCalls).isEqualTo(1);
        assertThat(queueRepository.replacedHistorySnapshots).containsExactly(List.of(first, second));
    }

    @Test
    void playbackControlCooldownIsTrackedPerSession() throws Exception {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());

        boolean firstUserFirstControl = (boolean) invokeMethod(session, "isRateLimited", new Class[]{String.class}, "session-a");
        boolean firstUserSecondControl = (boolean) invokeMethod(session, "isRateLimited", new Class[]{String.class}, "session-a");
        boolean secondUserFirstControl = (boolean) invokeMethod(session, "isRateLimited", new Class[]{String.class}, "session-b");

        assertThat(firstUserFirstControl).isFalse();
        assertThat(firstUserSecondControl).isTrue();
        assertThat(secondUserFirstControl).isFalse();
    }

    @Test
    void playerLoopDoesNotQueryRepositoryBackedRoomListEveryTick() {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        CountingRoomRepository roomRepository = (CountingRoomRepository) context.roomRepository();

        int before = roomRepository.findAllActiveCalls.get();
        context.musicPlayerService().playerLoop();

        assertThat(roomRepository.findAllActiveCalls.get()).isEqualTo(before);
    }

    @Test
    void concurrentAsyncEnqueueRechecksUserQueueCapAtAddTime() throws Exception {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        LocalCacheService localCacheService = mock(LocalCacheService.class);
        when(localCacheService.getStatus("cached-song-1")).thenReturn(CacheStatus.PENDING);
        when(localCacheService.getStatus("cached-song-2")).thenReturn(CacheStatus.PENDING);
        TestContext context = createContext(
                queueRepository,
                List.of(new DelayedCachedMusicApiService()),
                localCacheService,
                event -> {}
        );
        context.properties().getQueue().setMaxUserSongs(1);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());
        context.userService().handleConnect("session-1", "token-1", "Alice", context.roomId());

        session.enqueue(new EnqueueRequest("cached", "song-1"), "session-1");
        session.enqueue(new EnqueueRequest("cached", "song-2"), "session-1");
        pollUntil(() -> session.getQueueManager().getQueueSnapshot().size() >= 1, 3000);
        Thread.sleep(250);

        assertThat(session.getQueueManager().getQueueSnapshot()).hasSize(1);
    }

    @Test
    void downloadEventMatchesCachedPlatformCacheKey() {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        List<Object> events = new ArrayList<>();
        LocalCacheService localCacheService = mock(LocalCacheService.class);
        when(localCacheService.getStatus("youtube-abc123")).thenReturn(CacheStatus.COMPLETED);
        TestContext context = createContext(
                queueRepository,
                List.of(new TestCachedMusicApiService()),
                localCacheService,
                event -> events.add(event)
        );
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());
        Music song = new Music("abc123", "Video", List.of("Channel"), 180_000L, "youtube", "cover");
        session.getQueueManager().add(song, new UserSummary("user-1", "User", false), QueueItemStatus.PENDING);

        session.handleDownloadEvent(new DownloadStatusEvent(this, "youtube-abc123"));

        QueueUpdateEvent queueUpdate = events.stream()
                .filter(QueueUpdateEvent.class::isInstance)
                .map(QueueUpdateEvent.class::cast)
                .findFirst()
                .orElseThrow();
        assertThat(queueUpdate.getQueue())
                .extracting(MusicQueueItem::status)
                .containsExactly(QueueItemStatus.READY);
    }

    @Test
    void seekAllowsRequesterAndAdminButRejectsOtherUsers() throws Exception {
        RecordingQueueRepository queueRepository = new RecordingQueueRepository();
        TestContext context = createContext(queueRepository);
        var requester = context.userService().handleConnect("requester-session", "requester-token", "Requester", context.roomId());
        var listener = context.userService().handleConnect("listener-session", "listener-token", "Listener", context.roomId());
        var admin = context.userService().handleConnect("admin-session", "admin-token", "Admin", context.roomId());
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());
        Object playbackState = getFieldValue(session, "playbackState");
        invokeMethod(playbackState, "setCurrentTrack", new Class[]{PlayableMusic.class, String.class, String.class},
                new PlayableMusic("now-1", "Now Playing", List.of("Artist Now"), 210_000L, "netease", "url", "cover-now", false),
                requester.getPublicId(),
                requester.getName());

        assertThat(context.musicPlayerService().seekTo(10_000L, listener.getSessionId(), false))
                .contains("只有点播者可以调整这首歌的进度");
        assertThat(context.musicPlayerService().seekTo(20_000L, requester.getSessionId(), false)).isEmpty();
        assertThat(context.musicPlayerService().seekTo(30_000L, admin.getSessionId(), true)).isEmpty();
    }

    private TestContext createContext(RecordingQueueRepository queueRepository) {
        return createContext(queueRepository, List.of(), null, event -> {});
    }

    private TestContext createContext(RecordingQueueRepository queueRepository,
                                      List<IMusicApiService> apiServices,
                                      LocalCacheService localCacheService,
                                      org.springframework.context.ApplicationEventPublisher eventPublisher) {
        AppProperties properties = new AppProperties();
        properties.getQueue().setHistorySize(50);
        RoomRepository roomRepository = new CountingRoomRepository();
        RoomService roomService = new RoomService(
                new ObjectMapper(),
                eventPublisher,
                properties,
                roomRepository,
                new InMemoryMigrationStateRepository()
        );
        roomService.init();
        String roomId = roomService.createRoom("Focus", "owner-1", false, null).roomId();
        RoomSessionCoordinator roomSessionCoordinator = new RoomSessionCoordinator(roomService, eventPublisher);
        UserService userService = new UserService(eventPublisher, roomService, roomSessionCoordinator, new InMemoryUserProfileRepository());
        RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                queueRepository,
                new InMemoryChatRepository(),
                new InMemoryPlaybackStateRepository()
        );
        RoomStateMutationService mutationService = new RoomStateMutationService(new TestTransactionManager());
        PlaybackTransitionService playbackTransitionService = new PlaybackTransitionService(
                persistenceService,
                mutationService,
                eventPublisher
        );
        MusicPlayerService musicPlayerService = new MusicPlayerService(
                apiServices,
                userService,
                localCacheService,
                null,
                eventPublisher,
                properties,
                null,
                roomService,
                roomSessionCoordinator,
                persistenceService,
                mutationService,
                playbackTransitionService,
                null,
                null
        );
        return new TestContext(properties, roomRepository, roomId, userService, musicPlayerService);
    }

    private Object invokeMethod(Object target, String methodName, Class<?>[] parameterTypes, Object... args) throws Exception {
        Method method = target.getClass().getDeclaredMethod(methodName, parameterTypes);
        method.setAccessible(true);
        return method.invoke(target, args);
    }

    private Object getFieldValue(Object target, String fieldName) throws Exception {
        var field = target.getClass().getDeclaredField(fieldName);
        field.setAccessible(true);
        return field.get(target);
    }

    private static void pollUntil(java.util.function.BooleanSupplier condition, long timeoutMs) throws InterruptedException {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (System.currentTimeMillis() < deadline) {
            if (condition.getAsBoolean()) return;
            Thread.sleep(50);
        }
        throw new AssertionError("Condition not met within " + timeoutMs + "ms");
    }

    private record TestContext(
            AppProperties properties,
            RoomRepository roomRepository,
            String roomId,
            UserService userService,
            MusicPlayerService musicPlayerService
    ) {
        private TestContext restart(RecordingQueueRepository queueRepository) {
            RoomService roomService = new RoomService(
                    new ObjectMapper(),
                    event -> {},
                    properties,
                    roomRepository,
                    new InMemoryMigrationStateRepository()
            );
            roomService.init();
            RoomSessionCoordinator roomSessionCoordinator = new RoomSessionCoordinator(roomService, event -> {});
            UserService userService = new UserService(event -> {}, roomService, roomSessionCoordinator, new InMemoryUserProfileRepository());
            RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                    queueRepository,
                    new InMemoryChatRepository(),
                    new InMemoryPlaybackStateRepository()
            );
            RoomStateMutationService mutationService = new RoomStateMutationService(new TestTransactionManager());
            PlaybackTransitionService playbackTransitionService = new PlaybackTransitionService(
                    persistenceService,
                    mutationService,
                    event -> {}
            );
            return new TestContext(
                    properties,
                    roomRepository,
                    roomId,
                    userService,
                    new MusicPlayerService(
                            List.of(),
                            userService,
                            null,
                            null,
                            event -> {},
                            properties,
                            null,
                            roomService,
                            roomSessionCoordinator,
                            persistenceService,
                            mutationService,
                            playbackTransitionService,
                            null,
                            null
                    )
            );
        }
    }

    private static final class RecordingQueueRepository extends org.thornex.musicparty.persistence.InMemoryQueueRepository {
        private int appendHistoryCalls;
        private int replaceHistoryCalls;
        private final List<List<Music>> replacedHistorySnapshots = new ArrayList<>();

        @Override
        public void appendHistory(PersistedHistoryEntry historyEntry) {
            appendHistoryCalls++;
            super.appendHistory(historyEntry);
        }

        @Override
        public void replaceHistory(String roomId, List<Music> historyItems) {
            replaceHistoryCalls++;
            replacedHistorySnapshots.add(new ArrayList<>(historyItems));
            super.replaceHistory(roomId, historyItems);
        }
    }

    private static final class CountingRoomRepository extends InMemoryRoomRepository {
        final AtomicInteger findAllActiveCalls = new AtomicInteger();

        @Override
        public List<org.thornex.musicparty.persistence.PersistedRoom> findAllActive() {
            findAllActiveCalls.incrementAndGet();
            return super.findAllActive();
        }
    }

    private static final class TestCachedMusicApiService implements CachedMusicApiService {
        @Override
        public String getPlatformName() {
            return "youtube";
        }

        @Override
        public boolean isAvailable() {
            return true;
        }

        @Override
        public String cacheKey(String musicId) {
            return "youtube-" + musicId;
        }

        @Override
        public Mono<List<Music>> searchMusic(String keyword) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<List<Music>> searchMusic(String keyword, int offset, int limit) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<PlayableMusic> getPlayableMusic(String musicId) {
            return Mono.just(new PlayableMusic(musicId, "Video", List.of("Channel"), 180_000L, "youtube", "/media/youtube-" + musicId + ".m4a", "cover", false));
        }

        @Override
        public Mono<List<Playlist>> getUserPlaylists(String userId) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<List<UserSearchResult>> searchUsers(String keyword) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<String> getLyric(String musicId) {
            return Mono.just("");
        }
    }

    private static final class DelayedCachedMusicApiService implements CachedMusicApiService {
        @Override
        public String getPlatformName() {
            return "cached";
        }

        @Override
        public boolean isAvailable() {
            return true;
        }

        @Override
        public String cacheKey(String musicId) {
            return "cached-" + musicId;
        }

        @Override
        public void prefetchMusic(String musicId) {
        }

        @Override
        public Mono<List<Music>> searchMusic(String keyword) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<List<Music>> searchMusic(String keyword, int offset, int limit) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<PlayableMusic> getPlayableMusic(String musicId) {
            return Mono.delay(Duration.ofMillis(100))
                    .map(ignored -> new PlayableMusic(musicId, "Song " + musicId, List.of("Artist"), 180_000L, "cached", "/media/" + musicId, "cover", false));
        }

        @Override
        public Mono<List<Playlist>> getUserPlaylists(String userId) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<List<UserSearchResult>> searchUsers(String keyword) {
            return Mono.just(List.of());
        }

        @Override
        public Mono<String> getLyric(String musicId) {
            return Mono.just("");
        }
    }
}
