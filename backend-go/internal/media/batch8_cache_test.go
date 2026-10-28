package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func batch8CacheFile(root, key string) string {
	hash := sha256.Sum256([]byte(key))
	return filepath.Join(root, hex.EncodeToString(hash[:])+".media")
}

func batch8DiskBytes(t *testing.T, root string) int64 {
	t.Helper()
	entries, err := os.ReadDir(root)
	require.NoError(t, err)
	var total int64
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".media") || strings.HasSuffix(entry.Name(), ".part") {
			file, err := os.Open(filepath.Join(root, entry.Name()))
			if os.IsNotExist(err) {
				continue // A worker can remove a partial while the test observes disk use.
			}
			require.NoError(t, err)
			info, err := file.Stat()
			require.NoError(t, err)
			require.NoError(t, file.Close())
			total += info.Size()
		}
	}
	return total
}

func TestBatch8StartupCountsAndEvictsUnreadCache(t *testing.T) {
	root := t.TempDir()
	old := batch8CacheFile(root, "old")
	newer := batch8CacheFile(root, "new")
	require.NoError(t, os.WriteFile(old, []byte("123456"), 0o600))
	require.NoError(t, os.WriteFile(newer, []byte("abcdef"), 0o600))
	require.NoError(t, os.Chtimes(old, time.Unix(1, 0), time.Unix(1, 0)))
	cache, err := NewCache(root, 8, http.DefaultClient, 1, 1, 0, 0)
	require.NoError(t, err)
	defer cache.Close()
	require.LessOrEqual(t, batch8DiskBytes(t, root), int64(8))
	require.NoFileExists(t, old)
	_, found := cache.Get("new")
	require.True(t, found, "hashed filenames must remain addressable after scanning")
}

func TestBatch8DownloadEvictsUnreadPreexistingFile(t *testing.T) {
	root := t.TempDir()
	old := batch8CacheFile(root, "old")
	require.NoError(t, os.WriteFile(old, []byte("123456"), 0o600))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("abcdef")) }))
	defer upstream.Close()
	cache, err := NewCache(root, 10, upstream.Client(), 1, 1, 0, 0)
	require.NoError(t, err)
	defer cache.Close()
	require.NoError(t, cache.Submit("new", upstream.URL))
	_ = waitCache(t, cache, "new")
	require.NoFileExists(t, old)
	require.LessOrEqual(t, batch8DiskBytes(t, root), int64(10))
}

func TestBatch8UnknownLengthCannotOverflowPartialQuota(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush() // Force an unknown length response.
		_, _ = w.Write([]byte(strings.Repeat("x", 64<<10)))
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer upstream.Close()
	root := t.TempDir()
	cache, err := NewCache(root, 8, upstream.Client(), 1, 1, 0, 0)
	require.NoError(t, err)
	defer cache.Close()
	require.NoError(t, cache.Submit("large", upstream.URL))
	require.Eventually(t, func() bool {
		cache.mu.Lock()
		active := len(cache.inflight)
		cache.mu.Unlock()
		return active == 0 || batch8DiskBytes(t, root) > 0
	}, time.Second, time.Millisecond)
	require.LessOrEqual(t, batch8DiskBytes(t, root), int64(8), "quota must apply before writing partial bytes")
}

func TestBatch8ByteSizeRejectsOverflow(t *testing.T) {
	_, err := ParseByteSize("9223372036854775807GB")
	require.Error(t, err)
}

func TestBatch8StartupRemovesAbandonedPartials(t *testing.T) {
	root := t.TempDir()
	path := batch8CacheFile(root, "old")
	require.NoError(t, os.WriteFile(path, []byte("complete"), 0o600))
	partial := batch8CacheFile(root, "crashed") + ".part"
	require.NoError(t, os.WriteFile(partial, []byte(strings.Repeat("x", 64)), 0o600))
	cache, err := NewCache(root, 16, http.DefaultClient, 1, 1, 0, 0)
	require.NoError(t, err)
	defer cache.Close()
	require.NoFileExists(t, partial)
	_, found := cache.Get("old")
	require.True(t, found)
	require.EqualValues(t, 8, batch8DiskBytes(t, root))
}

func TestBatch8PlayingFileKeepsItsQuotaUntilReleased(t *testing.T) {
	root := t.TempDir()
	old := batch8CacheFile(root, "playing")
	require.NoError(t, os.WriteFile(old, []byte("12345678"), 0o600))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("abcdefgh")) }))
	defer upstream.Close()
	cache, err := NewCache(root, 8, upstream.Client(), 1, 1, 0, 0)
	require.NoError(t, err)
	defer cache.Close()
	path, release, ok := cache.Acquire("playing")
	require.True(t, ok)
	defer release()
	require.Equal(t, old, path)
	require.NoError(t, cache.Submit("new", upstream.URL))
	batch8WaitIdle(t, cache)
	require.FileExists(t, old, "playing files cannot be evicted to free phantom space")
	_, found := cache.Get("new")
	require.False(t, found)
	release()
	require.NoError(t, cache.Submit("new", upstream.URL))
	_ = waitCache(t, cache, "new")
	require.NoFileExists(t, old)
	batch8WaitIdle(t, cache)
}

func batch8WaitIdle(t *testing.T, cache *Cache) {
	t.Helper()
	require.Eventually(t, func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		return len(cache.inflight) == 0
	}, 4*time.Second, time.Millisecond)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.Zero(t, cache.reservedBytes, "all attempt reservations must be released")
	require.LessOrEqual(t, cache.usedBytes, cache.maxBytes)
}

func TestBatch8FileLimitRejectsKnownAndUnknownResponses(t *testing.T) {
	for _, known := range []bool{true, false} {
		t.Run(map[bool]string{true: "known", false: "unknown"}[known], func(t *testing.T) {
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if known {
					w.Header().Set("Content-Length", "64")
				} else {
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write([]byte(strings.Repeat("x", 64)))
			}))
			defer upstream.Close()
			root := t.TempDir()
			cache, err := NewCache(root, 128, upstream.Client(), 1, 1, 2, 0, CacheOptions{MaxFileBytes: 32, ReservationBytes: 8})
			require.NoError(t, err)
			defer cache.Close()
			require.NoError(t, cache.Submit("large", upstream.URL))
			batch8WaitIdle(t, cache)
			require.Zero(t, batch8DiskBytes(t, root))
			_, cached := cache.Get("large")
			require.False(t, cached)
		})
	}
}

func TestBatch8ConcurrentUnknownDownloadsShareOneQuota(t *testing.T) {
	const chunk = 32 << 10
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	releaseAll := sync.OnceFunc(func() { close(release) })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.(http.Flusher).Flush()
		_, _ = w.Write([]byte(strings.Repeat("a", chunk)))
		w.(http.Flusher).Flush()
		started <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write([]byte(strings.Repeat("b", chunk)))
	}))
	defer upstream.Close()
	defer releaseAll()
	root := t.TempDir()
	cache, err := NewCache(root, 3*chunk, upstream.Client(), 3, 3, 0, 0, CacheOptions{ReservationBytes: chunk / 2, MaxFileBytes: 2 * chunk})
	require.NoError(t, err)
	defer cache.Close()
	for _, key := range []string{"one", "two", "three"} {
		require.NoError(t, cache.Submit(key, upstream.URL))
	}
	for range 3 {
		select {
		case <-started:
		case <-time.After(3 * time.Second):
			t.Fatal("concurrent requests did not start")
		}
	}
	require.Eventually(t, func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		return cache.reservedBytes == 3*chunk
	}, 3*time.Second, time.Millisecond)
	// Freeze quota changes while observing disk. Writers can consume already
	// reserved bytes, but cannot grow or rename files during this snapshot.
	checkQuota := func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		require.LessOrEqual(t, batch8DiskBytes(t, root), cache.maxBytes)
		require.LessOrEqual(t, cache.usedBytes+cache.reservedBytes, cache.maxBytes)
		return len(cache.inflight) == 0
	}
	require.False(t, checkQuota())
	releaseAll()
	require.Eventually(t, checkQuota, 4*time.Second, time.Millisecond)
	batch8WaitIdle(t, cache)
	partials, err := filepath.Glob(filepath.Join(root, "*.part"))
	require.NoError(t, err)
	require.Empty(t, partials)
}

func TestBatch8TaskDeadlineBoundsRetryAfterAndBodyReads(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if status == http.StatusTooManyRequests {
					w.Header().Set("Retry-After", "3600")
					w.WriteHeader(status)
					return
				}
				_, _ = w.Write([]byte("partial"))
				w.(http.Flusher).Flush()
				<-r.Context().Done()
			}))
			defer upstream.Close()
			root := t.TempDir()
			cache, err := NewCache(root, 128, upstream.Client(), 1, 1, 10, time.Millisecond, CacheOptions{TaskTTL: 500 * time.Millisecond, ReservationBytes: 16})
			require.NoError(t, err)
			defer cache.Close()
			require.NoError(t, cache.Submit("slow", upstream.URL))
			batch8WaitIdle(t, cache)
			require.EqualValues(t, 1, calls.Load())
			require.Zero(t, batch8DiskBytes(t, root))
		})
	}
}

func TestBatch8FailedAttemptReleasesQuotaBeforeRetry(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Content-Length", "100")
			_, _ = w.Write([]byte("truncated"))
			return
		}
		_, _ = w.Write([]byte(strings.Repeat("x", 80)))
	}))
	defer upstream.Close()
	cache, err := NewCache(t.TempDir(), 128, upstream.Client(), 1, 1, 1, time.Millisecond)
	require.NoError(t, err)
	defer cache.Close()
	require.NoError(t, cache.Submit("retry", upstream.URL))
	_ = waitCache(t, cache, "retry")
	batch8WaitIdle(t, cache)
	require.EqualValues(t, 2, calls.Load())
	cache.mu.Lock()
	defer cache.mu.Unlock()
	require.EqualValues(t, 80, cache.usedBytes)
}

func TestBatch8CacheDeadlineDoesNotShortenPlaybackProxy(t *testing.T) {
	playing := make(chan struct{})
	release := make(chan struct{})
	releaseAll := sync.OnceFunc(func() { close(release) })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("a"))
		w.(http.Flusher).Flush()
		if r.URL.Path == "/play" {
			close(playing)
			select {
			case <-release:
				_, _ = w.Write([]byte("b"))
			case <-r.Context().Done():
			}
		} else {
			<-r.Context().Done()
		}
	}))
	defer upstream.Close()
	defer releaseAll()
	client := upstream.Client()
	cache, err := NewCache(t.TempDir(), 128, client, 1, 1, 0, 0, CacheOptions{TaskTTL: 150 * time.Millisecond})
	require.NoError(t, err)
	defer cache.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	recorder := httptest.NewRecorder()
	done := make(chan error, 1)
	go func() {
		done <- (Proxy{Client: client}).Serve(recorder, httptest.NewRequest("GET", "/play", nil).WithContext(ctx), upstream.URL+"/play", nil)
	}()
	select {
	case <-playing:
	case <-ctx.Done():
		t.Fatal("playback did not start")
	}
	require.NoError(t, cache.Submit("slow", upstream.URL+"/cache"))
	batch8WaitIdle(t, cache)
	select {
	case err := <-done:
		t.Fatalf("playback ended at cache deadline: %v", err)
	default:
	}
	releaseAll()
	require.NoError(t, <-done)
	require.Equal(t, "ab", recorder.Body.String())
}
