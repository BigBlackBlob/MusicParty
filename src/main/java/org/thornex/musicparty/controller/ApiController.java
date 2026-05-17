package org.thornex.musicparty.controller;

import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatus;
import org.springframework.web.bind.annotation.*;
import org.springframework.web.server.ResponseStatusException;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.CoverColorResponse;
import org.thornex.musicparty.dto.Album;
import org.thornex.musicparty.dto.LyricResponse;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.MusicPlatform;
import org.thornex.musicparty.dto.Playlist;
import org.thornex.musicparty.dto.UserSearchResult;
import org.thornex.musicparty.exception.ApiRequestException;
import org.thornex.musicparty.service.CoverColorService;
import org.thornex.musicparty.service.NavidromeAccessService;
import org.thornex.musicparty.service.RoomSubsonicSource;
import org.thornex.musicparty.service.SubsonicSourceRegistry;
import org.thornex.musicparty.service.api.IMusicApiService;
import org.thornex.musicparty.service.api.SubsonicMusicApiService;
import reactor.core.publisher.Mono;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;

@RestController
@RequestMapping("/api")
@Slf4j
public class ApiController {

    private final Map<String, IMusicApiService> apiServiceMap;
    private final AppProperties appProperties;
    private final CoverColorService coverColorService;
    private final NavidromeAccessService navidromeAccessService;
    private final SubsonicSourceRegistry subsonicSourceRegistry;
    private final SubsonicMusicApiService subsonicMusicApiService;

    public ApiController(List<IMusicApiService> apiServices,
                         AppProperties appProperties,
                         CoverColorService coverColorService,
                         NavidromeAccessService navidromeAccessService,
                         SubsonicSourceRegistry subsonicSourceRegistry,
                         SubsonicMusicApiService subsonicMusicApiService) {
        this.apiServiceMap = apiServices.stream()
                .collect(Collectors.toMap(IMusicApiService::getPlatformName, Function.identity()));
        this.appProperties = appProperties;
        this.coverColorService = coverColorService;
        this.navidromeAccessService = navidromeAccessService;
        this.subsonicSourceRegistry = subsonicSourceRegistry;
        this.subsonicMusicApiService = subsonicMusicApiService;
    }

    @GetMapping("/config")
    public Map<String, String> getConfig() {
        return Map.of(
                "authorName", appProperties.getAuthorName(),
                "backWords", appProperties.getBackWords()
        );
    }

    @GetMapping("/platforms")
    public List<MusicPlatform> getPlatforms(@RequestParam(required = false) String token,
                                            @RequestParam(required = false) String roomId) {
        String normalizedRoom = SubsonicSourceRegistry.normalizeRoomId(roomId);
        List<MusicPlatform> platforms = new ArrayList<>();
        platforms.add(new MusicPlatform("netease", "netease", true));
        platforms.add(new MusicPlatform("bilibili", "bilibili", false));

        boolean canShowNavidrome = navidromeAccessService.isEnabled()
                && navidromeAccessService.isConfigured()
                && (navidromeAccessService.allowsAllNamedUsers()
                || (token != null && navidromeAccessService.canUseBySessionToken(token)));
        if (canShowNavidrome) {
            platforms.add(new MusicPlatform("navidrome", "navidrome", true));
        }
        subsonicSourceRegistry.listEnabledConfigured(normalizedRoom).stream()
                .filter(source -> !"navidrome".equals(source.id()))
                .filter(source -> canUseSubsonicSource(source, token))
                .map(source -> new MusicPlatform(source.platformId(), source.label(), true, true))
                .forEach(platforms::add);

        return platforms;
    }

    private IMusicApiService getService(String platform) {
        IMusicApiService service = apiServiceMap.get(platform);
        if (service == null) {
            throw new ApiRequestException("Platform not supported: " + platform);
        }
        return service;
    }

    private boolean isSubsonicPlatform(String platform) {
        return subsonicMusicApiService.supports(platform);
    }

    private boolean canUseSubsonicSource(RoomSubsonicSource source, String token) {
        if (source.allowedUsers() != null && source.allowedUsers().contains("*")) return true;
        return token != null && navidromeAccessService.canUseBySessionToken(token, source.allowedUsers());
    }

    @GetMapping("/search/{platform}/{keyword}")
    public Mono<List<Music>> searchMusic(@PathVariable String platform, @PathVariable String keyword,
                                          @RequestParam(required = false) String token,
                                          @RequestParam(required = false) String roomId,
                                          @RequestParam(defaultValue = "0") int offset,
                                          @RequestParam(defaultValue = "20") int limit) {
        if ("navidrome".equals(platform)) {
            if (token == null || !navidromeAccessService.canUseBySessionToken(token)) {
                return Mono.error(new ResponseStatusException(HttpStatus.FORBIDDEN));
            }
        }
        String normalizedRoom = SubsonicSourceRegistry.normalizeRoomId(roomId);
        if (isSubsonicPlatform(platform) && !canUseSubsonicSource(subsonicSourceRegistry.findRoomSourceByPlatformId(normalizedRoom, platform).orElseThrow(), token)) {
            return Mono.error(new ResponseStatusException(HttpStatus.FORBIDDEN));
        }

        log.info("API search request: platform={}, keywordLength={}, offset={}, limit={}", 
                platform, keyword == null ? 0 : keyword.length(), offset, limit);
        if (isSubsonicPlatform(platform)) {
            return subsonicMusicApiService.searchMusic(normalizedRoom, platform, keyword, offset, limit)
                    .doOnSuccess(result -> log.info("API search success: platform={}, resultCount={}", platform, result == null ? 0 : result.size()))
                    .doOnError(error -> log.error("API search failed: platform={}, keyword={}", platform, keyword, error));
        }
        return getService(platform).searchMusic(keyword, offset, limit)
                .doOnSuccess(result -> log.info("API search success: platform={}, resultCount={}", platform, result == null ? 0 : result.size()))
                .doOnError(error -> log.error("API search failed: platform={}, keyword={}", platform, keyword, error));
    }

    @GetMapping("/user/playlists/{platform}/{userId}")
    public Mono<List<Playlist>> getUserPlaylists(@PathVariable String platform, @PathVariable String userId) {
        log.info("API playlist request: platform={}, userId={}", platform, userId);
        return getService(platform).getUserPlaylists(userId)
                .doOnSuccess(result -> log.info("API playlist success: platform={}, playlistCount={}", platform, result == null ? 0 : result.size()))
                .doOnError(error -> log.error("API playlist failed: platform={}, userId={}", platform, userId, error));
    }

    @GetMapping("/playlist/songs/{platform}/{playlistId}")
    public Mono<List<Music>> getPlaylistSongs(@PathVariable String platform,
                                              @PathVariable String playlistId,
                                              @RequestParam(defaultValue = "0") int offset,
                                              @RequestParam(defaultValue = "20") int limit) {
        log.info("API playlist songs request: platform={}, playlistId={}, offset={}, limit={}", platform, playlistId, offset, limit);
        return getService(platform).getPlaylistMusics(playlistId, offset, limit)
                .doOnSuccess(result -> log.info("API playlist songs success: platform={}, playlistId={}, resultCount={}", platform, playlistId, result == null ? 0 : result.size()))
                .doOnError(error -> log.error("API playlist songs failed: platform={}, playlistId={}, offset={}, limit={}", platform, playlistId, offset, limit, error));
    }

    @GetMapping("/album/search/{platform}")
    public Mono<List<Album>> searchAlbums(@PathVariable String platform,
                                          @RequestParam String keyword,
                                          @RequestParam(required = false) String token,
                                          @RequestParam(required = false) String roomId) {
        if ("navidrome".equals(platform) && (token == null || !navidromeAccessService.canUseBySessionToken(token))) {
            return Mono.error(new ResponseStatusException(HttpStatus.FORBIDDEN));
        }
        String normalizedRoom = SubsonicSourceRegistry.normalizeRoomId(roomId);
        if (isSubsonicPlatform(platform) && !canUseSubsonicSource(subsonicSourceRegistry.findRoomSourceByPlatformId(normalizedRoom, platform).orElseThrow(), token)) {
            return Mono.error(new ResponseStatusException(HttpStatus.FORBIDDEN));
        }
        log.info("API album search request: platform={}, keywordLength={}", platform, keyword == null ? 0 : keyword.length());
        if (isSubsonicPlatform(platform)) {
            return subsonicMusicApiService.searchAlbums(normalizedRoom, platform, keyword)
                    .doOnSuccess(result -> log.info("API album search success: platform={}, resultCount={}", platform, result == null ? 0 : result.size()))
                    .doOnError(error -> log.error("API album search failed: platform={}, keyword={}", platform, keyword, error));
        }
        return getService(platform).searchAlbums(keyword)
                .doOnSuccess(result -> log.info("API album search success: platform={}, resultCount={}", platform, result == null ? 0 : result.size()))
                .doOnError(error -> log.error("API album search failed: platform={}, keyword={}", platform, keyword, error));
    }

    @GetMapping("/album/songs/{platform}/{albumId}")
    public Mono<List<Music>> getAlbumSongs(@PathVariable String platform,
                                           @PathVariable String albumId,
                                           @RequestParam(required = false) String token,
                                           @RequestParam(required = false) String roomId) {
        if ("navidrome".equals(platform) && (token == null || !navidromeAccessService.canUseBySessionToken(token))) {
            return Mono.error(new ResponseStatusException(HttpStatus.FORBIDDEN));
        }
        String normalizedRoom = SubsonicSourceRegistry.normalizeRoomId(roomId);
        if (isSubsonicPlatform(platform) && !canUseSubsonicSource(subsonicSourceRegistry.findRoomSourceByPlatformId(normalizedRoom, platform).orElseThrow(), token)) {
            return Mono.error(new ResponseStatusException(HttpStatus.FORBIDDEN));
        }
        log.info("API album songs request: platform={}, albumId={}", platform, albumId);
        if (isSubsonicPlatform(platform)) {
            return subsonicMusicApiService.getAlbumMusics(normalizedRoom, platform, albumId)
                    .doOnSuccess(result -> log.info("API album songs success: platform={}, albumId={}, resultCount={}", platform, albumId, result == null ? 0 : result.size()))
                    .doOnError(error -> log.error("API album songs failed: platform={}, albumId={}", platform, albumId, error));
        }
        return getService(platform).getAlbumMusics(albumId)
                .doOnSuccess(result -> log.info("API album songs success: platform={}, albumId={}, resultCount={}", platform, albumId, result == null ? 0 : result.size()))
                .doOnError(error -> log.error("API album songs failed: platform={}, albumId={}", platform, albumId, error));
    }

    @GetMapping("/user/search/{platform}/{keyword}")
    public Mono<List<UserSearchResult>> searchUsers(@PathVariable String platform, @PathVariable String keyword) {
        log.info("API user search request: platform={}, keywordLength={}", platform, keyword == null ? 0 : keyword.length());
        return getService(platform).searchUsers(keyword)
                .doOnSuccess(result -> log.info("API user search success: platform={}, resultCount={}", platform, result == null ? 0 : result.size()))
                .doOnError(error -> log.error("API user search failed: platform={}, keyword={}", platform, keyword, error));
    }

    @GetMapping("/music/lyric/{platform}/{musicId}")
    public Mono<String> getLyric(@PathVariable String platform, @PathVariable String musicId) {
        log.info("API lyric request: platform={}, musicId={}", platform, musicId);
        if (isSubsonicPlatform(platform)) {
            return Mono.just("");
        }
        return getService(platform).getLyric(musicId)
                .doOnSuccess(result -> log.info("API lyric success: platform={}, musicId={}, length={}", platform, musicId, result == null ? 0 : result.length()))
                .doOnError(error -> log.error("API lyric failed: platform={}, musicId={}", platform, musicId, error));
    }

    @GetMapping("/music/lyric-detail/{platform}/{musicId}")
    public Mono<LyricResponse> getLyricDetail(@PathVariable String platform, @PathVariable String musicId) {
        log.info("API lyric detail request: platform={}, musicId={}", platform, musicId);
        if (isSubsonicPlatform(platform)) {
            return Mono.just(new LyricResponse("", "", ""));
        }
        return getService(platform).getLyricDetail(musicId)
                .doOnSuccess(result -> log.info(
                        "API lyric detail success: platform={}, musicId={}, lyricLength={}, translatedLength={}",
                        platform,
                        musicId,
                        result == null || result.lyric() == null ? 0 : result.lyric().length(),
                        result == null || result.translatedLyric() == null ? 0 : result.translatedLyric().length()
                ))
                .doOnError(error -> log.error("API lyric detail failed: platform={}, musicId={}", platform, musicId, error));
    }

    @GetMapping("/theme/extract-cover-color")
    public Mono<CoverColorResponse> extractCoverColor(@RequestParam String url) {
        log.info("API cover color request: urlLength={}", url == null ? 0 : url.length());
        return coverColorService.extract(url)
                .doOnSuccess(result -> log.info("API cover color success: found={}", result != null))
                .doOnError(error -> log.error("API cover color failed", error));
    }
}
