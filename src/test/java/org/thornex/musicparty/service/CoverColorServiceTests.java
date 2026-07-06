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
import java.util.concurrent.atomic.AtomicInteger;

import static org.assertj.core.api.Assertions.assertThat;

class CoverColorServiceTests {

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
}
