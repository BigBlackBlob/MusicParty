package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestQueueItemURLMatchesNowPlayingAfterAdvance(t *testing.T) {
	store := realtimeStore(t)
	manager := NewManager(store, 10, time.Second, time.Hour, &recordingBroadcaster{})
	t.Cleanup(manager.Close)
	runtime, err := manager.Room(context.Background(), "lounge")
	require.NoError(t, err)
	session := account.Session{PublicID: "user"}
	_, err = runtime.Enqueue(context.Background(), session, storesqlite.Music{ID: "first", Platform: "local", Duration: 60_000}, "first")
	require.NoError(t, err)
	_, err = runtime.Enqueue(context.Background(), session, storesqlite.Music{ID: "track:@special", Platform: "local", Duration: 60_000}, "special")
	require.NoError(t, err)
	before, err := runtime.Snapshot(context.Background())
	require.NoError(t, err)
	queued := before["queue"].([]clientQueueItem)
	require.Equal(t, "/api/local/media/track:@special", queued[0].Music.URL)
	_, err = runtime.Next(context.Background())
	require.NoError(t, err)
	current := nowPlaying(t, runtime)["music"].(*storesqlite.PlayableMusic)
	require.Equal(t, queued[0].Music.URL, current.URL)
}
