package media

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCacheServePartialReadsGrowingPartFile(t *testing.T) {
	const prefixSize = minPartialBytes + 64*1024
	body := []byte(strings.Repeat("a", prefixSize) + strings.Repeat("b", 1<<20-prefixSize))
	started := make(chan struct{})
	prefixWritten := make(chan struct{})
	continuePrefix := make(chan struct{})
	continueTail := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.Header().Set("Content-Length", "1048576")
		_, _ = w.Write(body[:32])
		w.(http.Flusher).Flush()
		close(started)
		select {
		case <-continuePrefix:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write(body[32:prefixSize])
		w.(http.Flusher).Flush()
		close(prefixWritten)
		select {
		case <-continueTail:
		case <-r.Context().Done():
			return
		}
		_, _ = w.Write(body[prefixSize:])
	}))
	defer upstream.Close()
	releasePrefix := sync.OnceFunc(func() { close(continuePrefix) })
	releaseTail := sync.OnceFunc(func() { close(continueTail) })
	defer releasePrefix()
	defer releaseTail()

	cache, err := NewCache(t.TempDir(), 2<<20, upstream.Client(), 1, 1, 0, time.Millisecond)
	require.NoError(t, err)
	defer cache.Close()

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/media", nil)
	served, err := cache.ServePartial(response, request, "missing", "")
	require.NoError(t, err)
	require.False(t, served)

	const key = "growing"
	require.NoError(t, cache.Submit(key, upstream.URL, nil))
	<-started
	part := cache.pathForKey(key) + ".part"
	require.Eventually(t, func() bool {
		info, statErr := os.Stat(part)
		return statErr == nil && info.Size() == 32
	}, time.Second, time.Millisecond)
	response = httptest.NewRecorder()
	served, err = cache.ServePartial(response, request, key, "")
	require.NoError(t, err)
	require.False(t, served, "a prefix below the threshold should use the fallback")

	result := make(chan struct {
		served bool
		err    error
		body   []byte
	}, 1)
	go func() {
		deadline := time.Now().Add(2 * time.Second)
		for time.Now().Before(deadline) {
			response := httptest.NewRecorder()
			served, err := cache.ServePartial(response, request, key, "")
			if served || err != nil {
				result <- struct {
					served bool
					err    error
					body   []byte
				}{served, err, response.Body.Bytes()}
				return
			}
			time.Sleep(time.Millisecond)
		}
		result <- struct {
			served bool
			err    error
			body   []byte
		}{false, os.ErrDeadlineExceeded, nil}
	}()

	releasePrefix()
	<-prefixWritten
	select {
	case got := <-result:
		require.NoError(t, got.err)
		require.True(t, got.served)
		require.True(t, bytes.Equal(got.body, body[:len(got.body)]))
		require.GreaterOrEqual(t, len(got.body), minPartialBytes)
	case <-time.After(3 * time.Second):
		t.Fatal("partial response did not become available")
	}
	releaseTail()
}
