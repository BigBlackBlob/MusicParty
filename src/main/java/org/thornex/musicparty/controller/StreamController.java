package org.thornex.musicparty.controller;

import jakarta.servlet.http.HttpServletRequest;
import jakarta.servlet.http.HttpServletResponse;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.GetMapping;
import org.springframework.web.bind.annotation.RequestMapping;
import org.springframework.web.bind.annotation.RestController;
import org.thornex.musicparty.security.ClientIpResolver;
import org.thornex.musicparty.service.stream.LiveStreamService;
import org.thornex.musicparty.service.stream.StreamTokenService;
import org.springframework.web.bind.annotation.RequestParam;

import java.io.IOException;
import java.io.OutputStream;
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
    public void streamAudio(HttpServletRequest request, HttpServletResponse response, @RequestParam(name = "key", required = false) String key) {
        if (!liveStreamService.isEnabled()) {
            response.setStatus(HttpServletResponse.SC_SERVICE_UNAVAILABLE);
            return;
        }

        if (!streamTokenService.validateToken(key)) {
            response.setStatus(HttpServletResponse.SC_FORBIDDEN);
            return;
        }

        response.setContentType("audio/mpeg");
        response.setHeader("Transfer-Encoding", "chunked");
        response.setHeader("Connection", "keep-alive");
        // 这是一个伪直播，不应该被缓存
        response.setHeader("Cache-Control", "no-cache, no-store, must-revalidate");
        response.setHeader("Pragma", "no-cache");
        response.setHeader("Expires", "0");

        String remoteAddr = clientIpResolver.resolve(request);
        OutputStream os = null;
        try {
            os = response.getOutputStream();
            CountDownLatch closed = liveStreamService.addListener(os, remoteAddr);
            closed.await();
            
        } catch (IOException | InterruptedException e) {
            if (e instanceof InterruptedException) {
                Thread.currentThread().interrupt();
            }
            log.debug("Stream client disconnected: {}", e.getMessage());
        } finally {
            if (os != null) {
                liveStreamService.removeListener(os, remoteAddr);
            }
        }
    }
}
