package media

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func seedReferencingRoom(t *testing.T, store *storesqlite.Store, roomID, trackID string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, storesqlite.NewRoomRepository(store).Upsert(ctx, storesqlite.Room{ID: roomID, Name: roomID, OwnerPublicID: "user", Visibility: "PUBLIC", CreatedAt: 1, LastActiveAt: 1}))
	music := `{"id":"` + trackID + `","name":"Gone","artists":["A"],"duration":1000,"platform":"local","coverUrl":""}`
	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "insert into room_queue(id, room_id, music_json, enqueuer_public_id, enqueuer_name_snapshot, status, sort_order, created_at) values (?, ?, ?, ?, ?, ?, ?, ?)",
			"queue-"+roomID, roomID, `{"queueId":"queue-`+roomID+`","music":`+music+`,"enqueuedBy":{"publicId":"user","name":"User","guest":false},"status":"PENDING"}`, "user", "User", "PENDING", 0, 1)
		return err
	}))
	require.NoError(t, store.Write(ctx, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, "insert into room_playback_state(room_id, current_music_json, position_anchor, timestamp_anchor, position_updated_at, last_persisted_at) values (?, ?, ?, ?, ?, ?)",
			roomID, music, 0, 1, 1, 1)
		return err
	}))
}

func seedCompletedTrack(t *testing.T, repository *storesqlite.LocalTrackRepository, root, id, outputRelative string) {
	t.Helper()
	hash, output := "hash-"+id, outputRelative
	require.NoError(t, repository.Upsert(context.Background(), storesqlite.LocalTrack{ID: id, OriginalHash: &hash, OGGPath: &output, Title: id, Artists: []string{"a"}, Status: "COMPLETED", CreatedAt: 1, UpdatedAt: 1}))
	path, err := ResolveWithin(root, outputRelative)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte("payload"), 0o640))
}

// The bug: deleting a local track left the room runtime serving it from memory. The library now tells
// the owner of the realtime state which rooms referenced the track, once the references are gone.
func TestDeletingLocalTrackNotifiesReferencingRooms(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		roomIDs  []string
		wantCall int
	}{
		{name: "referenced rooms", roomIDs: []string{"room-a", "room-b"}, wantCall: 1},
		{name: "no references", roomIDs: nil, wantCall: 0},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			fixture := newLibraryFixture(t)
			seedCompletedTrack(t, fixture.repository, fixture.root, "track", "media/track.ogg")
			for _, roomID := range scenario.roomIDs {
				seedReferencingRoom(t, fixture.store, roomID, "track")
			}
			var calls [][]string
			var tracks []string
			fixture.library.SetTracksRemovedHook(func(ctx context.Context, trackID string, roomIDs []string) {
				tracks = append(tracks, trackID)
				calls = append(calls, roomIDs)
			})
			require.NoError(t, fixture.library.Delete(context.Background(), "track"))
			require.Len(t, calls, scenario.wantCall)
			if scenario.wantCall > 0 {
				require.Equal(t, []string{"room-a", "room-b"}, calls[0])
				require.Equal(t, []string{"track"}, tracks, "the hook names the track so the caller can drop it from memory")
			}
			rooms, err := fixture.repository.RoomsReferencingLocalTrack(context.Background(), "track")
			require.NoError(t, err)
			require.Empty(t, rooms, "the references must be gone once the hook runs")
			track, err := fixture.repository.FindByID(context.Background(), "track")
			require.NoError(t, err)
			require.Equal(t, "DELETED", track.Status)
		})
	}
}
