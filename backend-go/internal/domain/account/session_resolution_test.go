package account

import (
	"context"
	"database/sql"
	"testing"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

type sessionFixture struct {
	guest          bool
	account        bool
	accountEnabled bool
}

// seedSessionFixture writes one profile + optional account + one session row,
// mirroring the shapes Resolve has to tell apart.
func seedSessionFixture(t *testing.T, store *storesqlite.Store, token string, fixture sessionFixture) string {
	t.Helper()
	publicID := "u_" + token
	now := time.Now().UnixMilli()
	require.NoError(t, store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		guest := 0
		if fixture.guest {
			guest = 1
		}
		if _, err := tx.ExecContext(ctx, "insert into user_profile(public_id,display_name,is_guest,current_room_id,created_at,last_seen_at) values(?,?,?,'lounge',?,?)", publicID, publicID, guest, now, now); err != nil {
			return err
		}
		if fixture.account {
			enabled := 0
			if fixture.accountEnabled {
				enabled = 1
			}
			if _, err := tx.ExecContext(ctx, "insert into user_account(username,public_id,password_hash,role,enabled,created_at,updated_at,last_login_at) values(?,?,?,'MEMBER',?,?,?,?)", "name_"+token, publicID, "unused", enabled, now, now, now); err != nil {
				return err
			}
		}
		_, err := tx.ExecContext(ctx, "insert into user_session(session_token_hash,public_id,created_at,last_seen_at) values(?,?,?,?)", tokenHash(token), publicID, now, now)
		return err
	}))
	return publicID
}

func countSessions(t *testing.T, store *storesqlite.Store, publicID string) int {
	t.Helper()
	var count int
	require.NoError(t, store.Reader().QueryRow("select count(1) from user_session where public_id=?", publicID).Scan(&count))
	return count
}

func TestResolveRejectsStaleAndInconsistentSessions(t *testing.T) {
	tests := []struct {
		name         string
		fixture      sessionFixture
		wantErr      error
		wantGuest    bool
		wantSessions int
	}{
		{
			name:         "guest profile without account resolves",
			fixture:      sessionFixture{guest: true},
			wantGuest:    true,
			wantSessions: 1,
		},
		{
			name:         "enabled account resolves as registered user",
			fixture:      sessionFixture{account: true, accountEnabled: true},
			wantGuest:    false,
			wantSessions: 1,
		},
		{
			name:         "disabled account is refused and its sessions are revoked",
			fixture:      sessionFixture{account: true, accountEnabled: false},
			wantErr:      ErrAccountDisabled,
			wantSessions: 0,
		},
		{
			name:         "non-guest profile without account is invalid",
			fixture:      sessionFixture{guest: false},
			wantErr:      ErrUnknownSession,
			wantSessions: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := testStore(t)
			service := New(store)
			token := "token-" + test.name
			publicID := seedSessionFixture(t, store, token, test.fixture)

			session, err := service.Resolve(context.Background(), token)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
				require.Empty(t, session.PublicID)
			} else {
				require.NoError(t, err)
				require.Equal(t, publicID, session.PublicID)
				require.Equal(t, test.wantGuest, session.Guest)
				require.True(t, session.Enabled)
			}
			require.Equal(t, test.wantSessions, countSessions(t, store, publicID))
		})
	}
}

func TestDisabledAccountErrorMapsToUnknownSession(t *testing.T) {
	store := testStore(t)
	service := New(store)
	publicID := seedSessionFixture(t, store, "disabled-token", sessionFixture{account: true, accountEnabled: false})

	_, err := service.Resolve(context.Background(), "disabled-token")
	require.ErrorIs(t, err, ErrAccountDisabled)
	// The HTTP layer keys 401 off ErrUnknownSession; the disabled error must match
	// it so a revoked token is never reported as a server error.
	require.ErrorIs(t, err, ErrUnknownSession)
	require.Equal(t, 0, countSessions(t, store, publicID))
}
