package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;

import java.time.Duration;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class ChatServicePerformanceTests {

    @Test
    void cleanupLastMessageTimesRemovesIdleEntries() {
        ChatService service = new ChatService(null, null, new AppProperties(), null, null, null, null, List.of());

        assertThat(service.canUserSendMessage("user-a")).isTrue();
        assertThat(service.getTrackedMessageRateLimitCount()).isEqualTo(1);

        service.cleanupLastMessageTimes(System.currentTimeMillis() + Duration.ofMinutes(10).toMillis());

        assertThat(service.getTrackedMessageRateLimitCount()).isZero();
    }
}
