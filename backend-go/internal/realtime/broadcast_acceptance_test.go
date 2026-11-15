package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestThousandItemQueueBroadcastsCompactPlaybackAndIncrementalNext(t *testing.T) {
	ctx := context.Background()
	broadcaster := &recordingBroadcaster{}
	manager := NewManager(realtimeStore(t), 32, 5*time.Second, time.Hour, broadcaster, QueueLimits{MaxSize: 1000, MaxUserSongs: 1000, HistorySize: 50})
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	music := make([]storesqlite.Music, 1001)
	for i := range music {
		music[i] = storesqlite.Music{ID: fmt.Sprint(i), Platform: "local", Duration: 60000}
	}
	_, err = runtime.EnqueueMany(ctx, account.Session{PublicID: "user"}, music, "")
	require.NoError(t, err)
	_, err = runtime.NextAs(ctx, "user", false)
	require.NoError(t, err)
	snapshot, err := runtime.Snapshot(ctx)
	require.NoError(t, err)
	require.Len(t, snapshot["queue"], 999, "initialization and resync still have the complete queue")
	broadcaster.mu.Lock()
	defer broadcaster.mu.Unlock()
	patches := []map[string]any{}
	for _, event := range broadcaster.events {
		payload := event.payload.(map[string]any)
		if event.kind == "player.state" {
			require.NotContains(t, payload, "queue")
			encoded, err := json.Marshal(payload)
			require.NoError(t, err)
			require.Less(t, len(encoded), 2000, "playback updates must not scale with the pending queue")
		}
		if event.kind == "queue.patch" {
			patches = append(patches, payload)
		}
	}
	require.Len(t, patches, 2)
	require.Equal(t, "append", patches[0]["operation"])
	require.Len(t, patches[0]["items"], 1000, "the first playing song must not remain in the pending queue patch")
	require.Equal(t, "remove", patches[1]["operation"])
	require.Len(t, patches[1]["queueIds"], 1)
	require.NotContains(t, patches[1], "queue")
	encoded, err := json.Marshal(patches[1])
	require.NoError(t, err)
	require.Less(t, len(encoded), 300, "a skip broadcasts only the consumed queue ID")
}

func TestEnqueueStartsExistingPendingSongWithConvergentPatch(t *testing.T) {
	for _, batch := range []bool{false, true} {
		t.Run(fmt.Sprintf("batch=%t", batch), func(t *testing.T) {
			ctx := context.Background()
			store := realtimeStore(t)
			old := []storesqlite.QueueItem{
				{QueueID: "waiting", Music: storesqlite.Music{ID: "waiting", Platform: "local", Duration: 60000}, EnqueuedBy: storesqlite.UserSummary{PublicID: "user"}, Status: "PENDING"},
				{QueueID: "priority", Music: storesqlite.Music{ID: "priority", Platform: "local", Duration: 60000}, EnqueuedBy: storesqlite.UserSummary{PublicID: "user"}, Status: "PENDING", PlaybackPriority: 1},
			}
			require.NoError(t, storesqlite.NewQueueRepository(store, time.Now).SynchronizeQueue(ctx, "lounge", old))
			broadcaster := &recordingBroadcaster{}
			manager := NewManager(store, 32, 5*time.Second, time.Hour, broadcaster)
			t.Cleanup(manager.Close)
			runtime, err := manager.Room(ctx, "lounge")
			require.NoError(t, err)
			music := storesqlite.Music{ID: "added", Platform: "local", Duration: 60000}
			if batch {
				_, err = runtime.EnqueueMany(ctx, account.Session{PublicID: "user"}, []storesqlite.Music{music, music}, "")
			} else {
				_, err = runtime.Enqueue(ctx, account.Session{PublicID: "user"}, music, "")
			}
			require.NoError(t, err)
			snapshot, err := runtime.Snapshot(ctx)
			require.NoError(t, err)
			broadcaster.mu.Lock()
			defer broadcaster.mu.Unlock()
			patch := broadcaster.events[0].payload.(map[string]any)
			require.Equal(t, []string{"priority"}, patch["queueIds"])
			pending := patch["items"].([]clientQueueItem)
			projected := append(clientQueue(old[:1]), pending...)
			require.Equal(t, snapshot["queue"], projected, "applying the patch to the old queue must match a full resync")
		})
	}
}
