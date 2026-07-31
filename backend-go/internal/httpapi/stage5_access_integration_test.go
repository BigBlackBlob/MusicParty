package httpapi

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
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

	request := func(path string) int {
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"name":"共享列表"}`))
		r.Header.Set("Content-Type", "application/json")
		if strings.Contains(path, "sessionToken=member-token") {
			r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "test-csrf"})
			r.Header.Set(CSRFHeaderName, "test-csrf")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	require.Equal(t, http.StatusForbidden, request("/api/rooms/public-room/playlists"))
	require.Equal(t, http.StatusOK, request("/api/rooms/public-room/playlists?sessionToken=member-token"))
	require.Equal(t, http.StatusForbidden, request("/api/rooms/private-room/playlists?sessionToken=member-token"))

	valid := signRoomToken(cfg.Auth.RoomAccessTokenSecret, "private-room", "member", time.Now().Add(time.Minute).UnixMilli(), 3)
	require.Equal(t, http.StatusOK, request("/api/rooms/private-room/playlists?sessionToken=member-token&roomAccessToken="+valid))
	expired := signRoomToken(cfg.Auth.RoomAccessTokenSecret, "private-room", "member", time.Now().Add(-time.Minute).UnixMilli(), 3)
	require.Equal(t, http.StatusForbidden, request("/api/rooms/private-room/playlists?sessionToken=member-token&roomAccessToken="+expired))
	wrongVersion := signRoomToken(cfg.Auth.RoomAccessTokenSecret, "private-room", "member", time.Now().Add(time.Minute).UnixMilli(), 2)
	require.Equal(t, http.StatusForbidden, request("/api/rooms/private-room/playlists?sessionToken=member-token&roomAccessToken="+wrongVersion))
}

func TestRoomMutationDistinguishesUnauthorizedAndForbidden(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "managed-room", "PUBLIC", 0)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewRoomAPI(rooms))

	request := func(method, path string, body string) int {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if strings.Contains(path, "sessionToken=member-token") {
			r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "member-token"})
			r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "test-csrf"})
			r.Header.Set(CSRFHeaderName, "test-csrf")
		}
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w.Code
	}
	require.Equal(t, http.StatusForbidden, request(http.MethodPut, "/api/rooms/managed-room", `{"name":"new"}`))
	require.Equal(t, http.StatusForbidden, request(http.MethodPut, "/api/rooms/managed-room?sessionToken=member-token", `{"name":"new"}`))
	require.Equal(t, http.StatusForbidden, request(http.MethodDelete, "/api/rooms/managed-room", ""))
	require.Equal(t, http.StatusForbidden, request(http.MethodDelete, "/api/rooms/managed-room?sessionToken=member-token", ""))
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

func signRoomToken(secret, roomID, publicID string, expiresAt int64, passwordVersion int) string {
	payload := []byte(fmt.Sprintf("%s|%s|%d|%d", roomID, publicID, expiresAt, passwordVersion))
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
