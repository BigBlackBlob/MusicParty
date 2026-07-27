package org.thornex.musicparty.service.api;

import io.micrometer.core.instrument.MeterRegistry;
import io.micrometer.core.instrument.Timer;
import org.springframework.stereotype.Service;
import org.thornex.musicparty.config.AppProperties;
import reactor.core.publisher.Mono;
import reactor.util.retry.Retry;

import java.io.IOException;
import java.time.Duration;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.Semaphore;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.function.Supplier;

@Service
public class UpstreamResilienceService {
    private final AppProperties appProperties;
    private final MeterRegistry meterRegistry;
    private final Map<String, PlatformState> platforms = new ConcurrentHashMap<>();
    private final Map<String, CachedMono<?>> cache = new ConcurrentHashMap<>();

    public UpstreamResilienceService(AppProperties appProperties, MeterRegistry meterRegistry) {
        this.appProperties = appProperties;
        this.meterRegistry = meterRegistry;
    }

    public <T> Mono<T> call(String platform, String key, Supplier<Mono<T>> request) {
        String normalizedPlatform = platform == null || platform.isBlank() ? "unknown" : platform;
        return Mono.defer(() -> execute(normalizedPlatform, request));
    }

    @SuppressWarnings("unchecked")
    public <T> Mono<T> cached(String platform, String key, Duration ttl, Supplier<Mono<T>> request) {
        String cacheKey = platform + ":" + key;
        long now = System.nanoTime();
        CachedMono<?> current = cache.get(cacheKey);
        boolean hit = current != null && current.expiresAtNanos > now;
        CachedMono<?> entry = cache.compute(cacheKey, (ignored, existing) -> {
            if (existing != null && existing.expiresAtNanos > now) {
                return existing;
            }
            Mono<T> value = call(platform, key, request)
                    .doOnError(error -> cache.remove(cacheKey))
                    .cache();
            return new CachedMono<>(value, now + ttl.toNanos());
        });
        if (hit) {
            meterRegistry.counter("musicparty.upstream.cache", "platform", platform, "result", "hit").increment();
        }
        if (!hit) meterRegistry.counter("musicparty.upstream.cache", "platform", platform, "result", "miss").increment();
        return (Mono<T>) entry.value;
    }

    private <T> Mono<T> execute(String platform, Supplier<Mono<T>> request) {
        PlatformState state = platforms.computeIfAbsent(platform, ignored -> new PlatformState(
                new Semaphore(Math.max(1, appProperties.getPerformance().getUpstreamMaxConcurrent()))));
        if (state.openUntilMillis > System.currentTimeMillis()) {
            meterRegistry.counter("musicparty.upstream.rejected", "platform", platform, "reason", "circuit_open").increment();
            return Mono.error(new UpstreamUnavailableException("Upstream circuit is open: " + platform));
        }
        if (!state.permits.tryAcquire()) {
            meterRegistry.counter("musicparty.upstream.rejected", "platform", platform, "reason", "bulkhead_full").increment();
            return Mono.error(new UpstreamUnavailableException("Upstream concurrency limit reached: " + platform));
        }

        Timer.Sample sample = Timer.start(meterRegistry);
        return Mono.defer(request)
                .retryWhen(Retry.backoff(2, Duration.ofMillis(200)).jitter(0.3).filter(this::isTransient))
                .doOnSuccess(ignored -> state.failures.set(0))
                .doOnError(error -> recordFailure(platform, state, error))
                .doFinally(ignored -> {
                    state.permits.release();
                    sample.stop(meterRegistry.timer("musicparty.upstream.latency", "platform", platform));
                });
    }

    private boolean isTransient(Throwable error) {
        if (error instanceof IOException || error instanceof java.util.concurrent.TimeoutException) return true;
        if (error instanceof org.springframework.web.reactive.function.client.WebClientRequestException) return true;
        if (error instanceof org.springframework.web.reactive.function.client.WebClientResponseException response) {
            int status = response.getStatusCode().value();
            return status == 429 || status >= 500;
        }
        return false;
    }

    private void recordFailure(String platform, PlatformState state, Throwable error) {
        meterRegistry.counter("musicparty.upstream.failures", "platform", platform).increment();
        if (state.failures.incrementAndGet() >= appProperties.getPerformance().getUpstreamFailureThreshold()) {
            state.openUntilMillis = System.currentTimeMillis() + appProperties.getPerformance().getUpstreamCircuitOpenMs();
            state.failures.set(0);
            meterRegistry.counter("musicparty.upstream.circuit_opened", "platform", platform).increment();
        }
    }

    private record CachedMono<T>(Mono<T> value, long expiresAtNanos) {}

    private static final class PlatformState {
        private final Semaphore permits;
        private final AtomicInteger failures = new AtomicInteger();
        private volatile long openUntilMillis;

        private PlatformState(Semaphore permits) { this.permits = permits; }
    }

    public static class UpstreamUnavailableException extends RuntimeException {
        public UpstreamUnavailableException(String message) { super(message); }
    }
}
