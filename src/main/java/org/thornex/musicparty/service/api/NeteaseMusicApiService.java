package org.thornex.musicparty.service.api;

import jakarta.annotation.PostConstruct;
import lombok.extern.slf4j.Slf4j;
import org.springframework.http.HttpStatusCode;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.reactive.function.client.WebClientResponseException;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.*;
import org.thornex.musicparty.enums.CacheStatus;
import org.thornex.musicparty.exception.ApiRequestException;
import org.thornex.musicparty.service.LocalCacheService;
import org.thornex.musicparty.service.SiteSettingService;
import reactor.core.publisher.Mono;
import com.fasterxml.jackson.databind.JsonNode;

import java.net.SocketTimeoutException;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.TimeoutException;
import java.util.stream.StreamSupport;

@Service
@Slf4j
public class NeteaseMusicApiService implements CachedMusicApiService {

    private final WebClient webClient;
    private final String baseUrl;
    private final String initialCookieFromConfig;
    private final String quality;
    private final SiteSettingService siteSettingService;
    private final LocalCacheService localCacheService;
    private volatile String currentCookie;
    private static final String PLATFORM = "netease";

    public NeteaseMusicApiService(WebClient webClient, AppProperties appProperties,
                                  SiteSettingService siteSettingService,
                                  LocalCacheService localCacheService) {
        this.webClient = webClient;
        this.baseUrl = appProperties.getNetease().getBaseUrl();
        this.initialCookieFromConfig = appProperties.getNetease().getCookie();
        this.quality = appProperties.getNetease().getQuality();
        this.siteSettingService = siteSettingService;
        this.localCacheService = localCacheService;
        this.siteSettingService.importSecretIfMissing(SiteSettingService.NETEASE_COOKIE, initialCookieFromConfig);
        this.currentCookie = siteSettingService.secretOrDefault(SiteSettingService.NETEASE_COOKIE, initialCookieFromConfig);
    }

    @PostConstruct
    public void initialize() {
        log.info("Initializing NeteaseCloudMusic API client with quality: {}...", quality);
        if (!StringUtils.hasText(currentCookie) || "YOUR_NETEASE_COOKIE_STRING_HERE".equals(currentCookie)) {
            log.info("Netease Cookie is empty. Service running in passive mode (waiting for config).");
        } else {
            log.info("Netease Cookie configured.");
        }
    }

    private void ensureConfigured() {
        if (!StringUtils.hasText(currentCookie) || "YOUR_NETEASE_COOKIE_STRING_HERE".equals(currentCookie)) {
            throw new ApiRequestException("尚未配置网易云 Cookie，请联系管理员设置");
        }
    }

    public void updateCookie(String newCookie) {
        this.currentCookie = newCookie;
        siteSettingService.putSecret(SiteSettingService.NETEASE_COOKIE, newCookie);
        checkCookie(newCookie).subscribe(isValid -> {
            if (isValid) {
                log.info("Netease cookie updated and verified successfully.");
            } else {
                log.warn("The newly updated Netease cookie appears to be invalid.");
            }
        });
    }

    private Mono<Boolean> checkCookie(String cookie) {
        return webClient.get()
                .uri(baseUrl + "/user/account?cookie={cookie}", cookie)
                .retrieve()
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> jsonNode.has("profile") && !jsonNode.get("profile").isNull())
                .onErrorReturn(false);
    }

    // UPDATED: Renamed method and removed encoding.
    private String getCookie() {
        return currentCookie != null ? currentCookie : "";
    }
    
    // Helper to force HTTPS
    private String upgradeToHttps(String url) {
        if (url != null && url.startsWith("http://")) {
            return url.replace("http://", "https://");
        }
        return url;
    }

    private String normalizeNeteaseImageUrl(String url) {
        String httpsUrl = upgradeToHttps(url);
        if (!StringUtils.hasText(httpsUrl)) {
            return "";
        }
        return httpsUrl;
    }

    private String getNeteaseCoverUrl(JsonNode song) {
        String coverUrl = song.path("al").path("picUrl").asText("");
        if (StringUtils.hasText(coverUrl)) {
            return normalizeNeteaseImageUrl(coverUrl);
        }

        coverUrl = song.path("album").path("picUrl").asText("");
        if (StringUtils.hasText(coverUrl)) {
            return normalizeNeteaseImageUrl(coverUrl);
        }

        long picId = song.path("album").path("picId").asLong(0);
        if (picId > 0) {
            return "https://p3.music.126.net/" + picId + ".jpg";
        }

        return "";
    }

    private long getNeteaseDuration(JsonNode song) {
        long duration = song.path("dt").asLong(0);
        if (duration > 0) {
            return duration;
        }
        return song.path("duration").asLong(0);
    }

    private Music toMusic(JsonNode song) {
        return toMusic(song, "");
    }

    private Music toMusic(JsonNode song, String fallbackCoverUrl) {
        JsonNode artistNode = song.has("artists") ? song.path("artists") : song.path("ar");
        List<String> artists = StreamSupport.stream(artistNode.spliterator(), false)
                .map(artist -> artist.path("name").asText())
                .toList();
        String coverUrl = getNeteaseCoverUrl(song);
        if (!StringUtils.hasText(coverUrl)) {
            coverUrl = fallbackCoverUrl;
        }
        return new Music(
                song.path("id").asText(),
                song.path("name").asText(),
                artists,
                getNeteaseDuration(song),
                PLATFORM,
                coverUrl
        );
    }


    @Override
    public String getPlatformName() {
        return PLATFORM;
    }

    private Mono<ApiRequestException> handleApiError(String apiName, org.springframework.web.reactive.function.client.ClientResponse response) {
        return response.bodyToMono(String.class)
                .flatMap(errorBody -> {
                    int status = response.statusCode().value();
                    String category;
                    if (status == 401 || status == 403) {
                        category = "网易云 Cookie 已失效或无权限";
                    } else if (status >= 400 && status < 500) {
                        category = "网易云上游客户端错误";
                    } else if (status >= 500) {
                        category = "网易云上游服务错误";
                    } else {
                        category = "网易云上游错误";
                    }
                    return Mono.error(new ApiRequestException(
                            String.format("%s: api=%s, status=%d, body=%s", category, apiName, status, errorBody)
                    ));
                });
    }

    // UPDATED: All API calls now use the raw cookie from getCookie()
    @Override
    public Mono<List<Music>> searchMusic(String keyword) {
        return searchMusic(keyword, 0, 30);
    }

    @Override
    public Mono<List<Music>> searchMusic(String keyword, int offset, int limit) {
        ensureConfigured();
        return webClient.get()
                .uri(baseUrl + "/cloudsearch?keywords={keyword}&limit={limit}&offset={offset}&cookie={cookie}", keyword, limit, offset, getCookie())
                .retrieve()
                .onStatus(HttpStatusCode::isError, response -> handleApiError("cloudsearch", response))
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> {
                    List<Music> musicList = new ArrayList<>();
                    JsonNode songs = jsonNode.path("result").path("songs");
                    if (songs.isArray()) {
                        for (JsonNode song : songs) {
                            musicList.add(toMusic(song));
                        }
                    }
                    return musicList;
                });
    }

    @Override
    public Mono<List<Album>> searchAlbums(String keyword) {
        ensureConfigured();
        return webClient.get()
                .uri(baseUrl + "/cloudsearch?keywords={keyword}&type=10&cookie={cookie}", keyword, getCookie())
                .retrieve()
                .onStatus(HttpStatusCode::isError, response -> handleApiError("search albums", response))
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> {
                    List<Album> albums = new ArrayList<>();
                    JsonNode albumNodes = jsonNode.path("result").path("albums");
                    if (albumNodes.isArray()) {
                        albumNodes.forEach(album -> albums.add(new Album(
                                album.path("id").asText(),
                                album.path("name").asText(),
                                album.path("artist").path("name").asText(""),
                                normalizeNeteaseImageUrl(album.path("picUrl").asText("")),
                                album.path("size").asInt(0),
                                PLATFORM
                        )));
                    }
                    return albums;
                });
    }

    @Override
    public Mono<List<Music>> getAlbumMusics(String albumId) {
        ensureConfigured();
        return webClient.get()
                .uri(baseUrl + "/album?id={albumId}&cookie={cookie}", albumId, getCookie())
                .retrieve()
                .onStatus(HttpStatusCode::isError, response -> handleApiError("get album tracks", response))
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> {
                    List<Music> musicList = new ArrayList<>();
                    String albumCoverUrl = normalizeNeteaseImageUrl(jsonNode.path("album").path("picUrl").asText(""));
                    JsonNode songs = jsonNode.path("songs");
                    if (songs.isArray()) {
                        songs.forEach(song -> musicList.add(toMusic(song, albumCoverUrl)));
                    }
                    return musicList;
                });
    }

    @Override
    public Mono<PlayableMusic> getPlayableMusic(String musicId) {
        ensureConfigured();
        return getMusicDetails(musicId)
                .map(music -> new PlayableMusic(
                        music.id(),
                        music.name(),
                        music.artists(),
                        music.duration(),
                        music.platform(),
                        "/api/netease/stream/" + music.id(),
                        music.coverUrl(),
                        false
                ));
    }

    @Override
    public void prefetchMusic(String musicId) {
        ensureConfigured();
        // 检查缓存状态，如果已经下载或正在下载，直接返回
        CacheStatus status = localCacheService.getStatus(musicId);
        if (status == CacheStatus.COMPLETED || status == CacheStatus.DOWNLOADING) {
            return;
        }

        log.info("Prefetching Netease music: {}", musicId);

        // CDN URL 作为下载源（由 resolveCdnUrl 提供）
        Mono<String> urlProvider = resolveCdnUrl(musicId);

        // 网易云 CDN 需要 Referer
        Map<String, String> headers = new HashMap<>();
        headers.put("Referer", "https://music.163.com/");
        headers.put("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36");

        // 扩展名根据音质决定：exhigh 默认 mp3；lossless/hires 可能是 .flac
        String extension = ".mp3";

        localCacheService.submitDownload(musicId, urlProvider, headers, extension);
    }

    /**
     * 解析网易云 CDN 直链。每次播放时由代理控制器实时调用，避免缓存过期。
     */
    public Mono<String> resolveCdnUrl(String musicId) {
        ensureConfigured();
        return webClient.get()
                .uri(baseUrl + "/song/url/v1?id={musicId}&level={quality}&cookie={cookie}", musicId, quality, getCookie())
                .retrieve()
                .onStatus(HttpStatusCode::isError, response -> handleApiError("get song URL", response))
                .bodyToMono(JsonNode.class)
                .timeout(Duration.ofSeconds(8))
                .map(jsonNode -> parseCdnUrl(jsonNode, musicId))
                .onErrorMap(this::classifyResolveError);
    }

    private String parseCdnUrl(JsonNode jsonNode, String musicId) {
        JsonNode data = jsonNode.path("data");
        if (!data.isArray() || data.isEmpty()) {
            throw new ApiRequestException("网易云未返回播放地址: songId=" + musicId);
        }
        JsonNode item = data.get(0);
        int code = item.path("code").asInt(jsonNode.path("code").asInt(200));
        if (code == 401 || code == 301 || code == 302) {
            throw new ApiRequestException("网易云 Cookie 已失效，请更新 Cookie");
        }
        if (code == 403 || code == 404 || item.path("freeTrialInfo").isObject()) {
            throw new ApiRequestException("网易云歌曲无版权或当前账号不可播放: songId=" + musicId);
        }
        String url = item.path("url").asText("");
        if (!StringUtils.hasText(url)) {
            throw new ApiRequestException("网易云返回空播放地址: songId=" + musicId + ", code=" + code);
        }
        return upgradeToHttps(url);
    }

    private Throwable classifyResolveError(Throwable error) {
        if (error instanceof ApiRequestException) {
            return error;
        }
        if (error instanceof TimeoutException || error instanceof SocketTimeoutException) {
            return new ApiRequestException("网易云播放地址解析超时");
        }
        if (error instanceof WebClientResponseException responseException) {
            int status = responseException.getStatusCode().value();
            if (status == 401 || status == 403) {
                return new ApiRequestException("网易云 Cookie 已失效或无权限: HTTP " + status);
            }
            if (status >= 400 && status < 500) {
                return new ApiRequestException("网易云上游客户端错误: HTTP " + status);
            }
            if (status >= 500) {
                return new ApiRequestException("网易云上游服务错误: HTTP " + status);
            }
        }
        return new ApiRequestException("网易云播放地址解析失败: " + error.getMessage());
    }

    private Mono<Music> getMusicDetails(String musicId) {
        ensureConfigured();
        return webClient.get()
                .uri(baseUrl + "/song/detail?ids={musicId}&cookie={cookie}", musicId, getCookie())
                .retrieve()
                .onStatus(HttpStatusCode::isError, response -> handleApiError("get song detail", response))
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> {
                    JsonNode song = jsonNode.path("songs").get(0);
                    List<String> artists = new ArrayList<>();
                    JsonNode artistNode = song.has("artists") ? song.path("artists") : song.path("ar");
                    artistNode.forEach(artist -> artists.add(artist.path("name").asText()));
                    return new Music(
                            song.path("id").asText(),
                            song.path("name").asText(),
                            artists,
                            song.path("dt").asLong(),
                            PLATFORM,
                            normalizeNeteaseImageUrl(song.path("al").path("picUrl").asText())
                    );
                });
    }

    @Override
    public Mono<List<Playlist>> getUserPlaylists(String userId) {
        ensureConfigured();
        return webClient.get()
                .uri(baseUrl + "/user/playlist?uid={userId}&cookie={cookie}", userId, getCookie())
                .retrieve()
                .onStatus(HttpStatusCode::isError, response -> handleApiError("get user playlists", response))
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> {
                    List<Playlist> playlists = new ArrayList<>();
                    jsonNode.path("playlist").forEach(pl -> playlists.add(new Playlist(
                            pl.path("id").asText(),
                            pl.path("name").asText(),
                            normalizeNeteaseImageUrl(pl.path("coverImgUrl").asText()),
                            pl.path("trackCount").asInt(),
                            PLATFORM
                    )));
                    return playlists;
                });
    }

    @Override
    public Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) {
        ensureConfigured();
        return webClient.get()
                .uri(baseUrl + "/playlist/track/all?id={playlistId}&limit={limit}&offset={offset}&cookie={cookie}", playlistId, limit, offset, getCookie())
                .retrieve()
                .onStatus(HttpStatusCode::isError, response -> handleApiError("get playlist tracks", response))
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> {
                    List<Music> musicList = new ArrayList<>();
                    jsonNode.path("songs").forEach(song -> {
                        musicList.add(toMusic(song));
                    });
                    return musicList;
                });
    }

    @Override
    public Mono<List<UserSearchResult>> searchUsers(String keyword) {
        ensureConfigured();
        // type=1002 表示搜索用户
        return webClient.get()
                .uri(baseUrl + "/search?keywords={keyword}&type=1002&cookie={cookie}", keyword, getCookie())
                .retrieve()
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> {
                    List<UserSearchResult> users = new ArrayList<>();
                    // 网易云返回结构: result.userprofiles
                    JsonNode profiles = jsonNode.path("result").path("userprofiles");
                    if (profiles.isArray()) {
                        profiles.forEach(u -> users.add(new UserSearchResult(
                                u.path("userId").asText(),
                                u.path("nickname").asText(),
                                upgradeToHttps(u.path("avatarUrl").asText()),
                                PLATFORM
                        )));
                    }
                    return users;
                });
    }

    @Override
    public Mono<String> getLyric(String musicId) {
        return getLyricDetail(musicId).map(LyricResponse::lyric);
    }

    @Override
    public Mono<LyricResponse> getLyricDetail(String musicId) {
        ensureConfigured();
        return webClient.get()
                .uri(baseUrl + "/lyric?id={id}", musicId)
                .retrieve()
                .bodyToMono(JsonNode.class)
                .map(json -> new LyricResponse(
                        json.path("lrc").path("lyric").asText(""),
                        json.path("tlyric").path("lyric").asText(""),
                        json.path("romalrc").path("lyric").asText("")
                ))
                .onErrorReturn(LyricResponse.empty());
    }
}
