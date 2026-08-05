package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	accountdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/httpapi"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/media"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/bilibili"
	localplatform "github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/local"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/netease"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/subsonic"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/youtube"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	securitycompat "github.com/BigBlackBlob/MusicParty/backend-go/internal/security"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
)

const (
	proxyMaxIdleConnections        = 64
	proxyMaxIdleConnectionsPerHost = 16
	proxyIdleConnectionTimeout     = 90 * time.Second
	proxyResponseHeaderTimeout     = 15 * time.Second
)

func main() {
	if err := run(); err != nil {
		slog.Error("musicparty stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	applicationContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(os.LookupEnv)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: parseLogLevel(cfg.Application.LogLevel)}))
	slog.SetDefault(logger)

	health := observability.NewHealth()
	metrics := observability.NewMetrics()
	databaseStore, err := openDatabase(applicationContext, cfg)
	if err != nil {
		return err
	}
	if databaseStore != nil {
		defer func() {
			if closeErr := databaseStore.Close(); closeErr != nil {
				logger.Error("close SQLite storage", "error", closeErr)
			}
		}()
	}
	neteaseCookie, bilibiliCookie, navidromeAllowedUsers, err := loadPlatformSettings(applicationContext, cfg, databaseStore)
	if err != nil {
		return err
	}
	platformClient := platform.NewClient(15 * time.Second)
	services := []platform.Service{
		netease.New(platformClient, cfg.Platforms.Netease.BaseURL, neteaseCookie),
		bilibili.New(platformClient, cfg.Platforms.Bilibili.BaseURL, bilibiliCookie),
		youtube.New(platformClient, cfg.Platforms.YouTube.Enabled, cfg.Platforms.YouTube.APIKey, cfg.Platforms.YouTube.MaxResults, cfg.Platforms.YouTube.YTDLPPath),
		subsonic.New(platformClient, subsonic.Config{ID: "navidrome", BaseURL: cfg.Platforms.Navidrome.BaseURL, Username: cfg.Platforms.Navidrome.Username, Password: cfg.Platforms.Navidrome.Password, Client: cfg.Platforms.Navidrome.Client, Version: cfg.Platforms.Navidrome.APIVersion, ProxyPrefix: "/api/navidrome/cover", Enabled: cfg.Platforms.Navidrome.Enabled}),
		subsonic.New(platformClient, subsonic.Config{ID: "subsonic-" + cfg.Platforms.Squidify.ID + "@lounge", Label: cfg.Platforms.Squidify.Label, RoomID: "lounge", BaseURL: cfg.Platforms.Squidify.BaseURL, Username: cfg.Platforms.Squidify.Username, Password: cfg.Platforms.Squidify.Password, Client: cfg.Platforms.Squidify.Client, Version: cfg.Platforms.Squidify.APIVersion, ProxyPrefix: "/api/subsonic/lounge/" + cfg.Platforms.Squidify.ID + "/cover", Enabled: cfg.Platforms.Squidify.Enabled}),
	}
	if databaseStore != nil && cfg.LocalLibrary.Enabled {
		services = append(services, localplatform.New(storesqlite.NewLocalTrackRepository(databaseStore)))
	}
	platformAPI := httpapi.NewPlatformAPI(cfg, services...)
	var authAPI *httpapi.AuthAPI
	var roomAPI *httpapi.RoomAPI
	var playlistAPI *httpapi.PlaylistAPI
	var adminAPI *httpapi.AdminAPI
	var mediaAPI *httpapi.MediaAPI
	var transcoder *media.Transcoder
	var mediaCache *media.Cache
	var webSocketAPI *httpapi.WebSocketAPI
	var realtimeManager *realtime.Manager
	var socketHub *wsruntime.Hub
	if databaseStore != nil {
		accountService := accountdomain.New(databaseStore)
		if cfg.Application.BootstrapAdminUsername != "" || cfg.Application.BootstrapAdminPassword != "" {
			if err := accountService.Bootstrap(applicationContext, cfg.Application.BootstrapAdminUsername, cfg.Application.BootstrapAdminPassword); err != nil {
				return fmt.Errorf("bootstrap administrator: %w", err)
			}
		}
		authAPI = httpapi.NewAuthAPI(cfg, accountService)
		roomService := roomdomain.New(databaseStore, accountService)
		roomAPI = httpapi.NewRoomAPI(roomService, cfg)
		roomAPI.SetAuthService(accountService)
		playlistAPI = httpapi.NewPlaylistAPI(cfg, databaseStore, accountService, roomService, platformAPI)
		adminAPI = httpapi.NewAdminAPI(databaseStore, accountService, platformClient, platformAPI)
		var library *media.LocalLibrary
		if cfg.LocalLibrary.Enabled {
			transcoder = media.NewTranscoder("ffmpeg", cfg.Performance.SubprocessThreads, cfg.Performance.SubprocessQueueCapacity, 30*time.Minute)
			defer transcoder.Close()
			var libraryErr error
			library, libraryErr = media.NewLocalLibrary(cfg.LocalLibrary.Path, cfg.LocalLibrary.MaxUploadBytes, storesqlite.NewLocalTrackRepository(databaseStore), transcoder)
			if libraryErr != nil {
				return fmt.Errorf("initialize local media library: %w", libraryErr)
			}
		}
		proxyClient := newProxyClient()
		cacheBytes, sizeErr := media.ParseByteSize(cfg.Cache.MaxSize)
		if sizeErr != nil {
			return fmt.Errorf("parse media cache size: %w", sizeErr)
		}
		mediaCache, sizeErr = media.NewCache("cached_media", cacheBytes, proxyClient, cfg.Performance.DownloadMaxActiveTasks, cfg.Performance.DownloadMaxQueuedTasks, cfg.Performance.DownloadMaxRetries, cfg.Performance.DownloadRetryInitialDelay, map[string]int{
			"netease": cfg.Performance.NeteaseDownloadConcurrency, "bilibili": cfg.Performance.BilibiliDownloadConcurrency,
			"youtube": cfg.Performance.YouTubeDownloadConcurrency, "local": cfg.Performance.LocalDownloadConcurrency,
		})
		if sizeErr != nil {
			return fmt.Errorf("initialize media cache: %w", sizeErr)
		}
		defer mediaCache.Close()
		mediaAPI = httpapi.NewMediaAPI(cfg, accountService, databaseStore, library, platformAPI, proxyClient, mediaCache)
		profiles := storesqlite.NewUserProfileRepository(databaseStore, time.Now)
		accounts := storesqlite.NewUserAccountRepository(databaseStore)
		platformAPI.SetAccess("navidrome", sessionAccess(profiles, accounts, navidromeAllowedUsers))
		platformAPI.SetAccess("subsonic-"+cfg.Platforms.Squidify.ID+"@lounge", sessionAccess(profiles, accounts, cfg.Platforms.Squidify.AllowedUsers))
		dynamicServices, dynamicAccess, loadErr := loadSubsonicServices(applicationContext, databaseStore, platformClient)
		if loadErr != nil {
			return fmt.Errorf("load Subsonic sources: %w", loadErr)
		}
		for _, service := range dynamicServices {
			platformAPI.Register(service)
		}
		for name, allowed := range dynamicAccess {
			platformAPI.SetAccess(name, sessionAccess(profiles, accounts, allowed))
		}
		socketHub = wsruntime.NewHub(cfg.Performance.WebSocketClientQueueCapacity)
		authAPI.SetSessionCloser(socketHub)
		authAPI.SetSessionPresenceUpdater(socketHub)
		realtimeManager = realtime.NewManager(databaseStore, cfg.Performance.RoomCommandQueueCapacity, cfg.Performance.RoomCommandQueueTimeout, cfg.Player.RoomEvictionIdle, socketHub)
		defer realtimeManager.Close()
		defer socketHub.Close()
		playlistAPI.SetRealtime(realtimeManager)
		webSocketAPI = httpapi.NewWebSocketAPI(cfg, databaseStore, accountService, roomService, socketHub, realtimeManager, platformAPI)
	}
	var handler http.Handler
	staticRoot := os.Getenv("STATIC_PATH")
	if staticRoot == "" {
		staticRoot = "static"
	}
	staticAPI := httpapi.NewStaticAPI(staticRoot)
	if authAPI != nil {
		handler = httpapi.NewHandler(cfg, logger, health, metrics, platformAPI, authAPI, roomAPI, playlistAPI, adminAPI, mediaAPI, webSocketAPI, staticAPI)
	} else {
		handler = httpapi.NewHandler(cfg, logger, health, metrics, platformAPI, staticAPI)
	}
	return serve(applicationContext, cfg, handler, health, logger)
}

func openDatabase(ctx context.Context, cfg config.Config) (*storesqlite.Store, error) {
	if !cfg.Database.Enabled {
		return nil, nil
	}
	databaseContext, cancel := context.WithTimeout(ctx, cfg.Database.ConnectionTimeout)
	defer cancel()
	store, err := storesqlite.OpenStore(databaseContext, storesqlite.StoreConfig{
		Path:               cfg.Database.Path,
		BusyTimeout:        cfg.Database.BusyTimeout,
		ReadConnections:    cfg.Performance.DBReadThreads,
		WriteQueueCapacity: cfg.Performance.DBWriteQueueCapacity,
	})
	if err == nil {
		err = storesqlite.EnsureCompatibleSchema(databaseContext, store, cfg.Database.InitSchema)
	}
	if err != nil {
		if store != nil {
			_ = store.Close()
		}
		return nil, fmt.Errorf("initialize SQLite storage: %w", err)
	}
	if err := ensureSystemRoom(ctx, store); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("ensure system room: %w", err)
	}
	return store, nil
}

func loadPlatformSettings(ctx context.Context, cfg config.Config, store *storesqlite.Store) (string, string, string, error) {
	neteaseCookie := cfg.Platforms.Netease.Cookie
	bilibiliCookie := cfg.Platforms.Bilibili.Sessdata
	navidromeAllowedUsers := cfg.Platforms.Navidrome.AllowedUsers
	if store == nil {
		return neteaseCookie, bilibiliCookie, navidromeAllowedUsers, nil
	}
	settings := storesqlite.NewSiteSettingRepository(store)
	var err error
	if neteaseCookie, err = settingOrDefault(ctx, settings, "netease.cookie", neteaseCookie); err != nil {
		return "", "", "", fmt.Errorf("load Netease cookie setting: %w", err)
	}
	if bilibiliCookie, err = settingOrDefault(ctx, settings, "bilibili.sessdata", bilibiliCookie); err != nil {
		return "", "", "", fmt.Errorf("load Bilibili cookie setting: %w", err)
	}
	if navidromeAllowedUsers, err = settingValue(ctx, settings, "navidrome.allowed-users", navidromeAllowedUsers); err != nil {
		return "", "", "", fmt.Errorf("load Navidrome allowed users setting: %w", err)
	}
	return neteaseCookie, bilibiliCookie, navidromeAllowedUsers, nil
}

func newProxyClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          proxyMaxIdleConnections,
		MaxIdleConnsPerHost:   proxyMaxIdleConnectionsPerHost,
		IdleConnTimeout:       proxyIdleConnectionTimeout,
		ResponseHeaderTimeout: proxyResponseHeaderTimeout,
	}}
}

func serve(ctx context.Context, cfg config.Config, handler http.Handler, health *observability.Health, logger *slog.Logger) error {
	server := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           handler,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", server.Addr, err)
	}
	health.SetReady(true)
	logger.Info("musicparty Go backend listening", "address", listener.Addr().String())

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Serve(listener) }()

	select {
	case <-ctx.Done():
		logger.Info("shutdown requested")
	case serveErr := <-serverErrors:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve: %w", serveErr)
		}
		return nil
	}

	health.SetReady(false)
	shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}

func ensureSystemRoom(ctx context.Context, store *storesqlite.Store) error {
	now := time.Now().UnixMilli()
	return store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `insert into room(id,name,owner_public_id,visibility,password_hash,password_version,system,created_at,last_active_at,deleted_at) values('lounge','Lounge','SYSTEM','PUBLIC',null,0,1,?,?,null) on conflict(id) do nothing`, now, now)
		return err
	})
}

func loadSubsonicServices(ctx context.Context, store *storesqlite.Store, client *platform.Client) ([]platform.Service, map[string]string, error) {
	repository := storesqlite.NewSubsonicSourceRepository(store)
	sources, err := repository.FindAll(ctx)
	if err != nil {
		return nil, nil, err
	}
	bindings, err := repository.FindAllRoomBindings(ctx)
	if err != nil {
		return nil, nil, err
	}
	settings := storesqlite.NewSiteSettingRepository(store)
	siteSecret := ""
	if value, findErr := settings.FindValue(ctx, "security.site-secret.v1"); findErr == nil && value != nil {
		siteSecret = *value
	} else if findErr != nil && !errors.Is(findErr, sql.ErrNoRows) {
		return nil, nil, findErr
	}
	byID := map[string]storesqlite.SubsonicSource{}
	for _, source := range sources {
		byID[source.ID] = source
	}
	result := []platform.Service{}
	access := map[string]string{}
	for _, binding := range bindings {
		source, ok := byID[binding.SourceID]
		if !ok || !source.Enabled || !binding.Enabled {
			continue
		}
		password, decryptErr := securitycompat.DecryptSubsonicCredential(source.Password, siteSecret)
		if decryptErr != nil {
			continue
		}
		name := "subsonic-" + source.ID + "@" + binding.RoomID
		label := source.Label
		if binding.DisplayLabel != nil && strings.TrimSpace(*binding.DisplayLabel) != "" {
			label = *binding.DisplayLabel
		}
		allowed := ""
		if source.AllowedUsers != nil {
			allowed = *source.AllowedUsers
		}
		if binding.AllowedUsers != nil {
			allowed = *binding.AllowedUsers
		}
		result = append(result, subsonic.New(client, subsonic.Config{ID: name, Label: label, RoomID: binding.RoomID, BaseURL: source.BaseURL, Username: source.Username, Password: password, Client: source.Client, Version: source.APIVersion, ProxyPrefix: "/api/subsonic/" + binding.RoomID + "/" + source.ID + "/cover", Enabled: true}))
		access[name] = allowed
	}
	return result, access, nil
}

func settingOrDefault(ctx context.Context, repository *storesqlite.SiteSettingRepository, key, fallback string) (string, error) {
	value, err := repository.FindValue(ctx, key)
	if err == nil && value != nil {
		return *value, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	if strings.TrimSpace(fallback) == "" {
		return fallback, nil
	}
	now := time.Now().UnixMilli()
	if err := repository.Upsert(ctx, storesqlite.SiteSetting{Key: key, Value: &fallback, Secret: true, UpdatedAt: now}); err != nil {
		return "", err
	}
	return fallback, nil
}

func settingValue(ctx context.Context, repository *storesqlite.SiteSettingRepository, key, fallback string) (string, error) {
	value, err := repository.FindValue(ctx, key)
	if err == nil && value != nil {
		return *value, nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	return fallback, nil
}

func sessionAccess(profiles *storesqlite.UserProfileRepository, accounts *storesqlite.UserAccountRepository, rawAllowed string) func(context.Context, string) bool {
	allowed := map[string]struct{}{}
	for _, value := range strings.Split(rawAllowed, ",") {
		if normalized := strings.ToLower(strings.TrimSpace(value)); normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	return func(ctx context.Context, token string) bool {
		if strings.TrimSpace(token) == "" || len(allowed) == 0 {
			return false
		}
		digest := sha256.Sum256([]byte(token))
		session, err := profiles.FindSessionByHash(ctx, hex.EncodeToString(digest[:]))
		if err != nil {
			return false
		}
		account, err := accounts.FindByPublicID(ctx, session.PublicID)
		if err != nil || !account.Enabled {
			return false
		}
		if _, ok := allowed["*"]; ok {
			return true
		}
		_, byID := allowed[strings.ToLower(account.PublicID)]
		_, byName := allowed[strings.ToLower(account.Username)]
		return byID || byName
	}
}

func parseLogLevel(raw string) slog.Level {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN", "WARNING":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
