package org.thornex.musicparty.controller;

import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.core.io.buffer.DataBuffer;
import org.springframework.core.io.buffer.DefaultDataBufferFactory;
import org.springframework.http.CacheControl;
import org.springframework.http.HttpStatus;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.http.server.reactive.ServerHttpRequest;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.thornex.musicparty.security.ClientIpResolver;
import org.thornex.musicparty.service.stream.LiveStreamService;
import org.thornex.musicparty.service.stream.StreamTokenService;
import org.springframework.web.bind.annotation.RequestParam;
import reactor.core.publisher.Flux;
import reactor.core.scheduler.Schedulers;

import java.io.OutputStream;
import java.time.Duration;
import java.util.concurrent.CountDownLatch;

@RestController
@RequestMapping("/radio")
@RequiredArgsConstructor
@Slf4j
public class StreamController {

    private final LiveStreamService liveStreamService;
    private final StreamTokenService streamTokenService;
    private final ClientIpResolver clientIpResolver;

    @GetMapping(value = "/stream", produces = "audio/mpeg")
    public ResponseEntity<Flux<DataBuffer>> streamAudio(ServerHttpRequest request, @RequestParam(name = "key", required = false) String key) {
        if (!liveStreamService.isEnabled()) {
            return ResponseEntity.status(HttpStatus.SERVICE_UNAVAILABLE).build();
        }

        if (!streamTokenService.validateToken(key)) {
            return ResponseEntity.status(HttpStatus.FORBIDDEN).build();
        }

        String remoteAddr = clientIpResolver.resolve(request);
        Flux<DataBuffer> body = Flux.<byte[]>create(sink -> {
            OutputStream os = new OutputStream() {
                @Override
                public void write(int b) {
                    sink.next(new byte[]{(byte) b});
                }

                @Override
                public void write(byte[] b, int off, int len) {
                    byte[] copy = java.util.Arrays.copyOfRange(b, off, off + len);
                    sink.next(copy);
                }
            };
            sink.onDispose(() -> liveStreamService.removeListener(os, remoteAddr));
            Schedulers.boundedElastic().schedule(() -> {
                try {
                    CountDownLatch closed = liveStreamService.addListener(os, remoteAddr);
                    closed.await();
                    sink.complete();
                } catch (InterruptedException e) {
                    Thread.currentThread().interrupt();
                    sink.complete();
                } catch (Exception e) {
                    log.debug("Stream client disconnected: {}", e.getMessage());
                    sink.complete();
                }
            });
        }).map(bytes -> (DataBuffer) new DefaultDataBufferFactory().wrap(bytes))
                .timeout(Duration.ofHours(24));

        return ResponseEntity.ok()
                .contentType(MediaType.valueOf("audio/mpeg"))
                .cacheControl(CacheControl.noStore())
                .body(body);
            }
}
