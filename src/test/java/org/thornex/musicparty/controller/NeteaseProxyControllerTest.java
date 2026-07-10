package org.thornex.musicparty.controller;

import org.junit.jupiter.api.Test;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import static org.assertj.core.api.Assertions.assertThat;

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
}
