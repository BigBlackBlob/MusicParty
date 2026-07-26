package org.thornex.musicparty.service.api;

import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Service;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.web.reactive.function.client.WebClient;
import org.springframework.web.util.UriComponentsBuilder;
import org.thornex.musicparty.config.AppProperties;
import org.thornex.musicparty.dto.Music;
import org.thornex.musicparty.dto.PlayableMusic;
import org.thornex.musicparty.dto.Playlist;
import org.thornex.musicparty.dto.UserSearchResult;
import org.thornex.musicparty.enums.CacheStatus;
import org.thornex.musicparty.exception.ApiRequestException;
import org.thornex.musicparty.service.LocalCacheService;
import org.thornex.musicparty.service.SiteSettingService;
import org.thornex.musicparty.util.BilibiliApiUtils;
import org.thornex.musicparty.util.BilibiliCookieSupport;
import reactor.core.publisher.Mono;
import com.fasterxml.jackson.databind.JsonNode;
import reactor.util.retry.Retry;

import java.util.*;
import java.util.stream.StreamSupport;

@Service
@Slf4j
public class BilibiliMusicApiService implements CachedMusicApiService {

    private final WebClient webClient;
    private final String baseUrl;
    private volatile String sessdata;
    private final LocalCacheService localCacheService;
    private static final String PLATFORM = "bilibili";
    private final BilibiliWbiService wbiService;
    private final SiteSettingService siteSettingService;
    private final UpstreamResilienceService resilience;

    private static class WbiSignatureException extends RuntimeException {
        public WbiSignatureException(String message) { super(message); }
    }

    public BilibiliMusicApiService(WebClient webClient, AppProperties appProperties, LocalCacheService localCacheService, BilibiliWbiService wbiService, SiteSettingService siteSettingService) {
        this(webClient, appProperties, localCacheService, wbiService, siteSettingService,
                new UpstreamResilienceService(appProperties, new io.micrometer.core.instrument.simple.SimpleMeterRegistry()));
    }

    @Autowired
    public BilibiliMusicApiService(WebClient webClient, AppProperties appProperties, LocalCacheService localCacheService,
                                   BilibiliWbiService wbiService, SiteSettingService siteSettingService,
                                   UpstreamResilienceService resilience) {
        this.webClient = webClient;
        this.baseUrl = appProperties.getBilibili().getBaseUrl();
        this.siteSettingService = siteSettingService;
        this.resilience = resilience;
        this.siteSettingService.importSecretIfMissing(SiteSettingService.BILIBILI_SESSDATA, appProperties.getBilibili().getSessdata());
        this.sessdata = siteSettingService.secretOrDefault(SiteSettingService.BILIBILI_SESSDATA, appProperties.getBilibili().getSessdata());
        this.localCacheService = localCacheService;
        this.wbiService = wbiService;
    }

    private void ensureConfigured() {
        if (!org.springframework.util.StringUtils.hasText(sessdata)) {
            throw new ApiRequestException("尚未配置 Bilibili SESSDATA，请联系管理员设置");
        }
    }

    public void updateSessdata(String newSessdata) {
        this.sessdata = newSessdata;
        this.siteSettingService.putSecret(SiteSettingService.BILIBILI_SESSDATA, newSessdata);
        this.wbiService.updateSessdata(newSessdata);
        log.info("Bilibili API Service SESSDATA updated.");
    }

    @Override
    public String getPlatformName() {
        return PLATFORM;
    }
    private WebClient.RequestHeadersSpec<?> buildBilibiliRequest(String uri) {
        return webClient.get()
                .uri(uri)
                .header("Cookie", BilibiliCookieSupport.toCookieHeader(this.sessdata))
                .header("Referer", "https://www.bilibili.com/");
    }

    @Override
    public Mono<List<Music>> searchMusic(String keyword) {
        return searchMusic(keyword, 0, 20);
    }

    @Override
    public Mono<List<Music>> searchMusic(String keyword, int offset, int limit) {
        ensureConfigured();
        // 1. 准备请求参数
        int pageNumber = (offset / limit) + 1;
        Map<String, String> params = new HashMap<>();
        params.put("search_type", "video");
        params.put("keyword", keyword);
        params.put("page", String.valueOf(pageNumber));
        params.put("page_size", String.valueOf(limit));

        // 2. 调用 WBI 签名服务
        Mono<List<Music>> requestMono = wbiService.signParams(params)
                .flatMap(signedParams -> {
                    // 3. 构建 QueryString，注意：WBI 签名要求参数顺序及 URL 编码
                    // 这里我们直接利用 UriComponentsBuilder 确保符合文档要求
                    UriComponentsBuilder builder = UriComponentsBuilder
                            .fromHttpUrl(baseUrl + "/x/web-interface/wbi/search/type");

                    signedParams.forEach(builder::queryParam);

                    return webClient.get()
                            .uri(builder.build().toUri()) // 使用编码后的 URI
                            .header("Cookie", BilibiliCookieSupport.toCookieHeader(sessdata))
                            .header("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
                            .header("Referer", "https://www.bilibili.com/") // 必须带 Referer
                            .retrieve()
                            .bodyToMono(JsonNode.class)
                            .handle((json, sink) -> {
                                int code = json.path("code").asInt();
                                // 🟢 关键点 1: 检测 WBI 潜在的错误码
                                // -403: 访问权限不足 (可能是签名挂了)
                                // -400: 请求错误 (可能是参数/签名校验不过)
                                if (code == -403 || code == -400) {
                                    sink.error(new WbiSignatureException("WBI signature invalid, code: " + code));
                                    return;
                                }

                                // 其他常规错误，不重试，直接记录日志返回空列表
                                if (code != 0) {
                                    log.error("Bilibili search failed: {}", json.path("message").asText());
                                    sink.next(new ArrayList<>());
                                    return;
                                }


                                List<Music> musicList = new ArrayList<>();

                                JsonNode results = json.path("data").path("result");
                                if (results.isArray()) {
                                    results.forEach(video -> {
                                        // 清洗标题中的 <em class="keyword">xxx</em> 标签
                                        String rawTitle = video.path("title").asText();
                                        String cleanTitle = rawTitle.replaceAll("<[^>]*>", "");

                                        // 处理时长
                                        String durationStr = video.path("duration").asText();
                                        long durationMs = BilibiliApiUtils.durationToMillis(durationStr);

                                        // 获取图片，确保有 https
                                        String picUrl = video.path("pic").asText();
                                        if (!picUrl.startsWith("http")) {
                                            picUrl = "https:" + picUrl;
                                        }

                                        musicList.add(new Music(
                                                video.path("bvid").asText(),
                                                cleanTitle,
                                                List.of(video.path("author").asText()),
                                                durationMs,
                                                PLATFORM,
                                                picUrl));
                                    });
                                }
                                sink.next(musicList);
                            });
                });

        // 添加重试机制
        return resilience.cached(PLATFORM, "search:" + keyword + ":" + offset + ":" + limit, java.time.Duration.ofSeconds(30), () -> requestMono.retryWhen(Retry.max(1) // 最多重试 1 次
                        .filter(throwable -> throwable instanceof WbiSignatureException) // 只针对签名异常重试
                        .doBeforeRetry(retrySignal -> {
                            log.warn("Detected WBI signature error, refreshing key and retrying...");
                            wbiService.invalidateCache(); // 清除缓存
                        }))
                // 如果重试后还是失败，降级为空列表
                .onErrorResume(WbiSignatureException.class, e -> {
                    log.error("Bilibili search failed after retry: {}", e.getMessage());
                    return Mono.just(new ArrayList<>());
                }));
    }

    @Override
    public void prefetchMusic(String bvid) {
        ensureConfigured();
        // 检查缓存状态，如果已经下载或正在下载，直接返回
        CacheStatus status = localCacheService.getStatus(bvid);
        if (status == CacheStatus.COMPLETED || status == CacheStatus.DOWNLOADING) {
            return;
        }

        log.info("Prefetching Bilibili music: {}", bvid);

        // 复用之前的解析逻辑，获取 DASH 音频流地址
        Mono<String> urlProvider = resolveDashAudioUrl(bvid);

        // 准备请求头 (防盗链)
        Map<String, String> headers = new HashMap<>();
        headers.put("Referer", "https://www.bilibili.com/video/" + bvid);
        headers.put("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36");

        // 提交异步下载任务 (.m4a 是 B站 dash 音频的常用格式)
        localCacheService.submitDownload(PLATFORM, bvid, urlProvider, headers, ".m4a");
    }

    @Override
    public Mono<PlayableMusic> getPlayableMusic(String bvid) {
        ensureConfigured();
        // 1. 检查本地缓存
        String localUrl = localCacheService.getLocalUrl(bvid);

        if (localUrl != null) {
            // 2. 如果本地存在，直接返回静态资源路径
            // 此时 needsProxy = false，因为对于前端来说，这就是一个普通的 http 链接
            return BilibiliApiUtils.getVideoDetails(bvid, webClient, baseUrl, sessdata)
                    .map(music -> new PlayableMusic(
                            music.id(), music.name(), music.artists(), music.duration(),
                            PLATFORM, localUrl, music.coverUrl(), false // needsProxy = false
                    ));
        } else {
            // 3. 如果本地没有（还在下载或下载失败），返回代理 URL 实现边下边播
            // 前端 <audio> 直接从 /api/bilibili/stream/{bvid} 拉流，
            // 后端代理转发到 B站 DASH 音频 CDN（带 Referer/UA），支持 Range
            // 同时触发后台预下载，下完后 handleDownloadEvent 会切换到 /media/ 本地文件
            prefetchMusic(bvid);

            String streamUrl = "/api/bilibili/stream/" + bvid;
            return BilibiliApiUtils.getVideoDetails(bvid, webClient, baseUrl, sessdata)
                    .map(music -> new PlayableMusic(
                            music.id(), music.name(), music.artists(), music.duration(),
                            PLATFORM, streamUrl, music.coverUrl(), false
                    ))
                    .onErrorResume(e -> {
                        // 元数据获取失败时仍返回代理 URL（至少能播），但日志记录
                        log.warn("Bilibili getVideoDetails failed for {}, returning stream-only playable", bvid, e);
                        return Mono.just(new PlayableMusic(
                                bvid, bvid, List.of("Bilibili"), 0L,
                                PLATFORM, streamUrl, "", false
                        ));
                    });
        }
    }

    // 供 BilibiliProxyController 调用：实时解析 DASH 音频 CDN 地址
    // 每次 stream 请求都重新解析（B站 CDN URL 有时效）
    public Mono<String> resolveStreamUrl(String bvid) {
        ensureConfigured();
        return resolveDashAudioUrl(bvid);
    }

    private Mono<String> resolveDashAudioUrl(String bvid) {
        return BilibiliApiUtils.getVideoCid(bvid, webClient, baseUrl, sessdata)
                .flatMap(cid -> {
                    Map<String, String> params = new HashMap<>();
                    params.put("bvid", bvid);
                    params.put("cid", cid);
                    params.put("fnval", "16"); // DASH
                    params.put("fnver", "0");
                    params.put("fourk", "1");
                    params.put("platform", "pc");

                    return wbiService.signParams(params)
                            .flatMap(signedParams -> {
                                UriComponentsBuilder builder = UriComponentsBuilder.fromHttpUrl(baseUrl + "/x/player/wbi/playurl");
                                signedParams.forEach(builder::queryParam);

                                return webClient.get()
                                        .uri(builder.build().toUri())
                                        .header("Cookie", BilibiliCookieSupport.toCookieHeader(sessdata))
                                        .header("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
                                        .header("Referer", "https://www.bilibili.com/video/" + bvid)
                                        .header("Origin", "https://www.bilibili.com")
                                        .header("Accept", "application/json, text/plain, */*")
                                        .header("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
                                        .header("Sec-Fetch-Site", "same-site")
                                        .header("Sec-Fetch-Mode", "cors")
                                        .header("Sec-Fetch-Dest", "empty")
                                        .retrieve()
                                        .bodyToMono(JsonNode.class)
                                        .flatMap(jsonNode -> {
                                            int code = jsonNode.path("code").asInt();
                                            if (code == -403 || code == -400) {
                                                return Mono.error(new WbiSignatureException("Invalid WBI signature, code: " + code));
                                            }
                                            if (code != 0) {
                                                return Mono.error(new ApiRequestException("Bilibili API Error, code: " + code));
                                            }
                                            JsonNode audioStreams = jsonNode.path("data").path("dash").path("audio");
                                            if (audioStreams.isMissingNode()) {
                                                return Mono.error(new ApiRequestException("No DASH audio found"));
                                            }

                                            String url = StreamSupport.stream(audioStreams.spliterator(), false)
                                                    .max(Comparator.comparingInt(a -> a.path("id").asInt()))
                                                    .map(a -> a.path("baseUrl").asText())
                                                    .orElseThrow(() -> new ApiRequestException("No audio url found in json"));
                                            return Mono.just(url);
                                        });
                            })
                            // 将 retryWhen 应用于整个 wbiService.signParams(...).flatMap(...) 链
                            .retryWhen(Retry.max(1)
                                    .filter(throwable -> throwable instanceof WbiSignatureException)
                                    .doBeforeRetry(retrySignal -> {
                                        log.warn("WBI signature error on getting play url. Invalidating cache and retrying...");
                                        wbiService.invalidateCache();
                                    })
                            );
                });
    }

    @Override
    public Mono<List<Playlist>> getUserPlaylists(String userId) {
        ensureConfigured();
        // API: /x/v3/fav/folder/created/list-all
        // 参数: up_mid (目标用户ID)
        // 注意：移除了 type=2，以获取所有类型的收藏夹
        String favListApi = baseUrl + "/x/v3/fav/folder/created/list-all";

        // 构建 URI
        String uri = UriComponentsBuilder.fromHttpUrl(favListApi)
                .queryParam("up_mid", userId)
                .build()
                .toUriString();

        return buildBilibiliRequest(uri)
                .retrieve()
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> {
                    if (jsonNode.path("code").asInt() != 0) {
                        // 如果用户隐私设置导致无法获取，或者用户不存在，返回空列表而非报错
                        log.warn("Failed to get Bilibili favorites for user {}: {}", userId, jsonNode.path("message").asText());
                        return new ArrayList<>();
                    }

                    List<Playlist> playlists = new ArrayList<>();
                    JsonNode list = jsonNode.path("data").path("list");

                    if (list.isArray()) {
                        list.forEach(fav -> {
                            // 过滤掉媒体数为0的空收藏夹
                            int count = fav.path("media_count").asInt();
                            if (count > 0) {
                                playlists.add(new Playlist(
                                        fav.path("id").asText(), // 这里是 media_id / fid
                                        fav.path("title").asText(),
                                        // B站收藏夹有时候没有封面，可以用默认图，或者取第一张
                                        // cover 字段通常存在
                                        fav.path("cover").asText(),
                                        count,
                                        PLATFORM
                                ));
                            }
                        });
                    }
                    return playlists;
                });
    }

    @Override
    public Mono<List<Music>> getPlaylistMusics(String playlistId, int offset, int limit) {
        ensureConfigured();
        int safeLimit = Math.min(limit, 20);

        int pageNumber = (offset / safeLimit) + 1;

        // API: /x/v3/fav/resource/list
        // 关键参数: media_id (收藏夹ID), pn (页码), ps (页大小), platform=web
        String favDetailApi = baseUrl + "/x/v3/fav/resource/list";

        String uri = UriComponentsBuilder.fromHttpUrl(favDetailApi)
                .queryParam("media_id", playlistId)
                .queryParam("ps", safeLimit)
                .queryParam("pn", pageNumber)
                .build()
                .toUriString();

        return buildBilibiliRequest(uri)
                .retrieve()
                .bodyToMono(JsonNode.class)
                .map(jsonNode -> {
                    List<Music> musicList = new ArrayList<>();

                    int code = jsonNode.path("code").asInt();
                    if (code != 0) {
                        // -404 通常表示空页或没有权限，视为正常结束
                        if (code == -404) return musicList;
                        log.error("Failed to get Bilibili favorite details: {}", jsonNode.path("message").asText());
                        return musicList;
                    }

                    JsonNode medias = jsonNode.path("data").path("medias");
                    // 注意：如果是空文件夹，medias可能是 null
                    if (medias.isArray()) {
                        medias.forEach(media -> {
                            String title = media.path("title").asText();
                            // 过滤失效视频
                            if ("已失效视频".equals(title)) {
                                musicList.add(new Music(
                                        "INVALID_SKIP", // 特殊 ID
                                        "已失效视频",
                                        List.of("Unknown"),
                                        0,
                                        PLATFORM,
                                        ""
                                ));
                                return; // 结束当前循环，继续下一个
                                }

                            // 构造 Music 对象
                            musicList.add(new Music(
                                    media.path("bvid").asText(),
                                    title,
                                    List.of(media.path("upper").path("name").asText()),
                                    media.path("duration").asLong() * 1000,
                                    PLATFORM,
                                    media.path("cover").asText()
                            ));
                        });
                    }
                    return musicList;
                });
    }

    @Override
    public Mono<List<UserSearchResult>> searchUsers(String keyword) {
        ensureConfigured();
        // 1. 准备 WBI 搜索参数
        Map<String, String> params = new HashMap<>();
        params.put("search_type", "bili_user"); // 搜索用户类型
        params.put("keyword", keyword);
        // params.put("page", "1"); // 默认第1页，可选

        // 2. 调用 WBI 签名服务
        return wbiService.signParams(params)
                .flatMap(signedParams -> {
                    // 3. 构建 URL: /x/web-interface/wbi/search/type
                    UriComponentsBuilder builder = UriComponentsBuilder
                            .fromHttpUrl(baseUrl + "/x/web-interface/wbi/search/type");

                    signedParams.forEach(builder::queryParam);

                    return webClient.get()
                            .uri(builder.build().toUri())
                            .header("Cookie", BilibiliCookieSupport.toCookieHeader(sessdata))
                            .header("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
                            .header("Referer", "https://www.bilibili.com/")
                            .retrieve()
                            .bodyToMono(JsonNode.class)
                            .map(jsonNode -> {
                                List<UserSearchResult> users = new ArrayList<>();

                                if (jsonNode.path("code").asInt() != 0) {
                                    log.error("Bilibili user search failed: {}", jsonNode.path("message").asText());
                                    return users;
                                }

                                JsonNode results = jsonNode.path("data").path("result");
                                if (results.isArray()) {
                                    results.forEach(u -> {
                                        String pic = u.path("upic").asText();
                                        if (!pic.startsWith("http")) {
                                            pic = "https:" + pic;
                                        }
                                        users.add(new UserSearchResult(
                                                u.path("mid").asText(),
                                                u.path("uname").asText(),
                                                pic,
                                                PLATFORM
                                        ));
                                    });
                                }
                                return users;
                            });
                });
    }

    @Override
    public Mono<String> getLyric(String musicId) {
        return Mono.just(""); // B站暂时不支持歌词
    }
}
