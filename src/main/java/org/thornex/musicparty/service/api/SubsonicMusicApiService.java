package org.thornex.musicparty.service.api;

import com.fasterxml.jackson.databind.JsonNode;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.thornex.musicparty.dto.*;
import org.thornex.musicparty.exception.ApiRequestException;
import org.thornex.musicparty.service.RoomSubsonicSource;
import org.thornex.musicparty.service.SubsonicSource;
import org.thornex.musicparty.service.SubsonicSourceRegistry;
import reactor.core.publisher.Mono;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;

@Service
@RequiredArgsConstructor
@Slf4j
public class SubsonicMusicApiService {
    private final SubsonicSourceRegistry registry;

    public boolean supports(String platform) {
        return sourceIdFromPlatform(platform) != null && registry.find(sourceIdFromPlatform(platform)).isPresent();
    }

    public Mono<List<Music>> searchMusic(String roomId, String platform, String keyword, int offset, int limit) {
        RoomSubsonicSource roomSource = requireRoomSource(roomId, platform);
        SubsonicSource source = roomSource.source();
        int safeLimit = Math.max(1, Math.min(limit, 100));
        int safeOffset = Math.max(0, offset);
        return registry.client(source).getJson("search3.view", Map.of(
                        "query", keyword,
                        "songCount", safeLimit,
                        "songOffset", safeOffset,
                        "artistCount", 0,
                        "albumCount", 0
                ))
                .map(node -> mapSongs(roomSource, node.path("subsonic-response").path("searchResult3").path("song")))
                .onErrorResume(e -> {
                    log.warn("Subsonic search failed: source={}, message={}", source.id(), e.getMessage());
                    return Mono.error(new ApiRequestException("Subsonic search failed: " + source.label()));
                });
    }

    public Mono<PlayableMusic> getPlayableMusic(String platform, String musicId) {
        RoomSubsonicSource roomSource = requireRoomSource(roomIdFromPlatform(platform), platform);
        SubsonicSource source = roomSource.source();
        return registry.client(source).getJson("getSong.view", Map.of("id", musicId))
                .map(node -> {
                    JsonNode song = node.path("subsonic-response").path("song");
                    String id = song.path("id").asText(musicId);
                    String title = song.path("title").asText("");
                    if (!StringUtils.hasText(id) || !StringUtils.hasText(title) || song.isMissingNode()) {
                        throw new ApiRequestException("Subsonic song not found: " + musicId);
                    }
                    String coverArtId = song.path("coverArt").asText("");
                    String coverUrl = StringUtils.hasText(coverArtId)
                            ? registry.client(source).buildLocalProxyPath(proxyPrefix(roomSource, "cover"), coverArtId)
                            : "";
                    String streamUrl = registry.client(source).buildLocalProxyPath(proxyPrefix(roomSource, "stream"), id);
                    return new PlayableMusic(
                            id,
                            title,
                            List.of(song.path("artist").asText("Unknown")),
                            song.path("duration").asLong(0) * 1000,
                            roomPlatformId(roomSource),
                            streamUrl,
                            coverUrl,
                            false
                    );
                })
                .onErrorResume(e -> {
                    log.warn("Subsonic getPlayableMusic failed: source={}, id={}, message={}", source.id(), musicId, e.getMessage());
                    return Mono.error(new ApiRequestException("Failed to get Subsonic song: " + musicId));
                });
    }

    public Mono<List<Album>> searchAlbums(String roomId, String platform, String keyword) {
        RoomSubsonicSource roomSource = requireRoomSource(roomId, platform);
        SubsonicSource source = roomSource.source();
        return registry.client(source).getJson("search3.view", Map.of(
                        "query", keyword,
                        "songCount", 0,
                        "artistCount", 0,
                        "albumCount", 50
                ))
                .map(node -> {
                    JsonNode albums = node.path("subsonic-response").path("searchResult3").path("album");
                    if (albums.isMissingNode() || !albums.isArray()) return List.<Album>of();
                    List<Album> result = new ArrayList<>();
                    for (JsonNode album : albums) {
                        String id = album.path("id").asText("");
                        String name = album.path("name").asText("");
                        if (!StringUtils.hasText(id) || !StringUtils.hasText(name)) continue;
                        String coverArtId = album.path("coverArt").asText("");
                        String coverUrl = StringUtils.hasText(coverArtId)
                                ? registry.client(source).buildLocalProxyPath(proxyPrefix(roomSource, "cover"), coverArtId)
                                : "";
                        result.add(new Album(id, name, album.path("artist").asText("Unknown"), coverUrl, album.path("songCount").asInt(0), roomPlatformId(roomSource)));
                    }
                    return result;
                })
                .onErrorResume(e -> {
                    log.warn("Subsonic album search failed: source={}, message={}", source.id(), e.getMessage());
                    return Mono.error(new ApiRequestException("Subsonic album search failed: " + source.label()));
                });
    }

    public Mono<List<Music>> getAlbumMusics(String roomId, String platform, String albumId) {
        RoomSubsonicSource roomSource = requireRoomSource(roomId, platform);
        SubsonicSource source = roomSource.source();
        return registry.client(source).getJson("getAlbum.view", Map.of("id", albumId))
                .map(node -> mapSongs(roomSource, node.path("subsonic-response").path("album").path("song")))
                .onErrorResume(e -> {
                    log.warn("Subsonic album tracks failed: source={}, album={}, message={}", source.id(), albumId, e.getMessage());
                    return Mono.error(new ApiRequestException("Subsonic album tracks failed: " + source.label()));
                });
    }

    private List<Music> mapSongs(RoomSubsonicSource roomSource, JsonNode songs) {
        if (songs.isMissingNode() || !songs.isArray()) return List.of();
        SubsonicSource source = roomSource.source();
        List<Music> result = new ArrayList<>();
        for (JsonNode song : songs) {
            String id = song.path("id").asText("");
            String title = song.path("title").asText("");
            if (!StringUtils.hasText(id) || !StringUtils.hasText(title)) continue;
            String coverArtId = song.path("coverArt").asText("");
            String coverUrl = StringUtils.hasText(coverArtId)
                    ? registry.client(source).buildLocalProxyPath(proxyPrefix(roomSource, "cover"), coverArtId)
                    : "";
            result.add(new Music(
                    id,
                    title,
                    List.of(song.path("artist").asText("Unknown")),
                    song.path("duration").asLong(0) * 1000,
                    roomPlatformId(roomSource),
                    coverUrl
            ));
        }
        return result;
    }

    private RoomSubsonicSource requireRoomSource(String roomId, String platform) {
        String normalizedRoom = roomIdFromPlatform(platform);
        if (!StringUtils.hasText(normalizedRoom)) {
            normalizedRoom = SubsonicSourceRegistry.normalizeRoomId(roomId);
        }
        return registry.findRoomSourceByPlatformId(normalizedRoom, sourcePlatformId(platform))
                .filter(RoomSubsonicSource::active)
                .orElseThrow(() -> new ApiRequestException("Unsupported Subsonic source: " + platform));
    }

    private String proxyPrefix(RoomSubsonicSource roomSource, String kind) {
        return "/api/subsonic/" + roomSource.roomId() + "/" + roomSource.id() + "/" + kind;
    }

    private String roomPlatformId(RoomSubsonicSource roomSource) {
        return roomSource.platformId() + "@" + roomSource.roomId();
    }

    private String sourcePlatformId(String platform) {
        String sourceId = sourceIdFromPlatform(platform);
        return sourceId == null ? platform : "subsonic-" + sourceId;
    }

    private String sourceIdFromPlatform(String platform) {
        if (!StringUtils.hasText(platform) || !platform.startsWith("subsonic-")) return null;
        String value = platform.substring("subsonic-".length());
        int roomMarker = value.indexOf('@');
        return roomMarker >= 0 ? value.substring(0, roomMarker) : value;
    }

    private String roomIdFromPlatform(String platform) {
        if (!StringUtils.hasText(platform)) return null;
        int roomMarker = platform.indexOf('@');
        return roomMarker >= 0 ? platform.substring(roomMarker + 1) : null;
    }
}
