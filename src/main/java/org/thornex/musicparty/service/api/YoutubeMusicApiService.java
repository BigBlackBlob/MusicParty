package org.thornex.musicparty.service.api;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.util.StringUtils;
import org.springframework.web.reactive.function.client.WebClient;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.PlayableMusic;
import org.thornex.musicparty.dto.Playlist;
import org.thornex.musicparty.dto.UserSearchResult;
import org.thornex.musicparty.enums.CacheStatus;
import org.thornex.musicparty.exception.ApiRequestException;
import org.thornex.musicparty.service.LocalCacheService;
import reactor.core.publisher.Mono;
import reactor.core.scheduler.Schedulers;

import java.io.IOException;
import java.time.Duration;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

@Service
@Slf4j
public class YoutubeMusicApiService implements CachedMusicApiService {
    private static final String PLATFORM = "youtube";
    private static final String BASE_URL = "https://www.googleapis.com/youtube/v3";
    private static final int DEFAULT_LIMIT = 20;

    private final WebClient webClient;
    private final LocalCacheService localCacheService;
    private final ObjectMapper objectMapper;
    private final AppProperties.YoutubeApiConfig config;
    private final boolean ytDlpAvailable;

    public YoutubeMusicApiService(WebClient webClient,
                                  LocalCacheService localCacheService,
                                  ObjectMapper objectMapper,
                                  AppProperties appProperties) {
        this.webClient = webClient;
        this.localCacheService = localCacheService;
        this.objectMapper = objectMapper;
        this.config = appProperties.getYoutube();
        this.ytDlpAvailable = isExecutableAvailable(config.getYtDlpPath());
    }

    @Override
    public String getPlatformName() {
        return PLATFORM;
    }

    @Override
    public boolean isAvailable() {
        return config != null
                && config.isEnabled()
                && StringUtils.hasText(config.getApiKey())
                && ytDlpAvailable;
    }

    @Override
    public String cacheKey(String musicId) {
        return "youtube-" + musicId;
    }

    @Override
    public Mono<List<Music>> searchMusic(String keyword) {
        return searchMusic(keyword, 0, DEFAULT_LIMIT);
    }

    @Override
    public Mono<List<Music>> searchMusic(String keyword, int offset, int limit) {
        ensureConfigured();
        int safeLimit = Math.max(1, Math.min(limit, Math.max(1, config.getMaxResults())));
        int page = Math.max(0, offset / safeLimit);
        return searchPage(keyword, safeLimit, page, null);
    }

    private Mono<List<Music>> searchPage(String keyword, int limit, int targetPage, String pageToken) {
        return webClient.get()
                .uri(builder -> builder
                        .scheme("https")
                        .host("www.googleapis.com")
                        .path("/youtube/v3/search")
                        .queryParam("part", "snippet")
                        .queryParam("type", "video")
                        .queryParam("q", keyword)
                        .queryParam("maxResults", limit)
                        .queryParam("key", config.getApiKey())
                        .queryParamIfPresent("pageToken", java.util.Optional.ofNullable(pageToken))
                        .build())
                .retrieve()
                .bodyToMono(JsonNode.class)
                .flatMap(json -> {
                    if (targetPage > 0 && StringUtils.hasText(json.path("nextPageToken").asText(null))) {
                        return searchPage(keyword, limit, targetPage - 1, json.path("nextPageToken").asText());
                    }
                    List<String> ids = new ArrayList<>();
                    Map<String, JsonNode> snippets = new LinkedHashMap<>();
                    json.path("items").forEach(item -> {
                        String id = item.path("id").path("videoId").asText();
                        if (StringUtils.hasText(id)) {
                            ids.add(id);
                            snippets.put(id, item.path("snippet"));
                        }
                    });
                    if (ids.isEmpty()) return Mono.just(List.of());
                    return fetchDurations(ids).map(durations -> ids.stream()
                            .map(id -> toMusic(id, snippets.get(id), durations.getOrDefault(id, 0L)))
                            .toList());
                });
    }

    private Mono<Map<String, Long>> fetchDurations(List<String> ids) {
        return webClient.get()
                .uri(BASE_URL + "/videos?part=contentDetails&id={ids}&key={key}", String.join(",", ids), config.getApiKey())
                .retrieve()
                .bodyToMono(JsonNode.class)
                .map(json -> {
                    Map<String, Long> durations = new HashMap<>();
                    json.path("items").forEach(item -> durations.put(
                            item.path("id").asText(),
                            parseIsoDurationMillis(item.path("contentDetails").path("duration").asText())));
                    return durations;
                });
    }

    private Music toMusic(String id, JsonNode snippet, long duration) {
        JsonNode thumbnails = snippet.path("thumbnails");
        String cover = thumbnails.path("high").path("url").asText(
                thumbnails.path("medium").path("url").asText(thumbnails.path("default").path("url").asText("")));
        return new Music(
                id,
                snippet.path("title").asText(),
                List.of(snippet.path("channelTitle").asText("YouTube")),
                duration,
                PLATFORM,
                cover
        );
    }

    @Override
    public void prefetchMusic(String videoId) {
        ensureConfigured();
        String key = cacheKey(videoId);
        CacheStatus status = localCacheService.getStatus(key);
        if (status == CacheStatus.COMPLETED || status == CacheStatus.DOWNLOADING || status == CacheStatus.PENDING) {
            return;
        }
        localCacheService.submitDynamicDownload(key, resolveDownloadSource(videoId));
    }

    @Override
    public Mono<PlayableMusic> getPlayableMusic(String videoId) {
        ensureConfigured();
        String localUrl = localCacheService.getLocalUrl(cacheKey(videoId));
        Mono<Music> details = fetchVideoDetails(videoId);
        if (localUrl != null) {
            return details.map(music -> new PlayableMusic(
                    music.id(), music.name(), music.artists(), music.duration(), PLATFORM, localUrl, music.coverUrl(), false));
        }
        prefetchMusic(videoId);
        return details.map(music -> new PlayableMusic(
                music.id(), music.name(), music.artists(), music.duration(), PLATFORM, "PENDING_DOWNLOAD", music.coverUrl(), false));
    }

    private Mono<Music> fetchVideoDetails(String videoId) {
        return webClient.get()
                .uri(BASE_URL + "/videos?part=snippet,contentDetails&id={id}&key={key}", videoId, config.getApiKey())
                .retrieve()
                .bodyToMono(JsonNode.class)
                .map(json -> {
                    JsonNode item = json.path("items").isArray() && json.path("items").size() > 0 ? json.path("items").get(0) : null;
                    if (item == null) throw new ApiRequestException("YouTube video not found: " + videoId);
                    return toMusic(videoId, item.path("snippet"), parseIsoDurationMillis(item.path("contentDetails").path("duration").asText()));
                });
    }

    private Mono<LocalCacheService.DownloadSource> resolveDownloadSource(String videoId) {
        return Mono.fromCallable(() -> {
                    ProcessBuilder pb = new ProcessBuilder(
                            config.getYtDlpPath(),
                            "--dump-single-json",
                            "--no-playlist",
                            "-f",
                            "bestaudio",
                            "https://www.youtube.com/watch?v=" + videoId);
                    Process process = pb.start();
                    String output = new String(process.getInputStream().readAllBytes(), java.nio.charset.StandardCharsets.UTF_8);
                    String error = new String(process.getErrorStream().readAllBytes(), java.nio.charset.StandardCharsets.UTF_8);
                    boolean finished = process.waitFor(60, java.util.concurrent.TimeUnit.SECONDS);
                    if (!finished) {
                        process.destroyForcibly();
                        throw new ApiRequestException("yt-dlp timed out");
                    }
                    if (process.exitValue() != 0) {
                        throw new ApiRequestException("yt-dlp failed: " + error);
                    }
                    JsonNode json = objectMapper.readTree(output);
                    String url = json.path("url").asText();
                    if (!StringUtils.hasText(url)) {
                        throw new ApiRequestException("yt-dlp did not return a playable URL");
                    }
                    Map<String, String> headers = new HashMap<>();
                    json.path("http_headers").fields().forEachRemaining(entry -> headers.put(entry.getKey(), entry.getValue().asText()));
                    String ext = json.path("ext").asText("m4a");
                    return new LocalCacheService.DownloadSource(url, headers, "." + ext.replaceAll("[^A-Za-z0-9]", ""));
                })
                .timeout(Duration.ofSeconds(70))
                .subscribeOn(Schedulers.boundedElastic());
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

    static long parseIsoDurationMillis(String duration) {
        if (!StringUtils.hasText(duration)) return 0;
        return java.time.Duration.parse(duration).toMillis();
    }

    private void ensureConfigured() {
        if (!isAvailable()) {
            throw new ApiRequestException("YouTube is not configured. Set YOUTUBE_API_KEY and install yt-dlp.");
        }
    }

    private boolean isExecutableAvailable(String path) {
        if (!StringUtils.hasText(path)) return false;
        try {
            Process process = new ProcessBuilder(path, "--version").start();
            boolean finished = process.waitFor(3, java.util.concurrent.TimeUnit.SECONDS);
            if (!finished) {
                process.destroyForcibly();
                return false;
            }
            return process.exitValue() == 0;
        } catch (IOException | InterruptedException e) {
            if (e instanceof InterruptedException) Thread.currentThread().interrupt();
            return false;
        }
    }
}
