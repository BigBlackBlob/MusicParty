package realtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func controlTestRuntime(t *testing.T) *RoomRuntime {
	m := NewManager(realtimeStore(t), 100, time.Second, time.Hour, nil)
	t.Cleanup(m.Close)
	r, err := m.Room(context.Background(), "lounge")
	require.NoError(t, err)
	return r
}

func TestControlMutationReplayConflictAndActor(t *testing.T) {
	r := controlTestRuntime(t)
	ctx := context.Background()
	m := ControlMutation{ScopeID: r.controlScope, ActorID: "user", MutationID: "one"}
	first, err := r.ToggleShuffle(ctx, m)
	require.NoError(t, err)
	replay, err := r.ToggleShuffle(ctx, m)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, first.Committed, replay.Committed)
	_, err = r.Next(ctx, m)
	require.ErrorIs(t, err, ErrMutationConflict)
	m.ActorID = "other"
	other, err := r.ToggleShuffle(ctx, m)
	require.NoError(t, err)
	require.Equal(t, first.Committed.StateVersion+1, other.Committed.StateVersion)
	m.ScopeID = "old-runtime"
	_, err = r.ToggleShuffle(ctx, m)
	require.ErrorIs(t, err, ErrMutationScope)
}

func TestControlMutationSeekReplayBeforePreconditionAndNoopRejection(t *testing.T) {
	r := controlTestRuntime(t)
	ctx := context.Background()
	m := ControlMutation{ScopeID: r.controlScope, ActorID: "user", MutationID: "seek"}
	_, err := r.Enqueue(ctx, account.Session{PublicID: "user"}, storesqlite.Music{ID: "song", Duration: 60000, Platform: "local"}, "")
	require.NoError(t, err)
	epoch := int64(1)
	first, err := r.Seek(ctx, 0, "user", false, &epoch, m)
	require.NoError(t, err)
	replayed, err := r.Seek(ctx, 0, "user", false, &epoch, m)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.Committed, replayed.Committed)
	_, err = r.Seek(ctx, 1, "user", false, &epoch, m)
	require.ErrorIs(t, err, ErrMutationConflict)
	m.MutationID = "rejected"
	_, err = r.Seek(ctx, 0, "user", false, &epoch, m)
	require.ErrorIs(t, err, ErrPreconditionFailed)
	rejected, err := r.Seek(ctx, 0, "user", false, &epoch, m)
	require.ErrorIs(t, err, ErrPreconditionFailed)
	require.True(t, rejected.Replayed)
	_, err = r.Next(ctx)
	require.NoError(t, err)
	m.MutationID = "noop"
	noop, err := r.Like(ctx, "user", nil, m)
	require.NoError(t, err)
	replayed, err = r.Like(ctx, "user", nil, m)
	require.NoError(t, err)
	require.False(t, replayed.Applied)
	require.True(t, replayed.Replayed)
	require.Equal(t, noop.Committed, replayed.Committed)
}

func TestControlMutationConcurrentTTLAndCapacity(t *testing.T) {
	r := controlTestRuntime(t)
	ctx := context.Background()
	m := ControlMutation{ScopeID: r.controlScope, ActorID: "user", MutationID: "same"}
	var wg sync.WaitGroup
	results := make(chan ControlResult, 20)
	errors := make(chan error, 20)
	for range 20 {
		wg.Add(1)
		go func() { defer wg.Done(); result, err := r.ToggleShuffle(ctx, m); results <- result; errors <- err }()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	executions := 0
	for result := range results {
		if !result.Replayed {
			executions++
		}
		require.Equal(t, int64(1), result.Committed.StateVersion)
	}
	require.Equal(t, 1, executions)
	for i := 1; i < controlMutationCapacity; i++ {
		m.MutationID = fmt.Sprint(i)
		_, err := r.ToggleShuffle(ctx, m)
		require.NoError(t, err)
	}
	m.MutationID = "overflow"
	_, err := r.ToggleShuffle(ctx, m)
	require.ErrorIs(t, err, ErrMutationCapacity)
	m.MutationID = "same"
	replay, err := r.ToggleShuffle(ctx, m)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	_, err = r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		key := controlKey{actor: "user", mutation: "same"}
		record := runtime.controlMutations[key]
		record.expires = time.Now().Add(-time.Second)
		runtime.controlMutations[key] = record
		return nil, nil
	})
	require.NoError(t, err)
	result, err := r.ToggleShuffle(ctx, m)
	require.NoError(t, err)
	require.False(t, result.Replayed)
}

func TestControlMutationUnknownIsReservedUntilScopeEnds(t *testing.T) {
	r := controlTestRuntime(t)
	ctx := context.Background()
	m := ControlMutation{ScopeID: r.controlScope, ActorID: "user", MutationID: "unknown"}
	calls := 0
	apply := func(*RoomRuntime) (any, error) { calls++; return nil, errors.New("uncertain commit") }
	_, err := r.executeControl(ctx, "control.next", 0, nil, []ControlMutation{m}, apply)
	require.Error(t, err)
	_, err = r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		key := controlKey{actor: "user", mutation: "unknown"}
		record := runtime.controlMutations[key]
		record.expires = time.Now().Add(-time.Hour)
		runtime.controlMutations[key] = record
		return nil, nil
	})
	require.NoError(t, err)
	_, err = r.executeControl(ctx, "control.next", 0, nil, []ControlMutation{m}, apply)
	require.ErrorIs(t, err, ErrMutationUnknown)
	require.Equal(t, 1, calls)
	other := controlTestRuntime(t)
	require.NotEqual(t, r.controlScope, other.controlScope)
	_, err = other.Next(ctx, m)
	require.ErrorIs(t, err, ErrMutationScope)
}

func TestControlMutationPauseAndNextExecuteOnce(t *testing.T) {
	r := controlTestRuntime(t)
	ctx := context.Background()
	for _, id := range []string{"first", "second", "third"} {
		_, err := r.Enqueue(ctx, account.Session{PublicID: "user"}, storesqlite.Music{ID: id, Duration: 60000, Platform: "local"}, "")
		require.NoError(t, err)
	}
	m := ControlMutation{ScopeID: r.controlScope, ActorID: "user", MutationID: "pause"}
	first, err := r.TogglePause(ctx, m)
	require.NoError(t, err)
	var expiry time.Time
	_, err = r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		expiry = runtime.controlMutations[controlKey{actor: "user", mutation: "pause"}].expires
		return nil, nil
	})
	require.NoError(t, err)
	replayed, err := r.TogglePause(ctx, m)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.Committed, replayed.Committed)
	_, err = r.Execute(ctx, func(runtime *RoomRuntime) (any, error) {
		require.True(t, runtime.state.Paused)
		require.Equal(t, expiry, runtime.controlMutations[controlKey{actor: "user", mutation: "pause"}].expires)
		return nil, nil
	})
	require.NoError(t, err)
	m.MutationID = "next"
	first, err = r.Next(ctx, m)
	require.NoError(t, err)
	replayed, err = r.Next(ctx, m)
	require.NoError(t, err)
	require.True(t, replayed.Replayed)
	require.Equal(t, first.Committed, replayed.Committed)
	state, err := r.Snapshot(ctx)
	require.NoError(t, err)
	require.Equal(t, "second", state["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic).ID)
}
