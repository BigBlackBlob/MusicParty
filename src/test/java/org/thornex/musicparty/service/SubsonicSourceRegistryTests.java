package org.thornex.musicparty.service;

import org.junit.jupiter.api.Test;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.SubsonicSourceRequest;
import org.thornex.musicparty.persistence.InMemorySubsonicSourceRepository;

import static org.assertj.core.api.Assertions.assertThat;

class SubsonicSourceRegistryTests {

    @Test
    void findsRoomSourceFromRoomQualifiedPlatformId() {
        AppProperties properties = new AppProperties();
        SubsonicSourceRegistry registry = new SubsonicSourceRegistry(
                new InMemorySubsonicSourceRepository(),
                properties,
                WebClient.builder().build(),
                new SubsonicCredentialCipher(properties)
        );
        registry.init();
        registry.upsert("room-a", new SubsonicSourceRequest(
                "squidify",
                "Squidify",
                "https://squidify.org:443",
                "user",
                "pass",
                "*",
                true
        ));

        assertThat(registry.findRoomSourceByPlatformId("lounge", "subsonic-squidify@room-a"))
                .isPresent()
                .get()
                .extracting(RoomSubsonicSource::roomId)
                .isEqualTo("room-a");
    }

    @Test
    void removingSystemSourceDisablesRoomBindingInsteadOfDeletingIt() {
        AppProperties properties = new AppProperties();
        properties.getSquidify().setEnabled(true);
        properties.getSquidify().setBaseUrl("https://squidify.org:443");
        properties.getSquidify().setUsername("user");
        properties.getSquidify().setPassword("pass");
        SubsonicSourceRegistry registry = new SubsonicSourceRegistry(
                new InMemorySubsonicSourceRepository(),
                properties,
                WebClient.builder().build(),
                new SubsonicCredentialCipher(properties)
        );
        registry.init();

        assertThat(registry.remove("lounge", "squidify")).isTrue();

        assertThat(registry.findRoomSource("lounge", "squidify"))
                .isPresent()
                .get()
                .extracting(RoomSubsonicSource::enabled)
                .isEqualTo(false);
        assertThat(registry.listEnabledConfigured("lounge"))
                .noneMatch(source -> source.id().equals("squidify"));
    }
}
