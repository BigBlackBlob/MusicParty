package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type LookupEnv func(string) (string, bool)

type Config struct {
	Server       Server
	Application  Application
	Platforms    Platforms
	Queue        Queue
	Player       Player
	Chat         Chat
	Cache        Cache
	Performance  Performance
	LocalLibrary LocalLibrary
	Database     Database
	Auth         Auth
}

type Server struct {
	Port                int
	ShutdownTimeout     time.Duration
	ReadHeaderTimeout   time.Duration
	IdleTimeout         time.Duration
	MultipartMaxFile    string
	MultipartMaxRequest string
}

type Application struct {
	Mode                   string
	Production             bool
	BaseURL                string
	AllowedOrigins         []string
	AuthorName             string
	BackWords              string
	LogLevel               string
	LegacyAdminPassword    string
	BootstrapAdminUsername string
	BootstrapAdminPassword string
}

type Platforms struct {
	Netease   Netease
	Bilibili  Bilibili
	YouTube   YouTube
	Navidrome Subsonic
	Squidify  Subsonic
}

type Netease struct {
	BaseURL string
	Cookie  string
	Quality string
}

type Bilibili struct {
	BaseURL  string
	Sessdata string
}

type YouTube struct {
	Enabled    bool
	APIKey     string
	YTDLPPath  string
	MaxResults int
}

type Subsonic struct {
	Enabled      bool
	ID           string
	Label        string
	BaseURL      string
	Username     string
	Password     string
	Client       string
	APIVersion   string
	AllowedUsers string
}

type Queue struct {
	MaxSize      int
	HistorySize  int
	MaxUserSongs int
}

type Player struct {
	MaxPlaylistImportSize int
	RoomEvictionIdle      time.Duration
}

type Chat struct {
	MaxHistorySize   int
	MinInterval      time.Duration
	MaxMessageLength int
}

type Cache struct {
	MaxSize string
}

type Performance struct {
	DownloadMaxQueuedTasks         int
	DownloadMaxActiveTasks         int
	DownloadReservationBytes       int64
	NeteaseDownloadConcurrency     int
	BilibiliDownloadConcurrency    int
	YouTubeDownloadConcurrency     int
	LocalDownloadConcurrency       int
	DownloadTaskTTL                time.Duration
	DownloadMaxRetries             int
	DownloadRetryInitialDelay      time.Duration
	DownloadRetryMaxDelay          time.Duration
	CoverColorCacheSize            int
	CoverColorMaxConcurrent        int
	WebSocketClientQueueCapacity   int
	RoomCommandQueueCapacity       int
	RoomCommandQueueTimeout        time.Duration
	DBWriteQueueCapacity           int
	DBReadThreads                  int
	DBReadQueueCapacity            int
	FileIOThreads                  int
	FileIOQueueCapacity            int
	SubprocessThreads              int
	SubprocessQueueCapacity        int
	ImageCPUThreads                int
	ImageCPUQueueCapacity          int
	UpstreamMaxConcurrent          int
	UpstreamFailureThreshold       int
	UpstreamCircuitOpen            time.Duration
	WebClientMaxConnections        int
	WebClientPendingAcquireMax     int
	WebClientPendingAcquireTimeout time.Duration
}

type LocalLibrary struct {
	Enabled               bool
	Path                  string
	AllowedUsers          string
	MaxUploadBytes        int64
	MaxEmbeddedCoverBytes int64
}

type Database struct {
	Enabled           bool
	Path              string
	InitSchema        bool
	MaxPoolSize       int
	MinIdle           int
	ConnectionTimeout time.Duration
	BusyTimeout       time.Duration
}

type Auth struct {
	RateLimitEnabled      bool
	MaxAttempts           int
	Window                time.Duration
	BlockDuration         time.Duration
	MaxTrackedIPs         int
	TrustedProxyCIDRs     []string
	RoomAccessTokenTTL    time.Duration
	RoomAccessTokenSecret string
	SecureCookies         bool
}

func Load(lookup LookupEnv) (Config, error) {
	if lookup == nil {
		return Config{}, errors.New("environment lookup is required")
	}
	r := reader{lookup: lookup}
	cfg := Config{
		Server: Server{
			Port:                r.int("SERVER_PORT", 8080),
			ShutdownTimeout:     r.durationMS("SHUTDOWN_TIMEOUT_MS", 15_000),
			ReadHeaderTimeout:   r.durationMS("HTTP_READ_HEADER_TIMEOUT_MS", 10_000),
			IdleTimeout:         r.durationMS("HTTP_IDLE_TIMEOUT_MS", 60_000),
			MultipartMaxFile:    r.string("MULTIPART_MAX_FILE_SIZE", "200MB"),
			MultipartMaxRequest: r.string("MULTIPART_MAX_REQUEST_SIZE", "220MB"),
		},
		Application: Application{
			Mode:                   r.string("APP_MODE", "server"),
			BaseURL:                r.string("BASE_URL", "http://localhost:8080"),
			AllowedOrigins:         r.list("ALLOWED_ORIGINS"),
			AuthorName:             r.string("APP_AUTHOR_NAME", "ThorNex"),
			BackWords:              r.string("APP_BACK_WORDS", "THORNEX"),
			LogLevel:               r.string("LOG_LEVEL", "INFO"),
			LegacyAdminPassword:    r.string("ADMIN_PASSWORD", ""),
			BootstrapAdminUsername: r.string("BOOTSTRAP_ADMIN_USERNAME", ""),
			BootstrapAdminPassword: r.string("BOOTSTRAP_ADMIN_PASSWORD", ""),
		},
		Platforms: Platforms{
			Netease: Netease{
				BaseURL: r.string("NETEASE_API_URL", "http://netease-api:3000"),
				Cookie:  r.string("NETEASE_COOKIE", ""),
				Quality: r.string("NETEASE_QUALITY", "exhigh"),
			},
			Bilibili: Bilibili{BaseURL: "https://api.bilibili.com", Sessdata: r.string("BILIBILI_SESSDATA", "")},
			YouTube: YouTube{
				Enabled: r.bool("YOUTUBE_ENABLED", true), APIKey: r.string("YOUTUBE_API_KEY", ""),
				YTDLPPath: r.string("YTDLP_PATH", "yt-dlp"), MaxResults: r.int("YOUTUBE_SEARCH_LIMIT", 20),
			},
			Navidrome: Subsonic{
				Enabled: r.bool("NAVIDROME_ENABLED", false), ID: "navidrome", Label: "Navidrome",
				BaseURL: r.string("NAVIDROME_BASE_URL", "http://navidrome:4533"), Username: r.string("NAVIDROME_USERNAME", ""),
				Password: r.string("NAVIDROME_PASSWORD", ""), Client: r.string("NAVIDROME_CLIENT", "musicparty"),
				APIVersion: r.string("NAVIDROME_API_VERSION", "1.16.1"), AllowedUsers: r.string("NAVIDROME_ALLOWED_USERS", ""),
			},
			Squidify: Subsonic{
				Enabled: r.bool("SQUIDIFY_ENABLED", true), ID: r.string("SQUIDIFY_ID", "squidify"), Label: r.string("SQUIDIFY_LABEL", "Squidify"),
				BaseURL: r.string("SQUIDIFY_BASE_URL", ""), Username: r.string("SQUIDIFY_USERNAME", ""), Password: r.string("SQUIDIFY_PASSWORD", ""),
				Client: r.string("SQUIDIFY_CLIENT", "musicparty"), APIVersion: r.string("SQUIDIFY_API_VERSION", "1.16.1"),
				AllowedUsers: r.string("SQUIDIFY_ALLOWED_USERS", "*"),
			},
		},
		Queue:  Queue{MaxSize: r.int("QUEUE_MAX_SIZE", 1000), HistorySize: r.int("QUEUE_HISTORY_SIZE", 50), MaxUserSongs: r.int("QUEUE_MAX_USER_SONGS", 100)},
		Player: Player{MaxPlaylistImportSize: r.int("PLAYLIST_IMPORT_LIMIT", 100), RoomEvictionIdle: r.durationMS("ROOM_EVICTION_IDLE_MS", 3_600_000)},
		Chat:   Chat{MaxHistorySize: r.int("CHAT_HISTORY_LIMIT", 1000), MinInterval: r.durationMS("CHAT_MIN_INTERVAL", 1000), MaxMessageLength: r.int("CHAT_MAX_LENGTH", 200)},
		Cache:  Cache{MaxSize: r.string("CACHE_MAX_SIZE", "1GB")},
		Performance: Performance{
			DownloadMaxQueuedTasks: r.int("DOWNLOAD_MAX_QUEUED_TASKS", 100),
			DownloadMaxActiveTasks: r.int("DOWNLOAD_MAX_ACTIVE_TASKS", 3), DownloadReservationBytes: r.int64("DOWNLOAD_RESERVATION_BYTES", 33_554_432),
			NeteaseDownloadConcurrency: r.int("NETEASE_DOWNLOAD_CONCURRENCY", 2), BilibiliDownloadConcurrency: r.int("BILIBILI_DOWNLOAD_CONCURRENCY", 1),
			YouTubeDownloadConcurrency: r.int("YOUTUBE_DOWNLOAD_CONCURRENCY", 1), LocalDownloadConcurrency: r.int("LOCAL_DOWNLOAD_CONCURRENCY", 2),
			DownloadTaskTTL: r.durationMS("DOWNLOAD_TASK_TTL_MS", 1_800_000), DownloadMaxRetries: r.int("DOWNLOAD_MAX_RETRIES", 3),
			DownloadRetryInitialDelay: r.durationMS("DOWNLOAD_RETRY_INITIAL_DELAY_MS", 1000), DownloadRetryMaxDelay: r.durationMS("DOWNLOAD_RETRY_MAX_DELAY_MS", 30_000),
			CoverColorCacheSize: r.int("COVER_COLOR_CACHE_SIZE", 256), CoverColorMaxConcurrent: r.int("COVER_COLOR_MAX_CONCURRENT", 4),
			WebSocketClientQueueCapacity: r.int("WEBSOCKET_CLIENT_QUEUE_CAPACITY", 256), RoomCommandQueueCapacity: r.int("ROOM_COMMAND_QUEUE_CAPACITY", 100),
			RoomCommandQueueTimeout: r.durationMS("ROOM_COMMAND_QUEUE_TIMEOUT_MS", 5000), DBWriteQueueCapacity: r.int("DB_WRITE_QUEUE_CAPACITY", 100),
			DBReadThreads: r.int("DB_READ_THREADS", 2), DBReadQueueCapacity: r.int("DB_READ_QUEUE_CAPACITY", 512), FileIOThreads: r.int("FILE_IO_THREADS", 4),
			FileIOQueueCapacity: r.int("FILE_IO_QUEUE_CAPACITY", 200), SubprocessThreads: r.int("SUBPROCESS_THREADS", 2), SubprocessQueueCapacity: r.int("SUBPROCESS_QUEUE_CAPACITY", 20),
			ImageCPUThreads: r.int("IMAGE_CPU_THREADS", 2), ImageCPUQueueCapacity: r.int("IMAGE_CPU_QUEUE_CAPACITY", 100), UpstreamMaxConcurrent: r.int("UPSTREAM_MAX_CONCURRENT", 4),
			UpstreamFailureThreshold: r.int("UPSTREAM_FAILURE_THRESHOLD", 5), UpstreamCircuitOpen: r.durationMS("UPSTREAM_CIRCUIT_OPEN_MS", 30_000),
			WebClientMaxConnections: r.int("WEB_CLIENT_MAX_CONNECTIONS", 20), WebClientPendingAcquireMax: r.int("WEB_CLIENT_PENDING_ACQUIRE_MAX", 50),
			WebClientPendingAcquireTimeout: r.durationMS("WEB_CLIENT_PENDING_ACQUIRE_TIMEOUT_MS", 5000),
		},
		LocalLibrary: LocalLibrary{Enabled: r.bool("LOCAL_LIBRARY_ENABLED", true), Path: r.string("LOCAL_LIBRARY_PATH", "data/local-library"), AllowedUsers: r.string("LOCAL_LIBRARY_ALLOWED_USERS", ""), MaxUploadBytes: r.int64("LOCAL_LIBRARY_MAX_UPLOAD_BYTES", 209_715_200), MaxEmbeddedCoverBytes: r.int64("LOCAL_LIBRARY_MAX_EMBEDDED_COVER_BYTES", 1_048_576)},
		Database:     Database{Enabled: r.bool("DB_ENABLED", true), Path: r.string("DB_PATH", "data/musicparty.db"), InitSchema: r.bool("DB_INIT_SCHEMA", true), MaxPoolSize: r.int("DB_MAX_POOL_SIZE", 1), MinIdle: r.int("DB_MIN_IDLE", 1), ConnectionTimeout: r.durationMS("DB_CONNECTION_TIMEOUT_MS", 5000), BusyTimeout: r.durationMS("SQLITE_BUSY_TIMEOUT_MS", 5000)},
		Auth:         Auth{RateLimitEnabled: r.bool("AUTH_RATE_LIMIT_ENABLED", true), MaxAttempts: r.int("AUTH_MAX_ATTEMPTS", 5), Window: r.durationSeconds("AUTH_WINDOW_SECONDS", 60), BlockDuration: r.durationSeconds("AUTH_BLOCK_DURATION", 300), MaxTrackedIPs: r.int("AUTH_MAX_TRACKED_IPS", 10_000), TrustedProxyCIDRs: r.list("TRUSTED_PROXY_CIDRS"), RoomAccessTokenTTL: r.durationMS("ROOM_ACCESS_TOKEN_TTL_MS", 300_000), RoomAccessTokenSecret: r.string("ROOM_ACCESS_TOKEN_SECRET", ""), SecureCookies: r.bool("AUTH_SECURE_COOKIES", true)},
	}
	cfg.Application.Production = looksLikeProduction(
		cfg.Application,
		r.string("APP_ENV", ""),
		r.string("ENV", ""),
		r.string("NODE_ENV", ""),
		r.string("SPRING_PROFILES_ACTIVE", ""),
	)
	if err := r.err(); err != nil {
		return Config{}, err
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) Validate() error {
	var problems []string
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		problems = append(problems, "SERVER_PORT must be between 1 and 65535")
	}
	baseURL, err := url.Parse(c.Application.BaseURL)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		problems = append(problems, "BASE_URL must be an absolute HTTP(S) URL")
	}
	for _, origin := range c.Application.AllowedOrigins {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
			problems = append(problems, fmt.Sprintf("ALLOWED_ORIGINS contains invalid origin %q", origin))
		}
	}
	if strings.EqualFold(c.Application.Mode, "server") && c.Application.Production {
		if isLocalhostURL(c.Application.BaseURL) {
			problems = append(problems, "BASE_URL must not point to localhost in server production deployments")
		}
		if !hasPublicOrigin(c.Application.AllowedOrigins) {
			problems = append(problems, "ALLOWED_ORIGINS must explicitly include the public origin in server production deployments")
		}
	}
	if c.Queue.MaxSize <= 0 || c.Performance.WebSocketClientQueueCapacity <= 0 || c.Performance.RoomCommandQueueCapacity <= 0 || c.Performance.DBWriteQueueCapacity <= 0 {
		problems = append(problems, "queue capacities must be positive")
	}
	if c.Database.ConnectionTimeout <= 0 || c.Database.BusyTimeout < 0 {
		problems = append(problems, "database timeouts are invalid")
	}
	for _, cidr := range c.Auth.TrustedProxyCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			problems = append(problems, fmt.Sprintf("TRUSTED_PROXY_CIDRS contains invalid CIDR %q", cidr))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

func looksLikeProduction(application Application, hints ...string) bool {
	if !strings.EqualFold(application.Mode, "server") {
		return false
	}
	for _, hint := range hints {
		for _, value := range strings.Split(hint, ",") {
			value = strings.TrimSpace(value)
			if strings.EqualFold(value, "prod") || strings.EqualFold(value, "production") {
				return true
			}
		}
	}
	return hasPublicOrigin(application.AllowedOrigins) || (application.BaseURL != "" && !isLocalhostURL(application.BaseURL))
}

func hasPublicOrigin(origins []string) bool {
	for _, origin := range origins {
		if !isLocalhostURL(origin) {
			return true
		}
	}
	return false
}

func isLocalhostURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	switch strings.ToLower(parsed.Hostname()) {
	case "localhost", "127.0.0.1", "0.0.0.0", "::1":
		return true
	default:
		return false
	}
}

type reader struct {
	lookup LookupEnv
	errors []string
}

func (r *reader) raw(name string) (string, bool) {
	value, ok := r.lookup(name)
	return strings.TrimSpace(value), ok
}

func (r *reader) string(name, fallback string) string {
	if value, ok := r.raw(name); ok {
		return value
	}
	return fallback
}

func (r *reader) list(name string) []string {
	value, ok := r.raw(name)
	if !ok || value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func (r *reader) bool(name string, fallback bool) bool {
	value, ok := r.raw(name)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		r.errors = append(r.errors, fmt.Sprintf("%s must be a boolean", name))
		return fallback
	}
	return parsed
}

func (r *reader) int(name string, fallback int) int {
	value, ok := r.raw(name)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		r.errors = append(r.errors, fmt.Sprintf("%s must be an integer", name))
		return fallback
	}
	return parsed
}

func (r *reader) int64(name string, fallback int64) int64 {
	value, ok := r.raw(name)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		r.errors = append(r.errors, fmt.Sprintf("%s must be an integer", name))
		return fallback
	}
	return parsed
}

func (r *reader) durationMS(name string, fallback int64) time.Duration {
	return time.Duration(r.int64(name, fallback)) * time.Millisecond
}

func (r *reader) durationSeconds(name string, fallback int64) time.Duration {
	return time.Duration(r.int64(name, fallback)) * time.Second
}

func (r *reader) err() error {
	if len(r.errors) == 0 {
		return nil
	}
	return errors.New(strings.Join(r.errors, "; "))
}
