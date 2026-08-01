package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;

import java.util.Set;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

class RuntimeMetricsServiceTests {

    @Test
    void snapshotIncludesMemoryThreadsRoomsSocketAndCacheGuards() {
        MusicPlayerService playerService = mock(MusicPlayerService.class);
        LocalCacheService localCacheService = mock(LocalCacheService.class);
        ChatService chatService = mock(ChatService.class);
        SocketRateLimiter socketRateLimiter = mock(SocketRateLimiter.class);
        org.thornex.musicparty.websocket.ReactiveSocketBroker broker = mock(org.thornex.musicparty.websocket.ReactiveSocketBroker.class);

        when(playerService.getActiveRoomIds()).thenReturn(Set.of("default", "room-a"));
        when(playerService.getLoadedRoomIds()).thenReturn(Set.of("default"));
        when(localCacheService.getTrackedCacheEntryCount()).thenReturn(3);
        when(localCacheService.getPendingDownloadTaskCount()).thenReturn(2);
        when(chatService.getLoadedRoomHistoryCount()).thenReturn(4);
        when(chatService.getTrackedMessageRateLimitCount()).thenReturn(5);
        when(socketRateLimiter.getTrackedWindowCount()).thenReturn(6);
        when(broker.getSessionCount()).thenReturn(8);

        RuntimeMetricsService.Snapshot snapshot = new RuntimeMetricsService(
                playerService,
                localCacheService,
                chatService,
                socketRateLimiter,
                broker,
                new AppProperties()
        ).snapshot();

        assertThat(snapshot.heapUsedBytes()).isPositive();
        assertThat(snapshot.threadCount()).isPositive();
        assertThat(snapshot.persistedRoomCount()).isEqualTo(2);
        assertThat(snapshot.loadedRoomSessionCount()).isEqualTo(1);
        assertThat(snapshot.cacheEntryCount()).isEqualTo(3);
        assertThat(snapshot.pendingDownloadTaskCount()).isEqualTo(2);
        assertThat(snapshot.loadedChatRoomCount()).isEqualTo(4);
        assertThat(snapshot.chatRateLimitEntryCount()).isEqualTo(5);
        assertThat(snapshot.socketRateLimitWindowCount()).isEqualTo(6);
        assertThat(snapshot.websocketSessionCount()).isEqualTo(8);
        assertThat(snapshot.downloadMaxQueuedTasks()).isEqualTo(100);
    }
}
