package realtime

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestLikeAndUnlikeRollbackPersonalPlaylistWhenRoomWriteFails(t *testing.T) {
	for _, liked := range []bool{true, false} {
		t.Run(map[bool]string{true: "like", false: "unlike"}[liked], func(t *testing.T) {
			ctx := context.Background()
			store := realtimeStore(t)
			broadcaster := &recordingBroadcaster{}
			manager := NewManager(store, 10, time.Second, time.Hour, broadcaster)
			t.Cleanup(manager.Close)
			runtime, err := manager.Room(ctx, "lounge")
			require.NoError(t, err)
			_, err = runtime.Enqueue(ctx, account.Session{PublicID: "user"}, storesqlite.Music{ID: "song", Platform: "local", Duration: 60000}, "")
			require.NoError(t, err)
			if !liked {
				_, err = runtime.Like(ctx, "user", nil)
				require.NoError(t, err)
			}
			before, err := runtime.Snapshot(ctx)
			require.NoError(t, err)
			broadcasts := broadcaster.count("player.state")
			require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `create trigger reject_like_state before insert on room_playback_state begin select raise(abort, 'injected room state failure'); end`)
				return err
			}))
			apply := func() error {
				if liked {
					_, err := runtime.Like(ctx, "user", nil)
					return err
				}
				_, err := runtime.Unlike(ctx, "user", nil)
				return err
			}
			require.ErrorContains(t, apply(), "injected room state failure")
			after, err := runtime.Snapshot(ctx)
			require.NoError(t, err)
			require.Equal(t, before["stateVersion"], after["stateVersion"])
			for _, field := range []string{"likedUserIds", "likeMarkers", "playEpoch"} {
				require.Equal(t, before["nowPlaying"].(map[string]any)[field], after["nowPlaying"].(map[string]any)[field])
			}
			require.Equal(t, broadcasts, broadcaster.count("player.state"))
			persisted, err := storesqlite.NewPlaybackStateRepository(store).FindByRoomID(ctx, "lounge")
			require.NoError(t, err)
			require.Equal(t, before["stateVersion"], persisted.StateVersion)
			_, persistedLike := persisted.LikedUserIDs["user"]
			require.Equal(t, !liked, persistedLike)
			var tracks int
			require.NoError(t, store.Reader().QueryRowContext(ctx, `select count(*) from user_playlist_track t join user_playlist p on p.id=t.playlist_id where p.owner_public_id='user' and p.system_key=?`, LikedSongsSystemKey).Scan(&tracks))
			if liked {
				require.Zero(t, tracks, "failed room like must roll back the personal track")
				var playlists int
				require.NoError(t, store.Reader().QueryRowContext(ctx, `select count(*) from user_playlist where owner_public_id='user' and system_key=?`, LikedSongsSystemKey).Scan(&playlists))
				require.Zero(t, playlists, "first-like playlist creation must roll back too")
			} else {
				require.Equal(t, 1, tracks, "failed room unlike must preserve the personal track")
			}
			require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, "drop trigger reject_like_state")
				return err
			}))
			require.NoError(t, apply(), "rolled back operations remain retryable")
		})
	}
}
