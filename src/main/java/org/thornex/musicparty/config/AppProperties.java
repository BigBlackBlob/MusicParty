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
        // SQLite has a single writer. Multiple pooled JDBC connections turn
        // concurrent application writes into SQLITE_BUSY failures.
        private int maxPoolSize = 1;
        private int minIdle = 1;
        private long connectionTimeoutMs = 5000;
        private long busyTimeoutMs = 5000;
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
        private int downloadMaxQueuedTasks = 100;
        private int downloadMaxActiveTasks = 3;
        private long downloadReservationBytes = 32L * 1024L * 1024L;
        private int neteaseDownloadConcurrency = 2;
        private int bilibiliDownloadConcurrency = 1;
        private int youtubeDownloadConcurrency = 1;
        private int localDownloadConcurrency = 2;
        private long downloadTaskTtlMs = 30 * 60 * 1000L;
        private int downloadMaxRetries = 3;
        private long downloadRetryInitialDelayMs = 1000;
        private long downloadRetryMaxDelayMs = 30_000;
        private int coverColorCacheSize = 256;
        private int coverColorMaxConcurrent = 4;
        private int websocketClientQueueCapacity = 256;
        private int roomCommandQueueCapacity = 100;
        private long roomCommandQueueTimeoutMs = 5000;
        private int dbWriteQueueCapacity = 100;
        private int dbReadThreads = 2;
        // WebSocket authorization uses this scheduler.  It must absorb a
        // reconnect burst without rejecting otherwise valid handshakes.
        private int dbReadQueueCapacity = 512;
        private int fileIoThreads = 4;
        private int fileIoQueueCapacity = 200;
        private int subprocessThreads = 2;
        private int subprocessQueueCapacity = 20;
        private int imageCpuThreads = Math.max(1, Runtime.getRuntime().availableProcessors());
        private int imageCpuQueueCapacity = 100;
        private int upstreamMaxConcurrent = 4;
        private int upstreamFailureThreshold = 5;
        private long upstreamCircuitOpenMs = 30000;
        private int webClientMaxConnections = 20;
        private int webClientPendingAcquireMax = 50;
        private long webClientPendingAcquireTimeoutMs = 5000;
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
