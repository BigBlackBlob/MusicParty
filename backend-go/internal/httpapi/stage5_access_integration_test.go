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
