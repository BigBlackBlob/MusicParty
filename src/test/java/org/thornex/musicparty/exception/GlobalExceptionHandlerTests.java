package org.thornex.musicparty.exception;

import org.junit.jupiter.api.Test;
import org.springframework.http.HttpStatus;
import org.springframework.web.multipart.MaxUploadSizeExceededException;

import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;

class GlobalExceptionHandlerTests {

    @Test
    void maxUploadSizeExceededReturnsPayloadTooLarge() {
        GlobalExceptionHandler handler = new GlobalExceptionHandler();

        var response = handler.handleMaxUploadSizeExceeded(new MaxUploadSizeExceededException(10));

        assertThat(response.getStatusCode()).isEqualTo(HttpStatus.PAYLOAD_TOO_LARGE);
        assertThat(response.getBody()).isInstanceOf(Map.class);
        Map<?, ?> body = (Map<?, ?>) response.getBody();
        assertThat(body.get("status")).isEqualTo(HttpStatus.PAYLOAD_TOO_LARGE.value());
        assertThat(body.get("error")).isEqualTo("MaxUploadSizeExceededException");
    }
}
