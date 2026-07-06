package org.thornex.musicparty.service.stream;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;

import java.io.ByteArrayOutputStream;

import static org.assertj.core.api.Assertions.assertThat;

class LiveStreamServicePerformanceTests {

    @Test
    void addListenerRejectsConnectionsAboveConfiguredLimit() {
        AppProperties properties = new AppProperties();
        properties.getPerformance().setStreamMaxListeners(1);
        LiveStreamService service = new LiveStreamService(null, event -> {
        }, properties, new InternalStreamProxyToken());
        service.init();

        assertThat(service.addListener(new ByteArrayOutputStream(), "10.0.0.1")).isNotNull();
        assertThat(service.addListener(new ByteArrayOutputStream(), "10.0.0.2")).isNull();
        assertThat(service.getStreamListenerCount()).isEqualTo(1);

        service.cleanup();
    }
}
