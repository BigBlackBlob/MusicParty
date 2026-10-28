package httpapi

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/realtime"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestRoomPlaylistWriteAccessMatchesRoomVisibility(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	cfg.Auth.RoomAccessTokenSecret = "contract-room-access-secret-32-bytes"
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "public-room", "PUBLIC", 0)
	insertStage5Room(t, store, "private-room", "PRIVATE", 3)
	api := NewPlaylistAPI(cfg, store, accounts, rooms, NewPlatformAPI(cfg))
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)

	request := func(path, proof string) int {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"name":"共享列表"}`))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "member-token"})
		r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "test-csrf"})
		r.Header.Set(CSRFHeaderName, "test-csrf")
		if proof != "" {
			r.AddCookie(&http.Cookie{Name: RoomAccessCookieName, Value: proof})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	require.Equal(t, http.StatusOK, request("/api/rooms/public-room/playlists", ""))
	require.Equal(t, http.StatusForbidden, request("/api/rooms/private-room/playlists", ""))

	valid := signRoomAccessToken(roomAccessSecret(cfg.Auth.RoomAccessTokenSecret), "private-room", "member", time.Now().Add(time.Minute).UnixMilli(), 3)
	require.Equal(t, http.StatusOK, request("/api/rooms/private-room/playlists", valid))
	expired := signRoomAccessToken(roomAccessSecret(cfg.Auth.RoomAccessTokenSecret), "private-room", "member", time.Now().Add(-time.Minute).UnixMilli(), 3)
	require.Equal(t, http.StatusForbidden, request("/api/rooms/private-room/playlists", expired))
	wrongVersion := signRoomAccessToken(roomAccessSecret(cfg.Auth.RoomAccessTokenSecret), "private-room", "member", time.Now().Add(time.Minute).UnixMilli(), 2)
	require.Equal(t, http.StatusForbidden, request("/api/rooms/private-room/playlists", wrongVersion))
}

func TestPrivateRoomPlaylistReadsRequireRoomAccess(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	cfg.Auth.RoomAccessTokenSecret = "contract-room-access-secret-32-bytes"
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "private-room", "PRIVATE", 3)
	api := NewPlaylistAPI(cfg, store, accounts, rooms, NewPlatformAPI(cfg))
	playlist, err := api.roomPlaylists.CreatePlaylist(context.Background(), "private-room", "Private playlist")
	require.NoError(t, err)
	_, err = api.roomPlaylists.AddTrack(context.Background(), "private-room", playlist.ID, &storesqlite.Music{ID: "private-song", Name: "Private song", Artists: []string{"Artist"}, Platform: "local"})
	require.NoError(t, err)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	paths := []string{
		"/api/rooms/private-room/playlists",
		"/api/rooms/private-room/playlists/" + playlist.ID + "/tracks",
		"/api/rooms/private-room/playlists/" + playlist.ID + "/export",
	}
	proof := signRoomAccessToken(roomAccessSecret(cfg.Auth.RoomAccessTokenSecret), "private-room", "member", time.Now().Add(time.Minute).UnixMilli(), 3)
	for _, path := range paths {
		for _, test := range []struct {
			name  string
			proof string
			want  int
		}{{name: "no access proof", want: http.StatusForbidden}, {name: "valid session and proof", proof: proof, want: http.StatusOK}} {
			t.Run(path+"/"+test.name, func(t *testing.T) {
				request := httptest.NewRequest(http.MethodGet, path, nil)
				request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "member-token"})
				if test.proof != "" {
					request.AddCookie(&http.Cookie{Name: RoomAccessCookieName, Value: test.proof})
				}
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, request)
				require.Equal(t, test.want, response.Code)
			})
		}
	}
}

func TestPersonalPlaylistEnqueueRequiresPrivateRoomAccess(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	cfg.Auth.RoomAccessTokenSecret = "contract-room-access-secret-32-bytes"
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "private-room", "PRIVATE", 3)
	api := NewPlaylistAPI(cfg, store, accounts, rooms, NewPlatformAPI(cfg))
	manager := realtime.NewManager(store, 10, time.Second, time.Hour, nil)
	defer manager.Close()
	api.SetRealtime(manager)
	playlist, err := api.users.CreatePlaylist(context.Background(), "member", "Personal")
	require.NoError(t, err)
	_, err = api.users.AddTrackIfAbsent(context.Background(), "member", playlist.ID, &storesqlite.Music{ID: "song", Name: "Song", Artists: []string{"Artist"}, Platform: "local"})
	require.NoError(t, err)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	proof := signRoomAccessToken(roomAccessSecret(cfg.Auth.RoomAccessTokenSecret), "private-room", "member", time.Now().Add(time.Minute).UnixMilli(), 3)
	path := "/api/me/playlists/" + playlist.ID + "/enqueue?roomId=private-room"
	denied := batch2Request(handler, http.MethodPost, path, "{}", "member-token", "")
	require.Equal(t, http.StatusForbidden, denied.Code)
	var count int
	require.NoError(t, store.Reader().QueryRow("select (select count(*) from room_queue where room_id='private-room') + (select count(*) from room_playback_state where room_id='private-room' and current_music_json is not null)").Scan(&count))
	require.Zero(t, count, "unauthorized enqueue must not persist queue items")
	allowed := batch2Request(handler, http.MethodPost, path, "{}", "member-token", proof)
	require.Equal(t, http.StatusOK, allowed.Code, allowed.Body.String())
	require.Contains(t, allowed.Body.String(), `"count":1`)
	require.NoError(t, store.Reader().QueryRow("select (select count(*) from room_queue where room_id='private-room') + (select count(*) from room_playback_state where room_id='private-room' and current_music_json is not null)").Scan(&count))
	require.Equal(t, 1, count)
}

func TestRoomMutationDistinguishesUnauthorizedAndForbidden(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "managed-room", "PUBLIC", 0)
	roomAPI := NewRoomAPI(rooms, cfg)
	roomAPI.SetAuthService(accounts)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), roomAPI)

	request := func(method, path string, body string, authenticated bool) int {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "test-csrf"})
		r.Header.Set(CSRFHeaderName, "test-csrf")
		if authenticated {
			r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "member-token"})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	require.Equal(t, http.StatusUnauthorized, request(http.MethodPut, "/api/rooms/managed-room", `{"name":"new"}`, false))
	require.Equal(t, http.StatusForbidden, request(http.MethodPut, "/api/rooms/managed-room", `{"name":"new"}`, true))
	require.Equal(t, http.StatusUnauthorized, request(http.MethodDelete, "/api/rooms/managed-room", "", false))
	require.Equal(t, http.StatusForbidden, request(http.MethodDelete, "/api/rooms/managed-room", "", true))
}

// Creators own the rooms they opened: renaming and deleting belong to them, while member, owner
// and invite administration stays with platform admins.
func TestRoomCreatorCanRenameAndDeleteOwnRoom(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	insertStage5User(t, store, "creator-token", "creator", "MEMBER")
	insertStage5User(t, store, "stranger-token", "stranger", "MEMBER")
	now := time.Now().UnixMilli()
	for _, room := range []struct {
		id, owner string
		system    bool
	}{
		{id: "creator-room", owner: "creator"},
		{id: "legacy-room", owner: "creator", system: true},
	} {
		flag := 0
		if room.system {
			flag = 1
		}
		require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "insert into room(id,name,owner_public_id,visibility,password_version,system,created_at,last_active_at) values(?,?,?,'PUBLIC',0,?,?,?)", room.id, room.id, room.owner, flag, now, now)
			return err
		}))
	}
	roomAPI := NewRoomAPI(rooms, cfg)
	roomAPI.SetAuthService(accounts)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), roomAPI)

	request := func(method, path, body, token string) int {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "test-csrf"})
		r.Header.Set(CSRFHeaderName, "test-csrf")
		if token != "" {
			r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	require.Equal(t, http.StatusUnauthorized, request(http.MethodPut, "/api/rooms/creator-room", `{"name":"renamed"}`, ""))
	require.Equal(t, http.StatusForbidden, request(http.MethodPut, "/api/rooms/creator-room", `{"name":"renamed"}`, "stranger-token"))
	require.Equal(t, http.StatusOK, request(http.MethodPut, "/api/rooms/creator-room", `{"name":"renamed"}`, "creator-token"))
	require.Equal(t, http.StatusForbidden, request(http.MethodDelete, "/api/rooms/legacy-room", "", "creator-token"), "system rooms stay admin-only even for their recorded owner")
	require.Equal(t, http.StatusForbidden, request(http.MethodDelete, "/api/rooms/creator-room", "", "stranger-token"))
	require.Equal(t, http.StatusOK, request(http.MethodDelete, "/api/rooms/creator-room", "", "creator-token"))
	require.Equal(t, http.StatusForbidden, request(http.MethodDelete, "/api/rooms/creator-room", "", "creator-token"), "a deleted room is not deletable a second time")
}

func TestPrivateRoomVerificationEstablishesCookieOnlyAccessProof(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	cfg.Auth.RoomAccessTokenSecret = "contract-room-access-secret-32-bytes"
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "private-room", "PRIVATE", 7)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte("correct horse battery staple"), bcrypt.DefaultCost)
	require.NoError(t, err)
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update room set password_hash=? where id='private-room'", string(passwordHash))
		return err
	}))

	roomAPI := NewRoomAPI(rooms, cfg)
	roomAPI.SetAuthService(accounts)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), roomAPI)
	verify := func(password string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/api/rooms/private-room/verify", strings.NewReader(`{"password":"`+password+`"}`))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set(CSRFHeaderName, "test-csrf")
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "member-token"})
		r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "test-csrf"})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	wrong := verify("wrong")
	require.Equal(t, http.StatusForbidden, wrong.Code)
	correct := verify("correct horse battery staple")
	require.Equal(t, http.StatusOK, correct.Code)
	require.NotContains(t, correct.Body.String(), "token")
	cookies := correct.Result().Cookies()
	require.Len(t, cookies, 1)
	require.Equal(t, RoomAccessCookieName, cookies[0].Name)
	require.True(t, cookies[0].HttpOnly)
	require.True(t, validRoomAccessToken(roomAccessSecret(cfg.Auth.RoomAccessTokenSecret), cookies[0].Value, "private-room", "member", 7, time.Now()))
}

func insertStage5User(t *testing.T, store interface {
	Write(context.Context, func(context.Context, *sql.Tx) error) error
}, token, publicID, role string) {
	t.Helper()
	hash := sha256.Sum256([]byte(token))
	now := time.Now().UnixMilli()
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "insert into user_profile(public_id,display_name,is_guest,current_room_id,created_at,last_seen_at) values(?,?,0,'lounge',?,?)", publicID, publicID, now, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "insert into user_account(username,public_id,password_hash,role,enabled,created_at,updated_at,last_login_at) values(?,?,?, ?,1,?,?,?)", publicID, publicID, "unused", role, now, now, now); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, "insert into user_session(session_token_hash,public_id,created_at,last_seen_at) values(?,?,?,?)", hex.EncodeToString(hash[:]), publicID, now, now)
		return err
	}))
}

func insertStage5Room(t *testing.T, store interface {
	Write(context.Context, func(context.Context, *sql.Tx) error) error
}, roomID, visibility string, passwordVersion int) {
	t.Helper()
	now := time.Now().UnixMilli()
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "insert into room(id,name,owner_public_id,visibility,password_version,system,created_at,last_active_at) values(?,?,?, ?,?,0,?,?)", roomID, roomID, "owner", visibility, passwordVersion, now, now)
		return err
	}))
}
