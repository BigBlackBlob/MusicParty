package org.thornex.musicparty.service.api;

import com.fasterxml.jackson.databind.JsonNode;
import lombok.extern.slf4j.Slf4j;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Service;
import org.springframework.util.DigestUtils;
import org.springframework.util.StringUtils;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.util.UriUtils;
import org.springframework.web.util.UriComponentsBuilder;
import org.thornex.musicparty.config.AppProperties;
import reactor.core.publisher.Mono;

import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.util.Map;
import java.util.UUID;

@Service
@ConditionalOnProperty(prefix = "app.music-api.navidrome", name = "enabled", havingValue = "true")
@Slf4j
public class NavidromeSubsonicClient {

    private final WebClient webClient;
    private final AppProperties appProperties;

    public NavidromeSubsonicClient(WebClient webClient, AppProperties appProperties) {
        this.webClient = webClient;
        this.appProperties = appProperties;
    }

    public Mono<JsonNode> getJson(String endpoint, Map<String, ?> params) {
        URI uri = buildUri(endpoint, params);
        return webClient.get()
                .uri(uri)
                .retrieve()
                .bodyToMono(JsonNode.class)
                .map(this::requireOkResponse);
    }

    public URI buildStreamUri(String songId) {
        return buildUri("stream.view", Map.of("id", songId, "format", "raw"));
    }

    public URI buildCoverUri(String coverArtId, int size) {
        return buildUri("getCoverArt.view", Map.of("id", coverArtId, "size", size));
    }

    public String buildLocalProxyPath(String pathPrefix, String id) {
        return pathPrefix.replaceAll("/+$", "") + "/" + UriUtils.encodePathSegment(id, StandardCharsets.UTF_8);
    }

    public boolean isConfigured() {
        return isConfigComplete(appProperties.getNavidrome());
    }

    public Mono<Void> ping() {
        return getJson("ping.view", Map.of()).then();
    }

    private URI buildUri(String endpoint, Map<String, ?> params) {
        AppProperties.NavidromeApiConfig config = appProperties.getNavidrome();
        validateConfig(config);

        String salt = UUID.randomUUID().toString().replace("-", "").substring(0, 8);
        String token = DigestUtils.md5DigestAsHex((config.getPassword() + salt).getBytes(StandardCharsets.UTF_8));

        UriComponentsBuilder builder = UriComponentsBuilder
                .fromHttpUrl(config.getBaseUrl())
                .pathSegment("rest", endpoint)
                .queryParam("u", config.getUsername())
                .queryParam("s", salt)
                .queryParam("t", token)
                .queryParam("v", config.getApiVersion())
                .queryParam("c", config.getClient())
                .queryParam("f", "json");

        if (params != null) {
            params.forEach((key, value) -> builder.queryParam(key, value));
        }

        return builder.build().encode().toUri();
    }

    private void validateConfig(AppProperties.NavidromeApiConfig config) {
        if (!isConfigComplete(config)) {
            throw new IllegalStateException("Navidrome is enabled but base-url, username, or password is missing");
        }
    }

    private boolean isConfigComplete(AppProperties.NavidromeApiConfig config) {
        return config != null
                && StringUtils.hasText(config.getBaseUrl())
                && StringUtils.hasText(config.getUsername())
                && StringUtils.hasText(config.getPassword());
    }

    JsonNode requireOkResponse(JsonNode node) {
        if (node == null || !node.has("subsonic-response") || !node.get("subsonic-response").isObject()) {
            throw new IllegalStateException("Malformed Navidrome response: missing subsonic-response");
        }

        JsonNode response = node.get("subsonic-response");
        if (!response.hasNonNull("status")) {
            throw new IllegalStateException("Malformed Navidrome response: missing status");
        }

        String status = response.path("status").asText();
        if (!"ok".equalsIgnoreCase(status)) {
            JsonNode error = response.path("error");
            String code = error.path("code").asText("unknown");
            String message = error.path("message").asText("Navidrome request failed");
            throw new IllegalStateException("Navidrome error " + code + ": " + message);
        }
        return node;
    }
}
