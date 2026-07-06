package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.enums.CacheStatus;
import reactor.core.publisher.Mono;

import static org.assertj.core.api.Assertions.assertThat;

class LocalCacheServicePerformanceTests {

    @Test
    void submitDynamicDownloadRejectsTasksWhenQueueGuardIsFull() {
        AppProperties properties = new AppProperties();
        properties.getPerformance().setDownloadMaxQueuedTasks(1);
        LocalCacheService service = newService(properties);

        service.submitDynamicDownload("first", Mono.never());
        service.submitDynamicDownload("second", Mono.never());

        assertThat(service.getTrackedCacheEntryCount()).isEqualTo(1);
        assertThat(service.getPendingDownloadTaskCount()).isEqualTo(1);
        assertThat(service.getStatus("second")).isNull();
    }

    @Test
    void cleanupStaleTasksRemovesExpiredFailedAndPendingEntries() {
        AppProperties properties = new AppProperties();
        properties.getPerformance().setDownloadTaskTtlMs(1000);
        LocalCacheService service = newService(properties);

        service.trackForTest("pending-old", CacheStatus.PENDING, 1000);
        service.trackForTest("failed-old", CacheStatus.FAILED, 1000);
        service.trackForTest("completed-old", CacheStatus.COMPLETED, 1000);

        service.cleanupStaleTasks(3000);

        assertThat(service.getStatus("pending-old")).isNull();
        assertThat(service.getStatus("failed-old")).isNull();
        assertThat(service.getStatus("completed-old")).isEqualTo(CacheStatus.COMPLETED);
    }

    private LocalCacheService newService(AppProperties properties) {
        ApplicationEventPublisher publisher = ignored -> {
        };
        return new LocalCacheService(WebClient.builder().build(), publisher, properties);
    }
}
