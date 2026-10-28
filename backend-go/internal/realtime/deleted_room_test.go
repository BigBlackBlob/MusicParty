package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestBatch3DeletedRoomRejectsRuntimeWrites(t *testing.T) {
	store := realtimeStore(t)
	accounts := account.New(store)
	owner, err := accounts.CreateGuestSession(context.Background(), "Owner")
	require.NoError(t, err)
	rooms := roomdomain.New(store, accounts)
	created, err := rooms.Create(context.Background(), owner.SessionToken, roomdomain.CreateInput{Name: "Disposable"})
	require.NoError(t, err)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	defer manager.Close()
	runtime, err := manager.Room(context.Background(), created.RoomID)
	require.NoError(t, err)
	_, err = runtime.Enqueue(context.Background(), owner, storesqlite.Music{ID: "first", Name: "First", Platform: "local", Duration: 60000}, "before-delete")
	require.NoError(t, err)
	deleted, err := rooms.Delete(context.Background(), created.RoomID, owner.SessionToken)
	require.NoError(t, err)
	require.True(t, deleted)
	_, err = runtime.Enqueue(context.Background(), owner, storesqlite.Music{ID: "second", Name: "Second", Platform: "local"}, "after-delete")
	require.Error(t, err, "a runtime retained by an in-flight handler must not recreate room data")
	_, err = runtime.Next(context.Background())
	require.Error(t, err, "playback advance must not recreate history or state")
	_, err = manager.Room(context.Background(), created.RoomID)
	require.Error(t, err, "deleted rooms must not get another runtime")
	for _, table := range []string{"room_queue", "room_playback_state", "room_history", "room_history_track"} {
		var count int
		require.NoError(t, store.Reader().QueryRow("select count(*) from "+table+" where room_id=?", created.RoomID).Scan(&count))
		require.Zero(t, count, table)
	}
}
