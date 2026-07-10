package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.springframework.core.io.buffer.DefaultDataBufferFactory;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.web.reactive.function.client.ClientResponse;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.CoverColorResponse;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;

import java.lang.reflect.Method;
import java.util.Base64;
import java.util.concurrent.Semaphore;
import java.util.concurrent.atomic.AtomicInteger;

import static org.assertj.core.api.Assertions.assertThat;

class CoverColorServiceTests {

    @Test
    void extractDoesNotAcquireConcurrencyPermitBeforeSubscription() throws Exception {
        AppProperties properties = new AppProperties();
        properties.getPerformance().setCoverColorMaxConcurrent(1);
        CoverColorService service = new CoverColorService(WebClient.builder().build(), properties);
        java.lang.reflect.Field semaphoreField = CoverColorService.class.getDeclaredField("concurrentExtracts");
        semaphoreField.setAccessible(true);
        Semaphore semaphore = (Semaphore) semaphoreField.get(service);

        service.extract("/media/cover.png");

        assertThat(semaphore.availablePermits()).isEqualTo(1);
    }

    @Test
    void rejectsLocalAndNonHttpCoverUrls() throws Exception {
        CoverColorService service = new CoverColorService(null, new AppProperties());
        Method method = CoverColorService.class.getDeclaredMethod("isSafeCoverUrl", String.class);
        method.setAccessible(true);

        assertThat((boolean) method.invoke(service, "file:///etc/passwd")).isFalse();
        assertThat((boolean) method.invoke(service, "http://localhost:8080/private.png")).isFalse();
        assertThat((boolean) method.invoke(service, "http://127.0.0.1/private.png")).isFalse();
        assertThat((boolean) method.invoke(service, "http://169.254.169.254/latest/meta-data")).isFalse();
    }

    @Test
    void allowsKnownLocalCoverPaths() throws Exception {
        CoverColorService service = new CoverColorService(null, new AppProperties());
        Method method = CoverColorService.class.getDeclaredMethod("isTrustedLocalCoverPath", String.class);
        method.setAccessible(true);

        assertThat((boolean) method.invoke(service, "/api/navidrome/cover/abc?token=user-token")).isTrue();
        assertThat((boolean) method.invoke(service, "/media/song.mp3")).isTrue();
        assertThat((boolean) method.invoke(service, "/actuator/env")).isFalse();
    }

    @Test
    void cachesExtractedCoverColorsByResolvedUrl() {
        AtomicInteger calls = new AtomicInteger();
        byte[] png = Base64.getDecoder().decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4XmNgYPgPAAEDAQD9b6mRAAAAAElFTkSuQmCC");
        WebClient client = WebClient.builder()
                .exchangeFunction(request -> {
                    calls.incrementAndGet();
                    return Mono.just(ClientResponse.create(HttpStatus.OK)
                            .header(HttpHeaders.CONTENT_TYPE, MediaType.IMAGE_PNG_VALUE)
                            .header(HttpHeaders.CONTENT_LENGTH, String.valueOf(png.length))
                            .body(Flux.just(new DefaultDataBufferFactory().wrap(png)))
                            .build());
                })
                .build();
        AppProperties properties = new AppProperties();
        properties.setBaseUrl("http://127.0.0.1:8080");
        CoverColorService service = new CoverColorService(client, properties);

        CoverColorResponse first = service.extract("/media/cover.png").block();
        CoverColorResponse second = service.extract("/media/cover.png").block();

        assertThat(first).isNotNull();
        assertThat(second).isEqualTo(first);
        assertThat(calls).hasValue(1);
    }
    @Test
    void concurrentRequestsForSameUrlShareOneExtraction() {
        AtomicInteger calls = new AtomicInteger();
        byte[] png = Base64.getDecoder().decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4XmNgYPgPAAEDAQD9b6mRAAAAAElFTkSuQmCC");
        WebClient client = WebClient.builder()
                .exchangeFunction(request -> {
                    calls.incrementAndGet();
                    return Mono.just(ClientResponse.create(HttpStatus.OK)
                            .header(HttpHeaders.CONTENT_TYPE, MediaType.IMAGE_PNG_VALUE)
                            .header(HttpHeaders.CONTENT_LENGTH, String.valueOf(png.length))
                            .body(Flux.just(new DefaultDataBufferFactory().wrap(png)))
                            .build());
                })
                .build();
        AppProperties properties = new AppProperties();
        properties.setBaseUrl("http://127.0.0.1:8080");
        CoverColorService service = new CoverColorService(client, properties);

        // 并发请求同一 URL
        Mono<CoverColorResponse> r1 = service.extract("/media/cover.png");
        Mono<CoverColorResponse> r2 = service.extract("/media/cover.png");

        CoverColorResponse first = r1.block();
        CoverColorResponse second = r2.block();

        assertThat(first).isNotNull();
        assertThat(second).isEqualTo(first);
        // 只应有一次上游调用（in-flight 去重）
        assertThat(calls).hasValue(1);
    }

    @Test
    void cacheHitDoesNotConsumeBoundedElasticThread() {
        AtomicInteger calls = new AtomicInteger();
        byte[] png = Base64.getDecoder().decode("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4XmNgYPgPAAEDAQD9b6mRAAAAAElFTkSuQmCC");
        WebClient client = WebClient.builder()
                .exchangeFunction(request -> {
                    calls.incrementAndGet();
                    return Mono.just(ClientResponse.create(HttpStatus.OK)
                            .header(HttpHeaders.CONTENT_TYPE, MediaType.IMAGE_PNG_VALUE)
                            .header(HttpHeaders.CONTENT_LENGTH, String.valueOf(png.length))
                            .body(Flux.just(new DefaultDataBufferFactory().wrap(png)))
                            .build());
                })
                .build();
        AppProperties properties = new AppProperties();
        properties.setBaseUrl("http://127.0.0.1:8080");
        CoverColorService service = new CoverColorService(client, properties);

        // 第一次提取填充缓存
        service.extract("/media/cover.png").block();
        int callsAfterFirst = calls.get();
        assertThat(callsAfterFirst).isEqualTo(1);

        // 第二次应命中缓存，不触发上游调用
        service.extract("/media/cover.png").block();
        assertThat(calls).hasValue(1);
    }
}
