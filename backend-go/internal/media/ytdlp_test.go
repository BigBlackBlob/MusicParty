package media

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGeneratedDownloadDeduplicatesAndPublishesOnlyCompleteFile(t *testing.T) {
	cache, err := NewCache(t.TempDir(), 4096, nil, 1, 2, 0, 0, CacheOptions{MaxFileBytes: 2048, ReservationBytes: 512})
	require.NoError(t, err)
	defer cache.Close()
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	producer := func(ctx context.Context, path string, maxBytes int64) error {
		calls.Add(1)
		if maxBytes != 2048 {
			return os.ErrInvalid
		}
		if err := os.WriteFile(path, []byte("partial"), 0o600); err != nil {
			return err
		}
		close(started)
		select {
		case <-release:
			return os.WriteFile(path, []byte("complete audio"), 0o600)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	require.NoError(t, cache.SubmitGenerated("youtube:video", producer))
	<-started
	require.NoError(t, cache.SubmitGenerated("youtube:video", producer))
	require.False(t, cache.Has("youtube:video"), "a growing output must never be visible")
	close(release)
	path := waitCache(t, cache, "youtube:video")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "complete audio", string(data))
	require.EqualValues(t, 1, calls.Load())
}

func TestGeneratedDownloadTimeoutCleansPartialForIframeFallback(t *testing.T) {
	cache, err := NewCache(t.TempDir(), 4096, nil, 1, 2, 0, 0, CacheOptions{MaxFileBytes: 2048, ReservationBytes: 512, TaskTTL: 40 * time.Millisecond})
	require.NoError(t, err)
	defer cache.Close()
	var calls atomic.Int32
	producer := func(ctx context.Context, path string, _ int64) error {
		calls.Add(1)
		if err := os.WriteFile(path+".part", []byte("growing"), 0o600); err != nil {
			return err
		}
		<-ctx.Done()
		return ctx.Err()
	}
	require.NoError(t, cache.SubmitGenerated("youtube:video", producer))
	require.NoError(t, cache.SubmitGenerated("youtube:video", producer))
	require.Eventually(t, func() bool { return !cache.IsInflight("youtube:video") }, time.Second, 10*time.Millisecond)
	require.False(t, cache.Has("youtube:video"))
	require.EqualValues(t, 1, calls.Load())
	entries, err := os.ReadDir(cache.root)
	require.NoError(t, err)
	require.Empty(t, entries, "failed host download leaves no playable file, so the client must keep the iframe fallback")
}
