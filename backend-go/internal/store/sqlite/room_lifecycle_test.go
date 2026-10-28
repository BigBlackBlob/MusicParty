package sqlite

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRoomWritesQueuedBehindDeletionCannotRecreateData(t *testing.T) {
	ctx := context.Background()
	roomID := "deleted-room"
	music := Music{ID: "song", Name: "Song", Artists: []string{"Artist"}, Platform: "local"}
	queue := []QueueItem{{QueueID: "queued", Music: music, EnqueuedBy: UserSummary{PublicID: "user", Name: "User"}, Status: "READY"}}
	state := PlaybackState{RoomID: roomID, LikedUserIDs: map[string]struct{}{}, LikeMarkers: []int64{}}
	message := ChatMessage{ID: "message", UserID: "user", UserName: "User", Content: "Message", Type: "CHAT"}
	for _, test := range []struct {
		name, table string
		write       func(*Store) error
	}{
		{"queue", "room_queue", func(s *Store) error { return NewQueueRepository(s, nil).SynchronizeQueue(ctx, roomID, queue) }},
		{"playback", "room_playback_state", func(s *Store) error { return NewPlaybackStateRepository(s).Upsert(ctx, state) }},
		{"queue and playback", "room_playback_state", func(s *Store) error {
			return NewRealtimeRepository(s, nil).CommitRoomState(ctx, roomID, queue, state, nil)
		}},
		{"history", "room_history", func(s *Store) error {
			return NewQueueRepository(s, nil).AppendHistory(ctx, HistoryEntry{ID: "history", RoomID: roomID, Music: music})
		}},
		{"room chat", "chat_message", func(s *Store) error { return NewChatRepository(s).AppendMessage(ctx, &roomID, message) }},
		{"room playlist", "room_playlist", func(s *Store) error {
			_, err := NewRoomPlaylistRepository(s, nil, nil).CreatePlaylist(ctx, roomID, "Late playlist")
			return err
		}},
		{"membership", "room_membership", func(s *Store) error {
			return NewRoomAccessRepository(s).UpsertMembership(ctx, RoomMembership{RoomID: roomID, PublicID: "user", Role: "MEMBER"})
		}},
		{"invite", "room_invite", func(s *Store) error {
			return NewRoomAccessRepository(s).CreateInvite(ctx, RoomInvite{ID: "invite", RoomID: roomID, CreatedByPublicID: "user", SecretHash: "hash"})
		}},
		{"source binding", "room_subsonic_source", func(s *Store) error {
			return NewSubsonicSourceRepository(s).UpsertRoomBinding(ctx, RoomSubsonicSource{RoomID: roomID, SourceID: "source"})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := openContractStore(t)
			defer store.Close()
			seedRoomAndUsers(t, store, roomID, "user")
			require.NoError(t, NewSubsonicSourceRepository(store).Upsert(ctx, SubsonicSource{ID: "source", Label: "Source"}))
			deletionReady, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			deleted := make(chan error, 1)
			go func() {
				deleted <- store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
					if _, err := tx.ExecContext(ctx, "update room set deleted_at=1 where id=?", roomID); err != nil {
						return err
					}
					close(deletionReady)
					<-release
					return nil
				})
			}()
			<-deletionReady
			written := make(chan error, 1)
			go func() { written <- test.write(store) }()
			require.Eventually(t, func() bool { return len(store.writes) > 0 }, time.Second, time.Millisecond, "write must queue behind the uncommitted deletion")
			once.Do(func() { close(release) })
			require.NoError(t, <-deleted)
			require.ErrorIs(t, <-written, ErrRoomGone)
			var count int
			require.NoError(t, store.Reader().QueryRowContext(ctx, "select count(*) from "+test.table+" where room_id=?", roomID).Scan(&count))
			require.Zero(t, count, "deleted room data must remain absent")
			require.NoError(t, NewChatRepository(store).AppendMessage(ctx, nil, message), "public chat does not belong to the deleted room")
		})
	}
}
