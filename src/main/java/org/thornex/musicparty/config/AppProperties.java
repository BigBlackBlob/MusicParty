package org.thornex.musicparty.config;

import lombok.Data;
import lombok.EqualsAndHashCode;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "app.music-api")
@Data
public class AppProperties {
    private NeteaseApiConfig  netease;
    private BilibiliApiConfig bilibili;
    private YoutubeApiConfig youtube = new YoutubeApiConfig();
    private NavidromeApiConfig navidrome = new NavidromeApiConfig();
    private SquidifyConfig squidify = new SquidifyConfig();
    private String adminPassword;
    private String bootstrapAdminUsername = "";
    private String bootstrapAdminPassword = "";
    private String baseUrl;
    private String allowedOrigins;
    private String mode = "server";
    private String authorName = "ThorNex";
    private String backWords = "THORNEX";
    private String ffmpegPath = "ffmpeg"; // 默认使用环境变量中的 ffmpeg

    // 新增配置项
    private QueueConfig queue = new QueueConfig();
    private PlayerConfig player = new PlayerConfig();
    private ChatConfig chat = new ChatConfig();
    private CacheConfig cache = new CacheConfig();
    private AuthConfig auth = new AuthConfig();
    private DatabaseConfig database = new DatabaseConfig();
    private LocalLibraryConfig localLibrary = new LocalLibraryConfig();
    private PerformanceConfig performance = new PerformanceConfig();
    private DesktopConfig desktop = new DesktopConfig();

    @Data
    public static class QueueConfig {
        private int maxSize = 1000;
        private int historySize = 50;
        private int maxUserSongs = 100;
    }

    @Data
    public static class PlayerConfig {
        private int maxPlaylistImportSize = 50;
        private long roomEvictionIdleMs = 60 * 60 * 1000L;
    }

    @Data
    public static class ChatConfig {
        private int maxHistorySize = 1000;
        private long minIntervalMs = 1000;
        private int maxMessageLength = 200;
    }

    @Data
    public static class CacheConfig {
        private org.springframework.util.unit.DataSize maxSize = org.springframework.util.unit.DataSize.ofGigabytes(1);
    }

    @Data
    public static class DatabaseConfig {
        private boolean enabled = true;
        private String path = "data/musicparty.db";
        private boolean initSchema = true;
    }

    @Data
    public static class LocalLibraryConfig {
        private boolean enabled = true;
        private String path = "data/local-library";
        private String allowedUsers = "";
        private long maxUploadBytes = 200L * 1024L * 1024L;
        private long maxEmbeddedCoverBytes = 1024L * 1024L;
    }

    @Data
    public static class PerformanceConfig {
        private int streamMaxListeners = 50;
        private int streamClientQueueCapacity = 32;
        private int streamWriterThreads = 8;
        private int downloadMaxQueuedTasks = 100;
        private long downloadTaskTtlMs = 30 * 60 * 1000L;
        private int coverColorCacheSize = 256;
        private int coverColorMaxConcurrent = 4;
    }

    @Data
    public static class DesktopConfig {
        private String profileDir = "data/desktop-profile";
        private String lanBaseUrl = "";
        private int neteaseApiPort = 3000;
        private boolean adminInitialized = false;
    }

    @Data
    public static class AuthConfig {
        private boolean rateLimitEnabled = true;
        private int maxAttempts = 5;
        private int windowSeconds = 60;
        private int blockDurationSeconds = 300;
        private int maxTrackedIps = 10000;
        private String trustedProxyCidrs = "";
        private long roomAccessTokenTtlMs = 5 * 60 * 1000L;
        private String roomAccessTokenSecret = "";
        private boolean secureCookies = true;
    }

    @Data
    public static class ApiConfig {
        private String baseUrl;
    }

    @EqualsAndHashCode(callSuper = true)
    @Data
    public static class BilibiliApiConfig extends ApiConfig {
        private String sessdata;
    }

    @Data
    public static class YoutubeApiConfig {
        private boolean enabled = true;
        private String apiKey = "";
        private String ytDlpPath = "yt-dlp";
        private int maxResults = 20;
    }

    @EqualsAndHashCode(callSuper = true)
    @Data
    public static class NeteaseApiConfig extends ApiConfig {
        private String cookie;
        private String quality = "exhigh"; // 默认音质：极高 (exhigh)
    }

    @EqualsAndHashCode(callSuper = true)
    @Data
    public static class NavidromeApiConfig extends ApiConfig {
        private boolean enabled = false;
        private String username;
        private String password;
        private String client = "musicparty";
        private String apiVersion = "1.16.1";
        private String allowedUsers = "";
    }

    @EqualsAndHashCode(callSuper = true)
    @Data
    public static class SquidifyConfig extends ApiConfig {
        private boolean enabled = true;
        private String id = "squidify";
        private String label = "Squidify";
        private String username = "";
        private String password = "";
        private String client = "musicparty";
        private String apiVersion = "1.16.1";
        private String allowedUsers = "*";
    }
}
