package account

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestBootstrapLoginAndProfile(t *testing.T) {
	store := testStore(t)
	service := New(store)
	require.NoError(t, service.Bootstrap(context.Background(), "Admin", "strong-password"))
	status, err := service.Status(context.Background())
	require.NoError(t, err)
	require.False(t, status["requiresSetup"])
	session, err := service.Login(context.Background(), "ADMIN", "strong-password")
	require.NoError(t, err)
	require.True(t, session.Admin())
	updated, err := service.UpdateProfile(context.Background(), session.SessionToken, "管理员")
	require.NoError(t, err)
	require.Equal(t, "管理员", updated.DisplayName)
	require.NoError(t, service.Logout(context.Background(), session.SessionToken))
	_, err = service.Resolve(context.Background(), session.SessionToken)
	require.ErrorIs(t, err, ErrUnknownSession)
}
func TestInviteCanOnlyBeRedeemedOnceConcurrently(t *testing.T) {
	store := testStore(t)
	service := New(store)
	now := time.Now().UnixMilli()
	rooms := storesqlite.NewRoomRepository(store)
	require.NoError(t, rooms.Upsert(context.Background(), storesqlite.Room{ID: "lounge", Name: "Lounge", OwnerPublicID: "system", Visibility: "PUBLIC", System: true, CreatedAt: now, LastActiveAt: now}))
	access := storesqlite.NewRoomAccessRepository(store)
	secret := "invite-secret"
	require.NoError(t, access.CreateInvite(context.Background(), storesqlite.RoomInvite{ID: "inv_test", RoomID: "lounge", CreatedByPublicID: "system", SecretHash: tokenHash(secret), ExpiresAt: now + 60000, MaxUses: 1, CreatedAt: now}))
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, name := range []string{"A", "B"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.RedeemInvite(context.Background(), secret, name)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	require.Equal(t, 1, successes)
	var accounts int
	require.NoError(t, store.Reader().QueryRow("select count(*) from user_account where role='MEMBER'").Scan(&accounts))
	require.Equal(t, 1, accounts)
}
func testStore(t *testing.T) *storesqlite.Store {
	t.Helper()
	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{Path: filepath.Join(t.TempDir(), "business.db"), BusyTimeout: time.Second, ReadConnections: 2, WriteQueueCapacity: 100})
	require.NoError(t, err)
	require.NoError(t, storesqlite.EnsureCompatibleSchema(context.Background(), store, true))
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}
