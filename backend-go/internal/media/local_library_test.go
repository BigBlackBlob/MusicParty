package media

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestLocalLibraryUploadTranscodeDuplicateAndDelete(t *testing.T) {
	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{Path: filepath.Join(t.TempDir(), "media.db"), BusyTimeout: time.Second, ReadConnections: 2})
	require.NoError(t, err)
	require.NoError(t, storesqlite.EnsureCompatibleSchema(context.Background(), store, true))
	defer store.Close()
	transcoder := NewTranscoder("fake", 1, 2, time.Second)
	transcoder.run = func(_ context.Context, _ string, arguments ...string) error {
		input := arguments[6]
		output := arguments[len(arguments)-1]
		from, err := os.Open(input)
		if err != nil {
			return err
		}
		defer from.Close()
		to, err := os.Create(output)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(to, from)
		return errorsJoin(copyErr, to.Close())
	}
	defer transcoder.Close()
	library, err := NewLocalLibrary(filepath.Join(t.TempDir(), "library"), 1024, storesqlite.NewLocalTrackRepository(store), transcoder)
	require.NoError(t, err)
	upload := Upload{FileName: "歌曲.mp3", ContentType: "audio/mpeg", Title: "歌曲", Artists: "甲;乙", UploadedBy: "tester", Size: 5, Reader: strings.NewReader("audio")}
	track, duplicate, err := library.Upload(context.Background(), upload)
	require.NoError(t, err)
	require.False(t, duplicate)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		trackPointer, findErr := storesqlite.NewLocalTrackRepository(store).FindByID(context.Background(), track.ID)
		require.NoError(t, findErr)
		if trackPointer.Status == "COMPLETED" {
			track = *trackPointer
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	require.Equal(t, "COMPLETED", track.Status)
	require.NotNil(t, track.OGGPath)
	duplicateTrack, duplicate, err := library.Upload(context.Background(), Upload{FileName: "copy.mp3", Size: 5, Reader: strings.NewReader("audio")})
	require.NoError(t, err)
	require.True(t, duplicate)
	require.Equal(t, track.ID, duplicateTrack.ID)
	require.NoError(t, library.Delete(context.Background(), track.ID))
	deleted, err := storesqlite.NewLocalTrackRepository(store).FindByID(context.Background(), track.ID)
	require.NoError(t, err)
	require.Equal(t, "DELETED", deleted.Status)
}

func TestLocalLibraryRejectsOversizedUpload(t *testing.T) {
	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{Path: filepath.Join(t.TempDir(), "media.db"), BusyTimeout: time.Second, ReadConnections: 2})
	require.NoError(t, err)
	require.NoError(t, storesqlite.EnsureCompatibleSchema(context.Background(), store, true))
	defer store.Close()
	transcoder := NewTranscoder("unused", 1, 1, time.Second)
	defer transcoder.Close()
	library, err := NewLocalLibrary(t.TempDir(), 4, storesqlite.NewLocalTrackRepository(store), transcoder)
	require.NoError(t, err)
	_, _, err = library.Upload(context.Background(), Upload{FileName: "过大.mp3", Title: "中文标题", Artists: "甲;乙", Size: 5, Reader: strings.NewReader("12345")})
	require.Error(t, err)
}

func errorsJoin(values ...error) error {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
