package org.thornex.musicparty.service.api;

import com.fasterxml.jackson.databind.JsonNode;
import lombok.extern.slf4j.Slf4j;
import org.springframework.util.DigestUtils;
import org.springframework.util.StringUtils;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import org.springframework.web.util.UriComponentsBuilder;
import org.springframework.web.util.UriUtils;
import org.thornex.musicparty.service.SubsonicSource;
import reactor.core.publisher.Mono;

import java.net.URI;
import java.nio.charset.StandardCharsets;
import java.time.Duration;
import java.util.Map;
import java.util.UUID;
import java.util.concurrent.TimeoutException;

@Slf4j
public class SubsonicClient {
    private static final Duration REQUEST_TIMEOUT = Duration.ofSeconds(10);

    private final WebClient webClient;
    private final SubsonicSource source;

    public SubsonicClient(WebClient webClient, SubsonicSource source) {
        this.webClient = webClient;
        this.source = source;
    }

    public SubsonicSource source() {
        return source;
    }

    public Mono<JsonNode> getJson(String endpoint, Map<String, ?> params) {
        URI uri = buildUri(endpoint, params);
        return webClient.get()
                .uri(uri)
                .retrieve()
                .bodyToMono(JsonNode.class)
                .timeout(REQUEST_TIMEOUT)
                .map(this::requireOkResponse)
                .onErrorMap(this::mapError);
    }

    public URI buildStreamUri(String songId) {
        return buildUri("stream.view", Map.of("id", songId, "format", "mp3"));
    }

    public URI buildCoverUri(String coverArtId, int size) {
        return buildUri("getCoverArt.view", Map.of("id", coverArtId, "size", size));
    }

    public String buildLocalProxyPath(String pathPrefix, String id) {
        return pathPrefix.replaceAll("/+$", "") + "/" + UriUtils.encodePathSegment(id, StandardCharsets.UTF_8);
    }

    public boolean isConfigured() {
        return source.isConfigured();
    }

    public Mono<Void> ping() {
        return getJson("ping.view", Map.of()).then();
    }

    public URI buildUri(String endpoint, Map<String, ?> params) {
        validateConfig(source);

        String salt = UUID.randomUUID().toString().replace("-", "").substring(0, 8);
        String token = DigestUtils.md5DigestAsHex((source.password() + salt).getBytes(StandardCharsets.UTF_8));

        UriComponentsBuilder builder = UriComponentsBuilder
                .fromHttpUrl(source.baseUrl())
                .pathSegment("rest", endpoint)
                .queryParam("u", source.username())
                .queryParam("s", salt)
                .queryParam("t", token)
                .queryParam("v", source.apiVersion())
                .queryParam("c", source.client())
                .queryParam("f", "json");

        if (params != null) {
            params.forEach((key, value) -> builder.queryParam(key, value));
        }

        return builder.build().encode().toUri();
    }

    private void validateConfig(SubsonicSource source) {
        if (source == null || !source.isConfigured()) {
            throw new IllegalStateException("Subsonic source is missing base URL, username, or password");
        }
    }

    private RuntimeException mapError(Throwable error) {
        if (error instanceof TimeoutException) {
            return new IllegalStateException("Subsonic request timed out for source " + source.id(), error);
        }
        if (error instanceof WebClientResponseException responseException) {
            return new IllegalStateException("Subsonic HTTP " + responseException.getStatusCode().value()
                    + " for source " + source.id(), error);
        }
        if (error instanceof RuntimeException runtimeException) {
            return runtimeException;
        }
        return new IllegalStateException("Subsonic request failed for source " + source.id(), error);
    }

    JsonNode requireOkResponse(JsonNode node) {
        if (node == null || !node.has("subsonic-response") || !node.get("subsonic-response").isObject()) {
            throw new IllegalStateException("Malformed Subsonic response: missing subsonic-response");
        }

        JsonNode response = node.get("subsonic-response");
        if (!response.hasNonNull("status")) {
            throw new IllegalStateException("Malformed Subsonic response: missing status");
        }

        String status = response.path("status").asText();
        if (!"ok".equalsIgnoreCase(status)) {
            JsonNode error = response.path("error");
            String code = error.path("code").asText("unknown");
            String message = error.path("message").asText("Subsonic request failed");
            throw new IllegalStateException("Subsonic error " + code + ": " + message);
        }
        return node;
    }

    public static SubsonicSource sourceFromLegacyNavidrome(String baseUrl,
                                                          String username,
                                                          String password,
                                                          String client,
                                                          String apiVersion,
                                                          String allowedUsers,
                                                          boolean enabled) {
        long now = System.currentTimeMillis();
        return new SubsonicSource(
                "navidrome",
                null,
                "Navidrome",
                baseUrl,
                username,
                password,
                StringUtils.hasText(client) ? client : "musicparty",
                StringUtils.hasText(apiVersion) ? apiVersion : "1.16.1",
                allowedUsers,
                enabled,
                true,
                now,
                now
        );
    }
}
