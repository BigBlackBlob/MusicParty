package org.thornex.musicparty.security;

import org.springframework.stereotype.Component;
import org.thornex.musicparty.config.AppProperties;

import java.util.ArrayDeque;
import java.util.Deque;
import java.util.Iterator;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

@Component
public class LoginRateLimiter {
    private static final long MS_PER_SECOND = 1000L;

    private final AppProperties appProperties;
    private final ConcurrentHashMap<String, Entry> attempts = new ConcurrentHashMap<>();

    public LoginRateLimiter(AppProperties appProperties) {
        this.appProperties = appProperties;
    }

    public boolean isEnabled() {
        return appProperties.getAuth().isRateLimitEnabled();
    }

    public long retryAfterMillis(String ip) {
        if (!isEnabled()) return 0L;
        Entry entry = attempts.get(normalize(ip));
        if (entry == null) return 0L;
        long remaining = entry.blockedUntil - System.currentTimeMillis();
        return remaining > 0 ? remaining : 0L;
    }

    public boolean isBlocked(String ip) {
        if (!isEnabled()) return false;
        String key = normalize(ip);
        Entry entry = attempts.get(key);
        if (entry == null) return false;
        return System.currentTimeMillis() < entry.blockedUntil;
    }

    public void recordFailure(String ip) {
        if (!isEnabled()) return;
        String key = normalize(ip);
        long now = System.currentTimeMillis();
        long windowMs = appProperties.getAuth().getWindowSeconds() * MS_PER_SECOND;
        int maxAttempts = appProperties.getAuth().getMaxAttempts();

        attempts.compute(key, (existing, prev) -> {
            Entry current = prev != null ? prev : new Entry();
            synchronized (current) {
                pruneOlder(current, now, windowMs);
                current.failures.addLast(now);
                if (current.failures.size() >= maxAttempts) {
                    current.blockedUntil = now + appProperties.getAuth().getBlockDurationSeconds() * MS_PER_SECOND;
                    current.failures.clear();
                }
            }
            return current;
        });
        enforceTrackingCap();
    }

    public void recordSuccess(String ip) {
        if (!isEnabled()) return;
        attempts.remove(normalize(ip));
    }

    private void pruneOlder(Entry entry, long now, long windowMs) {
        long cutoff = now - windowMs;
        Deque<Long> failures = entry.failures;
        while (!failures.isEmpty() && failures.peekFirst() < cutoff) {
            failures.pollFirst();
        }
    }

    private void enforceTrackingCap() {
        int maxTracked = appProperties.getAuth().getMaxTrackedIps();
        if (maxTracked <= 0 || attempts.size() <= maxTracked) {
            return;
        }
        int toRemove = attempts.size() - maxTracked / 2;
        Iterator<Map.Entry<String, Entry>> iterator = attempts.entrySet().iterator();
        while (iterator.hasNext() && toRemove > 0) {
            iterator.next();
            iterator.remove();
            toRemove--;
        }
    }

    private String normalize(String ip) {
        return ip == null ? "" : ip.trim();
    }

    private static final class Entry {
        final Deque<Long> failures = new ArrayDeque<>();
        volatile long blockedUntil = 0L;
    }
}
