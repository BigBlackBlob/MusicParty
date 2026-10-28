package sqlite

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatch6MetadataPreservesConcurrentTaskResult(t *testing.T) {
	ctx := context.Background()
	for _, deleted := range []bool{false, true} {
		t.Run(map[bool]string{false: "completed", true: "deleted"}[deleted], func(t *testing.T) {
			repository := NewLocalTrackRepository(openLocalTrackTestStore(t))
			seedLocalTrack(t, repository, "track", "source/track.flac", "PENDING")
			title, album := "edited", "album"
			metadata := LocalTrackMetadata{Title: &title, Artists: []string{"new artist"}, Album: &album}
			if deleted {
				require.NoError(t, repository.MarkDeleted(ctx, "track", 2))
			} else {
				require.NoError(t, repository.CompleteTranscode(ctx, "track", "media/track.ogg", 1234, nil, nil, 2))
			}
			_, err := repository.UpdateMetadata(ctx, "track", metadata, 3)
			if deleted {
				require.ErrorIs(t, err, ErrLocalTrackGone)
			} else {
				require.NoError(t, err)
			}
			track, err := repository.FindByID(ctx, "track")
			require.NoError(t, err)
			if deleted {
				require.Equal(t, "DELETED", track.Status)
				require.Equal(t, "track", track.Title)
			} else {
				require.Equal(t, "COMPLETED", track.Status)
				require.Equal(t, "media/track.ogg", *track.OGGPath)
				require.EqualValues(t, 1234, track.DurationMS)
				require.Nil(t, track.SourcePath)
				require.EqualValues(t, 2, *track.CompletedAt)
				require.Equal(t, title, track.Title)
			}
		})
	}
}
