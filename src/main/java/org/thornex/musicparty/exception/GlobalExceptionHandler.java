package org.thornex.musicparty.exception;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.http.ResponseEntity;
import org.springframework.web.multipart.MaxUploadSizeExceededException;
import org.springframework.web.bind.annotation.ControllerAdvice;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.server.ServerWebExchange;
import reactor.core.publisher.Mono;

import java.util.Map;

@ControllerAdvice
@Slf4j
public class GlobalExceptionHandler {

    @ExceptionHandler(ApiRequestException.class)
    public Mono<ResponseEntity<Object>> handleApiRequestException(ApiRequestException ex, ServerWebExchange exchange) {
        if (exchange.getResponse().isCommitted()) {
            log.warn("Response already committed while handling ApiRequestException: {}", ex.getMessage());
            return exchange.getResponse().setComplete().then(Mono.empty());
        }
        log.warn("Handled ApiRequestException: {}", ex.getMessage(), ex);
        // For failures related to external APIs, return 502 Bad Gateway
        Map<String, Object> body = Map.of(
                "message", ex.getMessage(),
                "status", HttpStatus.BAD_GATEWAY.value()
        );
        return Mono.just(new ResponseEntity<>(body, HttpStatus.BAD_GATEWAY));
    }

    @ExceptionHandler(MaxUploadSizeExceededException.class)
    public Mono<ResponseEntity<Object>> handleMaxUploadSizeExceeded(MaxUploadSizeExceededException ex, ServerWebExchange exchange) {
        if (exchange.getResponse().isCommitted()) {
            log.warn("Response already committed while handling MaxUploadSizeExceededException: {}", ex.getMessage());
            return exchange.getResponse().setComplete().then(Mono.empty());
        }
        log.warn("Upload request rejected because it exceeded multipart limits: {}", ex.getMessage());
        Map<String, Object> body = Map.of(
                "message", "Upload is too large. Increase MULTIPART_MAX_FILE_SIZE / MULTIPART_MAX_REQUEST_SIZE or choose a smaller file.",
                "error", ex.getClass().getSimpleName(),
                "status", HttpStatus.PAYLOAD_TOO_LARGE.value()
        );
        return Mono.just(new ResponseEntity<>(body, HttpStatus.PAYLOAD_TOO_LARGE));
    }

    @ExceptionHandler(Exception.class)
    public Mono<ResponseEntity<Object>> handleGenericException(Exception ex, ServerWebExchange exchange) {
        if (exchange.getResponse().isCommitted()) {
            log.warn("Response already committed while handling unexpected exception: {}", ex.getClass().getSimpleName());
            return exchange.getResponse().setComplete().then(Mono.empty());
        }
        log.error("Handled unexpected exception", ex);
        // For all other unexpected errors, return 500 Internal Server Error
        Map<String, Object> body = Map.of(
                "message", "An unexpected internal server error occurred.",
                "error", ex.getClass().getSimpleName(),
                "status", HttpStatus.INTERNAL_SERVER_ERROR.value()
        );
        return Mono.just(new ResponseEntity<>(body, HttpStatus.INTERNAL_SERVER_ERROR));
    }
}
