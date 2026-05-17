package org.thornex.musicparty.controller;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.*;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.servlet.mvc.method.annotation.StreamingResponseBody;
import org.thornex.musicparty.service.NavidromeAccessService;
import org.thornex.musicparty.service.RoomSubsonicSource;
import org.thornex.musicparty.service.SubsonicSourceRegistry;
import org.thornex.musicparty.service.stream.InternalStreamProxyToken;
import reactor.core.publisher.Mono;

import java.io.InputStream;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

@RestController
@RequestMapping("/api/subsonic")
@Slf4j
public class SubsonicProxyController {
    private final SubsonicSourceRegistry registry;
    private final NavidromeAccessService accessService;
    private final InternalStreamProxyToken internalStreamProxyToken;
    private final HttpClient httpClient = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(15))
            .followRedirects(HttpClient.Redirect.NORMAL)
            .build();

    public SubsonicProxyController(SubsonicSourceRegistry registry,
                                   NavidromeAccessService accessService,
                                   InternalStreamProxyToken internalStreamProxyToken) {
        this.registry = registry;
        this.accessService = accessService;
        this.internalStreamProxyToken = internalStreamProxyToken;
    }

    @GetMapping("/{sourceId}/stream/{songId}")
    public ResponseEntity<StreamingResponseBody> streamSong(
            @PathVariable String sourceId,
            @PathVariable String songId,
            @RequestParam(required = false) String token,
            @RequestHeader(value = InternalStreamProxyToken.HEADER_NAME, required = false) String internalToken,
            @RequestHeader(value = "Range", required = false) String rangeHeader) {
        RoomSubsonicSource source = registry.findRoomSource("lounge", sourceId).orElse(null);
        return streamSong("lounge", sourceId, songId, token, internalToken, rangeHeader, source);
    }

    @GetMapping("/{roomId}/{sourceId}/stream/{songId}")
    public ResponseEntity<StreamingResponseBody> streamSong(
            @PathVariable String roomId,
            @PathVariable String sourceId,
            @PathVariable String songId,
            @RequestParam(required = false) String token,
            @RequestHeader(value = InternalStreamProxyToken.HEADER_NAME, required = false) String internalToken,
            @RequestHeader(value = "Range", required = false) String rangeHeader) {
        RoomSubsonicSource source = registry.findRoomSource(roomId, sourceId).orElse(null);
        return streamSong(roomId, sourceId, songId, token, internalToken, rangeHeader, source);
    }

    private ResponseEntity<StreamingResponseBody> streamSong(String roomId,
                                                             String sourceId,
                                                             String songId,
                                                             String token,
                                                             String internalToken,
                                                             String rangeHeader,
                                                             RoomSubsonicSource source) {
        if (!canUse(source, token, internalToken)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).build();
        }

        URI uri = registry.client(source.source()).buildStreamUri(songId);
        try {
            log.debug("Subsonic stream proxy request: room={}, source={}, songId={}, range={}",
                    roomId, sourceId, songId, rangeHeader == null ? "" : rangeHeader);
            HttpRequest.Builder requestBuilder = HttpRequest.newBuilder(uri)
                    .timeout(Duration.ofSeconds(90))
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
            if (statusCode >= 400) {
                closeQuietly(upstream.body());
                log.warn("Subsonic stream upstream error: room={}, source={}, songId={}, status={}",
                        roomId, sourceId, songId, statusCode);
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
            log.debug("Subsonic stream proxy response: room={}, source={}, songId={}, status={}, contentType={}, contentLength={}",
                    roomId,
                    sourceId,
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
            log.warn("Subsonic stream proxy failed: room={}, source={}, songId={}, message={}", roomId, sourceId, songId, e.getMessage());
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
    }

    @GetMapping("/{sourceId}/cover/{coverArtId}")
    public Mono<ResponseEntity<byte[]>> coverArt(
            @PathVariable String sourceId,
            @PathVariable String coverArtId,
            @RequestParam(required = false) String token,
            @RequestHeader(value = InternalStreamProxyToken.HEADER_NAME, required = false) String internalToken) {
        RoomSubsonicSource source = registry.findRoomSource("lounge", sourceId).orElse(null);
        return coverArt("lounge", sourceId, coverArtId, token, internalToken, source);
    }

    @GetMapping("/{roomId}/{sourceId}/cover/{coverArtId}")
    public Mono<ResponseEntity<byte[]>> coverArt(
            @PathVariable String roomId,
            @PathVariable String sourceId,
            @PathVariable String coverArtId,
            @RequestParam(required = false) String token,
            @RequestHeader(value = InternalStreamProxyToken.HEADER_NAME, required = false) String internalToken) {
        RoomSubsonicSource source = registry.findRoomSource(roomId, sourceId).orElse(null);
        return coverArt(roomId, sourceId, coverArtId, token, internalToken, source);
    }

    private Mono<ResponseEntity<byte[]>> coverArt(String roomId,
                                                  String sourceId,
                                                  String coverArtId,
                                                  String token,
                                                  String internalToken,
                                                  RoomSubsonicSource source) {
        if (!canUse(source, token, internalToken)) {
            return Mono.just(ResponseEntity.status(HttpStatus.FORBIDDEN).build());
        }
        URI uri = registry.client(source.source()).buildCoverUri(coverArtId, 300);
        return Mono.fromCallable(() -> {
            HttpRequest request = HttpRequest.newBuilder(uri)
                    .timeout(Duration.ofSeconds(45))
                    .GET()
                    .build();
            HttpResponse<byte[]> upstream = httpClient.send(request, HttpResponse.BodyHandlers.ofByteArray());
            if (upstream.statusCode() == HttpStatus.NOT_FOUND.value()) {
                return ResponseEntity.status(HttpStatus.NOT_FOUND).<byte[]>build();
            }
            if (upstream.statusCode() >= 400) {
                return ResponseEntity.status(HttpStatus.BAD_GATEWAY).<byte[]>build();
            }
            HttpHeaders headers = new HttpHeaders();
            upstream.headers().firstValue(HttpHeaders.CONTENT_TYPE).ifPresent(value -> headers.set(HttpHeaders.CONTENT_TYPE, value));
            headers.set(HttpHeaders.CACHE_CONTROL, "private, max-age=86400");
            HttpStatus status = HttpStatus.resolve(upstream.statusCode());
            return new ResponseEntity<>(upstream.body(), headers, status == null ? HttpStatus.OK : status);
        }).onErrorResume(e -> {
            log.warn("Subsonic cover proxy failed: room={}, source={}, coverArtId={}, message={}", roomId, sourceId, coverArtId, e.getMessage());
            return Mono.just(ResponseEntity.status(HttpStatus.BAD_GATEWAY).build());
        });
    }

    private boolean canUse(RoomSubsonicSource source, String token, String internalToken) {
        if (source == null || !source.active()) return false;
        if (internalStreamProxyToken.matches(internalToken)) return true;
        if (source.allowedUsers() != null && source.allowedUsers().contains("*")) return token != null && !token.isBlank();
        return token != null && accessService.canUseBySessionToken(token, source.allowedUsers());
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
