package org.thornex.musicparty.service;

import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Service;

import java.time.Duration;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.atomic.AtomicInteger;

@Service
public class SocketRateLimiter {
    private static final Map<String, Rule> RULES = Map.ofEntries(
            Map.entry("chat", new Rule(5, Duration.ofSeconds(10))),
            Map.entry("enqueue", new Rule(8, Duration.ofSeconds(10))),
            Map.entry("queue", new Rule(12, Duration.ofSeconds(10))),
            Map.entry("control", new Rule(8, Duration.ofSeconds(10))),
            Map.entry("room", new Rule(4, Duration.ofSeconds(30))),
            Map.entry("profile", new Rule(6, Duration.ofSeconds(30)))
    );

    private final ConcurrentHashMap<String, Window> windows = new ConcurrentHashMap<>();

    public boolean allow(String sessionId, String action) {
        Rule rule = RULES.getOrDefault(action, new Rule(10, Duration.ofSeconds(10)));
        long now = System.currentTimeMillis();
        String key = sessionId + ":" + action;
        Window window = windows.compute(key, (ignored, existing) -> {
            if (existing == null || now >= existing.resetAt) {
                return new Window(now + rule.window().toMillis());
            }
            existing.count.incrementAndGet();
            return existing;
        });
        return window.count.get() <= rule.maxEvents();
    }

    @Scheduled(fixedDelay = 60000)
    void cleanup() {
        long now = System.currentTimeMillis();
        windows.entrySet().removeIf(entry -> now >= entry.getValue().resetAt);
    }

    private record Rule(int maxEvents, Duration window) {
    }

    private static final class Window {
        private final AtomicInteger count = new AtomicInteger(1);
        private final long resetAt;

        private Window(long resetAt) {
            this.resetAt = resetAt;
        }
    }
}
