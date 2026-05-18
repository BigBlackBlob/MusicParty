package org.thornex.musicparty.controller;

import lombok.extern.slf4j.Slf4j;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import org.springframework.web.servlet.mvc.method.annotation.StreamingResponseBody;
import org.thornex.musicparty.service.NavidromeAccessService;
import org.thornex.musicparty.service.api.NavidromeSubsonicClient;
import org.thornex.musicparty.service.stream.InternalStreamProxyToken;
import reactor.core.publisher.Mono;

import java.io.InputStream;
import java.net.URI;
import java.time.Duration;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;

@RestController
@RequestMapping("/api/navidrome")
@ConditionalOnProperty(prefix = "app.music-api.navidrome", name = "enabled", havingValue = "true")
@Slf4j
public class NavidromeProxyController {

    private final WebClient webClient;
    private final NavidromeAccessService navidromeAccessService;
    private final NavidromeSubsonicClient subsonicClient;
    private final InternalStreamProxyToken internalStreamProxyToken;
    private final HttpClient httpClient = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(5))
            .followRedirects(HttpClient.Redirect.NORMAL)
            .build();

    public NavidromeProxyController(WebClient webClient,
                                    NavidromeAccessService navidromeAccessService,
                                    NavidromeSubsonicClient subsonicClient,
                                    InternalStreamProxyToken internalStreamProxyToken) {
        this.webClient = webClient;
        this.navidromeAccessService = navidromeAccessService;
        this.subsonicClient = subsonicClient;
        this.internalStreamProxyToken = internalStreamProxyToken;
    }

    @GetMapping("/stream/{songId}")
    public ResponseEntity<StreamingResponseBody> streamSong(
            @PathVariable String songId,
            @RequestParam(required = false) String token,
            @RequestHeader(value = InternalStreamProxyToken.HEADER_NAME, required = false) String internalToken,
            @RequestHeader(value = "Range", required = false) String rangeHeader) {

        if (!canUse(token, internalToken)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).build();
        }

        URI uri = subsonicClient.buildStreamUri(songId);
        log.debug("Proxying stream for songId: {}", songId);

        try {
            HttpRequest.Builder requestBuilder = HttpRequest.newBuilder(uri)
                    .timeout(Duration.ofSeconds(30))
                    .GET()
                    .header(HttpHeaders.ACCEPT, MediaType.APPLICATION_OCTET_STREAM_VALUE);
            if (rangeHeader != null && !rangeHeader.isBlank()) {
                requestBuilder.header(HttpHeaders.RANGE, rangeHeader);
            }

            HttpResponse<InputStream> upstream = httpClient.send(requestBuilder.build(), HttpResponse.BodyHandlers.ofInputStream());
            int statusCode = upstream.statusCode();
            if (statusCode == HttpStatus.NOT_FOUND.value()) {
                closeQuietly(upstream.body());
                return ResponseEntity.notFound().build();
            }
            if (statusCode == HttpStatus.UNAUTHORIZED.value() || statusCode == HttpStatus.FORBIDDEN.value()) {
                closeQuietly(upstream.body());
                log.warn("Navidrome stream upstream auth error for songId: {}, status={}", songId, statusCode);
                return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
            }
            if (statusCode >= 400) {
                closeQuietly(upstream.body());
                log.warn("Navidrome stream upstream error for songId: {}, status={}", songId, statusCode);
                return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
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
            log.debug("Navidrome stream proxy response: songId={}, status={}, contentType={}, contentLength={}",
                    songId,
                    statusCode,
                    upstream.headers().firstValue(HttpHeaders.CONTENT_TYPE).orElse(""),
                    upstream.headers().firstValue(HttpHeaders.CONTENT_LENGTH).orElse(""));

            StreamingResponseBody body = outputStream -> {
                try (InputStream inputStream = upstream.body()) {
                    inputStream.transferTo(outputStream);
                    outputStream.flush();
                }
            };

            HttpStatus status = HttpStatus.resolve(statusCode);
            return new ResponseEntity<>(body, headers, status == null ? HttpStatus.OK : status);
        } catch (Exception e) {
            log.error("Navidrome stream proxy error for songId: {}", songId, e);
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @GetMapping("/cover/{coverArtId}")
    public Mono<ResponseEntity<byte[]>> coverArt(
            @PathVariable String coverArtId,
            @RequestParam(required = false) String token,
            @RequestHeader(value = InternalStreamProxyToken.HEADER_NAME, required = false) String internalToken) {

        if (!canUse(token, internalToken)) {
            return Mono.just(ResponseEntity.status(HttpStatus.FORBIDDEN).build());
        }

        URI uri = subsonicClient.buildCoverUri(coverArtId, 300);
        log.debug("Proxying cover for coverArtId: {}", coverArtId);

        return webClient.get()
                .uri(uri)
                .retrieve()
                .toEntity(byte[].class)
                .map(response -> {
                    HttpHeaders headers = new HttpHeaders();
                    if (response.getHeaders().getContentType() != null) {
                        headers.setContentType(response.getHeaders().getContentType());
                    }
                    headers.set(HttpHeaders.CACHE_CONTROL, "private, max-age=86400");
                    return new ResponseEntity<>(response.getBody(), headers, response.getStatusCode());
                })
                .onErrorResume(e -> {
                    if (e instanceof WebClientResponseException.NotFound) {
                        log.debug("Navidrome cover not found for coverArtId: {}", coverArtId);
                        return Mono.just(ResponseEntity.status(HttpStatus.NOT_FOUND).build());
                    }
                    if (e instanceof WebClientResponseException responseException) {
                        log.warn("Navidrome cover upstream error for coverArtId: {}, status={}", coverArtId, responseException.getStatusCode());
                    } else {
                        log.warn("Navidrome cover proxy error for coverArtId: {}", coverArtId, e);
                    }
                    return Mono.just(ResponseEntity.status(HttpStatus.BAD_GATEWAY).build());
                });
    }

    private void copyHeader(HttpResponse<?> response, HttpHeaders headers, String name) {
        response.headers().firstValue(name).ifPresent(value -> headers.set(name, value));
    }

    private boolean canUse(String token, String internalToken) {
        if (internalStreamProxyToken.matches(internalToken)) return true;
        return token != null && navidromeAccessService.canUseBySessionToken(token);
    }

    private void closeQuietly(InputStream inputStream) {
        try {
            if (inputStream != null) {
                inputStream.close();
            }
        } catch (Exception ignored) {
        }
    }
}
