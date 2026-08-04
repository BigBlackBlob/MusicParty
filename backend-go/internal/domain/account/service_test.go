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

func TestGuestSession(t *testing.T) {
	store := testStore(t)
	service := New(store)
	ctx := context.Background()

	t.Run("create guest session", func(t *testing.T) {
		session, err := service.CreateGuestSession(ctx, "访客小明")
		require.NoError(t, err)
		require.Equal(t, "访客小明", session.DisplayName)
		require.Equal(t, "GUEST", session.Role)
		require.True(t, session.Guest)
		require.Empty(t, session.Username)
		require.NotEmpty(t, session.SessionToken)

		// Verify session can be resolved
		resolved, err := service.Resolve(ctx, session.SessionToken)
		require.NoError(t, err)
		require.Equal(t, session.PublicID, resolved.PublicID)
		require.True(t, resolved.Guest)
		require.Equal(t, "GUEST", resolved.Role)
	})

	t.Run("guest session persists across resolves", func(t *testing.T) {
		session, err := service.CreateGuestSession(ctx, "持久访客")
		require.NoError(t, err)

		// Resolve multiple times
		for i := 0; i < 3; i++ {
			resolved, err := service.Resolve(ctx, session.SessionToken)
			require.NoError(t, err)
			require.Equal(t, session.PublicID, resolved.PublicID)
			require.Equal(t, "持久访客", resolved.DisplayName)
		}
	})

	t.Run("guest can update profile", func(t *testing.T) {
		session, err := service.CreateGuestSession(ctx, "Old Name")
		require.NoError(t, err)

		updated, err := service.UpdateProfile(ctx, session.SessionToken, "New Name")
		require.NoError(t, err)
		require.Equal(t, "New Name", updated.DisplayName)
		require.Equal(t, session.PublicID, updated.PublicID)
	})

	t.Run("guest can upgrade to user with invite", func(t *testing.T) {
		// Create guest session
		guestSession, err := service.CreateGuestSession(ctx, "访客升级测试")
		require.NoError(t, err)
		require.True(t, guestSession.Guest)
		require.Equal(t, "GUEST", guestSession.Role)

		// Create invite
		rooms := storesqlite.NewRoomRepository(store)
		now := time.Now().UnixMilli()
		require.NoError(t, rooms.Upsert(ctx, storesqlite.Room{ID: "test-room", Name: "Test Room", OwnerPublicID: "system", Visibility: "PUBLIC", System: false, CreatedAt: now, LastActiveAt: now}))

		access := storesqlite.NewRoomAccessRepository(store)
		secret := "upgrade-invite-secret"
		require.NoError(t, access.CreateInvite(ctx, storesqlite.RoomInvite{ID: "inv_upgrade", RoomID: "test-room", CreatedByPublicID: "system", SecretHash: tokenHash(secret), ExpiresAt: now + 60000, MaxUses: 1, CreatedAt: now}))

		// Upgrade guest to user
		upgraded, err := service.UpgradeGuestToUser(ctx, guestSession.SessionToken, secret)
		require.NoError(t, err)
		require.False(t, upgraded.Guest)
		require.Equal(t, "MEMBER", upgraded.Role)
		require.Equal(t, guestSession.PublicID, upgraded.PublicID)
		require.Equal(t, guestSession.DisplayName, upgraded.DisplayName)
		require.NotEmpty(t, upgraded.Username)

		// Verify new session works
		resolved, err := service.Resolve(ctx, upgraded.SessionToken)
		require.NoError(t, err)
		require.False(t, resolved.Guest)
		require.Equal(t, "MEMBER", resolved.Role)
	})

	t.Run("cannot upgrade already registered user", func(t *testing.T) {
		// Create regular user via invite
		rooms := storesqlite.NewRoomRepository(store)
		now := time.Now().UnixMilli()
		require.NoError(t, rooms.Upsert(ctx, storesqlite.Room{ID: "another-room", Name: "Another Room", OwnerPublicID: "system", Visibility: "PUBLIC", System: false, CreatedAt: now, LastActiveAt: now}))

		access := storesqlite.NewRoomAccessRepository(store)
		secret1 := "first-invite"
		require.NoError(t, access.CreateInvite(ctx, storesqlite.RoomInvite{ID: "inv_first", RoomID: "another-room", CreatedByPublicID: "system", SecretHash: tokenHash(secret1), ExpiresAt: now + 60000, MaxUses: 1, CreatedAt: now}))

		userSession, err := service.RedeemInvite(ctx, secret1, "Regular User")
		require.NoError(t, err)
		require.False(t, userSession.Guest)

		// Try to upgrade (should fail)
		secret2 := "second-invite"
		require.NoError(t, access.CreateInvite(ctx, storesqlite.RoomInvite{ID: "inv_second", RoomID: "another-room", CreatedByPublicID: "system", SecretHash: tokenHash(secret2), ExpiresAt: now + 60000, MaxUses: 1, CreatedAt: now}))

		_, err = service.UpgradeGuestToUser(ctx, userSession.SessionToken, secret2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "already a registered user")
	})
}

func testStore(t *testing.T) *storesqlite.Store {
	t.Helper()
	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{Path: filepath.Join(t.TempDir(), "business.db"), BusyTimeout: time.Second, ReadConnections: 2, WriteQueueCapacity: 100})
	require.NoError(t, err)
	require.NoError(t, storesqlite.EnsureCompatibleSchema(context.Background(), store, true))
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}
