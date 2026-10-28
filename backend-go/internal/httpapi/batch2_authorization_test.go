package httpapi

import (
	"context"
	"database/sql"
	"github.com/go-chi/chi/v5"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	wsruntime "github.com/BigBlackBlob/MusicParty/backend-go/internal/ws"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/require"
)

type restrictedFixture struct {
	fixturePlatform
	calls atomic.Int32
}

func (*restrictedFixture) Name() string { return "navidrome" }
func (f *restrictedFixture) PlaylistSongs(context.Context, string, int, int) ([]platform.Music, error) {
	f.calls.Add(1)
	return []platform.Music{{ID: "song", Name: "Song", Platform: "navidrome", Artists: []string{"Artist"}}}, nil
}
func (f *restrictedFixture) AlbumSongs(ctx context.Context, id string) ([]platform.Music, error) {
	return f.PlaylistSongs(ctx, id, 0, 1)
}
func (*restrictedFixture) StreamURL(context.Context, string) (string, error) { return "", nil }
func (*restrictedFixture) CoverURL(context.Context, string) (string, error)  { return "", nil }

func batch2Request(handler http.Handler, method, path, body, token, proof string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(CSRFHeaderName, "csrf")
	r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "csrf"})
	if token != "" {
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
	}
	if proof != "" {
		r.AddCookie(&http.Cookie{Name: RoomAccessCookieName, Value: proof})
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	return w
}

func TestPlatformAllowlistLiveUpdateMatchesReload(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "admin-token", "admin", "PLATFORM_ADMIN")
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5User(t, store, "disabled-token", "disabled", "MEMBER")
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update user_account set enabled=0 where public_id='disabled'")
		return err
	}))
	guest, err := accounts.CreateGuestSession(context.Background(), "Guest")
	require.NoError(t, err)
	cfg := testConfig(t)
	platforms := NewPlatformAPI(cfg, &restrictedFixture{})
	admin := NewAdminAPI(store, accounts, platform.NewClient(0), platforms)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), admin)
	for _, allowed := range []string{"*", guest.PublicID, "MEMBER", "nobody"} {
		t.Run(allowed, func(t *testing.T) {
			empty := ""
			require.NoError(t, storesqlite.NewSiteSettingRepository(store).Upsert(context.Background(), storesqlite.SiteSetting{Key: "navidrome.allowed-users", Value: &empty, UpdatedAt: time.Now().UnixMilli()}))
			response := batch2Request(handler, http.MethodPost, "/api/admin/navidrome-access/grant", `{"userName":"`+allowed+`"}`, "admin-token", "")
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			saved, err := storesqlite.NewSiteSettingRepository(store).FindValue(context.Background(), "navidrome.allowed-users")
			require.NoError(t, err)
			require.NotNil(t, saved)
			reloaded := PlatformAccess(accounts, *saved)
			for _, test := range []struct {
				token string
				want  bool
			}{
				{"admin-token", allowed == "*"},
				{"member-token", allowed == "*" || allowed == "MEMBER"},
				{guest.SessionToken, allowed == "*" || allowed == guest.PublicID},
				{"disabled-token", false}, {"unknown", false}, {"", false},
			} {
				require.Equal(t, test.want, platforms.PlatformAllowed(context.Background(), "navidrome", test.token), test.token)
				require.Equal(t, test.want, reloaded(context.Background(), test.token), test.token)
			}
			response = batch2Request(handler, http.MethodPost, "/api/admin/navidrome-access/revoke", `{"userName":"`+allowed+`"}`, "admin-token", "")
			require.Equal(t, http.StatusOK, response.Code)
			require.False(t, platforms.PlatformAllowed(context.Background(), "navidrome", guest.SessionToken))
			require.False(t, platforms.PlatformAllowed(context.Background(), "navidrome", "member-token"))
		})
	}
}

func TestPlaylistImportsUsePlatformAuthorization(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "room", "PUBLIC", 0)
	cfg := testConfig(t)
	service := &restrictedFixture{}
	platforms := NewPlatformAPI(cfg, service)
	api := NewPlaylistAPI(cfg, store, accounts, roomdomain.New(store, accounts), platforms)
	userPlaylist, err := api.users.CreatePlaylist(context.Background(), "member", "Personal")
	require.NoError(t, err)
	roomPlaylist, err := api.roomPlaylists.CreatePlaylist(context.Background(), "room", "Room")
	require.NoError(t, err)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	for _, path := range []string{"/api/me/playlists/" + userPlaylist.ID + "/import", "/api/rooms/room/playlists/" + roomPlaylist.ID + "/import"} {
		platforms.SetAccess("navidrome", PlatformAccess(accounts, "nobody"))
		before := service.calls.Load()
		denied := batch2Request(handler, http.MethodPost, path, `{"platform":"navidrome","playlistId":"remote"}`, "member-token", "")
		require.Equal(t, http.StatusForbidden, denied.Code, denied.Body.String())
		require.Equal(t, before, service.calls.Load(), "denied import must not call provider")
		platforms.SetAccess("navidrome", PlatformAccess(accounts, "member"))
		allowed := batch2Request(handler, http.MethodPost, path, `{"platform":"navidrome","playlistId":"remote"}`, "member-token", "")
		require.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())
		require.Equal(t, before+1, service.calls.Load())
	}
}

func TestPrivateRoomPlaylistAuthorizationMatchesWebSocket(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	for _, user := range []struct{ id, role string }{{"owner", "MEMBER"}, {"member", "MEMBER"}, {"admin", "PLATFORM_ADMIN"}} {
		insertStage5User(t, store, user.id+"-token", user.id, user.role)
	}
	guest, err := accounts.CreateGuestSession(context.Background(), "Guest")
	require.NoError(t, err)
	insertStage5Room(t, store, "private-room", "PRIVATE", 3)
	insertStage5Room(t, store, "other-room", "PRIVATE", 3)
	cfg := testConfig(t)
	api := NewPlaylistAPI(cfg, store, accounts, roomdomain.New(store, accounts), NewPlatformAPI(cfg))
	ws := NewWebSocketAPI(cfg, store, accounts, nil, nil, nil, nil)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	proof := func(room, id string, version int, expiry time.Time) string {
		return signRoomAccessToken(api.roomSecret, room, id, expiry.UnixMilli(), version)
	}
	future := time.Now().Add(time.Minute)
	for _, test := range []struct {
		name, token, publicID, proof string
		allowed                      bool
	}{
		{"owner", "owner-token", "owner", "", true},
		{"admin", "admin-token", "admin", "", true},
		{"member without proof", "member-token", "member", "", false},
		{"member with proof", "member-token", "member", proof("private-room", "member", 3, future), true},
		{"wrong room", "member-token", "member", proof("other-room", "member", 3, future), false},
		{"wrong user", "member-token", "member", proof("private-room", "owner", 3, future), false},
		{"old version", "member-token", "member", proof("private-room", "member", 2, future), false},
		{"expired", "member-token", "member", proof("private-room", "member", 3, time.Now().Add(-time.Minute)), false},
		{"guest without proof", guest.SessionToken, guest.PublicID, "", false},
		{"guest with proof", guest.SessionToken, guest.PublicID, proof("private-room", guest.PublicID, 3, future), true},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, err := accounts.Resolve(context.Background(), test.token)
			require.NoError(t, err)
			request := httptest.NewRequest(http.MethodGet, "/ws", nil)
			request.AddCookie(&http.Cookie{Name: RoomAccessCookieName, Value: test.proof})
			require.Equal(t, test.allowed, ws.canEnterRoom(request, "private-room", session))
			want := http.StatusForbidden
			if test.allowed {
				want = http.StatusOK
			}
			for _, method := range []string{http.MethodGet, http.MethodPost} {
				response := batch2Request(handler, method, "/api/rooms/private-room/playlists", `{"name":"Shared"}`, test.token, test.proof)
				require.Equal(t, want, response.Code, response.Body.String())
			}
		})
	}
}

func TestPlatformPolicyCallbackRunsOutsideRegistryLock(t *testing.T) {
	api := NewPlatformAPI(testConfig(t), fixturePlatform{})
	api.SetAccess("local", func(context.Context, string) bool {
		api.SetAccess("local", func(context.Context, string) bool { return false })
		return true
	})
	require.True(t, api.PlatformAllowed(context.Background(), "local", "token"))
	require.False(t, api.PlatformAllowed(context.Background(), "local", "token"))
}

func TestWebSocketRemoteCollectionsRespectPlatformRevocation(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "lounge", "PUBLIC", 0)
	cfg := testConfig(t)
	service := &restrictedFixture{}
	platforms := NewPlatformAPI(cfg, service)
	hub := wsruntime.NewHub(256)
	manager := realtime.NewManager(store, 100, time.Second, time.Hour, hub)
	api := NewWebSocketAPI(cfg, store, accounts, roomdomain.New(store, accounts), hub, manager, platforms)
	server := httptest.NewServer(NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api))
	defer server.Close()
	defer manager.Close()
	defer hub.Close()
	connection := dialWebSocket(t, server, "member-token", "lounge")
	defer connection.CloseNow()
	readUntilType(t, connection, "player.state")
	for _, kind := range []string{"enqueue.playlist", "enqueue.album"} {
		send := func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			require.NoError(t, connection.Write(ctx, websocket.MessageText, []byte(`{"type":"`+kind+`","payload":{"platform":"navidrome","playlistId":"remote","albumId":"album","mutationId":"`+kind+`"}}`)))
		}
		platforms.SetAccess("navidrome", PlatformAccess(accounts, "nobody"))
		before := service.calls.Load()
		send()
		denied := readUntilType(t, connection, "enqueue.nack")
		require.Equal(t, kind, denied.Payload.(map[string]any)["mutationId"])
		require.Equal(t, before, service.calls.Load())
		platforms.SetAccess("navidrome", PlatformAccess(accounts, "member"))
		send()
		readUntilType(t, connection, "enqueue.ack")
		platforms.SetAccess("navidrome", PlatformAccess(accounts, "nobody"))
		before = service.calls.Load()
		send()
		readUntilType(t, connection, "enqueue.nack")
		require.Equal(t, before, service.calls.Load())
	}
}

// Race detection must exercise the request paths while an administrator replaces policies.
func TestPlatformRequestsDuringPolicyUpdates(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	cfg := testConfig(t)
	platforms := NewPlatformAPI(cfg, &restrictedFixture{})
	mediaAPI := NewMediaAPI(cfg, accounts, store, nil, platforms, http.DefaultClient, nil)
	router := chi.NewRouter()
	platforms.Routes(router)
	mediaAPI.Routes(router)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 100 {
			platforms.SetAccess("navidrome", PlatformAccess(accounts, "member"))
			platforms.SetAccess("navidrome", PlatformAccess(accounts, "nobody"))
		}
	}()
	for _, path := range []string{"/api/platforms", "/api/search/navidrome/query", "/api/navidrome/stream/track", "/api/navidrome/cover/track"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 30 {
				response := batch2Request(router, http.MethodGet, path, "", "member-token", "")
				if response.Code != http.StatusOK && response.Code != http.StatusForbidden && response.Code != http.StatusNotFound {
					t.Errorf("unexpected response for %s: %d", path, response.Code)
				}
			}
		}()
	}
	wg.Wait()
}
