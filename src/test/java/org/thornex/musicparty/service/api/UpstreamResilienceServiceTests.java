package org.thornex.musicparty.service.api;

import io.micrometer.core.instrument.simple.SimpleMeterRegistry;
import org.junit.jupiter.api.Test;
import org.springframework.http.HttpHeaders;
import org.springframework.http.HttpStatus;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import reactor.core.publisher.Mono;
import reactor.test.StepVerifier;

import java.time.Duration;
import java.util.concurrent.TimeoutException;
import java.util.concurrent.atomic.AtomicInteger;

import org.thornex.musicparty.config.AppProperties;

import static org.assertj.core.api.Assertions.assertThat;

class UpstreamResilienceServiceTests {

    @Test
    void retriesRateLimitAndServerErrorsButNotAuthenticationFailures() {
        assertRetryCount(HttpStatus.TOO_MANY_REQUESTS, 3);
        assertRetryCount(HttpStatus.INTERNAL_SERVER_ERROR, 3);
        assertRetryCount(HttpStatus.UNAUTHORIZED, 1);
        assertRetryCount(HttpStatus.FORBIDDEN, 1);
    }

    @Test
    void retriesTimeoutsWithBoundedAttempts() {
        UpstreamResilienceService service = service(4, 10);
        AtomicInteger attempts = new AtomicInteger();

        StepVerifier.create(service.call("timeout", "one", () -> {
                    attempts.incrementAndGet();
                    return Mono.error(new TimeoutException("local fixture timeout"));
                }))
                .expectError()
                .verify(Duration.ofSeconds(3));

        assertThat(attempts).hasValue(3);
    }

    @Test
    void saturatedPlatformDoesNotBlockAnotherPlatform() {
        UpstreamResilienceService service = service(1, 10);
        Mono<String> held = service.call("platform-a", "held", () -> Mono.never());

        held.subscribe();
        StepVerifier.create(service.call("platform-a", "rejected", () -> Mono.just("not reached")))
                .expectError(UpstreamResilienceService.UpstreamUnavailableException.class)
                .verify(Duration.ofSeconds(1));
        StepVerifier.create(service.call("platform-b", "independent", () -> Mono.just("ok")))
                .expectNext("ok")
                .verifyComplete();
    }

    @Test
    void opensCircuitAfterConfiguredConsecutiveFailures() {
        UpstreamResilienceService service = service(2, 1);
        StepVerifier.create(service.call("failing", "first", () -> Mono.error(responseError(HttpStatus.BAD_GATEWAY))))
                .expectError()
                .verify(Duration.ofSeconds(3));

        StepVerifier.create(service.call("failing", "blocked", () -> Mono.just("not reached")))
                .expectError(UpstreamResilienceService.UpstreamUnavailableException.class)
                .verify(Duration.ofSeconds(1));
    }

    private void assertRetryCount(HttpStatus status, int expectedAttempts) {
        UpstreamResilienceService service = service(4, 10);
        AtomicInteger attempts = new AtomicInteger();
        StepVerifier.create(service.call("status-" + status.value(), "one", () -> {
                    attempts.incrementAndGet();
                    return Mono.error(responseError(status));
                }))
                .expectError()
                .verify(Duration.ofSeconds(3));
        assertThat(attempts).hasValue(expectedAttempts);
    }

    private UpstreamResilienceService service(int maxConcurrent, int failureThreshold) {
        AppProperties properties = new AppProperties();
        properties.getPerformance().setUpstreamMaxConcurrent(maxConcurrent);
        properties.getPerformance().setUpstreamFailureThreshold(failureThreshold);
        properties.getPerformance().setUpstreamCircuitOpenMs(5_000);
        return new UpstreamResilienceService(properties, new SimpleMeterRegistry());
    }

    private WebClientResponseException responseError(HttpStatus status) {
        return WebClientResponseException.create(status.value(), status.getReasonPhrase(), HttpHeaders.EMPTY,
                new byte[0], null);
    }
}
