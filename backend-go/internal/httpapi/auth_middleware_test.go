package httpapi

import (
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/stretchr/testify/require"
)

// A token whose account was disabled must be answered with 401, not with a
// server error, by every session middleware.
func TestMiddlewaresRejectDisabledAccountWithUnauthorized(t *testing.T) {
	middlewares := map[string]func(*account.Service) func(http.Handler) http.Handler{
		"RequireSession":  RequireSession,
		"RequireAdmin":    RequireAdmin,
		"RequireNonGuest": RequireNonGuest,
	}
	for name, middleware := range middlewares {
		t.Run(name, func(t *testing.T) {
			store := httpStore(t)
			accounts := account.New(store)
			insertStage5User(t, store, "disabled-token", "disabled-user", "ADMIN")
			require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, "update user_account set enabled=0 where public_id=?", "disabled-user")
				return err
			}))

			reached := false
			handler := middleware(accounts)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
			}))
			request := httptest.NewRequest(http.MethodGet, "/api/rooms/lounge", nil)
			request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "disabled-token"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)

			require.False(t, reached)
			require.Equal(t, http.StatusUnauthorized, response.Code)
		})
	}
}
