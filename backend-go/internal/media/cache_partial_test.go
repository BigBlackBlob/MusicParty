package media

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testMediaSize = 2 << 20

func makeMediaBody() []byte {
	body := make([]byte, testMediaSize)
	for i := range body {
		body[i] = byte(i * 31)
	}
	return body
}

func slowMediaServer(body []byte, unknownLength bool, fail bool) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !unknownLength {
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		}
		flush := w.(http.Flusher)
		chunkSize := 64 * 1024
		limit := len(body)
		if fail {
			limit = 600 * 1024
		}
		for offset := 0; offset < limit; offset += chunkSize {
			end := min(offset+chunkSize, limit)
			_, _ = w.Write(body[offset:end])
			flush.Flush()
			time.Sleep(8 * time.Millisecond)
		}
	}))
}

func startPartialCache(t *testing.T, body []byte, unknownLength, fail bool) (*Cache, *httptest.Server, string) {
	t.Helper()
	upstream := slowMediaServer(body, unknownLength, fail)
	t.Cleanup(upstream.Close)
	cache, err := NewCache(t.TempDir(), 4<<20, upstream.Client(), 1, 1, 0, time.Millisecond)
	require.NoError(t, err)
	t.Cleanup(cache.Close)
	key := fmt.Sprintf("partial-%p", &cache)
	require.NoError(t, cache.Submit(key, upstream.URL, nil))
	return cache, upstream, key
}

func waitForWritten(t *testing.T, cache *Cache, key string, minimum int64) {
	t.Helper()
	require.Eventually(t, func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		entry := cache.inflight[key]
		return entry != nil && entry.total >= 0 && entry.written >= minimum
	}, 3*time.Second, 5*time.Millisecond)
}

func TestCacheServePartialFollowsGrowingFileAndAdvertisesTotal(t *testing.T) {
	body := makeMediaBody()
	for _, test := range []struct {
		name        string
		rangeHeader string
		status      int
	}{
		{name: "full response"},
		{name: "range response", rangeHeader: "bytes=0-", status: http.StatusPartialContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			cache, _, key := startPartialCache(t, body, false, false)
			waitForWritten(t, cache, key, minPartialBytes)
			request := httptest.NewRequest(http.MethodGet, "/media", nil)
			request.Header.Set("Range", test.rangeHeader)
			response := httptest.NewRecorder()
			served, err := cache.ServePartial(response, request, key, "")
			require.NoError(t, err)
			require.True(t, served)
			status := test.status
			if status == 0 {
				status = http.StatusOK
			}
			require.Equal(t, status, response.Code)
			require.Equal(t, fmt.Sprint(len(body)), response.Header().Get("Content-Length"))
			if test.rangeHeader != "" {
				require.Equal(t, fmt.Sprintf("bytes 0-%d/%d", len(body)-1, len(body)), response.Header().Get("Content-Range"))
			}
			require.Equal(t, body, response.Body.Bytes())
		})
	}
}

func TestCacheServePartialFallsBackWhenRangeIsUnavailable(t *testing.T) {
	body := makeMediaBody()
	cache, _, key := startPartialCache(t, body, false, false)
	waitForWritten(t, cache, key, minPartialBytes)
	request := httptest.NewRequest(http.MethodGet, "/media", nil)
	request.Header.Set("Range", "bytes=1048576-")
	served, err := cache.ServePartial(httptest.NewRecorder(), request, key, "")
	require.NoError(t, err)
	require.False(t, served)
}

func TestCacheServePartialFallsBackForUnknownLength(t *testing.T) {
	body := makeMediaBody()
	cache, _, key := startPartialCache(t, body, true, false)
	require.Eventually(t, func() bool {
		cache.mu.Lock()
		defer cache.mu.Unlock()
		entry := cache.inflight[key]
		return entry != nil && entry.written >= minPartialBytes
	}, 3*time.Second, 5*time.Millisecond)
	served, err := cache.ServePartial(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/media", nil), key, "")
	require.NoError(t, err)
	require.False(t, served)
}

func TestCacheServePartialReturnsErrorWhenUpstreamDies(t *testing.T) {
	body := makeMediaBody()
	cache, _, key := startPartialCache(t, body, false, true)
	waitForWritten(t, cache, key, minPartialBytes)
	response := httptest.NewRecorder()
	served, err := cache.ServePartial(response, httptest.NewRequest(http.MethodGet, "/media", nil), key, "")
	require.True(t, served)
	require.Error(t, err)
	require.NotEqual(t, len(body), response.Body.Len())
}

func TestCacheServePartialNotInflight(t *testing.T) {
	cache, err := NewCache(t.TempDir(), 4<<20, nil, 1, 1, 0, time.Millisecond)
	require.NoError(t, err)
	defer cache.Close()
	served, err := cache.ServePartial(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/media", nil), "missing", "")
	require.NoError(t, err)
	require.False(t, served)
	_, err = os.Stat(cache.pathForKey("missing") + ".part")
	require.ErrorIs(t, err, os.ErrNotExist)
}
