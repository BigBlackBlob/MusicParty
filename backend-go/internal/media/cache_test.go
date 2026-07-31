package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheDownloadsAtomicallyAndEvictsLRU(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.URL.Path))
	}))
	defer server.Close()
	cache, err := NewCache(t.TempDir(), 4, server.Client(), 1, 2, 0, time.Millisecond)
	require.NoError(t, err)
	defer cache.Close()
	require.NoError(t, cache.Submit("one", server.URL+"/one"))
	path := waitCache(t, cache, "one")
	require.FileExists(t, path)
	require.NoFileExists(t, path+".part")
	require.NoError(t, cache.Submit("two", server.URL+"/two"))
	_ = waitCache(t, cache, "two")
	_, oneExists := cache.Get("one")
	require.False(t, oneExists)
}

func TestCacheRetriesTransientFailuresAndDeduplicatesInflightKey(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests.Add(1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	defer server.Close()
	cache, err := NewCache(t.TempDir(), 1024, server.Client(), 1, 2, 2, time.Millisecond)
	require.NoError(t, err)
	defer cache.Close()
	require.NoError(t, cache.Submit("local:same", server.URL))
	require.NoError(t, cache.Submit("local:same", server.URL))
	path := waitCache(t, cache, "local:same")
	require.Equal(t, int32(3), requests.Load())
	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "ok", string(contents))
}

func TestCacheQueueFullAndCancellationRemovePartialFile(t *testing.T) {
	started := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case started <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial"))
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	root := t.TempDir()
	cache, err := NewCache(root, 1024, server.Client(), 1, 1, 0, time.Millisecond)
	require.NoError(t, err)
	require.NoError(t, cache.Submit("one", server.URL))
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("download did not start")
	}
	require.NoError(t, cache.Submit("two", server.URL))
	require.ErrorIs(t, cache.Submit("three", server.URL), ErrDownloadQueueFull)
	cache.Close()
	partials, err := filepath.Glob(filepath.Join(root, "*.part"))
	require.NoError(t, err)
	require.Empty(t, partials)
	require.ErrorIs(t, cache.Submit("closed", server.URL), context.Canceled)
}

func TestParseByteSize(t *testing.T) {
	value, err := ParseByteSize("1GB")
	require.NoError(t, err)
	require.Equal(t, int64(1<<30), value)
	_, err = ParseByteSize("bad")
	require.Error(t, err)
}

func TestRetryAfter(t *testing.T) {
	delay, ok := retryAfter("3", time.Now())
	require.True(t, ok)
	require.Equal(t, 3*time.Second, delay)
}

func waitCache(t *testing.T, cache *Cache, key string) string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if path, ok := cache.Get(key); ok {
			return path
		}
		time.Sleep(10 * time.Millisecond)
	}
	entries, _ := filepath.Glob(filepath.Join(cache.root, "*"))
	t.Fatalf("cache entry %q did not complete, files=%v", key, entries)
	return ""
}
