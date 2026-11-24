package realtime

import (
	"context"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestSoundCloudPlaybackSelectsEngineAtTrackStart(t *testing.T) {
	for _, ready := range []bool{true, false} {
		name := "download ready"
		if !ready {
			name = "download not ready"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			manager := NewManager(realtimeStore(t), 10, time.Second, time.Hour, nil)
			t.Cleanup(manager.Close)
			const id = "54202938"
			prepared := make(chan string, 2)
			cacheReady := make(chan struct{})
			manager.SetSoundCloudCache(func(string) bool {
				select {
				case <-cacheReady:
					return true
				default:
					return false
				}
			}, func(_ context.Context, id string) { prepared <- id })
			room, err := manager.Room(ctx, "lounge")
			require.NoError(t, err)
			user := account.Session{PublicID: "user"}
			first, err := room.Enqueue(ctx, user, storesqlite.Music{ID: "first", Platform: "local", Duration: 60000}, "first")
			require.NoError(t, err)
			require.NoError(t, room.ReportPlayback(ctx, "playback.started", PlaybackReport{PlayEpoch: 1, QueueID: first.QueueID}, nil))
			_, err = room.Enqueue(ctx, user, storesqlite.Music{ID: id, Platform: "soundcloud", CoverURL: "cover", Duration: 180000}, "soundcloud")
			require.NoError(t, err)
			select {
			case got := <-prepared:
				require.Equal(t, id, got, "enqueue after playback.started must still prepare the next track")
			default:
				t.Fatal("next SoundCloud track was not prepared")
			}
			if ready {
				close(cacheReady)
			}
			_, err = room.Next(ctx)
			require.NoError(t, err)
			assertMusic := func() {
				snapshot, err := room.Snapshot(ctx)
				require.NoError(t, err)
				track := snapshot["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic)
				require.Equal(t, "soundcloud", track.Platform)
				require.Equal(t, "cover", track.CoverURL)
				if ready {
					require.Equal(t, "audio", track.PlayerType)
					require.Equal(t, "/api/soundcloud/stream/"+id, track.URL)
				} else {
					require.Equal(t, "soundcloud-widget", track.PlayerType)
					require.Empty(t, track.URL)
				}
			}
			assertMusic()
			if !ready {
				close(cacheReady)
				assertMusic() // A late download must not switch an active iframe.
			}
		})
	}
}

func TestRestoredSoundCloudTrackFallsBackToWidgetWhenCacheIsGone(t *testing.T) {
	ctx := context.Background()
	manager := NewManager(realtimeStore(t), 10, time.Second, time.Hour, nil)
	t.Cleanup(manager.Close)
	const id = "54202938"
	manager.SetSoundCloudCache(func(string) bool { return false }, func(context.Context, string) {})
	room, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	user := account.Session{PublicID: "user"}
	first, err := room.Enqueue(ctx, user, storesqlite.Music{ID: "first", Platform: "local", Duration: 60000}, "first")
	require.NoError(t, err)
	require.NoError(t, room.ReportPlayback(ctx, "playback.started", PlaybackReport{PlayEpoch: 1, QueueID: first.QueueID}, nil))
	_, err = room.Enqueue(ctx, user, storesqlite.Music{ID: id, Platform: "soundcloud", Duration: 180000}, "soundcloud")
	require.NoError(t, err)
	_, err = room.Next(ctx)
	require.NoError(t, err)

	// A restored room must not keep pointing at a host stream whose session
	// cache no longer holds the audio.
	restored, err := manager.Room(ctx, "lounge")
	require.NoError(t, err)
	snapshot, err := restored.Snapshot(ctx)
	require.NoError(t, err)
	track := snapshot["nowPlaying"].(map[string]any)["music"].(*storesqlite.PlayableMusic)
	require.Equal(t, "soundcloud-widget", track.PlayerType)
	require.Empty(t, track.URL)
	require.Equal(t, id, track.EmbedID)
}
