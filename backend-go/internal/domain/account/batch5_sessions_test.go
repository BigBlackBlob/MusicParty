package account

import (
	"context"
	"database/sql"
	"testing"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"

	"github.com/stretchr/testify/require"
)

func TestBatch5SessionsExpireAndPasswordChangeRevokesAll(t *testing.T) {
	ctx := context.Background()
	service := New(testStore(t))
	require.NoError(t, service.Bootstrap(ctx, "admin", "old-password"))
	first, err := service.Login(ctx, "admin", "old-password")
	require.NoError(t, err)
	second, err := service.Login(ctx, "admin", "old-password")
	require.NoError(t, err)
	require.NoError(t, service.ChangePassword(ctx, first.SessionToken, "old-password", "new-password"))
	_, err = service.Resolve(ctx, second.SessionToken)
	require.ErrorIs(t, err, ErrUnknownSession)
	_, err = service.Resolve(ctx, first.SessionToken)
	require.ErrorIs(t, err, ErrUnknownSession)
	third, err := service.Login(ctx, "admin", "new-password")
	require.NoError(t, err)
	service.now = func() time.Time { return time.Now().Add(12 * time.Hour) }
	_, err = service.Resolve(ctx, third.SessionToken)
	require.ErrorIs(t, err, ErrUnknownSession)
}

func TestBatch5GuestExpiryCleanupPreservesOwnedData(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	service := New(store)
	guest, err := service.CreateGuestSession(ctx, "Guest")
	require.NoError(t, err)
	playlist, err := storesqlite.NewUserPlaylistRepository(store, time.Now, nil).CreatePlaylist(ctx, guest.PublicID, "Keep")
	require.NoError(t, err)
	start := time.Now()
	service.now = func() time.Time { return start.Add(89 * 24 * time.Hour) }
	_, err = service.Resolve(ctx, guest.SessionToken)
	require.NoError(t, err)
	service.now = func() time.Time { return start.Add(90 * 24 * time.Hour) }
	_, err = service.Resolve(ctx, guest.SessionToken)
	require.ErrorIs(t, err, ErrUnknownSession)
	var revoked []string
	service.SetSessionsRevokedHook(func(hashes []string) { revoked = append(revoked, hashes...) })
	require.NoError(t, service.CleanupExpiredSessions(ctx))
	require.Equal(t, []string{tokenHash(guest.SessionToken)}, revoked)
	require.Equal(t, 0, countSessions(t, store, guest.PublicID))
	_, err = storesqlite.NewUserPlaylistRepository(store, time.Now, nil).FindPlaylist(ctx, guest.PublicID, playlist.ID)
	require.NoError(t, err)
	var profiles int
	require.NoError(t, store.Reader().QueryRow("select count(*) from user_profile where public_id=?", guest.PublicID).Scan(&profiles))
	require.Equal(t, 1, profiles)
}

func TestBatch5FailedPasswordRevocationRollsBackHashAndSkipsHook(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	service := New(store)
	require.NoError(t, service.Bootstrap(ctx, "admin", "old-password"))
	session, err := service.Login(ctx, "admin", "old-password")
	require.NoError(t, err)
	called := false
	service.SetSessionsRevokedHook(func([]string) { called = true })
	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `create trigger reject_revoke before delete on user_session begin select raise(abort,'forced rollback'); end`)
		return err
	}))
	require.Error(t, service.ChangePassword(ctx, session.SessionToken, "old-password", "new-password"))
	require.False(t, called)
	_, err = service.Resolve(ctx, session.SessionToken)
	require.NoError(t, err)
	_, err = service.Login(ctx, "admin", "old-password")
	require.NoError(t, err)
}

func TestBatch5LoginCheckedBeforePasswordChangeCannotMintNewSession(t *testing.T) {
	ctx := context.Background()
	store := testStore(t)
	service := New(store)
	require.NoError(t, service.Bootstrap(ctx, "admin", "old-password"))
	session, err := service.Login(ctx, "admin", "old-password")
	require.NoError(t, err)
	checked, release := make(chan struct{}), make(chan struct{})
	loggingIn := New(store)
	loggingIn.now = func() time.Time { close(checked); <-release; return time.Now() }
	result := make(chan error, 1)
	go func() { _, err := loggingIn.Login(ctx, "admin", "old-password"); result <- err }()
	<-checked
	err = service.ChangePassword(ctx, session.SessionToken, "old-password", "new-password")
	close(release)
	require.NoError(t, err)
	require.ErrorIs(t, <-result, ErrInvalidLogin)
	require.Equal(t, 0, countSessions(t, store, session.PublicID))
}
