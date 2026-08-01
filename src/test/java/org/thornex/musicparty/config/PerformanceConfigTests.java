package org.thornex.musicparty.config;

import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;

class PerformanceConfigTests {

    @Test
    void exposesConservativeRuntimeGuardDefaults() {
        AppProperties properties = new AppProperties();

        assertThat(properties.getPerformance().getDownloadMaxQueuedTasks()).isEqualTo(100);
        assertThat(properties.getPerformance().getDownloadTaskTtlMs()).isEqualTo(30 * 60 * 1000L);
        assertThat(properties.getPerformance().getCoverColorCacheSize()).isEqualTo(256);
        assertThat(properties.getPerformance().getCoverColorMaxConcurrent()).isEqualTo(4);
        assertThat(properties.getLocalLibrary().getMaxEmbeddedCoverBytes()).isEqualTo(1_048_576L);
    }

}
