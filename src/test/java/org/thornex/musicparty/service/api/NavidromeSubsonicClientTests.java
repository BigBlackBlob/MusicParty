package org.thornex.musicparty.service.api;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

class NavidromeSubsonicClientTests {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Test
    void requireOkResponseRejectsMissingSubsonicResponse() throws Exception {
        NavidromeSubsonicClient client = newClient();

        assertThatThrownBy(() -> client.requireOkResponse(MAPPER.readTree("{\"status\":\"ok\"}")))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("missing subsonic-response");
    }

    @Test
    void requireOkResponseRejectsMissingStatus() throws Exception {
        NavidromeSubsonicClient client = newClient();

        assertThatThrownBy(() -> client.requireOkResponse(MAPPER.readTree("{\"subsonic-response\":{}}")))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("missing status");
    }

    @Test
    void requireOkResponseRejectsSubsonicError() throws Exception {
        NavidromeSubsonicClient client = newClient();

        assertThatThrownBy(() -> client.requireOkResponse(MAPPER.readTree("""
                {"subsonic-response":{"status":"failed","error":{"code":40,"message":"Wrong username or password"}}}
                """)))
                .isInstanceOf(IllegalStateException.class)
                .hasMessageContaining("Navidrome error 40: Wrong username or password");
    }

    @Test
    void buildLocalProxyPathEncodesPathSegment() {
        NavidromeSubsonicClient client = newClient();

        assertThat(client.buildLocalProxyPath("/api/navidrome/stream", "a/b c"))
                .isEqualTo("/api/navidrome/stream/a%2Fb%20c");
    }

    @Test
    void isConfiguredRequiresBaseUrlUsernameAndPassword() {
        AppProperties properties = new AppProperties();
        properties.setNavidrome(new AppProperties.NavidromeApiConfig());
        NavidromeSubsonicClient client = new NavidromeSubsonicClient(WebClient.builder().build(), properties);

        assertThat(client.isConfigured()).isFalse();

        properties.getNavidrome().setBaseUrl("http://127.0.0.1:4533");
        properties.getNavidrome().setUsername("admin");
        properties.getNavidrome().setPassword("secret");

        assertThat(client.isConfigured()).isTrue();
    }

    private NavidromeSubsonicClient newClient() {
        AppProperties properties = new AppProperties();
        AppProperties.NavidromeApiConfig config = new AppProperties.NavidromeApiConfig();
        config.setBaseUrl("http://127.0.0.1:4533");
        config.setUsername("admin");
        config.setPassword("secret");
        properties.setNavidrome(config);
        return new NavidromeSubsonicClient(WebClient.builder().build(), properties);
    }
}
