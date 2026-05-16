package org.thornex.musicparty.config;

import lombok.Data;
import lombok.EqualsAndHashCode;
import org.springframework.boot.context.properties.ConfigurationProperties;

@ConfigurationProperties(prefix = "app.music-api")
@Data
public class AppProperties {
    private NeteaseApiConfig  netease;
    private BilibiliApiConfig bilibili;
    private NavidromeApiConfig navidrome = new NavidromeApiConfig();
    private String adminPassword;
    private String baseUrl;
    private String allowedOrigins;
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
    public static class AuthConfig {
        private boolean rateLimitEnabled = true;
        private int maxAttempts = 5;
        private int windowSeconds = 60;
        private int blockDurationSeconds = 300;
        private long roomAccessTokenTtlMs = 5 * 60 * 1000L;
        private String roomAccessTokenSecret = "";
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
}
