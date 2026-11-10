package realtime

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestManagerDoesNotReturnCachedRuntimeForDeletedRoom(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, nil)
	t.Cleanup(manager.Close)
	_, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "update room set deleted_at=? where id='lounge'", time.Now().UnixMilli())
		return err
	}))
	_, err = manager.Room(ctx, "lounge")
	require.ErrorIs(t, err, storesqlite.ErrRoomGone)
}

func TestRoomLookupWaitingForDatabaseDoesNotBlockOtherRoomClose(t *testing.T) {
	ctx := context.Background()
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, nil)
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	// Saturate the read pool to deterministically park a lookup in database I/O.
	first, err := store.Reader().Conn(ctx)
	require.NoError(t, err)
	defer first.Close()
	second, err := store.Reader().Conn(ctx)
	require.NoError(t, err)
	defer second.Close()
	lookupCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	lookupDone := make(chan error, 1)
	go func() { _, err := manager.Room(lookupCtx, "missing"); lookupDone <- err }()
	require.Eventually(t, func() bool { return store.Reader().Stats().WaitCount > 0 }, time.Second, time.Millisecond)
	closed := make(chan struct{})
	go func() { manager.CloseRoom("lounge"); close(closed) }()
	select {
	case <-closed:
	case <-time.After(time.Second):
		cancel()
		t.Fatal("a blocked database lookup holds the global manager lock")
	}
	require.ErrorIs(t, runtime.ctx.Err(), context.Canceled)
	cancel()
	require.ErrorIs(t, <-lookupDone, context.Canceled)
}

func TestConcurrentRoomLoadsReturnOneRuntime(t *testing.T) {
	manager := NewManager(realtimeStore(t), 32, time.Second, time.Hour, nil)
	t.Cleanup(manager.Close)
	type result struct {
		runtime *RoomRuntime
		err     error
	}
	results := make(chan result, 32)
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 32 {
		workers.Go(func() {
			<-start
			runtime, err := manager.Room(context.Background(), "lounge")
			results <- result{runtime, err}
		})
	}
	close(start)
	workers.Wait()
	close(results)
	var first *RoomRuntime
	for result := range results {
		require.NoError(t, result.err)
		if first == nil {
			first = result.runtime
		}
		require.Same(t, first, result.runtime)
	}
}

func TestManagerHonorsCanceledCallerForCachedRuntime(t *testing.T) {
	manager := NewManager(realtimeStore(t), 10, time.Second, time.Hour, nil)
	t.Cleanup(manager.Close)
	_, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = manager.Room(ctx, "lounge")
	require.ErrorIs(t, err, context.Canceled)
}
