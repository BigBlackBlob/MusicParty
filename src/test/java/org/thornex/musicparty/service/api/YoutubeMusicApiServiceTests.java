package org.thornex.musicparty.service.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.service.LocalCacheService;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.Mockito.mock;

class YoutubeMusicApiServiceTests {
    @Test
    void parsesYoutubeIsoDurations() {
        assertThat(YoutubeMusicApiService.parseIsoDurationMillis("PT3M42S")).isEqualTo(222_000L);
        assertThat(YoutubeMusicApiService.parseIsoDurationMillis("PT1H2M3S")).isEqualTo(3_723_000L);
        assertThat(YoutubeMusicApiService.parseIsoDurationMillis("")).isZero();
    }

    @Test
    void availableWhenEnabledApiKeyAndYtDlpAreConfigured() {
        AppProperties properties = new AppProperties();
        AppProperties.YoutubeApiConfig youtube = new AppProperties.YoutubeApiConfig();
        youtube.setEnabled(true);
        youtube.setApiKey("key");
        youtube.setYtDlpPath("java");
        properties.setYoutube(youtube);

        YoutubeMusicApiService service = new YoutubeMusicApiService(
                WebClient.builder().build(),
                mock(LocalCacheService.class),
                new ObjectMapper(),
                properties
        );

        assertThat(service.isAvailable()).isTrue();
    }

    @Test
    void unavailableWhenDisabled() {
        AppProperties properties = new AppProperties();
        AppProperties.YoutubeApiConfig youtube = new AppProperties.YoutubeApiConfig();
        youtube.setEnabled(false);
        youtube.setApiKey("key");
        youtube.setYtDlpPath("java");
        properties.setYoutube(youtube);

        YoutubeMusicApiService service = createService(properties);

        assertThat(service.isAvailable()).isFalse();
    }

    @Test
    void unavailableWhenApiKeyMissing() {
        AppProperties properties = new AppProperties();
        AppProperties.YoutubeApiConfig youtube = new AppProperties.YoutubeApiConfig();
        youtube.setEnabled(true);
        youtube.setApiKey("");
        youtube.setYtDlpPath("java");
        properties.setYoutube(youtube);

        YoutubeMusicApiService service = createService(properties);

        assertThat(service.isAvailable()).isFalse();
    }

    @Test
    void unavailableWhenYtDlpMissing() {
        AppProperties properties = new AppProperties();
        AppProperties.YoutubeApiConfig youtube = new AppProperties.YoutubeApiConfig();
        youtube.setEnabled(true);
        youtube.setApiKey("key");
        youtube.setYtDlpPath("definitely-not-a-real-yt-dlp-binary");
        properties.setYoutube(youtube);

        YoutubeMusicApiService service = createService(properties);

        assertThat(service.isAvailable()).isFalse();
    }

    @Test
    void usesPlatformPrefixedCacheKey() {
        AppProperties properties = new AppProperties();
        AppProperties.YoutubeApiConfig youtube = new AppProperties.YoutubeApiConfig();
        youtube.setEnabled(false);
        properties.setYoutube(youtube);

        YoutubeMusicApiService service = createService(properties);

        assertThat(service.cacheKey("abc123")).isEqualTo("youtube-abc123");
    }

    private YoutubeMusicApiService createService(AppProperties properties) {
        return new YoutubeMusicApiService(
                WebClient.builder().build(),
                mock(LocalCacheService.class),
                new ObjectMapper(),
                properties
        );
    }
}
