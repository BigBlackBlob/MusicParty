package org.thornex.musicparty.controller;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.util.StringUtils;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.servlet.mvc.method.annotation.StreamingResponseBody;
import org.thornex.musicparty.service.UserService;
import org.thornex.musicparty.service.api.NeteaseMusicApiService;
import org.thornex.musicparty.service.stream.InternalStreamProxyToken;

import java.io.InputStream;
import java.net.URI;
import java.net.http.HttpClient;
import java.net.http.HttpRequest;
import java.net.http.HttpResponse;
import java.time.Duration;

@RestController
@RequestMapping("/api/netease")
@Slf4j
public class NeteaseProxyController {

    private final NeteaseMusicApiService neteaseService;
    private final UserService userService;
    private final InternalStreamProxyToken internalStreamProxyToken;
    private final HttpClient httpClient = HttpClient.newBuilder()
            .connectTimeout(Duration.ofSeconds(10))
            .followRedirects(HttpClient.Redirect.NORMAL)
            .build();

    public NeteaseProxyController(NeteaseMusicApiService neteaseService,
                                  UserService userService,
                                  InternalStreamProxyToken internalStreamProxyToken) {
        this.neteaseService = neteaseService;
        this.userService = userService;
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

        String cdnUrl;
        try {
            cdnUrl = neteaseService.resolveCdnUrl(songId).block(Duration.ofSeconds(8));
        } catch (Exception e) {
            log.warn("Netease resolve cdn url failed: songId={}, message={}", songId, e.getMessage());
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
        if (!StringUtils.hasText(cdnUrl)) {
            // 该歌曲 VIP/下架，没有可用 URL
            return ResponseEntity.notFound().build();
        }

        try {
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
                return ResponseEntity.notFound().build();
            }
            if (statusCode >= 400) {
                closeQuietly(upstream.body());
                log.warn("Netease stream upstream error: songId={}, status={}", songId, statusCode);
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

            StreamingResponseBody body = outputStream -> {
                try (InputStream inputStream = upstream.body()) {
                    inputStream.transferTo(outputStream);
                    outputStream.flush();
                }
            };

            HttpStatus status = HttpStatus.resolve(statusCode);
            return new ResponseEntity<>(body, headers, status == null ? HttpStatus.OK : status);
        } catch (Exception e) {
            log.warn("Netease stream proxy failed: songId={}, message={}", songId, e.getMessage());
            return ResponseEntity.status(HttpStatus.BAD_GATEWAY).build();
        }
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
