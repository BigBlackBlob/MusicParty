package org.thornex.musicparty.service;

import org.springframework.stereotype.Service;
import org.springframework.beans.factory.annotation.Autowired;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.websocket.ReactiveSocketBroker;

import java.lang.management.ManagementFactory;

@Service
public class RuntimeMetricsService {
    private final MusicPlayerService musicPlayerService;
    private final LocalCacheService localCacheService;
    private final ChatService chatService;
    private final SocketRateLimiter socketRateLimiter;
    private final ReactiveSocketBroker broker;
    private final AppProperties appProperties;
    private final RoomService roomService;

    public RuntimeMetricsService(MusicPlayerService musicPlayerService,
                                 LocalCacheService localCacheService,
                                 ChatService chatService,
                                 SocketRateLimiter socketRateLimiter,
                                  ReactiveSocketBroker broker,
                                  AppProperties appProperties) {
        this(musicPlayerService, localCacheService, chatService, socketRateLimiter, broker, appProperties, null);
    }

    @Autowired
    public RuntimeMetricsService(MusicPlayerService musicPlayerService,
                                 LocalCacheService localCacheService,
                                 ChatService chatService,
                                 SocketRateLimiter socketRateLimiter,
                                 ReactiveSocketBroker broker,
                                 AppProperties appProperties,
                                 RoomService roomService) {
        this.musicPlayerService = musicPlayerService;
        this.localCacheService = localCacheService;
        this.chatService = chatService;
        this.socketRateLimiter = socketRateLimiter;
        this.broker = broker;
        this.appProperties = appProperties;
        this.roomService = roomService;
    }

    public Snapshot snapshot() {
        Runtime runtime = Runtime.getRuntime();
        return new Snapshot(
                runtime.totalMemory() - runtime.freeMemory(),
                runtime.totalMemory(),
                runtime.maxMemory(),
                ManagementFactory.getThreadMXBean().getThreadCount(),
                roomService == null ? musicPlayerService.getActiveRoomIds().size() : roomService.getPersistedRoomCount(),
                musicPlayerService.getLoadedRoomIds().size(),
                musicPlayerService.getActivePlaybackRoomIds().size(),
                broker.getSessionCount(),
                broker.getSubscribedRoomCount(),
                localCacheService.getTrackedCacheEntryCount(),
                localCacheService.getPendingDownloadTaskCount(),
                chatService.getLoadedRoomHistoryCount(),
                chatService.getTrackedMessageRateLimitCount(),
                socketRateLimiter.getTrackedWindowCount(),
                appProperties.getPerformance().getDownloadMaxQueuedTasks()
        );
    }

    public record Snapshot(
            long heapUsedBytes,
            long heapCommittedBytes,
            long heapMaxBytes,
            int threadCount,
            int persistedRoomCount,
            int loadedRoomSessionCount,
            int activePlaybackRoomCount,
            int websocketSessionCount,
            int subscribedRoomCount,
            int cacheEntryCount,
            int pendingDownloadTaskCount,
            int loadedChatRoomCount,
            int chatRateLimitEntryCount,
            int socketRateLimitWindowCount,
            int downloadMaxQueuedTasks
    ) {
    }
}
