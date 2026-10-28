package media

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestBatch6ReuploadRetriesFailedTrackWithMissingSource(t *testing.T) {
	f := newLibraryFixture(t)
	hash := sha256.Sum256([]byte("audio"))
	digest := hex.EncodeToString(hash[:])
	ctx := context.Background()
	require.NoError(t, f.repository.Upsert(ctx, storesqlite.LocalTrack{ID: "failed", OriginalHash: &digest, Title: "custom title", Artists: []string{"a"}, Status: "FAILED", CreatedAt: 1, UpdatedAt: 1}))
	f.library.transcoder.run = func(_ context.Context, _ string, args ...string) error {
		return os.WriteFile(args[len(args)-1], []byte("ogg"), 0o600)
	}
	track, duplicate, err := f.library.Upload(ctx, Upload{FileName: "retry.mp3", Size: 5, Reader: strings.NewReader("audio")})
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, "failed", track.ID)
	require.Equal(t, "custom title", track.Title)
	require.NotEqual(t, "FAILED", track.Status, "a duplicate upload must restart failed work")
	require.Eventually(t, func() bool {
		current, err := f.repository.FindByID(ctx, track.ID)
		return err == nil && current.Status == "COMPLETED"
	}, 3*time.Second, 10*time.Millisecond)
}

func TestBatch6CompletionWriteFailureKeepsRecoveryMaterials(t *testing.T) {
	f := newLibraryFixture(t)
	seedTranscodingTrack(t, f.repository, f.root, "track", "source/track.mp3", "media/track.ogg")
	require.NoError(t, f.store.Write(context.Background(), func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `create trigger reject_completion before update on local_track when new.status='COMPLETED' begin select raise(abort,'forced completion failure'); end`)
		return err
	}))
	f.library.completeTranscode("track", "source/track.mp3", "media/track.ogg", nil)
	require.FileExists(t, filepath.Join(f.root, "source", "track.mp3"))
	require.FileExists(t, filepath.Join(f.root, "media", "track.ogg"))
	current, err := f.repository.FindByID(context.Background(), "track")
	require.NoError(t, err)
	require.Equal(t, "PROCESSING", current.Status)
	require.NotNil(t, current.SourcePath)
	require.NotEmpty(t, *f.reported)
}

func TestBatch6RecoveryDrainsBacklogAndRejectsMissingSources(t *testing.T) {
	f := newLibraryFixture(t)
	f.library.SetErrorReporter(func(error) {})
	ctx := context.Background()
	var mu sync.Mutex
	processed := map[string]int{}
	f.library.transcoder.run = func(_ context.Context, _ string, args ...string) error {
		mu.Lock()
		processed[filepath.Base(args[6])]++
		mu.Unlock()
		return os.WriteFile(args[len(args)-1], []byte("ogg"), 0o600)
	}
	// More than a recovery page and far more than the one-slot transcode queue.
	for i := range 70 {
		id := fmt.Sprintf("track-%03d", i)
		seedTranscodingTrack(t, f.repository, f.root, id, "source/"+id+".mp3", "media/"+id+".ogg")
		if i%2 == 0 {
			require.NoError(t, f.repository.UpdateStatus(ctx, id, storesqlite.LocalTrackStatusUpdate{Status: "PENDING", UpdatedAt: 1}))
		}
	}
	missing := "source/missing.mp3"
	require.NoError(t, f.repository.Upsert(ctx, storesqlite.LocalTrack{ID: "missing", SourcePath: &missing, Title: "missing", Artists: []string{"a"}, Status: "PENDING", CreatedAt: 1, UpdatedAt: 1}))
	recoveryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	require.NoError(t, f.library.RecoverPending(recoveryCtx))
	// Wait for completion callbacks as well as accepted jobs.
	require.Eventually(t, func() bool {
		f.library.jobsMu.Lock()
		defer f.library.jobsMu.Unlock()
		return len(f.library.inFlight) == 0
	}, 5*time.Second, 10*time.Millisecond)
	tracks, err := f.repository.FindAll(ctx)
	require.NoError(t, err)
	for _, track := range tracks {
		if track.ID == "missing" {
			require.Equal(t, "FAILED", track.Status)
			require.NotNil(t, track.ErrorMessage)
			continue
		}
		require.Equal(t, "COMPLETED", track.Status, track.ID)
		require.Nil(t, track.SourcePath)
	}
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, processed, 70)
	for _, count := range processed {
		require.Equal(t, 1, count, "recovery must submit each track once")
	}
}

func TestBatch6RecoverySkipsTrackDeletedWhileQueued(t *testing.T) {
	f := newLibraryFixture(t)
	ctx := context.Background()
	seedTranscodingTrack(t, f.repository, f.root, "a", "source/a.mp3", "media/a.ogg")
	seedTranscodingTrack(t, f.repository, f.root, "b", "source/b.mp3", "media/b.ogg")
	require.NoError(t, os.Remove(filepath.Join(f.root, "media", "b.ogg")))
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	f.library.transcoder.run = func(ctx context.Context, _ string, args ...string) error {
		if filepath.Base(args[6]) != "a.mp3" {
			return fmt.Errorf("deleted job unexpectedly executed")
		}
		close(started)
		select {
		case <-release:
			return os.WriteFile(args[len(args)-1], []byte("ogg"), 0o600)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	require.NoError(t, f.library.RecoverPending(ctx))
	<-started
	require.NoError(t, f.library.Delete(ctx, "b"))
	// Let the first worker finish without closing the release channel twice.
	release <- struct{}{}
	require.Eventually(t, func() bool {
		f.library.jobsMu.Lock()
		defer f.library.jobsMu.Unlock()
		return len(f.library.inFlight) == 0
	}, 5*time.Second, 10*time.Millisecond)
	track, err := f.repository.FindByID(ctx, "b")
	require.NoError(t, err)
	require.Equal(t, "DELETED", track.Status)
	require.NoFileExists(t, filepath.Join(f.root, "media", "b.ogg"))
	require.Empty(t, *f.reported)
}

func TestBatch6InterruptedRecoveryResumesOnNextStartup(t *testing.T) {
	f := newLibraryFixture(t)
	ctx := context.Background()
	for _, id := range []string{"a", "b", "c"} {
		seedTranscodingTrack(t, f.repository, f.root, id, "source/"+id+".mp3", "media/"+id+".ogg")
	}
	started := make(chan struct{}, 1)
	f.library.transcoder.run = func(ctx context.Context, _ string, args ...string) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	}
	recoveryCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- f.library.RecoverPending(recoveryCtx) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("recovery did not start")
	}
	require.Eventually(t, func() bool { return len(f.library.transcoder.jobs) == 1 }, 3*time.Second, 10*time.Millisecond)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	f.library.transcoder.Close()
	for _, id := range []string{"a", "b", "c"} {
		track, err := f.repository.FindByID(ctx, id)
		require.NoError(t, err)
		require.Contains(t, []string{"PENDING", "PROCESSING"}, track.Status)
		require.NotNil(t, track.SourcePath)
		require.FileExists(t, filepath.Join(f.root, filepath.FromSlash(*track.SourcePath)))
	}
	nextTranscoder := NewTranscoder("fake", 1, 1, time.Second)
	t.Cleanup(nextTranscoder.Close)
	nextTranscoder.run = func(_ context.Context, _ string, args ...string) error {
		return os.WriteFile(args[len(args)-1], []byte("ogg"), 0o600)
	}
	nextLibrary, err := NewLocalLibrary(f.root, 1024, f.repository, nextTranscoder)
	require.NoError(t, err)
	require.NoError(t, nextLibrary.RecoverPending(ctx))
	require.Eventually(t, func() bool {
		tracks, err := f.repository.FindAll(ctx)
		if err != nil {
			return false
		}
		for _, track := range tracks {
			if track.Status != "COMPLETED" {
				return false
			}
		}
		return true
	}, 5*time.Second, 10*time.Millisecond)
}

func TestBatch6DuplicateDuringRecoveryKeepsRunningInput(t *testing.T) {
	f := newLibraryFixture(t)
	ctx := context.Background()
	hash := sha256.Sum256([]byte("audio"))
	digest := hex.EncodeToString(hash[:])
	source := "source/track.mp3"
	require.NoError(t, os.WriteFile(filepath.Join(f.root, filepath.FromSlash(source)), []byte("audio"), 0o600))
	require.NoError(t, f.repository.Upsert(ctx, storesqlite.LocalTrack{ID: "track", OriginalHash: &digest, SourcePath: &source, Title: "title", Artists: []string{"a"}, Status: "PENDING", CreatedAt: 1, UpdatedAt: 1}))
	started, release := make(chan struct{}), make(chan struct{})
	defer close(release)
	f.library.transcoder.run = func(ctx context.Context, _ string, args ...string) error {
		close(started) // A second execution would also fail this test.
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		data, err := os.ReadFile(args[6])
		if err != nil {
			return err
		}
		return os.WriteFile(args[len(args)-1], data, 0o600)
	}
	require.NoError(t, f.library.RecoverPending(ctx))
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("recovered job did not start")
	}
	track, duplicate, err := f.library.Upload(ctx, Upload{FileName: "retry.mp3", Size: 5, Reader: strings.NewReader("audio")})
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, "track", track.ID)
	require.Equal(t, source, *track.SourcePath)
	files, err := os.ReadDir(filepath.Join(f.root, "source"))
	require.NoError(t, err)
	require.Len(t, files, 1, "duplicate input must be discarded")
	release <- struct{}{}
	require.Eventually(t, func() bool {
		current, err := f.repository.FindByID(ctx, "track")
		return err == nil && current.Status == "COMPLETED"
	}, 3*time.Second, 10*time.Millisecond)
}
