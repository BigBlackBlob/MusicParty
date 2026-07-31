package media

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProxyPreservesRangeAndCancelsUpstream(t *testing.T) {
	cancelled := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "bytes=1-2", r.Header.Get("Range"))
		w.Header().Set("Content-Type", "audio/test")
		w.Header().Set("Content-Range", "bytes 1-2/3")
		w.WriteHeader(http.StatusPartialContent)
		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
		<-r.Context().Done()
		close(cancelled)
	}))
	defer upstream.Close()
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodGet, "/stream", nil).WithContext(ctx)
	r.Header.Set("Range", "bytes=1-2")
	w := httptest.NewRecorder()
	done := make(chan error, 1)
	go func() { done <- (Proxy{Client: upstream.Client()}).Serve(w, r, upstream.URL, nil) }()
	time.Sleep(20 * time.Millisecond)
	cancel()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("upstream request was not cancelled")
	}
	<-done
	require.Equal(t, http.StatusPartialContent, w.Code)
	require.Equal(t, "bytes 1-2/3", w.Header().Get("Content-Range"))
}

func TestProxyMapsUpstreamStatuses(t *testing.T) {
	for _, test := range []struct {
		upstream, want int
	}{{http.StatusNotFound, http.StatusNotFound}, {http.StatusInternalServerError, http.StatusBadGateway}} {
		t.Run(http.StatusText(test.upstream), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(test.upstream) }))
			defer server.Close()
			response := httptest.NewRecorder()
			err := (Proxy{Client: server.Client()}).Serve(response, httptest.NewRequest(http.MethodGet, "/", nil), server.URL, nil)
			require.NoError(t, err)
			require.Equal(t, test.want, response.Code)
		})
	}
}
