package org.thornex.musicparty.config;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class PerformanceConfigTests {

    @Test
    void exposesConservativeRuntimeGuardDefaults() {
        AppProperties properties = new AppProperties();

        assertThat(properties.getPerformance().getStreamMaxListeners()).isEqualTo(50);
        assertThat(properties.getPerformance().getStreamClientQueueCapacity()).isEqualTo(32);
        assertThat(properties.getPerformance().getStreamWriterThreads()).isEqualTo(8);
        assertThat(properties.getPerformance().getDownloadMaxQueuedTasks()).isEqualTo(100);
        assertThat(properties.getPerformance().getDownloadTaskTtlMs()).isEqualTo(30 * 60 * 1000L);
        assertThat(properties.getPerformance().getCoverColorCacheSize()).isEqualTo(256);
        assertThat(properties.getPerformance().getCoverColorMaxConcurrent()).isEqualTo(4);
        assertThat(properties.getLocalLibrary().getMaxEmbeddedCoverBytes()).isEqualTo(1_048_576L);
    }

    @Test
    void exposesDesktopHostRuntimeDefaults() {
        AppProperties properties = new AppProperties();

        assertThat(properties.getMode()).isEqualTo("server");
        assertThat(properties.getDesktop().getProfileDir()).isEqualTo("data/desktop-profile");
        assertThat(properties.getDesktop().getLanBaseUrl()).isEmpty();
        assertThat(properties.getDesktop().getNeteaseApiPort()).isEqualTo(3000);
        assertThat(properties.getDesktop().isAdminInitialized()).isFalse();
    }
}
