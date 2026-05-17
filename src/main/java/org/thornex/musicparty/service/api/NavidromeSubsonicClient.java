package org.thornex.musicparty.service.api;

import com.fasterxml.jackson.databind.JsonNode;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Service;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.service.SubsonicSource;
import reactor.core.publisher.Mono;

import java.net.URI;
import java.util.Map;

@Service
@ConditionalOnProperty(prefix = "app.music-api.navidrome", name = "enabled", havingValue = "true")
public class NavidromeSubsonicClient {
    private final AppProperties appProperties;
    private final WebClient webClient;

    public NavidromeSubsonicClient(WebClient webClient, AppProperties appProperties) {
        this.webClient = webClient;
        this.appProperties = appProperties;
    }

    public Mono<JsonNode> getJson(String endpoint, Map<String, ?> params) {
        return delegate().getJson(endpoint, params);
    }

    public URI buildStreamUri(String songId) {
        return delegate().buildStreamUri(songId);
    }

    public URI buildCoverUri(String coverArtId, int size) {
        return delegate().buildCoverUri(coverArtId, size);
    }

    public String buildLocalProxyPath(String pathPrefix, String id) {
        return delegate().buildLocalProxyPath(pathPrefix, id);
    }

    public boolean isConfigured() {
        return source().isConfigured();
    }

    public Mono<Void> ping() {
        return delegate().ping();
    }

    JsonNode requireOkResponse(JsonNode node) {
        return delegate().requireOkResponse(node);
    }

    private SubsonicClient delegate() {
        return new SubsonicClient(webClient, source());
    }

    private SubsonicSource source() {
        AppProperties.NavidromeApiConfig config = appProperties.getNavidrome();
        return SubsonicClient.sourceFromLegacyNavidrome(
                config.getBaseUrl(),
                config.getUsername(),
                config.getPassword(),
                config.getClient(),
                config.getApiVersion(),
                config.getAllowedUsers(),
                config.isEnabled()
        );
    }
}
