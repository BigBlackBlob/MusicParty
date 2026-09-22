package realtime

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

type recordedBroadcast struct {
	room, kind string
	payload    any
}

type recordingBroadcaster struct {
	mu     sync.Mutex
	events []recordedBroadcast
}

func (b *recordingBroadcaster) BroadcastRoom(room, kind string, payload any) {
	b.mu.Lock()
	b.events = append(b.events, recordedBroadcast{room: room, kind: kind, payload: payload})
	b.mu.Unlock()
}
func (b *recordingBroadcaster) BroadcastAll(string, any) {}
func (b *recordingBroadcaster) count(kind string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, event := range b.events {
		if event.kind == kind {
			count++
		}
	}
	return count
}

func realtimeStore(t *testing.T) *storesqlite.Store {
	t.Helper()
	ctx := context.Background()
	store, err := storesqlite.OpenStore(ctx, storesqlite.StoreConfig{Path: filepath.Join(t.TempDir(), "realtime.db"), BusyTimeout: time.Second, ReadConnections: 2})
	require.NoError(t, err)
	require.NoError(t, storesqlite.EnsureCompatibleSchema(ctx, store, true))
	now := time.Now().UnixMilli()
	require.NoError(t, storesqlite.NewUserProfileRepository(store, time.Now).UpsertProfile(ctx, storesqlite.UserProfile{PublicID: "user", DisplayName: "User", CurrentRoomID: "lounge", CreatedAt: now, LastSeenAt: now}))
	require.NoError(t, storesqlite.NewRoomRepository(store).Upsert(ctx, storesqlite.Room{ID: "lounge", Name: "Lounge", OwnerPublicID: "user", Visibility: "PUBLIC", CreatedAt: now, LastActiveAt: now}))
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}

func TestRoomRuntimeSerializesConcurrentMutationsAndDeduplicates(t *testing.T) {
	store := realtimeStore(t)
	broadcaster := &recordingBroadcaster{}
	manager := NewManager(store, 100, 5*time.Second, time.Hour, broadcaster)
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user", DisplayName: "User"}

	var wg sync.WaitGroup
	errs := make(chan error, 50)
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, enqueueErr := runtime.Enqueue(context.Background(), session, storesqlite.Music{ID: string(rune('a' + i)), Name: "Track", Duration: 60_000, Platform: "local"}, "mutation-"+string(rune('a'+i)))
			errs <- enqueueErr
		}()
	}
	wg.Wait()
	close(errs)
	for enqueueErr := range errs {
		require.NoError(t, enqueueErr)
	}
	snapshot, err := runtime.Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot["queue"], 49)
	require.NotNil(t, snapshot["nowPlaying"])

	_, err = runtime.Enqueue(context.Background(), session, storesqlite.Music{ID: "duplicate", Name: "Duplicate"}, "mutation-a")
	require.ErrorIs(t, err, ErrDuplicate)
	require.Equal(t, 50, broadcaster.count("queue.patch"))
}

func TestRoomRuntimeProgressAutoAdvanceAndIdleEviction(t *testing.T) {
	store := realtimeStore(t)
	broadcaster := &recordingBroadcaster{}
	manager := NewManager(store, 10, time.Second, 1200*time.Millisecond, broadcaster)
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user", DisplayName: "User"}
	_, err = runtime.Enqueue(context.Background(), session, storesqlite.Music{ID: "short", Name: "Short", Duration: 10, Platform: "local"}, "one")
	require.NoError(t, err)
	_, err = runtime.Enqueue(context.Background(), session, storesqlite.Music{ID: "next", Name: "Next", Duration: 60_000, Platform: "local"}, "two")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		snapshot, snapshotErr := runtime.Snapshot(context.Background())
		if snapshotErr != nil {
			return false
		}
		nowPlaying, ok := snapshot["nowPlaying"].(map[string]any)
		if !ok {
			return false
		}
		music, ok := nowPlaying["music"].(*storesqlite.PlayableMusic)
		return ok && music.ID == "next"
	}, 3*time.Second, 50*time.Millisecond)
	require.GreaterOrEqual(t, broadcaster.count("player.progress"), 1)

	time.Sleep(3 * time.Second)
	_, err = runtime.Snapshot(context.Background())
	require.Error(t, err)
}

func TestPauseFreezesCurrentPosition(t *testing.T) {
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	_, err = runtime.Enqueue(context.Background(), account.Session{PublicID: "user", DisplayName: "User"}, storesqlite.Music{ID: "track", Name: "Track", Duration: 60_000}, "one")
	require.NoError(t, err)
	time.Sleep(30 * time.Millisecond)
	_, err = runtime.TogglePause(context.Background())
	require.NoError(t, err)
	first, err := runtime.Snapshot(context.Background())
	require.NoError(t, err)
	time.Sleep(30 * time.Millisecond)
	second, err := runtime.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, first["nowPlaying"].(map[string]any)["currentPosition"], second["nowPlaying"].(map[string]any)["currentPosition"])
}

func TestControlResultsCarryWatermarksAndPreconditions(t *testing.T) {
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user", DisplayName: "User"}
	_, err = runtime.Enqueue(context.Background(), session, storesqlite.Music{ID: "track", Name: "Track", Duration: 60_000}, "one")
	require.NoError(t, err)

	result, err := runtime.TogglePause(context.Background())
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Committed)
	require.EqualValues(t, 1, result.Committed.PlayEpoch)
	require.GreaterOrEqual(t, result.Committed.StateVersion, int64(1))

	staleEpoch := int64(99)
	_, err = runtime.Seek(context.Background(), 1000, session.PublicID, false, &staleEpoch)
	require.ErrorIs(t, err, ErrPreconditionFailed)

	_, err = runtime.Seek(context.Background(), 1000, "other", false, nil)
	require.ErrorIs(t, err, ErrSeekForbidden)

	currentEpoch := result.Committed.PlayEpoch
	result, err = runtime.Seek(context.Background(), 1000, session.PublicID, false, &currentEpoch)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Committed)
	require.EqualValues(t, 2, result.Committed.PlayEpoch)

	result, err = runtime.Like(context.Background(), session.PublicID, nil)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Committed)

	_, err = runtime.Like(context.Background(), session.PublicID, &staleEpoch)
	require.ErrorIs(t, err, ErrPreconditionFailed)
	likedWatermark := *result.Committed
	result, err = runtime.Like(context.Background(), session.PublicID, nil)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Equal(t, &likedWatermark, result.Committed)

	result, err = runtime.ToggleShuffle(context.Background())
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Committed)

	result, err = runtime.Next(context.Background())
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.NotNil(t, result.Committed)
	// advance with an empty queue clears playback without bumping the epoch
	require.EqualValues(t, 2, result.Committed.PlayEpoch)
	emptyWatermark := *result.Committed
	result, err = runtime.Like(context.Background(), session.PublicID, nil)
	require.NoError(t, err)
	require.False(t, result.Applied)
	require.Equal(t, &emptyWatermark, result.Committed)
}

func TestPreviousWalksHistoryCursorAndResetsOnAdvance(t *testing.T) {
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user", DisplayName: "User"}
	for index, id := range []string{"a", "b", "c"} {
		_, err = runtime.Enqueue(context.Background(), session, storesqlite.Music{ID: id, Name: strings.ToUpper(id), Duration: 60_000, Platform: "local"}, fmt.Sprintf("m%d", index))
		require.NoError(t, err)
	}
	snapshot, err := runtime.Snapshot(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 0, snapshot["historyCursor"])
	_, err = runtime.Previous(context.Background(), nil)
	require.ErrorIs(t, err, ErrNoHistory)

	result, err := runtime.Next(context.Background())
	require.NoError(t, err)
	require.True(t, result.Applied)
	staleEpoch := result.Committed.PlayEpoch - 5
	_, err = runtime.Previous(context.Background(), &staleEpoch)
	require.ErrorIs(t, err, ErrPreconditionFailed)

	snapshot, err = runtime.Snapshot(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 1, snapshot["historyCursor"])
	epoch := snapshot["playEpoch"].(int64)
	result, err = runtime.Previous(context.Background(), &epoch)
	require.NoError(t, err)
	require.True(t, result.Applied)
	snapshot, err = runtime.Snapshot(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 0, snapshot["historyCursor"])
	require.Equal(t, "a", snapshot["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic).ID)
	_, err = runtime.Previous(context.Background(), nil)
	require.ErrorIs(t, err, ErrNoHistory)

	// advance from a walked-back position re-pushes the current track and
	// resets the cursor over the extended history.
	result, err = runtime.Next(context.Background())
	require.NoError(t, err)
	require.True(t, result.Applied)
	snapshot, err = runtime.Snapshot(context.Background())
	require.NoError(t, err)
	require.EqualValues(t, 2, snapshot["historyCursor"])
	require.Equal(t, "c", snapshot["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic).ID)

	_, err = runtime.Enqueue(context.Background(), session, storesqlite.Music{ID: "d", Name: "D", Duration: 60_000, Platform: "local"}, "m3")
	require.NoError(t, err)
	require.NoError(t, runtime.Clear(context.Background(), "clear-1"))
	snapshot, err = runtime.Snapshot(context.Background())
	require.NoError(t, err)
	require.Empty(t, snapshot["queue"].([]storesqlite.QueueItem))
	require.ErrorIs(t, runtime.Clear(context.Background(), "clear-2"), ErrStaleQueue)
}

func TestTakeNextPreservesListAndShuffleQueueSemantics(t *testing.T) {
	items := []storesqlite.QueueItem{
		{QueueID: "a"}, {QueueID: "b"}, {QueueID: "c"}, {QueueID: "d"},
	}
	state := storesqlite.PlaybackState{}
	selected := takeNext(&state, &items)
	require.Equal(t, "a", selected.QueueID)
	require.Equal(t, []string{"b", "c", "d"}, queueIDs(items))

	items = []storesqlite.QueueItem{{QueueID: "a"}, {QueueID: "b"}, {QueueID: "c"}, {QueueID: "d"}}
	state.Shuffle = true
	selected = takeNext(&state, &items)
	require.NotContains(t, queueIDs(items), selected.QueueID)
	require.Len(t, items, 3)
	originalOrder := map[string]int{"a": 0, "b": 1, "c": 2, "d": 3}
	for index := 1; index < len(items); index++ {
		require.Less(t, originalOrder[items[index-1].QueueID], originalOrder[items[index].QueueID])
	}
}

func TestQueueEventsMonotonicAndStaleMutationsRejected(t *testing.T) {
	store := realtimeStore(t)
	b := &recordingBroadcaster{}
	m := NewManager(store, 10, time.Second, time.Hour, b)
	t.Cleanup(m.Close)
	r, err := m.Room(context.Background(), "lounge")
	require.NoError(t, err)
	s := account.Session{PublicID: "user", DisplayName: "User"}
	_, err = r.Enqueue(context.Background(), s, storesqlite.Music{ID: "a", Name: "A"}, "m1")
	require.NoError(t, err)
	item, err := r.Enqueue(context.Background(), s, storesqlite.Music{ID: "b", Name: "B"}, "m2")
	require.NoError(t, err)
	require.ErrorIs(t, r.Remove(context.Background(), []string{"missing"}, "stale"), ErrStaleQueue)
	require.NoError(t, r.Remove(context.Background(), []string{item.QueueID}, "m3"))
	b.mu.Lock()
	defer b.mu.Unlock()
	var versions []int64
	for _, e := range b.events {
		if e.kind != "queue.patch" {
			continue
		}
		if p, ok := e.payload.(map[string]any); ok {
			if v, ok := p["queueVersion"].(int64); ok {
				versions = append(versions, v)
			}
		}
	}
	require.Equal(t, []int64{1, 2, 3}, versions)
}

func TestConcurrentClientsQueueConflictAndDeduplication(t *testing.T) {
	store := realtimeStore(t)
	m := NewManager(store, 20, time.Second, time.Hour, &recordingBroadcaster{})
	t.Cleanup(m.Close)
	r, err := m.Room(context.Background(), "lounge")
	require.NoError(t, err)
	s := account.Session{PublicID: "user", DisplayName: "User"}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _ = r.Enqueue(context.Background(), s, storesqlite.Music{ID: string(rune('a' + i)), Name: "x"}, "client-"+string(rune('a'+i)))
		}(i)
	}
	wg.Wait()
	_, err = r.Enqueue(context.Background(), s, storesqlite.Music{ID: "dup", Name: "dup"}, "client-a")
	require.ErrorIs(t, err, ErrDuplicate)
}

func queueIDs(items []storesqlite.QueueItem) []string {
	ids := make([]string, len(items))
	for index := range items {
		ids[index] = items[index].QueueID
	}
	return ids
}
