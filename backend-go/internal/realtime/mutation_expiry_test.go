package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestFailedQueueMutationsCannotExhaustRoomLedger(t *testing.T) {
	r := controlTestRuntime(t)
	ctx := context.Background()
	for i := range 2100 {
		id, err := json.Marshal([2]string{"actor", fmt.Sprint(i)})
		require.NoError(t, err)
		_, err = r.executeQueue(ctx, string(id), "enqueue", func(*RoomRuntime, context.Context) (any, error) {
			return nil, storesqlite.ErrLocalTrackGone
		})
		require.ErrorIs(t, err, storesqlite.ErrLocalTrackGone)
	}
	_, err := r.executeQueue(ctx, `["other","valid"]`, "enqueue", func(*RoomRuntime, context.Context) (any, error) { return "accepted", nil })
	require.NoError(t, err)
	_, err = r.Execute(ctx, func(runtime *RoomRuntime, _ context.Context) (any, error) {
		require.Len(t, runtime.mutations, 1)
		require.Len(t, runtime.mutationOrder, 1)
		return nil, nil
	})
	require.NoError(t, err)
}

func TestQueueMutationExpiryReleasesActorBudgetWithoutExtendingReplays(t *testing.T) {
	r := controlTestRuntime(t)
	ctx := context.Background()
	id := func(actor, mutation string) string {
		encoded, err := json.Marshal([2]string{actor, mutation})
		require.NoError(t, err)
		return string(encoded)
	}
	calls := 0
	apply := func(*RoomRuntime, context.Context) (any, error) { calls++; return calls, nil }
	for i := range mutationActorCapacity {
		_, err := r.executeQueue(ctx, id("actor", fmt.Sprint(i)), "enqueue", apply)
		require.NoError(t, err)
	}
	_, err := r.executeQueue(ctx, id("actor", "overflow"), "enqueue", apply)
	require.ErrorIs(t, err, ErrMutationCapacity)
	_, err = r.executeQueue(ctx, id("other", "admitted"), "enqueue", apply)
	require.NoError(t, err)
	value, err := r.executeQueue(ctx, id("actor", "0"), "enqueue", apply)
	require.NoError(t, err)
	require.Equal(t, 1, value)
	require.Equal(t, mutationActorCapacity+1, calls)
	_, err = r.Execute(ctx, func(runtime *RoomRuntime, _ context.Context) (any, error) {
		for key, record := range runtime.mutations {
			record.expires = time.Now().Add(-time.Second)
			runtime.mutations[key] = record
		}
		runtime.purgeQueueMutations(time.Now())
		require.Empty(t, runtime.mutations)
		require.Empty(t, runtime.mutationOrder)
		require.Empty(t, runtime.mutationCounts)
		return nil, nil
	})
	require.NoError(t, err)
	_, err = r.executeQueue(ctx, id("actor", "0"), "new-fingerprint", apply)
	require.NoError(t, err)
	require.Equal(t, mutationActorCapacity+2, calls)
}
