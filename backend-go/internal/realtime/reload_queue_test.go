package realtime

import (
	"context"
	"database/sql"
	"slices"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

// The bug: a local track deleted from the library stayed in the in-memory queue and was written back
// by the next persistence. ReloadQueue makes the runtime adopt the persisted queue.
func TestReloadQueueDropsRemovedTracksAndBumpsVersion(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	broadcaster := &recordingBroadcaster{}
	manager := NewManager(store, 10, time.Second, time.Hour, broadcaster)
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user", DisplayName: "User"}
	for index, id := range []string{"playing", "keep", "gone"} {
		_, err = runtime.Enqueue(ctx, session, storesqlite.Music{ID: id, Name: id, Duration: 60_000, Platform: "local"}, "mutation-"+string(rune('a'+index)))
		require.NoError(t, err)
	}
	before, err := runtime.Snapshot(ctx)
	require.NoError(t, err)
	require.Len(t, before["queue"], 2)
	version := before["queueVersion"].(int64)

	// The library removed the deleted track from every room: the rows are gone, memory is not.
	queueRepo := storesqlite.NewQueueRepository(store, time.Now)
	persisted, err := queueRepo.LoadQueue(ctx, "lounge")
	require.NoError(t, err)
	kept := slices.DeleteFunc(slices.Clone(persisted), func(item storesqlite.QueueItem) bool { return item.Music.ID == "gone" })
	require.Len(t, kept, 1)
	require.NoError(t, queueRepo.SynchronizeQueue(ctx, "lounge", kept))

	states := broadcaster.count("player.state")
	require.NoError(t, manager.ReloadQueue(ctx, "lounge", "gone"))

	after, err := runtime.Snapshot(ctx)
	require.NoError(t, err)
	queue, ok := after["queue"].([]storesqlite.QueueItem)
	require.True(t, ok)
	require.Len(t, queue, 1)
	require.Equal(t, "keep", queue[0].Music.ID)
	require.Equal(t, version+1, after["queueVersion"].(int64), "the queue version must grow, clients treat <= 0 as invalid")
	require.Equal(t, states+1, broadcaster.count("player.state"), "the client must learn about the reload in one broadcast")
}

// A deletion must never create a runtime for a room nobody listens to.
func TestReloadQueueSkipsRoomsWithoutRuntime(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	t.Cleanup(manager.Close)
	require.NoError(t, manager.ReloadQueue(ctx, "never-loaded", "gone"))
	manager.mu.Lock()
	defer manager.mu.Unlock()
	require.Empty(t, manager.runtimes)
}

// A deleted track can also be the one playing. Dropping it from memory matters more than reloading
// the queue: the next persistence would otherwise write the removed row back into playback_state.
func TestReloadQueueDropsDeletedCurrentTrack(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	broadcaster := &recordingBroadcaster{}
	manager := NewManager(store, 10, time.Second, time.Hour, broadcaster)
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user", DisplayName: "User"}
	for index, id := range []string{"gone", "keep"} {
		_, err = runtime.Enqueue(ctx, session, storesqlite.Music{ID: id, Name: id, Duration: 60_000, Platform: "local"}, "mutation-"+string(rune('a'+index)))
		require.NoError(t, err)
	}
	before, err := runtime.Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, "gone", before["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic).ID)
	version := before["queueVersion"].(int64)
	patches := broadcaster.count("queue.patch")
	states := broadcaster.count("player.state")

	// DeleteLocalReferences already nulled the column in the database; memory still plays it.
	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update room_playback_state set current_music_json=null where room_id=?", "lounge")
		return err
	}))
	require.NoError(t, manager.ReloadQueue(ctx, "lounge", "gone"))

	after, err := runtime.Snapshot(ctx)
	require.NoError(t, err)
	music := after["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic)
	require.Equal(t, "keep", music.ID, "the next queued track takes over")
	queue, ok := after["queue"].([]storesqlite.QueueItem)
	require.True(t, ok)
	require.Empty(t, queue)
	require.Equal(t, version+1, after["queueVersion"].(int64))
	require.Equal(t, patches+1, broadcaster.count("queue.patch"))
	require.Equal(t, states+1, broadcaster.count("player.state"))

	history, err := storesqlite.NewQueueRepository(store, time.Now).LoadHistory(ctx, "lounge", 10)
	require.NoError(t, err)
	require.Empty(t, history, "the deleted track must not be recorded as played")
}

// Removing a track that is not the current one must leave playback untouched.
func TestReloadQueueKeepsUnrelatedCurrentTrack(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user", DisplayName: "User"}
	_, err = runtime.Enqueue(ctx, session, storesqlite.Music{ID: "playing", Name: "playing", Duration: 60_000, Platform: "local"}, "mutation-a")
	require.NoError(t, err)
	require.NoError(t, manager.ReloadQueue(ctx, "lounge", "other"))

	after, err := runtime.Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, "playing", after["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic).ID)
}

func TestReloadQueueRemovesDeletedHistoryAndPreservesCursor(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user", DisplayName: "User"}
	for i, id := range []string{"older", "gone", "newer", "playing"} {
		_, err = runtime.Enqueue(ctx, session, storesqlite.Music{ID: id, Name: id, Platform: "local", Duration: 60000}, id)
		require.NoError(t, err)
		if i > 0 {
			_, err = runtime.Next(ctx)
			require.NoError(t, err)
		}
	}
	_, err = runtime.Previous(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, storesqlite.NewLocalTrackRepository(store).DeleteLocalReferences(ctx, "gone"))
	require.NoError(t, manager.ReloadQueue(ctx, "lounge", "gone"))
	_, err = runtime.Previous(ctx, nil)
	require.NoError(t, err)
	snapshot, err := runtime.Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, "older", snapshot["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic).ID)
}
