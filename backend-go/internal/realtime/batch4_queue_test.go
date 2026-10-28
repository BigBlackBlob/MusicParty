package realtime

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestBatch4RebuiltQueueHasFreshScopeAndValidVersion(t *testing.T) {
	store := realtimeStore(t)
	ctx := context.Background()
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	_, err = runtime.EnqueueMany(ctx, account.Session{PublicID: "user"}, []storesqlite.Music{{ID: "playing"}, {ID: "pending"}}, "seed")
	require.NoError(t, err)
	before, err := runtime.Snapshot(ctx)
	require.NoError(t, err)
	runtime.Close()
	rebuilt, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	after, err := rebuilt.Snapshot(ctx)
	require.NoError(t, err)
	require.Greater(t, after["queueVersion"].(int64), int64(0))
	require.NotEmpty(t, after["queueEpoch"])
	require.NotEqual(t, before["queueEpoch"], after["queueEpoch"])
	require.Len(t, after["queue"], 1)
}

func TestBatch4CapacityIsAtomicAndRetriesReplay(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{}, QueueLimits{MaxSize: 3, MaxUserSongs: 2, HistorySize: 1})
	defer manager.Close()
	r, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	owner := account.Session{PublicID: "user"}
	playing, err := r.Enqueue(ctx, owner, storesqlite.Music{ID: "playing"}, "start")
	require.NoError(t, err)
	replayed, err := r.Enqueue(ctx, owner, storesqlite.Music{ID: "playing", Name: "updated upstream metadata"}, "start")
	require.NoError(t, err)
	require.Equal(t, playing.QueueID, replayed.QueueID)
	_, err = r.EnqueueMany(ctx, owner, []storesqlite.Music{{ID: "a"}, {ID: "b"}}, "batch")
	require.NoError(t, err)
	before, err := r.Snapshot(ctx)
	require.NoError(t, err)
	_, err = r.Enqueue(ctx, owner, storesqlite.Music{ID: "c"}, "user-overflow")
	require.ErrorIs(t, err, ErrQueueCapacity)
	_, err = r.EnqueueMany(ctx, account.Session{PublicID: "mate"}, []storesqlite.Music{{ID: "c"}, {ID: "d"}}, "batch-overflow")
	require.ErrorIs(t, err, ErrQueueCapacity)
	after, err := r.Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, before["queue"], after["queue"])
	require.Equal(t, before["queueVersion"], after["queueVersion"])
	for i := range 3 {
		_, err = r.Next(ctx)
		require.NoError(t, err, fmt.Sprint(i))
	}
	historySize, err := r.Execute(ctx, func(runtime *RoomRuntime) (any, error) { return len(runtime.history), nil })
	require.NoError(t, err)
	require.Equal(t, 1, historySize)
	history, err := r.queueRepo.LoadHistory(ctx, "lounge", 10)
	require.NoError(t, err)
	require.Len(t, history, 3, "history setting must not delete persisted history")
}

func TestBatch4ConcurrentEnqueuesRespectCapacity(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	manager := NewManager(store, 32, time.Second, time.Hour, &recordingBroadcaster{}, QueueLimits{MaxSize: 3, MaxUserSongs: 3, HistorySize: 1})
	defer manager.Close()
	r, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user"}
	_, err = r.Enqueue(ctx, session, storesqlite.Music{ID: "playing"}, "seed")
	require.NoError(t, err)
	results := make(chan error, 20)
	var workers sync.WaitGroup
	for i := range 20 {
		workers.Go(func() {
			_, err := r.Enqueue(ctx, session, storesqlite.Music{ID: fmt.Sprint(i)}, fmt.Sprint(i))
			results <- err
		})
	}
	workers.Wait()
	close(results)
	accepted := 0
	for err := range results {
		if err == nil {
			accepted++
		} else {
			require.ErrorIs(t, err, ErrQueueCapacity)
		}
	}
	require.Equal(t, 3, accepted)
	persisted, err := r.queueRepo.LoadQueue(ctx, "lounge")
	require.NoError(t, err)
	require.Len(t, persisted, 3)
}

func TestBatch4PrioritySurvivesRebuildAndOrdinaryReorder(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	defer manager.Close()
	r, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	items, err := r.EnqueueMany(ctx, account.Session{PublicID: "user"}, []storesqlite.Music{{ID: "playing"}, {ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}, "seed")
	require.NoError(t, err)
	_, err = r.ToggleShuffle(ctx)
	require.NoError(t, err)
	require.NoError(t, r.Top(ctx, []string{items[2].QueueID, items[1].QueueID}, "batch-top"))
	require.NoError(t, r.Top(ctx, []string{items[3].QueueID}, "single-top"))
	require.NoError(t, r.ReorderByID(ctx, items[4].QueueID, items[3].QueueID, "before", "drag"))
	r.Close()
	r, err = manager.Room(ctx, "lounge")
	require.NoError(t, err)
	for i, wanted := range []string{"c", "a", "b"} {
		if i == 1 {
			// Natural completion uses the same priority selection as manual next.
			_, err = r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
				runtime.state.CurrentMusic.Duration = 1
				runtime.onTick(time.Now().Add(time.Second))
				return nil, nil
			})
		} else {
			_, err = r.Next(ctx)
		}
		require.NoError(t, err)
		snapshot, err := r.Snapshot(ctx)
		require.NoError(t, err)
		music := snapshot["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic)
		require.Equal(t, wanted, music.ID)
	}
}
