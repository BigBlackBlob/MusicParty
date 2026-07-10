package org.thornex.musicparty.controller;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.service.LocalCacheService;
import org.thornex.musicparty.service.UserService;
import org.thornex.musicparty.service.api.NeteaseMusicApiService;
import org.thornex.musicparty.service.stream.InternalStreamProxyToken;
import reactor.core.publisher.Mono;

import java.io.IOException;
import java.lang.reflect.Method;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.concurrent.CyclicBarrier;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;
import java.util.concurrent.Future;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicInteger;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

class NeteaseProxyControllerTest {

    private static final Path BASE = Path.of("src/main/java/org/thornex/musicparty");

    @Test
    void neteaseProxyControllerHasStreamEndpointWithRangeSupport() throws IOException {
        String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
        assertThat(src).contains("@GetMapping(\"/stream/{songId}\")");
        assertThat(src).contains("@RequestMapping(\"/api/netease\")");
        assertThat(src).contains("\"Range\"");
        assertThat(src).contains("internalStreamProxyToken");
        assertThat(src).contains("getUserBySessionToken");
    }

    @Test
    void neteaseProxyAppliesErrorSuppressionToFluxBody() throws IOException {
        String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
        assertThat(src).contains("withErrorSuppression");
    }

    @Test
    void neteaseProxyHasCdnUrlShortTermCache() throws IOException {
        String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
        assertThat(src).contains("cdnUrlCache");
        assertThat(src).contains("CDN_TTL_MS");
        assertThat(src).contains("resolveCdnWithCache");
    }

    @Test
    void neteaseProxyHasInflightCdnResolveDedup() throws IOException {
        String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
        assertThat(src).contains("inflightCdnResolves");
        assertThat(src).contains("resolveCdnDedup");
        assertThat(src).contains(".cache()");
    }

    @Test
    void concurrentCdnCacheMissesShareOneUpstreamResolve() throws Exception {
        AtomicInteger resolves = new AtomicInteger();
        NeteaseMusicApiService neteaseService = mock(NeteaseMusicApiService.class);
        when(neteaseService.resolveCdnUrl("42")).thenAnswer(invocation -> {
            resolves.incrementAndGet();
            Thread.sleep(100);
            return Mono.just("https://cdn.example/song.mp3");
        });
        NeteaseProxyController controller = new NeteaseProxyController(
                neteaseService,
                mock(UserService.class),
                mock(InternalStreamProxyToken.class),
                mock(LocalCacheService.class));
        Method resolveCdnDedup = NeteaseProxyController.class.getDeclaredMethod("resolveCdnDedup", String.class);
        resolveCdnDedup.setAccessible(true);
        CyclicBarrier start = new CyclicBarrier(2);
        ExecutorService executor = Executors.newFixedThreadPool(2);
        try {
            Future<Mono<String>> first = executor.submit(() -> invokeResolve(resolveCdnDedup, controller, start));
            Future<Mono<String>> second = executor.submit(() -> invokeResolve(resolveCdnDedup, controller, start));

            assertThat(first.get(1, TimeUnit.SECONDS).block()).isEqualTo("https://cdn.example/song.mp3");
            assertThat(second.get(1, TimeUnit.SECONDS).block()).isEqualTo("https://cdn.example/song.mp3");
            assertThat(resolves).hasValue(1);
        } finally {
            executor.shutdownNow();
        }
    }

    @SuppressWarnings("unchecked")
    private Mono<String> invokeResolve(Method resolveCdnDedup, NeteaseProxyController controller,
                                       CyclicBarrier start) throws Exception {
        start.await(1, TimeUnit.SECONDS);
        return (Mono<String>) resolveCdnDedup.invoke(controller, "42");
    }

    @Test
    void reactiveStreamUtilsHasErrorSuppressionHelper() throws IOException {
        String src = Files.readString(BASE.resolve("controller/ReactiveStreamUtils.java"));
        assertThat(src).contains("withErrorSuppression");
        assertThat(src).contains("onErrorResume");
    }

    @Test
    void bilibiliProxyAlsoAppliesErrorSuppression() throws IOException {
        String src = Files.readString(BASE.resolve("controller/BilibiliProxyController.java"));
        assertThat(src).contains("withErrorSuppression");
    }

    @Test
    void neteaseProxyHasLocalCacheFallback() throws IOException {
        String src = Files.readString(BASE.resolve("controller/NeteaseProxyController.java"));
        assertThat(src).contains("tryLocalFile");
        assertThat(src).contains("localCacheService");
        assertThat(src).contains("streamLocalFile");
    }

    @Test
    void neteaseServiceImplementsCachedMusicApiService() throws IOException {
        String src = Files.readString(BASE.resolve("service/api/NeteaseMusicApiService.java"));
        assertThat(src).contains("implements CachedMusicApiService");
        assertThat(src).contains("prefetchMusic");
        assertThat(src).contains("localCacheService");
    }
}
