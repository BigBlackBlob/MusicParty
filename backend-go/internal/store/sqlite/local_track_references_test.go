package sqlite

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

func seedReferenceRoom(t *testing.T, store *Store, id string) {
	t.Helper()
	if err := NewRoomRepository(store).Upsert(context.Background(), Room{ID: id, Name: id, OwnerPublicID: "user", Visibility: "PUBLIC", CreatedAt: 1, LastActiveAt: 1}); err != nil {
		t.Fatalf("seed room %s: %v", id, err)
	}
}

func execReferenceSQL(t *testing.T, store *Store, query string, args ...any) {
	t.Helper()
	if err := store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, query, args...)
		return err
	}); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}

// seedTrackReferences puts the track into a room queue, a room playback state and a room history, and
// adds rows that mention the same id on another platform so the platform filter is covered too.
func seedTrackReferences(t *testing.T, store *Store, trackID string) {
	t.Helper()
	now := time.Now().UnixMilli()
	localMusic := `{"id":"` + trackID + `","name":"Gone","artists":["A"],"duration":1000,"platform":"local","coverUrl":""}`
	for _, roomID := range []string{"room-history", "room-playback", "room-queue", "room-other"} {
		seedReferenceRoom(t, store, roomID)
	}
	execReferenceSQL(t, store, "insert into room_queue(id, room_id, music_json, enqueuer_public_id, enqueuer_name_snapshot, status, sort_order, created_at) values (?, ?, ?, ?, ?, ?, ?, ?)",
		"queue-1", "room-queue", `{"queueId":"queue-1","music":`+localMusic+`,"enqueuedBy":{"publicId":"user","name":"User","guest":false},"status":"PENDING"}`, "user", "User", "PENDING", 0, now)
	execReferenceSQL(t, store, "insert into room_playback_state(room_id, current_music_json, position_anchor, timestamp_anchor, position_updated_at, last_persisted_at) values (?, ?, ?, ?, ?, ?)",
		"room-playback", localMusic, 0, now, now, now)
	execReferenceSQL(t, store, "insert into room_history(id, room_id, music_json, enqueuer_public_id, played_at) values (?, ?, ?, ?, ?)",
		"history-1", "room-history", localMusic, "user", now)
	execReferenceSQL(t, store, "insert into room_queue(id, room_id, music_json, enqueuer_public_id, enqueuer_name_snapshot, status, sort_order, created_at) values (?, ?, ?, ?, ?, ?, ?, ?)",
		"queue-2", "room-other", `{"queueId":"queue-2","music":{"id":"shared-id","name":"Remote","artists":["A"],"duration":1000,"platform":"netease","coverUrl":""},"enqueuedBy":{"publicId":"user","name":"User","guest":false},"status":"PENDING"}`, "user", "User", "PENDING", 0, now)
}

func TestRoomsReferencingLocalTrack(t *testing.T) {
	ctx := context.Background()
	for _, scenario := range []struct {
		name    string
		trackID string
		want    []string
	}{
		{name: "queue playback state and history", trackID: "track-gone", want: []string{"room-history", "room-playback", "room-queue"}},
		{name: "unreferenced local track", trackID: "track-keep", want: []string{}},
		{name: "same id on another platform", trackID: "shared-id", want: []string{}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := openLocalTrackTestStore(t)
			seedTrackReferences(t, store, "track-gone")
			rooms, err := NewLocalTrackRepository(store).RoomsReferencingLocalTrack(ctx, scenario.trackID)
			if err != nil {
				t.Fatalf("RoomsReferencingLocalTrack() error = %v", err)
			}
			if len(rooms) != len(scenario.want) {
				t.Fatalf("RoomsReferencingLocalTrack() = %v, want %v", rooms, scenario.want)
			}
			for index, roomID := range scenario.want {
				if rooms[index] != roomID {
					t.Fatalf("RoomsReferencingLocalTrack() = %v, want %v", rooms, scenario.want)
				}
			}
		})
	}
}

// Once the references are removed the rooms must stop being reported, otherwise the realtime layer
// would be reloaded forever.
func TestRoomsReferencingLocalTrackAfterDeleteLocalReferences(t *testing.T) {
	ctx := context.Background()
	store := openLocalTrackTestStore(t)
	seedTrackReferences(t, store, "track-gone")
	repository := NewLocalTrackRepository(store)
	before, err := repository.RoomsReferencingLocalTrack(ctx, "track-gone")
	if err != nil {
		t.Fatalf("RoomsReferencingLocalTrack() error = %v", err)
	}
	if len(before) != 3 {
		t.Fatalf("RoomsReferencingLocalTrack() before delete = %v, want three rooms", before)
	}
	if err := repository.DeleteLocalReferences(ctx, "track-gone"); err != nil {
		t.Fatalf("DeleteLocalReferences() error = %v", err)
	}
	after, err := repository.RoomsReferencingLocalTrack(ctx, "track-gone")
	if err != nil {
		t.Fatalf("RoomsReferencingLocalTrack() error = %v", err)
	}
	if len(after) != 0 {
		t.Fatalf("RoomsReferencingLocalTrack() after delete = %v, want none", after)
	}
}
