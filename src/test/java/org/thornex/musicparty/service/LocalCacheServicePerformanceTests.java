package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.springframework.context.ApplicationEventPublisher;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.enums.CacheStatus;
import reactor.core.publisher.Mono;

import java.lang.reflect.Modifier;
import java.lang.reflect.Method;
import java.time.Duration;
import java.util.concurrent.TimeoutException;

import static org.assertj.core.api.Assertions.assertThat;

class LocalCacheServicePerformanceTests {

    @Test
    void cacheEntryMutableFieldsAreVolatileForCrossThreadVisibility() throws Exception {
        assertThat(Modifier.isVolatile(LocalCacheService.CacheEntry.class.getDeclaredField("fileName").getModifiers())).isTrue();
        assertThat(Modifier.isVolatile(LocalCacheService.CacheEntry.class.getDeclaredField("status").getModifiers())).isTrue();
        assertThat(Modifier.isVolatile(LocalCacheService.CacheEntry.class.getDeclaredField("size").getModifiers())).isTrue();
        assertThat(Modifier.isVolatile(LocalCacheService.CacheEntry.class.getDeclaredField("lastAccessTime").getModifiers())).isTrue();
        assertThat(Modifier.isVolatile(LocalCacheService.CacheEntry.class.getDeclaredField("originalUrl").getModifiers())).isTrue();
    }

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

    @Test
    void cleanupStaleTasksDeletesOrphanPartFilesForExpiredEntries() throws Exception {
        AppProperties properties = new AppProperties();
        properties.getPerformance().setDownloadTaskTtlMs(1000);
        LocalCacheService service = newService(properties);

        // 创建一个带 fileName 的 PENDING 条目，并在磁盘上造一个 .part 文件
        LocalCacheService.CacheEntry entry = new LocalCacheService.CacheEntry();
        entry.setId("stale-with-part");
        entry.setFileName("stale-with-part.m4a");
        entry.setStatus(CacheStatus.PENDING);
        entry.setLastAccessTime(1000);
        java.lang.reflect.Field cacheIndexField = LocalCacheService.class.getDeclaredField("cacheIndex");
        cacheIndexField.setAccessible(true);
        @SuppressWarnings("unchecked")
        java.util.Map<String, LocalCacheService.CacheEntry> cacheIndex =
                (java.util.Map<String, LocalCacheService.CacheEntry>) cacheIndexField.get(service);
        cacheIndex.put("stale-with-part", entry);

        // 造 .part 文件（在临时目录里，避免污染项目目录）
        java.nio.file.Path cacheDir = java.nio.file.Paths.get(org.thornex.musicparty.config.LocalResourceConfig.CACHE_DIR);
        java.nio.file.Files.createDirectories(cacheDir);
        java.nio.file.Path partFile = cacheDir.resolve("stale-with-part.m4a.part");
        java.nio.file.Files.writeString(partFile, "partial");

        assertThat(java.nio.file.Files.exists(partFile)).isTrue();

        service.cleanupStaleTasks(3000);

        assertThat(service.getStatus("stale-with-part")).isNull();
        assertThat(java.nio.file.Files.exists(partFile)).as(".part 文件应被清理").isFalse();

        // 清理测试目录
        java.nio.file.Files.deleteIfExists(partFile);
    }

    @Test
    void downloadRetryClassificationHonorsRetryAfterAndDoesNotRetryAuthFailures() throws Exception {
        AppProperties properties = new AppProperties();
        properties.getPerformance().setDownloadMaxRetries(2);
        properties.getPerformance().setDownloadRetryInitialDelayMs(10);
        properties.getPerformance().setDownloadRetryMaxDelayMs(100);
        LocalCacheService service = newService(properties);
        Method retryDelay = LocalCacheService.class.getDeclaredMethod("retryDelay", Throwable.class, int.class);
        retryDelay.setAccessible(true);

        HttpHeaders headers = new HttpHeaders();
        headers.set("Retry-After", "1");
        WebClientResponseException rateLimited = WebClientResponseException.create(429, "Too Many Requests", headers,
                new byte[0], null);
        WebClientResponseException serverError = WebClientResponseException.create(500, "Server Error", HttpHeaders.EMPTY,
                new byte[0], null);
        WebClientResponseException unauthorized = WebClientResponseException.create(401, "Unauthorized", HttpHeaders.EMPTY,
                new byte[0], null);

        assertThat((Duration) retryDelay.invoke(service, rateLimited, 0)).isEqualTo(Duration.ofMillis(100));
        assertThat((Duration) retryDelay.invoke(service, serverError, 0)).isNotNull();
        assertThat((Duration) retryDelay.invoke(service, new TimeoutException("fixture"), 0)).isNotNull();
        assertThat(retryDelay.invoke(service, unauthorized, 0)).isNull();
    }

    private LocalCacheService newService(AppProperties properties) {
        ApplicationEventPublisher publisher = ignored -> {
        };
        return new LocalCacheService(WebClient.builder().build(), publisher, properties);
    }
}
