package room

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestLastOwnerCannotBeRemoved(t *testing.T) {
	store := roomStore(t)
	accounts := account.New(store)
	require.NoError(t, accounts.Bootstrap(context.Background(), "admin", "strong-password"))
	admin, err := accounts.Login(context.Background(), "admin", "strong-password")
	require.NoError(t, err)
	now := time.Now().UnixMilli()
	var publicID string
	require.NoError(t, store.Reader().QueryRow("select public_id from user_account where username='admin'").Scan(&publicID))
	require.NoError(t, storesqlite.NewRoomRepository(store).Upsert(context.Background(), storesqlite.Room{ID: "room", Name: "Room", OwnerPublicID: publicID, Visibility: "PRIVATE", CreatedAt: now, LastActiveAt: now}))
	require.NoError(t, storesqlite.NewRoomAccessRepository(store).UpsertMembership(context.Background(), storesqlite.RoomMembership{RoomID: "room", PublicID: publicID, Role: "OWNER", CreatedAt: now, UpdatedAt: now}))
	service := New(store, accounts)
	changed, err := service.SetOwner(context.Background(), "room", publicID, admin.SessionToken, false)
	require.False(t, changed)
	require.EqualError(t, err, "A room must retain an owner")
}

func TestPermanentInviteCreateAndList(t *testing.T) {
	ctx := context.Background()
	store := roomStore(t)
	accounts := account.New(store)
	require.NoError(t, accounts.Bootstrap(ctx, "admin", "strong-password"))
	admin, err := accounts.Login(ctx, "admin", "strong-password")
	require.NoError(t, err)
	roomID, publicID := seedManagedRoom(t, store, admin)
	service := New(store, accounts)
	service.now = func() time.Time { return time.UnixMilli(1000) }

	created, err := service.CreateInvite(ctx, roomID, admin.SessionToken, "permanent")
	require.NoError(t, err)
	require.True(t, created.Permanent)
	require.Equal(t, int64(PermanentInviteExpiresAt), created.ExpiresAt)

	oldExpiresAt := int64(2000)
	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "insert into room_invite(id,room_id,created_by_public_id,secret_hash,label,expires_at,max_uses,created_at) values(?,?,?,?,?,?,1,?)", "old", roomID, publicID, "old-hash", "old", oldExpiresAt, 500)
		return err
	}))
	invites, err := service.ListInvites(ctx, roomID, admin.SessionToken)
	require.NoError(t, err)
	require.Len(t, invites, 2)
	for _, invite := range invites {
		if invite.ID == created.ID {
			require.True(t, invite.Permanent)
		} else {
			require.False(t, invite.Permanent)
		}
	}
}

func TestPermanentInviteRedeemsOnceAndCanBeRevoked(t *testing.T) {
	ctx := context.Background()
	store := roomStore(t)
	accounts := account.New(store)
	require.NoError(t, accounts.Bootstrap(ctx, "admin", "strong-password"))
	admin, err := accounts.Login(ctx, "admin", "strong-password")
	require.NoError(t, err)
	roomID, _ := seedManagedRoom(t, store, admin)
	service := New(store, accounts)
	service.now = func() time.Time { return time.UnixMilli(1000) }
	created, err := service.CreateInvite(ctx, roomID, admin.SessionToken, "one-time")
	require.NoError(t, err)

	member, err := accounts.RedeemInvite(ctx, created.Secret, "Member")
	require.NoError(t, err)
	_, err = accounts.RedeemInvite(ctx, created.Secret, "Again")
	require.Error(t, err)

	revoked, err := service.CreateInvite(ctx, roomID, admin.SessionToken, "revoked")
	require.NoError(t, err)
	changed, err := service.RevokeInvite(ctx, roomID, revoked.ID, admin.SessionToken)
	require.NoError(t, err)
	require.True(t, changed)
	_, err = accounts.RedeemInvite(ctx, revoked.Secret, "Revoked")
	require.Error(t, err)

	_, err = service.CreateInvite(ctx, roomID, member.SessionToken, "forbidden")
	require.Error(t, err)
	_, err = service.ListInvites(ctx, roomID, member.SessionToken)
	require.Error(t, err)
	_, err = service.RevokeInvite(ctx, roomID, created.ID, member.SessionToken)
	require.Error(t, err)
}

func seedManagedRoom(t *testing.T, store *storesqlite.Store, admin account.Session) (string, string) {
	t.Helper()
	now := time.Now().UnixMilli()
	require.NoError(t, storesqlite.NewRoomRepository(store).Upsert(context.Background(), storesqlite.Room{ID: "room", Name: "Room", OwnerPublicID: admin.PublicID, Visibility: "PRIVATE", CreatedAt: now, LastActiveAt: now}))
	require.NoError(t, storesqlite.NewRoomAccessRepository(store).UpsertMembership(context.Background(), storesqlite.RoomMembership{RoomID: "room", PublicID: admin.PublicID, Role: "OWNER", CreatedAt: now, UpdatedAt: now}))
	return "room", admin.PublicID
}
func roomStore(t *testing.T) *storesqlite.Store {
	t.Helper()
	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{Path: filepath.Join(t.TempDir(), "room.db"), BusyTimeout: time.Second, ReadConnections: 2})
	require.NoError(t, err)
	require.NoError(t, storesqlite.EnsureCompatibleSchema(context.Background(), store, true))
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}
