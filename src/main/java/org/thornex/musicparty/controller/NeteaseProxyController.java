package org.thornex.musicparty.controller;

import lombok.extern.slf4j.Slf4j;
import org.springframework.core.io.buffer.DataBuffer;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.util.StringUtils;
import org.springframework.web.bind.annotation.*;
import org.thornex.musicparty.config.LocalResourceConfig;
import org.thornex.musicparty.service.LocalCacheService;
import org.thornex.musicparty.service.UserService;
import org.thornex.musicparty.service.api.NeteaseMusicApiService;
import org.thornex.musicparty.service.stream.InternalStreamProxyToken;
import java.io.IOException;
import java.io.InputStream;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.time.Duration;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;
import reactor.core.scheduler.Schedulers;

@RestController
@RequestMapping("/api/netease")
@Slf4j
public class NeteaseProxyController {

    private final NeteaseMusicApiService neteaseService;
    private final UserService userService;
    private final InternalStreamProxyToken internalStreamProxyToken;
    private final LocalCacheService localCacheService;
    private final HttpClient httpClient = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(10))
            .followRedirects(HttpClient.Redirect.NORMAL)
            .build();
    // CDN URL 短时缓存：避免每次 Range 请求都打 网易云 ncm-api 容器（CPU 密集解密）
    // 网易云 CDN URL 有时效，缓存 30 秒平衡时效性与 API 调用频率
    private static final long CDN_TTL_MS = 30_000;
    private record CdnEntry(String url, long expireAt) {}
    private final Map<String, CdnEntry> cdnUrlCache = new ConcurrentHashMap<>();
    // 并发去重：同一 songId 的并发 CDN 解析请求共享同一个 Mono，避免 cache miss 时打爆 ncm-api
    private final Map<String, Mono<String>> inflightCdnResolves = new ConcurrentHashMap<>();

    public NeteaseProxyController(NeteaseMusicApiService neteaseService,
                                  UserService userService,
                                  InternalStreamProxyToken internalStreamProxyToken,
                                  LocalCacheService localCacheService) {
        this.neteaseService = neteaseService;
        this.userService = userService;
        this.internalStreamProxyToken = internalStreamProxyToken;
        this.localCacheService = localCacheService;
    }

    @GetMapping("/stream/{songId}")
    public Mono<ResponseEntity<Flux<DataBuffer>>> streamSong(
            @PathVariable String songId,
            @RequestParam(required = false) String token,
            @RequestHeader(value = InternalStreamProxyToken.HEADER_NAME, required = false) String internalToken,
            @RequestHeader(value = "Range", required = false) String rangeHeader) {

        if (!canUse(token, internalToken)) {
            return Mono.just(ResponseEntity.status(HttpStatus.FORBIDDEN).build());
        }

        // Failsafe 0: 先检查本地缓存——如果后台下载已完成，直接读本地文件（最快、最稳）
        ResponseEntity<Flux<DataBuffer>> localFallback = tryLocalFile(songId, rangeHeader);
        if (localFallback != null) {
            log.debug("Netease stream using local cache for {}", songId);
            return Mono.just(localFallback);
        }

        return resolveCdnDedup(songId)
                .timeout(Duration.ofSeconds(8))
                .flatMap(cdnUrl -> streamResolvedUrl(songId, cdnUrl, rangeHeader))
                .onErrorResume(e -> {
                    log.warn("Netease resolve cdn url failed: songId={}, message={}", songId, e.getMessage());
                    return Mono.just(ResponseEntity.status(HttpStatus.BAD_GATEWAY).<Flux<DataBuffer>>build());
                });
    }

    private Mono<ResponseEntity<Flux<DataBuffer>>> streamResolvedUrl(String songId, String cdnUrl, String rangeHeader) {
        if (!StringUtils.hasText(cdnUrl)) {
            return Mono.just(ResponseEntity.notFound().build());
        }

        return Mono.<ResponseEntity<Flux<DataBuffer>>>fromCallable(() -> {
            HttpRequest.Builder requestBuilder = HttpRequest.newBuilder(URI.create(cdnUrl))
                    .timeout(Duration.ofSeconds(30))
                    .GET()
                    .header(HttpHeaders.ACCEPT, MediaType.APPLICATION_OCTET_STREAM_VALUE)
                    .header(HttpHeaders.USER_AGENT, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
                    .header(HttpHeaders.REFERER, "https://music.163.com/");
            if (rangeHeader != null && !rangeHeader.isBlank()) {
                requestBuilder.header(HttpHeaders.RANGE, rangeHeader);
            }

            HttpResponse<InputStream> upstream = httpClient.send(requestBuilder.build(), HttpResponse.BodyHandlers.ofInputStream());
            int statusCode = upstream.statusCode();
            if (statusCode == HttpStatus.NOT_FOUND.value()) {
                closeQuietly(upstream.body());
                return ResponseEntity.notFound().<Flux<DataBuffer>>build();
            }
            if (statusCode >= 400) {
                closeQuietly(upstream.body());
                log.warn("Netease stream upstream error: songId={}, status={}", songId, statusCode);
                return ResponseEntity.status(HttpStatus.BAD_GATEWAY).<Flux<DataBuffer>>build();
            }

            HttpHeaders headers = new HttpHeaders();
            copyHeader(upstream, headers, HttpHeaders.CONTENT_TYPE);
            copyHeader(upstream, headers, HttpHeaders.CONTENT_LENGTH);
            copyHeader(upstream, headers, HttpHeaders.CONTENT_RANGE);
            copyHeader(upstream, headers, HttpHeaders.ACCEPT_RANGES);
            if (!headers.containsKey(HttpHeaders.CONTENT_TYPE)) {
                headers.setContentType(MediaType.APPLICATION_OCTET_STREAM);
            }
            headers.set(HttpHeaders.ACCESS_CONTROL_ALLOW_ORIGIN, "*");
            headers.set(HttpHeaders.ACCESS_CONTROL_EXPOSE_HEADERS, "Content-Length, Content-Range, Accept-Ranges");

            Flux<DataBuffer> body = ReactiveStreamUtils.withErrorSuppression(
                    ReactiveStreamUtils.readInputStream(upstream.body()),
                    "netease stream songId=" + songId,
                    log);

            HttpStatus status = HttpStatus.resolve(statusCode);
            return new ResponseEntity<>(body, headers, status == null ? HttpStatus.OK : status);
        }).subscribeOn(Schedulers.boundedElastic()).onErrorResume(e -> {
            log.warn("Netease stream proxy failed: songId={}, message={}", songId, e.getMessage());
            return Mono.just(ResponseEntity.status(HttpStatus.BAD_GATEWAY).<Flux<DataBuffer>>build());
        });
    }

    // Failsafe 0: 检查本地缓存是否已完成，是则直接读文件
    private ResponseEntity<Flux<DataBuffer>> tryLocalFile(String songId, String rangeHeader) {
        try {
            String localUrl = localCacheService.getLocalUrl(songId);
            if (localUrl == null) return null;
            // localUrl = /media/xxx.mp3
            String fileName = localUrl.substring("/media/".length());
            Path path = Paths.get(LocalResourceConfig.CACHE_DIR, fileName);
            if (!Files.exists(path)) return null;
            return streamLocalFile(path, "audio/mpeg", rangeHeader);
        } catch (Exception e) {
            log.debug("Local file fallback check failed for {}: {}", songId, e.getMessage());
            return null;
        }
    }

    // 本地文件流式传输（支持 Range）
    private ResponseEntity<Flux<DataBuffer>> streamLocalFile(Path path, String contentType, String rangeHeader) throws IOException {
        long length = Files.size(path);
        long start = 0;
        long end = length - 1;
        HttpStatus status = HttpStatus.OK;
        if (rangeHeader != null && rangeHeader.startsWith("bytes=")) {
            String[] parts = rangeHeader.substring("bytes=".length()).split("-", 2);
            try {
                start = parts[0].isBlank() ? 0 : Long.parseLong(parts[0]);
                if (parts.length > 1 && !parts[1].isBlank()) {
                    end = Math.min(Long.parseLong(parts[1]), length - 1);
                }
                status = HttpStatus.PARTIAL_CONTENT;
            } catch (NumberFormatException ignored) {
                start = 0;
                end = length - 1;
            }
        }
        if (start < 0 || start >= length || end < start) {
            return ResponseEntity.status(HttpStatus.REQUESTED_RANGE_NOT_SATISFIABLE).build();
        }
        long contentLength = end - start + 1;
        HttpHeaders headers = new HttpHeaders();
        headers.set(HttpHeaders.CONTENT_TYPE, contentType);
        headers.set(HttpHeaders.ACCEPT_RANGES, "bytes");
        headers.setContentLength(contentLength);
        headers.set(HttpHeaders.ACCESS_CONTROL_ALLOW_ORIGIN, "*");
        headers.set(HttpHeaders.ACCESS_CONTROL_EXPOSE_HEADERS, "Content-Length, Content-Range, Accept-Ranges");
        if (status == HttpStatus.PARTIAL_CONTENT) {
            headers.set(HttpHeaders.CONTENT_RANGE, "bytes " + start + "-" + end + "/" + length);
        }
        Flux<DataBuffer> body = ReactiveStreamUtils.withErrorSuppression(
                ReactiveStreamUtils.readPath(path, start, contentLength),
                "netease local file=" + path.getFileName(),
                log);
        return new ResponseEntity<>(body, headers, status);
    }

    private boolean canUse(String token, String internalToken) {
        if (internalStreamProxyToken.matches(internalToken)) return true;
        return StringUtils.hasText(token) && userService.getUserBySessionToken(token).isPresent();
    }

    // Failsafe: CDN URL 短时缓存（30 秒），避免每次 Range 请求都打 网易云 ncm-api 容器
    private Mono<String> resolveCdnWithCache(String songId) {
        CdnEntry cached = cdnUrlCache.get(songId);
        if (cached != null && cached.expireAt() > System.currentTimeMillis()) {
            return Mono.just(cached.url());
        }
        // 缓存 miss 或过期：实际解析
        return neteaseService.resolveCdnUrl(songId)
                .doOnNext(url -> cdnUrlCache.put(songId, new CdnEntry(url, System.currentTimeMillis() + CDN_TTL_MS)));
    }


    // 并发去重：同一 songId 的并发请求共享同一个 in-flight Mono，避免 cache miss 时多次打 ncm-api
    // .cache() 将 Mono 变为 hot 源，所有订阅者共享同一次上游调用；.doFinally 在终止后清理 inflight 表
    private Mono<String> resolveCdnDedup(String songId) {
        CdnEntry cached = cdnUrlCache.get(songId);
        if (cached != null && cached.expireAt() > System.currentTimeMillis()) {
            return Mono.just(cached.url());
        }
        return inflightCdnResolves.computeIfAbsent(songId, id ->
                neteaseService.resolveCdnUrl(id)
                        .doOnNext(url -> cdnUrlCache.put(id, new CdnEntry(url, System.currentTimeMillis() + CDN_TTL_MS)))
                        .doFinally(signal -> inflightCdnResolves.remove(id))
                        .cache());
    }

    private void copyHeader(HttpResponse<?> response, HttpHeaders headers, String name) {
        response.headers().firstValue(name).ifPresent(value -> headers.set(name, value));
    }

    private void closeQuietly(InputStream inputStream) {
        try {
            if (inputStream != null) inputStream.close();
        } catch (Exception ignored) {
        }
    }
}
