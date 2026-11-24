package media

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestDownloadSoundCloudMaterializesPlayableFile exercises the real download
// path against a public track. It is skipped unless SOUNDCLOUD_LIVE_TEST=1.
func TestDownloadSoundCloudMaterializesPlayableFile(t *testing.T) {
	if os.Getenv("SOUNDCLOUD_LIVE_TEST") != "1" {
		t.Skip("set SOUNDCLOUD_LIVE_TEST=1 to run the live SoundCloud download")
	}
	target := filepath.Join(t.TempDir(), "track.m4a")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	err := DownloadSoundCloud(ctx, "yt-dlp", "ffmpeg", "293", target, 64<<20)
	require.NoError(t, err)
	info, err := os.Stat(target)
	require.NoError(t, err)
	require.Positive(t, info.Size())
}

// TestDownloadSoundCloudRemuxesIntoCacheOwnedName guards the cache integration.
//
// The cache hands the producer a path that ends in ".part", which ffmpeg
// cannot infer a container from. Without an explicit format the remux fails
// and the track silently stays on the widget.
func TestDownloadSoundCloudRemuxesIntoCacheOwnedName(t *testing.T) {
	if os.Getenv("SOUNDCLOUD_LIVE_TEST") != "1" {
		t.Skip("set SOUNDCLOUD_LIVE_TEST=1 to run the live SoundCloud download")
	}
	target := filepath.Join(t.TempDir(), "9f1f516e6623ff9ce4f03465f20e43d24ab160691f029e6b3ddb83526ee83f9.media.part")
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Minute)
	defer cancel()
	err := DownloadSoundCloud(ctx, "yt-dlp", "ffmpeg", "293", target, 64<<20)
	require.NoError(t, err)
	info, err := os.Stat(target)
	require.NoError(t, err)
	require.Positive(t, info.Size())
}
