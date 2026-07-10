package org.thornex.musicparty.exception;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.mock.http.server.reactive.MockServerHttpRequest;
import org.springframework.mock.web.server.MockServerWebExchange;
import org.springframework.web.multipart.MaxUploadSizeExceededException;
import reactor.core.publisher.Mono;
import reactor.test.StepVerifier;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

class GlobalExceptionHandlerTests {

    private GlobalExceptionHandler handler = new GlobalExceptionHandler();

    private MockServerWebExchange uncommittedExchange() {
        return MockServerWebExchange.from(MockServerHttpRequest.get("/").build());
    }

    private MockServerWebExchange committedExchange() {
        MockServerWebExchange exchange = uncommittedExchange();
        exchange.getResponse().setStatusCode(HttpStatus.OK);
        // Drive the mock response through its real commit lifecycle so
        // isCommitted() reports true (no public setCommitted exists in 6.1.x).
        exchange.getResponse().setComplete().block();
        return exchange;
    }

    @Test
    void maxUploadSizeExceededReturnsPayloadTooLargeWhenUncommitted() {
        MockServerWebExchange exchange = uncommittedExchange();

        Mono<ResponseEntity<Object>> result = handler.handleMaxUploadSizeExceeded(
                new MaxUploadSizeExceededException(10), exchange);

        StepVerifier.create(result)
                .assertNext(response -> {
                    assertThat(response.getStatusCode()).isEqualTo(HttpStatus.PAYLOAD_TOO_LARGE);
                    assertThat(response.getBody()).isInstanceOf(Map.class);
                    Map<?, ?> body = (Map<?, ?>) response.getBody();
                    assertThat(body.get("status")).isEqualTo(HttpStatus.PAYLOAD_TOO_LARGE.value());
                    assertThat(body.get("error")).isEqualTo("MaxUploadSizeExceededException");
                })
                .verifyComplete();
    }

    @Test
    void maxUploadSizeExceededSkipsBodyWhenResponseCommitted() {
        MockServerWebExchange exchange = committedExchange();

        Mono<ResponseEntity<Object>> result = handler.handleMaxUploadSizeExceeded(
                new MaxUploadSizeExceededException(10), exchange);

        StepVerifier.create(result)
                .verifyComplete();
        assertThat(exchange.getResponse().isCommitted()).isTrue();
    }

    @Test
    void apiRequestExceptionReturnsBadGatewayWhenUncommitted() {
        MockServerWebExchange exchange = uncommittedExchange();

        Mono<ResponseEntity<Object>> result = handler.handleApiRequestException(
                new ApiRequestException("boom"), exchange);

        StepVerifier.create(result)
                .assertNext(response -> {
                    assertThat(response.getStatusCode()).isEqualTo(HttpStatus.BAD_GATEWAY);
                    assertThat(response.getBody()).isInstanceOf(Map.class);
                    Map<?, ?> body = (Map<?, ?>) response.getBody();
                    assertThat(body.get("status")).isEqualTo(HttpStatus.BAD_GATEWAY.value());
                    assertThat(body.get("message")).isEqualTo("boom");
                })
                .verifyComplete();
    }

    @Test
    void apiRequestExceptionSkipsBodyWhenResponseCommitted() {
        MockServerWebExchange exchange = committedExchange();

        Mono<ResponseEntity<Object>> result = handler.handleApiRequestException(
                new ApiRequestException("boom"), exchange);

        StepVerifier.create(result)
                .verifyComplete();
        assertThat(exchange.getResponse().isCommitted()).isTrue();
    }

    @Test
    void genericExceptionReturnsInternalServerErrorWhenUncommitted() {
        MockServerWebExchange exchange = uncommittedExchange();

        Mono<ResponseEntity<Object>> result = handler.handleGenericException(
                new IllegalStateException("kaboom"), exchange);

        StepVerifier.create(result)
                .assertNext(response -> {
                    assertThat(response.getStatusCode()).isEqualTo(HttpStatus.INTERNAL_SERVER_ERROR);
                    assertThat(response.getBody()).isInstanceOf(Map.class);
                    Map<?, ?> body = (Map<?, ?>) response.getBody();
                    assertThat(body.get("status")).isEqualTo(HttpStatus.INTERNAL_SERVER_ERROR.value());
                    assertThat(body.get("error")).isEqualTo("IllegalStateException");
                })
                .verifyComplete();
    }

    @Test
    void genericExceptionSkipsBodyWhenResponseCommitted() {
        MockServerWebExchange exchange = committedExchange();

        Mono<ResponseEntity<Object>> result = handler.handleGenericException(
                new IllegalStateException("kaboom"), exchange);

        StepVerifier.create(result)
                .verifyComplete();
        assertThat(exchange.getResponse().isCommitted()).isTrue();
    }
}
