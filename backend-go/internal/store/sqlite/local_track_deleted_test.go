package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func openLocalTrackTestStore(t *testing.T) *Store {
	t.Helper()
	store := openTestStore(t, StoreConfig{Path: filepath.Join(t.TempDir(), "local-tracks.db"), BusyTimeout: time.Second, ReadConnections: 2})
	if err := EnsureCompatibleSchema(context.Background(), store, true); err != nil {
		t.Fatalf("EnsureCompatibleSchema(): %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func seedLocalTrack(t *testing.T, repository *LocalTrackRepository, id, sourcePath, status string) {
	t.Helper()
	hash := "hash-" + id
	if err := repository.Upsert(context.Background(), LocalTrack{ID: id, OriginalHash: &hash, SourcePath: &sourcePath, Title: id, Artists: []string{"a"}, Status: status, CreatedAt: 1, UpdatedAt: 1}); err != nil {
		t.Fatalf("seed local track %s: %v", id, err)
	}
}

// A transcode result that lands after the track was deleted must not resurrect the row.
func TestLocalTrackWritesRejectDeletedRows(t *testing.T) {
	ctx := context.Background()
	ogg, status, progress := "media/track.ogg", "COMPLETED", 100
	for _, scenario := range []struct {
		name  string
		id    string
		write func(repository *LocalTrackRepository, id string) error
	}{
		{name: "UpdateStatus", id: "deleted", write: func(repository *LocalTrackRepository, id string) error {
			return repository.UpdateStatus(ctx, id, LocalTrackStatusUpdate{Status: status, OGGPath: &ogg, ProgressPercent: &progress, UpdatedAt: 30})
		}},
		{name: "CompleteTranscode", id: "deleted", write: func(repository *LocalTrackRepository, id string) error {
			return repository.CompleteTranscode(ctx, id, ogg, 1000, nil, nil, 30)
		}},
		{name: "UpdateStatusMissingRow", id: "absent", write: func(repository *LocalTrackRepository, id string) error {
			return repository.UpdateStatus(ctx, id, LocalTrackStatusUpdate{Status: status, UpdatedAt: 30})
		}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store := openLocalTrackTestStore(t)
			repository := NewLocalTrackRepository(store)
			if scenario.id == "deleted" {
				seedLocalTrack(t, repository, scenario.id, "source/track.flac", "PROCESSING")
				if err := repository.MarkDeleted(ctx, scenario.id, 20); err != nil {
					t.Fatalf("MarkDeleted(): %v", err)
				}
			}
			err := scenario.write(repository, scenario.id)
			if !errors.Is(err, ErrLocalTrackGone) {
				t.Fatalf("write on %s track error = %v, want ErrLocalTrackGone", scenario.id, err)
			}
			if scenario.id != "deleted" {
				return
			}
			track, err := repository.FindByID(ctx, scenario.id)
			if err != nil {
				t.Fatalf("FindByID(): %v", err)
			}
			if track.Status != "DELETED" || track.OGGPath != nil || track.SourcePath == nil || *track.SourcePath != "source/track.flac" {
				t.Fatalf("deleted track after stale write = %+v", track)
			}
		})
	}
}

func TestLocalTrackWritesStillApplyToLiveRows(t *testing.T) {
	ctx := context.Background()
	store := openLocalTrackTestStore(t)
	repository := NewLocalTrackRepository(store)
	seedLocalTrack(t, repository, "live", "source/live.flac", "PROCESSING")
	if err := repository.CompleteTranscode(ctx, "live", "media/live.ogg", 1234, nil, nil, 40); err != nil {
		t.Fatalf("CompleteTranscode(): %v", err)
	}
	track, err := repository.FindByID(ctx, "live")
	if err != nil {
		t.Fatalf("FindByID(): %v", err)
	}
	if track.Status != "COMPLETED" || track.OGGPath == nil || *track.OGGPath != "media/live.ogg" || track.SourcePath != nil {
		t.Fatalf("completed track = %+v", track)
	}
}
