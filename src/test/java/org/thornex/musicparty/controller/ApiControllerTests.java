package org.thornex.musicparty.controller;

import org.junit.jupiter.api.Test;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.MusicPlatform;
import org.thornex.musicparty.persistence.InMemorySubsonicSourceRepository;
import org.thornex.musicparty.service.NavidromeAccessService;
import org.thornex.musicparty.service.SubsonicCredentialCipher;
import org.thornex.musicparty.service.SubsonicSourceRegistry;
import org.springframework.web.reactive.function.client.WebClient;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class ApiControllerTests {

    @Test
    void platformsIncludesNavidromeWithoutTokenWhenWildcardAccessIsConfigured() {
        AppProperties properties = new AppProperties();
        properties.getNavidrome().setEnabled(true);
        properties.getNavidrome().setBaseUrl("http://127.0.0.1:4533");
        properties.getNavidrome().setUsername("admin");
        properties.getNavidrome().setPassword("secret");
        properties.getNavidrome().setAllowedUsers("*");
        ApiController controller = new ApiController(
                List.of(),
                properties,
                null,
                new NavidromeAccessService(properties, null),
                newRegistry(properties),
                null
        );

        List<MusicPlatform> platforms = controller.getPlatforms(null, "lounge");

        assertThat(platforms).extracting(MusicPlatform::id).contains("navidrome");
    }

    @Test
    void platformsIncludesSeededSquidifyWhenConfigured() {
        AppProperties properties = new AppProperties();
        properties.getSquidify().setEnabled(true);
        properties.getSquidify().setBaseUrl("https://squidify.example");
        properties.getSquidify().setUsername("guest");
        properties.getSquidify().setPassword("guest");
        ApiController controller = new ApiController(
                List.of(),
                properties,
                null,
                new NavidromeAccessService(properties, null),
                newRegistry(properties),
                null
        );

        List<MusicPlatform> platforms = controller.getPlatforms(null, "lounge");

        assertThat(platforms).anySatisfy(platform -> {
            assertThat(platform.id()).isEqualTo("subsonic-squidify");
            assertThat(platform.subsonic()).isTrue();
        });
    }

    private SubsonicSourceRegistry newRegistry(AppProperties properties) {
        SubsonicSourceRegistry registry = new SubsonicSourceRegistry(
                new InMemorySubsonicSourceRepository(),
                properties,
                WebClient.builder().build(),
                new SubsonicCredentialCipher(properties)
        );
        registry.init();
        return registry;
    }
}
