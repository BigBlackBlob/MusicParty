package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/stretchr/testify/require"
	"log/slog"
)

// The HTTP contract requires every error body to carry status, code and message. Room membership
// refusals used to answer with prose only, which left the desktop matching on English text.
func TestRoomMembershipErrorsCarryContractCodes(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	cfg := testConfig(t)
	insertStage5User(t, store, "admin-token", "admin", "ADMIN")
	insertStage5User(t, store, "member-token", "member", "MEMBER")
	insertStage5Room(t, store, "owned-room", "PUBLIC", 0)
	insertMembership(t, store, "owned-room", "owner", "OWNER")

	roomAPI := NewRoomAPI(rooms, cfg)
	roomAPI.SetAuthService(accounts)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), roomAPI)

	call := func(method, path, token string) *httptest.ResponseRecorder {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		r.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "test-csrf"})
		r.Header.Set(CSRFHeaderName, "test-csrf")
		r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		return w
	}

	lastOwnerRemoval := call(http.MethodDelete, "/api/rooms/owned-room/members/owner", "admin-token")
	require.Equal(t, http.StatusBadRequest, lastOwnerRemoval.Code, lastOwnerRemoval.Body.String())
	lastOwnerDemotion := call(http.MethodDelete, "/api/rooms/owned-room/owners/owner", "admin-token")
	require.Equal(t, http.StatusBadRequest, lastOwnerDemotion.Code, lastOwnerDemotion.Body.String())
	for _, response := range []*httptest.ResponseRecorder{lastOwnerRemoval, lastOwnerDemotion} {
		body := decodeErrorBody(t, response)
		require.Equal(t, roomErrorCodeLastOwner, body["code"])
		require.Equal(t, float64(http.StatusBadRequest), body["status"])
		require.Equal(t, "A room must retain an owner", body["message"])
	}

	missing := call(http.MethodDelete, "/api/rooms/owned-room/members/nobody", "admin-token")
	require.Equal(t, http.StatusNotFound, missing.Code, missing.Body.String())
	body := decodeErrorBody(t, missing)
	require.Equal(t, statusCode(http.StatusNotFound), body["code"])
	require.Equal(t, float64(http.StatusNotFound), body["status"])
	require.NotEmpty(t, body["message"])

	// A demotion that leaves a second owner behind still succeeds, so the refusal above is about the
	// invariant and not about owners being unremovable.
	insertMembership(t, store, "owned-room", "second", "OWNER")
	require.Equal(t, http.StatusNoContent, call(http.MethodDelete, "/api/rooms/owned-room/members/owner", "admin-token").Code)
}

func insertMembership(t *testing.T, store interface {
	Write(context.Context, func(context.Context, *sql.Tx) error) error
}, roomID, publicID, role string) {
	t.Helper()
	now := time.Now().UnixMilli()
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "insert into user_profile(public_id,display_name,is_guest,current_room_id,created_at,last_seen_at) values(?,?,1,'lounge',?,?) on conflict(public_id) do nothing", publicID, publicID, now, now); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, "insert into room_membership(room_id,public_id,role,created_at,updated_at) values(?,?,?,?,?)", roomID, publicID, role, now, now); err != nil {
			return err
		}
		return nil
	}))
}

func decodeErrorBody(t *testing.T, response *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	require.Contains(t, response.Header().Get("Content-Type"), "application/json", response.Body.String())
	var body map[string]any
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body), response.Body.String())
	return body
}
