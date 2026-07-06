package org.thornex.musicparty.service;

import org.springframework.stereotype.Service;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.service.stream.LiveStreamService;
import org.thornex.musicparty.websocket.ReactiveSocketBroker;

import java.lang.management.ManagementFactory;

@Service
public class RuntimeMetricsService {
    private final MusicPlayerService musicPlayerService;
    private final LocalCacheService localCacheService;
    private final ChatService chatService;
    private final SocketRateLimiter socketRateLimiter;
    private final LiveStreamService liveStreamService;
    private final ReactiveSocketBroker broker;
    private final AppProperties appProperties;

    public RuntimeMetricsService(MusicPlayerService musicPlayerService,
                                 LocalCacheService localCacheService,
                                 ChatService chatService,
                                 SocketRateLimiter socketRateLimiter,
                                 LiveStreamService liveStreamService,
                                 ReactiveSocketBroker broker,
                                 AppProperties appProperties) {
        this.musicPlayerService = musicPlayerService;
        this.localCacheService = localCacheService;
        this.chatService = chatService;
        this.socketRateLimiter = socketRateLimiter;
        this.liveStreamService = liveStreamService;
        this.broker = broker;
        this.appProperties = appProperties;
    }

    public Snapshot snapshot() {
        Runtime runtime = Runtime.getRuntime();
        return new Snapshot(
                runtime.totalMemory() - runtime.freeMemory(),
                runtime.totalMemory(),
                runtime.maxMemory(),
                ManagementFactory.getThreadMXBean().getThreadCount(),
                musicPlayerService.getActiveRoomIds().size(),
                musicPlayerService.getLoadedRoomIds().size(),
                broker.getSessionCount(),
                broker.getSubscribedRoomCount(),
                liveStreamService.getStreamListenerCount(),
                localCacheService.getTrackedCacheEntryCount(),
                localCacheService.getPendingDownloadTaskCount(),
                chatService.getLoadedRoomHistoryCount(),
                chatService.getTrackedMessageRateLimitCount(),
                socketRateLimiter.getTrackedWindowCount(),
                appProperties.getPerformance().getDownloadMaxQueuedTasks(),
                appProperties.getPerformance().getStreamMaxListeners()
        );
    }

    public record Snapshot(
            long heapUsedBytes,
            long heapCommittedBytes,
            long heapMaxBytes,
            int threadCount,
            int activeRoomCount,
            int loadedRoomSessionCount,
            int websocketSessionCount,
            int subscribedRoomCount,
            int streamListenerCount,
            int cacheEntryCount,
            int pendingDownloadTaskCount,
            int loadedChatRoomCount,
            int chatRateLimitEntryCount,
            int socketRateLimitWindowCount,
            int downloadMaxQueuedTasks,
            int streamMaxListeners
    ) {
    }
}
