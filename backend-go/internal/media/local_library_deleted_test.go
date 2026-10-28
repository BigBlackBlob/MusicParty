package media

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

type libraryFixture struct {
	library    *LocalLibrary
	repository *storesqlite.LocalTrackRepository
	store      *storesqlite.Store
	root       string
	reported   *[]error
}

func newLibraryFixture(t *testing.T) *libraryFixture {
	t.Helper()
	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{Path: filepath.Join(t.TempDir(), "library.db"), BusyTimeout: time.Second, ReadConnections: 2})
	require.NoError(t, err)
	require.NoError(t, storesqlite.EnsureCompatibleSchema(context.Background(), store, true))
	t.Cleanup(func() { _ = store.Close() })
	transcoder := NewTranscoder("fake", 1, 1, time.Second)
	t.Cleanup(transcoder.Close)
	root := filepath.Join(t.TempDir(), "library")
	repository := storesqlite.NewLocalTrackRepository(store)
	library, err := NewLocalLibrary(root, 1024, repository, transcoder)
	require.NoError(t, err)
	reported := &[]error{}
	library.SetErrorReporter(func(err error) { *reported = append(*reported, err) })
	return &libraryFixture{library: library, repository: repository, store: store, root: root, reported: reported}
}

func seedTranscodingTrack(t *testing.T, repository *storesqlite.LocalTrackRepository, root, id, sourceRelative, outputRelative string) {
	t.Helper()
	hash := "hash-" + id
	source := sourceRelative
	require.NoError(t, repository.Upsert(context.Background(), storesqlite.LocalTrack{ID: id, OriginalHash: &hash, SourcePath: &source, Title: id, Artists: []string{"a"}, Status: "PROCESSING", CreatedAt: 1, UpdatedAt: 1}))
	for _, relative := range []string{sourceRelative, outputRelative} {
		path, err := ResolveWithin(root, relative)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte("payload"), 0o640))
	}
}

// The bug: deleting a track mid-transcode let the finished callback write the row back and remove
// the source file. Now the rejected write keeps the source and drops the stale output instead.
func TestCompleteTranscodeAfterDeleteKeepsSourceAndDropsOutput(t *testing.T) {
	fixture := newLibraryFixture(t)
	seedTranscodingTrack(t, fixture.repository, fixture.root, "track", "source/track.mp3", "media/track.ogg")
	require.NoError(t, fixture.repository.MarkDeleted(context.Background(), "track", 20))

	fixture.library.completeTranscode("track", "source/track.mp3", "media/track.ogg", nil)

	source, err := ResolveWithin(fixture.root, "source/track.mp3")
	require.NoError(t, err)
	_, statErr := os.Stat(source)
	require.NoError(t, statErr, "the source file must survive a rejected completion")
	output, err := ResolveWithin(fixture.root, "media/track.ogg")
	require.NoError(t, err)
	_, statErr = os.Stat(output)
	require.True(t, errors.Is(statErr, os.ErrNotExist), "the stale transcode output must be removed, stat error = %v", statErr)
	require.NotEmpty(t, *fixture.reported, "the failed write must be reported, not silently ignored")

	track, err := fixture.repository.FindByID(context.Background(), "track")
	require.NoError(t, err)
	require.Equal(t, "DELETED", track.Status)
	require.Nil(t, track.OGGPath)
}

func TestCompleteTranscodeRemovesSourceOnlyAfterSuccessfulWrite(t *testing.T) {
	fixture := newLibraryFixture(t)
	seedTranscodingTrack(t, fixture.repository, fixture.root, "live", "source/live.mp3", "media/live.ogg")

	fixture.library.completeTranscode("live", "source/live.mp3", "media/live.ogg", nil)

	source, err := ResolveWithin(fixture.root, "source/live.mp3")
	require.NoError(t, err)
	_, statErr := os.Stat(source)
	require.True(t, errors.Is(statErr, os.ErrNotExist), "a completed transcode drops the source, stat error = %v", statErr)
	output, err := ResolveWithin(fixture.root, "media/live.ogg")
	require.NoError(t, err)
	require.FileExists(t, output)
	require.Empty(t, *fixture.reported)

	track, err := fixture.repository.FindByID(context.Background(), "live")
	require.NoError(t, err)
	require.Equal(t, "COMPLETED", track.Status)
	require.NotNil(t, track.OGGPath)
	require.Equal(t, "media/live.ogg", *track.OGGPath)
}
