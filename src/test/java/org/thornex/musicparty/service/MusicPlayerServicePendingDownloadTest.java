package org.thornex.musicparty.service;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.EnqueueRequest;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.PlayableMusic;
import org.thornex.musicparty.dto.Playlist;
import org.thornex.musicparty.dto.UserSearchResult;
import org.thornex.musicparty.enums.CacheStatus;
import org.thornex.musicparty.event.DownloadStatusEvent;
import org.thornex.musicparty.persistence.InMemoryChatRepository;
import org.thornex.musicparty.persistence.InMemoryMigrationStateRepository;
import org.thornex.musicparty.persistence.InMemoryPlaybackStateRepository;
import org.thornex.musicparty.persistence.InMemoryQueueRepository;
import org.thornex.musicparty.persistence.InMemoryRoomRepository;
import org.thornex.musicparty.persistence.InMemoryUserProfileRepository;
import org.thornex.musicparty.persistence.RoomRepository;
import org.thornex.musicparty.service.api.CachedMusicApiService;
import org.thornex.musicparty.service.api.IMusicApiService;
import reactor.core.publisher.Mono;

import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

import static org.assertj.core.api.Assertions.assertThat;

class MusicPlayerServicePendingDownloadTest {

    @Test
    void pendingDownloadDoesNotBecomeCurrentMusicAndResolvesWhenDownloadCompletes() throws Exception {
        // 模拟 LocalCacheService：初始 PENDING，手动设 COMPLETED
        Map<String, CacheStatus> cacheStatuses = new ConcurrentHashMap<>();
        FakeCachedApiService apiService = new FakeCachedApiService("youtube", "youtube-", cacheStatuses);

        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        TestContext context = createContext(List.of(apiService), cacheStatuses, queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());

        // 用户连接 + 入队一首 YouTube 歌
        context.userService().handleConnect("s1", "t1", "Alice", context.roomId());
        Music song = new Music("vid-1", "Video A", List.of("Channel"), 180_000L, "youtube", "cover-a");
        session.enqueue(new EnqueueRequest("youtube", "vid-1"), "s1");

        // 等异步 enqueue 完成（getPlayableMusic → 入队，初始 PENDING）
        pollUntil(() -> session.getQueueManager().getQueueSnapshot().size() >= 1, 3000);

        // 入队时 isCachedPlatform("youtube")=true → 初始 QueueItemStatus=PENDING
        // pollNext 只挑 READY/FAILED，所以 PENDING 的歌不会被选中播放
        // 但 enqueue 回调里 currentMusic==null 会自动 playNextInQueue → pollNext 跳过 PENDING → 返回 null → loading=false
        // 这是现有设计：缓存平台的歌要先下完才会被 pollNext 选中
        // 我们先把 cache 状态设成 COMPLETED + markCompleted，让 getQueueWithUpdatedStatus 映射成 READY
        cacheStatuses.put("youtube-vid-1", CacheStatus.COMPLETED);
        apiService.markCompleted("vid-1");

        // 发 DownloadStatusEvent：触发 handleDownloadEvent
        // → broadcastQueueUpdate（队列状态刷新）+ currentMusic==null → playNextInQueue
        // → pollNext 现在看到 READY → getPlayableMusic 返回有效 url → applyNewSong
        session.handleDownloadEvent(new DownloadStatusEvent(this, "youtube-vid-1"));

        // 等待：playNextInQueue 完成 → nowPlaying 有有效 url
        pollUntil(() -> session.getCurrentPlayerState().nowPlaying() != null
                && session.getCurrentPlayerState().nowPlaying().music() != null
                && "/media/youtube-vid-1.m4a".equals(session.getCurrentPlayerState().nowPlaying().music().url())
                && !session.getCurrentPlayerState().isLoading(), 8000);

        assertThat(session.getCurrentPlayerState().nowPlaying().music().url()).isEqualTo("/media/youtube-vid-1.m4a");
    }

    @Test
    void pendingDownloadUrlNeverReachesFrontend() throws Exception {
        // 验证：当 getPlayableMusic 返回 PENDING_DOWNLOAD 时，它不会成为 nowPlaying
        Map<String, CacheStatus> cacheStatuses = new ConcurrentHashMap<>();
        FakeCachedApiService apiService = new FakeCachedApiService("youtube", "youtube-", cacheStatuses);

        InMemoryQueueRepository queueRepository = new InMemoryQueueRepository();
        TestContext context = createContext(List.of(apiService), cacheStatuses, queueRepository);
        MusicPlayerService.RoomPlayerSession session = context.musicPlayerService().getSession(context.roomId());

        context.userService().handleConnect("s1", "t1", "Alice", context.roomId());
        session.enqueue(new EnqueueRequest("youtube", "vid-1"), "s1");
        pollUntil(() -> session.getQueueManager().getQueueSnapshot().size() >= 1, 3000);

        // 手动把这首歌从队列里 poll 出来（绕过 pollNext 的 READY 过滤）来模拟
        // "被选中但 url=PENDING_DOWNLOAD" 的场景——这测的是 playNextInQueue 的拦截逻辑
        // 直接设 cache 状态 DOWNLOADING（mapCacheStatusToEnum → PENDING）但让 getPlayableMusic 返回 PENDING
        cacheStatuses.put("youtube-vid-1", CacheStatus.DOWNLOADING);

        // 直接触发 playNextInQueue（绕过 pollNext 的 PENDING 过滤）
        // 用反射调用，因为 pollNext 会跳过 PENDING
        java.lang.reflect.Method playNext = session.getClass().getDeclaredMethod("playNextInQueue");
        playNext.setAccessible(true);
        // 先手动把队列项状态设成 READY 让 pollNext 能选中
        var queueItem = session.getQueueManager().getQueueSnapshot().get(0);
        java.lang.reflect.Method withStatus = queueItem.getClass().getDeclaredMethod("withStatus", org.thornex.musicparty.enums.QueueItemStatus.class);
        withStatus.setAccessible(true);
        // 替换队列里的 item 状态（需要通过 queueManager，但 pollNext 用 buildStatusMap 从 cache 读）
        // 直接把 cache 设 COMPLETED 让 buildStatusMap 映射成 READY，但 getPlayableMusic 仍返回 PENDING（因为没 markCompleted）
        cacheStatuses.put("youtube-vid-1", CacheStatus.COMPLETED);

        synchronized (session) {
            playNext.invoke(session);
        }

        // playNextInQueue → pollNext 选中（READY）→ getPlayableMusic 返回 PENDING_DOWNLOAD
        // → waitForPendingDownload 启动 → currentMusic/nowPlaying 保持 null，loading=true
        pollUntil(() -> session.getCurrentPlayerState().isLoading(), 3000);
        assertThat(session.getCurrentPlayerState().nowPlaying()).isNull();

        // 现在模拟下载完成：markCompleted + handleDownloadEvent
        apiService.markCompleted("vid-1");
        session.handleDownloadEvent(new DownloadStatusEvent(this, "youtube-vid-1"));

        // 轮询器发现 COMPLETED → 重新 resolve → applyNewSong
        pollUntil(() -> session.getCurrentPlayerState().nowPlaying() != null
                && "/media/youtube-vid-1.m4a".equals(session.getCurrentPlayerState().nowPlaying().music().url())
                && !session.getCurrentPlayerState().isLoading(), 12000);

        assertThat(session.getCurrentPlayerState().nowPlaying().music().url()).isEqualTo("/media/youtube-vid-1.m4a");
    }

    // 轮询直到 condition 为 true 或超时
    private static void pollUntil(java.util.function.BooleanSupplier condition, long timeoutMs) throws InterruptedException {
        long deadline = System.currentTimeMillis() + timeoutMs;
        while (System.currentTimeMillis() < deadline) {
            if (condition.getAsBoolean()) return;
            Thread.sleep(100);
        }
        throw new AssertionError("Condition not met within " + timeoutMs + "ms");
    }

    // --- helpers ---

    private TestContext createContext(List<IMusicApiService> apiServices,
                                       Map<String, CacheStatus> cacheStatuses,
                                       InMemoryQueueRepository queueRepository) {
        AppProperties properties = new AppProperties();
        properties.getQueue().setHistorySize(50);
        RoomRepository roomRepository = new InMemoryRoomRepository();
        org.springframework.context.ApplicationEventPublisher eventPublisher = event -> {};
        RoomService roomService = new RoomService(
                new ObjectMapper(), eventPublisher, properties, roomRepository, new InMemoryMigrationStateRepository()
        );
        roomService.init();
        String roomId = roomService.createRoom("Test", "owner-1", false, null).roomId();
        RoomSessionCoordinator roomSessionCoordinator = new RoomSessionCoordinator(roomService, eventPublisher);
        UserService userService = new UserService(eventPublisher, roomService, roomSessionCoordinator, new InMemoryUserProfileRepository());
        RoomStatePersistenceService persistenceService = new RoomStatePersistenceService(
                queueRepository, new InMemoryChatRepository(), new InMemoryPlaybackStateRepository()
        );
        RoomStateMutationService mutationService = new RoomStateMutationService(new TestTransactionManager());
        PlaybackTransitionService playbackTransitionService = new PlaybackTransitionService(
                persistenceService, mutationService, eventPublisher
        );
        // 用 FakeLocalCacheService 包装 cacheStatuses
        LocalCacheService fakeCache = new FakeLocalCacheService(cacheStatuses);
        MusicPlayerService musicPlayerService = new MusicPlayerService(
                apiServices, userService, fakeCache, null, eventPublisher, properties,
                null, roomService, roomSessionCoordinator, persistenceService, mutationService,
                playbackTransitionService, null, null
        );
        return new TestContext(properties, roomId, userService, musicPlayerService);
    }

    private record TestContext(
            AppProperties properties,
            String roomId,
            UserService userService,
            MusicPlayerService musicPlayerService
    ) {}

    // --- Fake API service: 初始返回 PENDING_DOWNLOAD，markCompleted 后返回有效 url ---
    private static final class FakeCachedApiService implements CachedMusicApiService {
        private final String platform;
        private final String keyPrefix;
        private final Map<String, CacheStatus> cacheStatuses;
        private final Map<String, Boolean> completed = new ConcurrentHashMap<>();

        FakeCachedApiService(String platform, String keyPrefix, Map<String, CacheStatus> cacheStatuses) {
            this.platform = platform;
            this.keyPrefix = keyPrefix;
            this.cacheStatuses = cacheStatuses;
        }

        void markCompleted(String musicId) {
            completed.put(musicId, true);
        }

        @Override
        public String getPlatformName() { return platform; }

        @Override
        public boolean isAvailable() { return true; }

        @Override
        public String cacheKey(String musicId) { return keyPrefix + musicId; }

        @Override
        public void prefetchMusic(String musicId) {
            // 模拟提交下载：设 cache 状态为 PENDING（如果还没设）
            cacheStatuses.computeIfAbsent(cacheKey(musicId), k -> CacheStatus.PENDING);
        }

        @Override
        public Mono<PlayableMusic> getPlayableMusic(String musicId) {
            String key = cacheKey(musicId);
            if (Boolean.TRUE.equals(completed.get(musicId))) {
                return Mono.just(new PlayableMusic(
                        musicId, "Video " + musicId, List.of("Channel"), 180_000L,
                        platform, "/media/" + key + ".m4a", "cover", false
                ));
            }
            // 未完成 → 返回 PENDING_DOWNLOAD
            prefetchMusic(musicId);
            return Mono.just(new PlayableMusic(
                    musicId, "Video " + musicId, List.of("Channel"), 180_000L,
                    platform, "PENDING_DOWNLOAD", "cover", false
            ));
        }

        @Override
        public Mono<List<Music>> searchMusic(String keyword) { return Mono.just(List.of()); }
        @Override
        public Mono<List<Music>> searchMusic(String keyword, int offset, int limit) { return Mono.just(List.of()); }
        @Override
        public Mono<List<Playlist>> getUserPlaylists(String userId) { return Mono.just(List.of()); }
        @Override
        public Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) { return Mono.just(List.of()); }
        @Override
        public Mono<List<UserSearchResult>> searchUsers(String keyword) { return Mono.just(List.of()); }
        @Override
        public Mono<String> getLyric(String musicId) { return Mono.just(""); }
    }

    // --- Fake LocalCacheService：只暴露 getStatus，不真下载 ---
    private static final class FakeLocalCacheService extends LocalCacheService {
        private final Map<String, CacheStatus> cacheStatuses;

        FakeLocalCacheService(Map<String, CacheStatus> cacheStatuses) {
            super(null, null, new AppProperties());
            this.cacheStatuses = cacheStatuses;
        }

        @Override
        public CacheStatus getStatus(String musicId) {
            return cacheStatuses.get(musicId);
        }

        @Override
        public void submitDynamicDownload(String musicId, Mono<DownloadSource> sourceProvider) {
            // 不真下载，只标记 PENDING
            cacheStatuses.computeIfAbsent(musicId, k -> CacheStatus.PENDING);
        }

        @Override
        public void submitDownload(String musicId, Mono<String> urlProvider, Map<String, String> headers, String extension) {
            cacheStatuses.computeIfAbsent(musicId, k -> CacheStatus.PENDING);
        }

        @Override
        public String getLocalUrl(String musicId) {
            CacheStatus s = cacheStatuses.get(musicId);
            if (s == CacheStatus.COMPLETED) return "/media/" + musicId + ".m4a";
            return null;
        }

        @Override
        public CacheEntry getCacheEntry(String musicId) {
            CacheStatus s = cacheStatuses.get(musicId);
            if (s == null) return null;
            CacheEntry e = new CacheEntry();
            e.setId(musicId);
            e.setStatus(s);
            e.setFileName(musicId + ".m4a");
            return e;
        }
    }
}
