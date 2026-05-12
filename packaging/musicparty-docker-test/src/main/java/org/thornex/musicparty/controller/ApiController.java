package org.thornex.musicparty.controller;

import lombok.extern.slf4j.Slf4j;
import org.springframework.web.bind.annotation.*;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.CoverColorResponse;
import org.thornex.musicparty.dto.Album;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.Playlist;
import org.thornex.musicparty.dto.UserSearchResult;
import org.thornex.musicparty.exception.ApiRequestException;
import org.thornex.musicparty.service.CoverColorService;
import org.thornex.musicparty.service.api.IMusicApiService;
import org.thornex.musicparty.service.api.NeteaseMusicApiService;
import reactor.core.publisher.Mono;

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

    public ApiController(List<IMusicApiService> apiServices, AppProperties appProperties, CoverColorService coverColorService) {
        this.apiServiceMap = apiServices.stream()
                .collect(Collectors.toMap(IMusicApiService::getPlatformName, Function.identity()));
        this.appProperties = appProperties;
        this.coverColorService = coverColorService;
    }

    @GetMapping("/config")
    public Map<String, String> getConfig() {
        return Map.of(
                "authorName", appProperties.getAuthorName(),
                "backWords", appProperties.getBackWords()
        );
    }

    private IMusicApiService getService(String platform) {
        IMusicApiService service = apiServiceMap.get(platform);
        if (service == null) {
            throw new ApiRequestException("Platform not supported: " + platform);
        }
        return service;
    }

    @GetMapping("/search/{platform}/{keyword}")
    public Mono<List<Music>> searchMusic(@PathVariable String platform, @PathVariable String keyword) {
        log.info("API search request: platform={}, keywordLength={}", platform, keyword == null ? 0 : keyword.length());
        return getService(platform).searchMusic(keyword)
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

    @GetMapping("/album/search/netease")
    public Mono<List<Album>> searchNeteaseAlbums(@RequestParam String keyword) {
        log.info("API album search request: platform=netease, keywordLength={}", keyword == null ? 0 : keyword.length());
        IMusicApiService service = getService("netease");
        if (!(service instanceof NeteaseMusicApiService neteaseService)) {
            throw new ApiRequestException("Netease album search is unavailable");
        }
        return neteaseService.searchAlbums(keyword)
                .doOnSuccess(result -> log.info("API album search success: platform=netease, resultCount={}", result == null ? 0 : result.size()))
                .doOnError(error -> log.error("API album search failed: platform=netease, keyword={}", keyword, error));
    }

    @GetMapping("/album/songs/netease/{albumId}")
    public Mono<List<Music>> getNeteaseAlbumSongs(@PathVariable String albumId) {
        log.info("API album songs request: platform=netease, albumId={}", albumId);
        IMusicApiService service = getService("netease");
        if (!(service instanceof NeteaseMusicApiService neteaseService)) {
            throw new ApiRequestException("Netease album songs are unavailable");
        }
        return neteaseService.getAlbumMusics(albumId)
                .doOnSuccess(result -> log.info("API album songs success: platform=netease, albumId={}, resultCount={}", albumId, result == null ? 0 : result.size()))
                .doOnError(error -> log.error("API album songs failed: platform=netease, albumId={}", albumId, error));
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
        return getService(platform).getLyric(musicId)
                .doOnSuccess(result -> log.info("API lyric success: platform={}, musicId={}, length={}", platform, musicId, result == null ? 0 : result.length()))
                .doOnError(error -> log.error("API lyric failed: platform={}, musicId={}", platform, musicId, error));
    }

    @GetMapping("/theme/extract-cover-color")
    public Mono<CoverColorResponse> extractCoverColor(@RequestParam String url) {
        log.info("API cover color request: urlLength={}", url == null ? 0 : url.length());
        return coverColorService.extract(url)
                .doOnSuccess(result -> log.info("API cover color success: found={}", result != null))
                .doOnError(error -> log.error("API cover color failed", error));
    }
}
