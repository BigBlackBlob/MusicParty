package room

import (
	"context"
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
func roomStore(t *testing.T) *storesqlite.Store {
	t.Helper()
	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{Path: filepath.Join(t.TempDir(), "room.db"), BusyTimeout: time.Second, ReadConnections: 2})
	require.NoError(t, err)
	require.NoError(t, storesqlite.EnsureCompatibleSchema(context.Background(), store, true))
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}
