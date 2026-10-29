package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/media"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/netease"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

func TestSubsonicSourceUpdateSeparatesDiskCacheAcrossRestart(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, r.URL.Query().Get("u"))
	}))
	defer upstream.Close()
	cfg := testConfig(t)
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "media-token", "listener", "MEMBER")
	client := platform.NewClient(time.Second)
	source := storesqlite.SubsonicSource{ID: "shared", BaseURL: upstream.URL, Username: "old-account", Enabled: true}
	allowed := "listener"
	source.AllowedUsers = &allowed
	bindings := []storesqlite.RoomSubsonicSource{{SourceID: "shared", RoomID: "lounge", Enabled: true}}
	platforms := NewPlatformAPI(cfg)
	platforms.replaceSubsonicSource(client, accounts, source, bindings, "password")
	root := t.TempDir()
	cache, err := media.NewCache(root, 4096, upstream.Client(), 1, 4, 0, 0)
	require.NoError(t, err)
	defer cache.Close()
	api := NewMediaAPI(cfg, accounts, store, nil, platforms, upstream.Client(), cache)
	request := func() string {
		t.Helper()
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "media-token"})
		require.NoError(t, api.proxyPlatform(w, r, "subsonic-shared@lounge", "song", false))
		require.Equal(t, http.StatusOK, w.Code)
		return w.Body.String()
	}
	require.Equal(t, "old-account", request())
	require.Eventually(t, func() bool { files, _ := filepath.Glob(filepath.Join(root, "*.media")); return len(files) > 0 }, time.Second, time.Millisecond)
	source.Username = "new-account"
	platforms.replaceSubsonicSource(client, accounts, source, bindings, "password")
	require.Equal(t, "new-account", request(), "live source replacement must ignore the old cache")
	require.Eventually(t, func() bool { files, _ := filepath.Glob(filepath.Join(root, "*.media")); return len(files) == 2 }, time.Second, time.Millisecond)
	cache.Close()
	// Reopen the same on-disk cache and registry, as on process restart.
	cache, err = media.NewCache(root, 4096, upstream.Client(), 1, 4, 0, 0)
	require.NoError(t, err)
	defer cache.Close()
	api.cache = cache
	source.Username = "new-account"
	platforms = NewPlatformAPI(cfg)
	platforms.replaceSubsonicSource(client, accounts, source, bindings, "password")
	api.platforms = platforms
	require.Equal(t, "new-account", request())
}

func TestWebSocketPlaylistImportPagesBeyondProviderLimit(t *testing.T) {
	for _, total := range []int{5, 1000} {
		t.Run(strconv.Itoa(total), func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
				limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
				songs := []map[string]any{}
				for i := offset; i < min(total, offset+limit); i++ {
					songs = append(songs, map[string]any{"id": i + 1, "name": fmt.Sprint(i + 1), "dt": 60000, "ar": []map[string]string{{"name": "Artist"}}})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"songs": songs})
			}))
			defer upstream.Close()
			store := httpStore(t)
			accounts := account.New(store)
			insertStage5User(t, store, "member-token", "member", "MEMBER")
			insertStage5Room(t, store, "lounge", "PUBLIC", 0)
			cfg := testConfig(t)
			cfg.Player.MaxPlaylistImportSize = 1000
			platforms := NewPlatformAPI(cfg, netease.New(platform.NewClient(time.Second), upstream.URL, "cookie"))
			hub := wsruntime.NewHub(256)
			manager := realtime.NewManager(store, 100, time.Second, time.Hour, hub, realtime.QueueLimits{MaxSize: 2000, MaxUserSongs: 2000, HistorySize: 100})
			defer hub.Close()
			defer manager.Close()
			api := NewWebSocketAPI(cfg, store, accounts, roomdomain.New(store, accounts), hub, manager, platforms)
			server := httptest.NewServer(NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api))
			defer server.Close()
			connection := dialWebSocket(t, server, "member-token", "lounge")
			defer connection.CloseNow()
			connection.SetReadLimit(2 << 20)
			readUntilType(t, connection, "player.state")
			require.NoError(t, connection.Write(context.Background(), websocket.MessageText, []byte(`{"type":"enqueue.playlist","payload":{"platform":"netease","playlistId":"list","mutationId":"import"}}`)))
			// Both success and failure end in an enqueue response, so the regression fails immediately.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			for {
				_, data, err := connection.Read(ctx)
				require.NoError(t, err)
				var envelope struct {
					Type    string `json:"type"`
					Payload struct {
						Count int `json:"count"`
					} `json:"payload"`
				}
				require.NoError(t, json.Unmarshal(data, &envelope))
				if envelope.Type == "enqueue.nack" {
					t.Fatalf("import rejected: %s", data)
				}
				if envelope.Type == "enqueue.ack" {
					require.Equal(t, total, envelope.Payload.Count)
					break
				}
			}
		})
	}
}

type sourceSnapshotFixture struct {
	fixturePlatform
	namespace string
	started   chan struct{}
	release   chan struct{}
}

func (s *sourceSnapshotFixture) CacheNamespace() string { return s.namespace }
func (s *sourceSnapshotFixture) ResolvePlayable(ctx context.Context, id string) (platform.PlayableMusic, error) {
	if s.started != nil {
		close(s.started)
		select {
		case <-s.release:
		case <-ctx.Done():
			return platform.PlayableMusic{}, ctx.Err()
		}
	}
	return platform.PlayableMusic{Music: platform.Music{ID: id, Name: s.namespace}}, nil
}

func TestSourceUpdateCannotBeOverwrittenByOldPlayableRequest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	old := &sourceSnapshotFixture{namespace: "old", started: make(chan struct{}), release: make(chan struct{})}
	api := NewPlatformAPI(testConfig(t), old)
	done := make(chan error, 1)
	go func() { _, err := api.ResolvePlayable(ctx, "local", "song", ""); done <- err }()
	select {
	case <-old.started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	api.Register(&sourceSnapshotFixture{namespace: "new"})
	resolved, err := api.ResolvePlayable(ctx, "local", "song", "")
	require.NoError(t, err)
	require.Equal(t, "new", resolved.Name)
	close(old.release)
	require.NoError(t, <-done)
	resolved, err = api.ResolvePlayable(ctx, "local", "song", "")
	require.NoError(t, err)
	require.Equal(t, "new", resolved.Name)
}
