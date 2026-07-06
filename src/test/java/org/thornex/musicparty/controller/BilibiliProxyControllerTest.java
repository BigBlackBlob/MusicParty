package org.thornex.musicparty.controller;

import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;

import static org.assertj.core.api.Assertions.assertThat;

class BilibiliProxyControllerTest {

    private static final Path BASE = Path.of("src/main/java/org/thornex/musicparty");

    @Test
    void bilibiliServiceReturnsProxyUrlInsteadOfPendingDownload() throws IOException {
        String src = Files.readString(BASE.resolve("service/api/BilibiliMusicApiService.java"));
        // P1 核心改动：未缓存时返回 /api/bilibili/stream/{bvid} 而非 PENDING_DOWNLOAD
        assertThat(src).contains("/api/bilibili/stream/");
        // 不应再有 PENDING_DOWNLOAD 作为 getPlayableMusic 的返回值
        // (prefetchMusic 仍可能在其他上下文提及，但 getPlayableMusic 不应用它)
        assertThat(src).doesNotContain("\"PENDING_DOWNLOAD\", music.coverUrl()");
        // 暴露了 resolveStreamUrl 供 controller 调用
        assertThat(src).contains("public Mono<String> resolveStreamUrl");
    }

    @Test
    void bilibiliProxyControllerHasStreamEndpointWithRangeSupport() throws IOException {
        String src = Files.readString(BASE.resolve("controller/BilibiliProxyController.java"));
        // 端点路径
        assertThat(src).contains("@GetMapping(\"/stream/{bvid}\")");
        assertThat(src).contains("@RequestMapping(\"/api/bilibili\")");
        // Range 支持
        assertThat(src).contains("\"Range\"");
        // 防盗链头 (HttpHeaders.REFERER 常量)
        assertThat(src).contains("HttpHeaders.REFERER");
        assertThat(src).contains("www.bilibili.com/video/");
        // 鉴权
        assertThat(src).contains("internalStreamProxyToken");
        assertThat(src).contains("getUserBySessionToken");
        // ACAO + exposed headers for crossorigin
        assertThat(src).contains("ACCESS_CONTROL_ALLOW_ORIGIN");
        assertThat(src).contains("Content-Range");
    }

    @Test
    void bilibiliProxyControllerHasFailsafeMechanisms() throws IOException {
        String src = Files.readString(BASE.resolve("controller/BilibiliProxyController.java"));
        // Failsafe 1: 代理失败时 fallback 到本地缓存
        assertThat(src).contains("tryLocalFile");
        assertThat(src).contains("localCacheService");
        assertThat(src).contains("fallback to local cache");
        // Failsafe 2: CDN 403/404 时重新解析
        assertThat(src).contains("statusCode == 403 || statusCode == 404");
        assertThat(src).contains("re-resolving");
        // Failsafe 3: CDN URL 短时缓存
        assertThat(src).contains("cdnUrlCache");
        assertThat(src).contains("CDN_TTL_MS");
        // 本地文件流式传输支持 Range
        assertThat(src).contains("streamLocalFile");
        assertThat(src).contains("PARTIAL_CONTENT");
    }

    @Test
    void frontendAudioUrlIncludesBilibiliTokenRequirement() throws IOException {
        Path webBase = Path.of("music-party-web/src/utils/audioUrl.js");
        String src = Files.readString(webBase);
        assertThat(src).contains("platform === 'bilibili'");
        assertThat(src).contains("/api/bilibili/");
    }
}
