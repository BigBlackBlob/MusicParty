package org.thornex.musicparty.service.api;

import lombok.extern.slf4j.Slf4j;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.PlayableMusic;
import org.thornex.musicparty.dto.Playlist;
import org.thornex.musicparty.dto.LyricResponse;
import org.thornex.musicparty.dto.UserSearchResult;
import org.thornex.musicparty.exception.ApiRequestException;
import reactor.core.publisher.Mono;
import com.fasterxml.jackson.databind.JsonNode;

import java.util.*;

@Service
@ConditionalOnProperty(prefix = "app.music-api.navidrome", name = "enabled", havingValue = "true")
@Slf4j
public class NavidromeMusicApiService implements IMusicApiService {

    private final AppProperties appProperties;
    private final NavidromeSubsonicClient subsonicClient;
    private static final String PLATFORM = "navidrome";

    public NavidromeMusicApiService(AppProperties appProperties, NavidromeSubsonicClient subsonicClient) {
        this.appProperties = appProperties;
        this.subsonicClient = subsonicClient;
        log.info("NavidromeMusicApiService initialized with base-url: {}", appProperties.getNavidrome().getBaseUrl());
    }

    @Override
    public String getPlatformName() {
        return PLATFORM;
    }

    @Override
    public Mono<List<Music>> searchMusic(String keyword) {
        Map<String, String> params = new HashMap<>();
        params.put("query", keyword);
        params.put("songCount", "50");
        params.put("artistCount", "0");
        params.put("albumCount", "0");

        log.debug("Navidrome search request");

        return subsonicClient.getJson("search3.view", params)
                .map(node -> {
                    JsonNode searchResult = node.path("subsonic-response").path("searchResult3");
                    JsonNode songs = searchResult.path("song");
                    if (songs.isMissingNode() || !songs.isArray()) return List.<Music>of();

                    List<Music> result = new ArrayList<>();
                    for (JsonNode song : songs) {
                        String id = song.path("id").asText("");
                        String title = song.path("title").asText("");
                        if (!StringUtils.hasText(id) || !StringUtils.hasText(title)) {
                            continue;
                        }
                        List<String> artists = List.of(song.path("artist").asText("Unknown"));
                        long durationMs = song.path("duration").asLong(0) * 1000;
                        String coverArtId = song.path("coverArt").asText("");
                        String coverUrl = StringUtils.hasText(coverArtId)
                                ? subsonicClient.buildLocalProxyPath("/api/navidrome/cover", coverArtId) : "";

                        result.add(new Music(id, title, artists, durationMs, PLATFORM, coverUrl));
                    }
                    return result;
                })
                .onErrorResume(e -> {
                    log.warn("Navidrome search failed: {}", e.getMessage());
                    return Mono.error(new ApiRequestException("Navidrome search failed"));
                });
    }

    @Override
    public Mono<PlayableMusic> getPlayableMusic(String musicId) {
        Map<String, String> params = new HashMap<>();
        params.put("id", musicId);

        return subsonicClient.getJson("getSong.view", params)
                .map(node -> {
                    JsonNode song = node.path("subsonic-response").path("song");
                    String id = song.path("id").asText(musicId);
                    String title = song.path("title").asText("");
                    if (!StringUtils.hasText(id) || !StringUtils.hasText(title) || song.isMissingNode()) {
                        throw new ApiRequestException("Navidrome song not found: " + musicId);
                    }
                    List<String> artists = List.of(song.path("artist").asText("Unknown"));
                    long durationMs = song.path("duration").asLong(0) * 1000;
                    String coverArtId = song.path("coverArt").asText("");
                    String coverUrl = StringUtils.hasText(coverArtId)
                            ? subsonicClient.buildLocalProxyPath("/api/navidrome/cover", coverArtId) : "";
                    String streamUrl = subsonicClient.buildLocalProxyPath("/api/navidrome/stream", id);

                    return new PlayableMusic(id, title, artists, durationMs, PLATFORM, streamUrl, coverUrl, false);
                })
                .onErrorResume(e -> {
                    log.error("Navidrome getPlayableMusic failed for id: {}", musicId, e);
                    return Mono.error(new ApiRequestException("Failed to get Navidrome song: " + musicId));
                });
    }

    @Override
    public Mono<List<Playlist>> getUserPlaylists(String userId) {
        return Mono.just(List.of());
    }

    @Override
    public Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) {
        return Mono.just(List.of());
    }

    @Override
    public Mono<List<UserSearchResult>> searchUsers(String keyword) {
        return Mono.just(List.of());
    }

    @Override
    public Mono<String> getLyric(String musicId) {
        return Mono.just("");
    }

    @Override
    public Mono<LyricResponse> getLyricDetail(String musicId) {
        return Mono.just(new LyricResponse("", "", ""));
    }

    @Override
    public void prefetchMusic(String musicId) {
    }
}
