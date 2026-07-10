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
import org.thornex.musicparty.service.api.BilibiliMusicApiService;
import org.thornex.musicparty.service.stream.InternalStreamProxyToken;
import reactor.core.publisher.Flux;
import reactor.core.publisher.Mono;
import reactor.core.scheduler.Schedulers;

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
import java.util.concurrent.ConcurrentHashMap;
import java.util.Map;

@RestController
@RequestMapping("/api/bilibili")
@Slf4j
public class BilibiliProxyController {

    private final BilibiliMusicApiService bilibiliService;
    private final UserService userService;
    private final InternalStreamProxyToken internalStreamProxyToken;
    private final LocalCacheService localCacheService;
    private final HttpClient httpClient = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(10))
            .followRedirects(HttpClient.Redirect.NORMAL)
            .build();

    // CDN URL 短时缓存：避免每次 Range 请求都打 B站 API（被风控）
    // B站 CDN URL 有时效，缓存 30 秒平衡时效性与 API 调用频率
    private static final long CDN_TTL_MS = 30_000;
    private record CdnEntry(String url, long expireAt) {}
    private final Map<String, CdnEntry> cdnUrlCache = new ConcurrentHashMap<>();

    public BilibiliProxyController(BilibiliMusicApiService bilibiliService,
                                   UserService userService,
                                   InternalStreamProxyToken internalStreamProxyToken,
                                   LocalCacheService localCacheService) {
        this.bilibiliService = bilibiliService;
        this.userService = userService;
        this.internalStreamProxyToken = internalStreamProxyToken;
        this.localCacheService = localCacheService;
    }

    @GetMapping("/stream/{bvid}")
    public Mono<ResponseEntity<Flux<DataBuffer>>> streamSong(
            @PathVariable String bvid,
            @RequestParam(required = false) String token,
            @RequestHeader(value = InternalStreamProxyToken.HEADER_NAME, required = false) String internalToken,
            @RequestHeader(value = "Range", required = false) String rangeHeader) {

        if (!canUse(token, internalToken)) {
            return Mono.just(ResponseEntity.status(HttpStatus.FORBIDDEN).build());
        }

        // Failsafe 1: 先检查本地缓存——如果后台下载已完成，直接读本地文件（最快、最稳）
        ResponseEntity<Flux<DataBuffer>> localFallback = tryLocalFile(bvid, rangeHeader);
        if (localFallback != null) {
            log.debug("Bilibili stream using local cache for {}", bvid);
            return Mono.just(localFallback);
        }

        // Failsafe 2: 解析 CDN URL（带短时缓存），失败时 fallback 到本地缓存
        return resolveCdnWithCache(bvid)
                .timeout(Duration.ofSeconds(10))
                .flatMap(cdnUrl -> streamFromCdn(bvid, cdnUrl, rangeHeader, true))
                .onErrorResume(e -> {
                    log.warn("Bilibili resolve stream url failed: bvid={}, message={}", bvid, e.getMessage());
                    // 最终 fallback：再查一次本地缓存（可能解析期间下载完成了）
                    ResponseEntity<Flux<DataBuffer>> fallback = tryLocalFile(bvid, rangeHeader);
                    if (fallback != null) {
                        log.info("Bilibili stream fallback to local cache after resolve failure: bvid={}", bvid);
                        return Mono.just(fallback);
                    }
                    return Mono.just(ResponseEntity.status(HttpStatus.BAD_GATEWAY).<Flux<DataBuffer>>build());
                });
    }

    // Failsafe 1: 检查本地缓存是否已完成，是则直接读文件
    private ResponseEntity<Flux<DataBuffer>> tryLocalFile(String bvid, String rangeHeader) {
        try {
            String localUrl = localCacheService.getLocalUrl(bvid);
            if (localUrl == null) return null;
            // localUrl = /media/xxx.m4a
            String fileName = localUrl.substring("/media/".length());
            Path path = Paths.get(LocalResourceConfig.CACHE_DIR, fileName);
            if (!Files.exists(path)) return null;
            return streamLocalFile(path, "audio/mp4", rangeHeader);
        } catch (Exception e) {
            log.debug("Local file fallback check failed for {}: {}", bvid, e.getMessage());
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
                "local file=" + path.getFileName(),
                log);
        return new ResponseEntity<>(body, headers, status);
    }

    // Failsafe 3: CDN URL 短时缓存（30 秒），避免每次 Range 请求都打 B站 API
    private Mono<String> resolveCdnWithCache(String bvid) {
        CdnEntry cached = cdnUrlCache.get(bvid);
        if (cached != null && cached.expireAt() > System.currentTimeMillis()) {
            return Mono.just(cached.url());
        }
        // 缓存 miss 或过期：实际解析
        return bilibiliService.resolveStreamUrl(bvid)
                .doOnNext(url -> cdnUrlCache.put(bvid, new CdnEntry(url, System.currentTimeMillis() + CDN_TTL_MS)));
    }

    // 从 CDN 流式代理（allowRetry=true 时 CDN 403/404 会重新解析一次再试）
    private Mono<ResponseEntity<Flux<DataBuffer>>> streamFromCdn(String bvid, String cdnUrl, String rangeHeader, boolean allowRetry) {
        if (!StringUtils.hasText(cdnUrl)) {
            return Mono.just(ResponseEntity.notFound().build());
        }

        return Mono.<ResponseEntity<Flux<DataBuffer>>>fromCallable(() -> {
            HttpRequest.Builder requestBuilder = HttpRequest.newBuilder(URI.create(cdnUrl))
                    .timeout(Duration.ofSeconds(30))
                    .GET()
                    .header(HttpHeaders.ACCEPT, MediaType.APPLICATION_OCTET_STREAM_VALUE)
                    .header(HttpHeaders.USER_AGENT, "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
                    .header(HttpHeaders.REFERER, "https://www.bilibili.com/video/" + bvid);
            if (rangeHeader != null && !rangeHeader.isBlank()) {
                requestBuilder.header(HttpHeaders.RANGE, rangeHeader);
            }

            HttpResponse<InputStream> upstream = httpClient.send(requestBuilder.build(), HttpResponse.BodyHandlers.ofInputStream());
            int statusCode = upstream.statusCode();

            // Failsafe 2: CDN 403/404（URL 过期），重新解析一次再试
            if (allowRetry && (statusCode == 403 || statusCode == 404)) {
                closeQuietly(upstream.body());
                log.info("Bilibili CDN URL expired (status={}), re-resolving: bvid={}", statusCode, bvid);
                cdnUrlCache.remove(bvid); // 清缓存
                return null; // 触发外层重新解析
            }
            if (statusCode == HttpStatus.NOT_FOUND.value()) {
                closeQuietly(upstream.body());
                return ResponseEntity.notFound().<Flux<DataBuffer>>build();
            }
            if (statusCode >= 400) {
                closeQuietly(upstream.body());
                log.warn("Bilibili stream upstream error: bvid={}, status={}", bvid, statusCode);
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
                    "bilibili stream bvid=" + bvid,
                    log);
            HttpStatus status = HttpStatus.resolve(statusCode);
            return new ResponseEntity<>(body, headers, status == null ? HttpStatus.OK : status);
        }).subscribeOn(Schedulers.boundedElastic())
          .flatMap(result -> {
              if (result == null) {
                  // CDN URL 过期，重新解析再试一次（allowRetry=false 避免无限循环）
                  return resolveCdnWithCache(bvid)
                          .flatMap(newUrl -> streamFromCdn(bvid, newUrl, rangeHeader, false))
                          .onErrorResume(e -> {
                              // 重新解析也失败，fallback 到本地
                              ResponseEntity<Flux<DataBuffer>> fb = tryLocalFile(bvid, rangeHeader);
                              if (fb != null) return Mono.just(fb);
                              return Mono.just(ResponseEntity.status(HttpStatus.BAD_GATEWAY).<Flux<DataBuffer>>build());
                          });
              }
              return Mono.just(result);
          })
          .onErrorResume(e -> {
              log.warn("Bilibili stream proxy failed: bvid={}, message={}", bvid, e.getMessage());
              // 代理失败，fallback 到本地
              ResponseEntity<Flux<DataBuffer>> fb = tryLocalFile(bvid, rangeHeader);
              if (fb != null) {
                  log.info("Bilibili stream fallback to local cache after proxy failure: bvid={}", bvid);
                  return Mono.just(fb);
              }
              return Mono.just(ResponseEntity.status(HttpStatus.BAD_GATEWAY).<Flux<DataBuffer>>build());
          });
    }

    private boolean canUse(String token, String internalToken) {
        if (internalStreamProxyToken.matches(internalToken)) return true;
        return StringUtils.hasText(token) && userService.getUserBySessionToken(token).isPresent();
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
