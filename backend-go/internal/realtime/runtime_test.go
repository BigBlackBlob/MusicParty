package realtime

import (
	"context"
	"path/filepath"
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
	manager := NewManager(store, 100, 5*time.Second, time.Hour, broadcaster, nil)
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
	manager := NewManager(store, 10, time.Second, 1200*time.Millisecond, broadcaster, nil)
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
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{}, nil)
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	_, err = runtime.Enqueue(context.Background(), account.Session{PublicID: "user", DisplayName: "User"}, storesqlite.Music{ID: "track", Name: "Track", Duration: 60_000}, "one")
	require.NoError(t, err)
	time.Sleep(30 * time.Millisecond)
	require.NoError(t, runtime.TogglePause(context.Background()))
	first, err := runtime.Snapshot(context.Background())
	require.NoError(t, err)
	time.Sleep(30 * time.Millisecond)
	second, err := runtime.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, first["nowPlaying"].(map[string]any)["currentPosition"], second["nowPlaying"].(map[string]any)["currentPosition"])
}
