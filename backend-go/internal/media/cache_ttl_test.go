package media

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheEntryTTLRemovesExpiredFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("audio"))
	}))
	defer server.Close()
	root := t.TempDir()
	cache, err := NewCache(root, 1024, server.Client(), 1, 2, 0, time.Millisecond, CacheOptions{EntryTTL: time.Hour})
	require.NoError(t, err)
	defer cache.Close()
	require.NoError(t, cache.Submit("soundcloud:1", server.URL, nil))
	path := waitCache(t, cache, "soundcloud:1")
	require.FileExists(t, path)

	// Age the file past its lifetime instead of sleeping for it.
	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(path, old, old))
	require.False(t, cache.Has("soundcloud:1"))
	require.NoFileExists(t, path)
}

func TestCacheSweepExpiredRemovesOnlyStaleEntries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("audio"))
	}))
	defer server.Close()
	root := t.TempDir()
	cache, err := NewCache(root, 1024, server.Client(), 1, 2, 0, time.Millisecond, CacheOptions{EntryTTL: time.Hour})
	require.NoError(t, err)
	defer cache.Close()
	require.NoError(t, cache.Submit("soundcloud:old", server.URL, nil))
	oldPath := waitCache(t, cache, "soundcloud:old")
	require.NoError(t, cache.Submit("soundcloud:new", server.URL, nil))
	newPath := waitCache(t, cache, "soundcloud:new")

	old := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(oldPath, old, old))
	require.Equal(t, 1, cache.SweepExpired())
	require.NoFileExists(t, oldPath)
	require.FileExists(t, newPath)
	require.True(t, cache.Has("soundcloud:new"))
}

func TestEphemeralCacheWipesPreviousSessionFiles(t *testing.T) {
	root := t.TempDir()
	stale := filepath.Join(root, "stale.media")
	require.NoError(t, os.WriteFile(stale, []byte("old session"), 0o640))

	cache, err := NewCache(root, 1024, http.DefaultClient, 1, 2, 0, time.Millisecond, CacheOptions{Ephemeral: true})
	require.NoError(t, err)
	defer cache.Close()
	require.NoFileExists(t, stale)
	require.False(t, cache.Has("stale"))
}

func TestCachePurgeRemovesCommittedFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("audio"))
	}))
	defer server.Close()
	cache, err := NewCache(t.TempDir(), 1024, server.Client(), 1, 2, 0, time.Millisecond)
	require.NoError(t, err)
	defer cache.Close()
	require.NoError(t, cache.Submit("soundcloud:1", server.URL, nil))
	path := waitCache(t, cache, "soundcloud:1")
	require.FileExists(t, path)

	cache.Purge()
	require.NoFileExists(t, path)
	require.False(t, cache.Has("soundcloud:1"))
}

func TestCachePurgeKeyRemovesOneTrack(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("audio"))
	}))
	defer server.Close()
	cache, err := NewCache(t.TempDir(), 1024, server.Client(), 1, 2, 0, time.Millisecond)
	require.NoError(t, err)
	defer cache.Close()
	require.NoError(t, cache.Submit("soundcloud:1", server.URL, nil))
	first := waitCache(t, cache, "soundcloud:1")
	require.NoError(t, cache.Submit("soundcloud:2", server.URL, nil))
	second := waitCache(t, cache, "soundcloud:2")

	cache.PurgeKey("soundcloud:1")
	require.NoFileExists(t, first)
	require.FileExists(t, second)
}
