package netease

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestStreamFirstMatchingEntryDecides(t *testing.T) {
	for _, tc := range []struct {
		name, body, kind string
		calls            int
	}{
		{"unavailable does not rescue trial", `{"data":[{"id":42,"code":404,"url":null}]}`, platform.StreamUnavailable, 1},
		{"empty URL is final", `{"data":[{"id":42,"url":""}]}`, platform.StreamUnavailable, 1},
		{"top trial is final", `{"data":[{"id":42,"url":"trial","freeTrialInfo":{"end":30}}]}`, platform.StreamTrialOnly, 1},
		{"all levels unavailable", `{"data":[]}`, platform.StreamUnavailable, 3},
		{"unrelated track falls through", `{"data":[{"id":99,"url":"wrong"}]}`, platform.StreamTrialOnly, 2},
		{"matching entry after unrelated", `{"data":[{"id":99,"url":"wrong"},{"id":42,"url":"trial","freeTrialInfo":{"end":30}}]}`, platform.StreamTrialOnly, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				n := calls.Add(1)
				body := tc.body
				if n > 1 && tc.name != "all levels unavailable" {
					body = `{"data":[{"id":42,"url":"trial","freeTrialInfo":{"end":30}}]}`
				}
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			s := New(&platform.Client{HTTP: server.Client(), MaxBody: 4096}, server.URL, "A")
			got, err := s.StreamURL(context.Background(), "42")
			require.Empty(t, got)
			var e *platform.StreamResolutionError
			require.ErrorAs(t, err, &e)
			require.Equal(t, tc.kind, e.Kind)
			require.EqualValues(t, tc.calls, calls.Load())
		})
	}
}
func TestStreamCredentialUpdateDuringResolution(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie := r.URL.Query().Get("cookie")
		if calls.Add(1) == 1 {
			close(started)
			<-release
		}
		fmt.Fprintf(w, `{"data":[{"id":42,"url":"https://cdn.test/%s"}]}`, cookie)
	}))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	s := New(&platform.Client{HTTP: server.Client(), MaxBody: 4096}, server.URL, "A")
	done := make(chan error, 1)
	go func() { _, err := s.StreamURL(context.Background(), "42"); done <- err }()
	<-started
	s.UpdateCredential("B")
	close(release)
	require.NoError(t, <-done)
	got, err := s.StreamURL(context.Background(), "42")
	require.NoError(t, err)
	require.Equal(t, "https://cdn.test/B", got)
	require.EqualValues(t, 2, calls.Load())
}

// observingContext signals entry into the waiter's select.
type observingContext struct {
	context.Context
	observed chan struct{}
}

func (c observingContext) Done() <-chan struct{} { close(c.observed); return c.Context.Done() }
func TestStreamCancelledLeaderPublishesToWaiter(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { close(started); <-release }))
	defer server.Close()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	s := New(&platform.Client{HTTP: server.Client(), MaxBody: 4096}, server.URL, "A")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	leader := make(chan error, 1)
	go func() { _, err := s.StreamURL(ctx, "42"); leader <- err }()
	<-started
	observed := make(chan struct{})
	waiter := make(chan error, 1)
	go func() { _, err := s.StreamURL(observingContext{context.Background(), observed}, "42"); waiter <- err }()
	<-observed
	cancel()
	for _, done := range []chan error{leader, waiter} {
		select {
		case err := <-done:
			require.ErrorIs(t, err, context.Canceled)
		case <-time.After(3 * time.Second):
			t.Fatal("caller remained blocked")
		}
	}
}
func TestStreamWaiterHonorsOwnCancellation(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		fmt.Fprint(w, `{"data":[{"id":42,"url":"full"}]}`)
	}))
	defer server.Close()
	s := New(&platform.Client{HTTP: server.Client(), MaxBody: 4096}, server.URL, "A")
	leader := make(chan error, 1)
	go func() { _, err := s.StreamURL(context.Background(), "42"); leader <- err }()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := s.StreamURL(ctx, "42")
	close(release)
	require.ErrorIs(t, err, context.Canceled)
	require.NoError(t, <-leader)
}
func TestStreamCacheSizeCap(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprintf(w, `{"data":[{"id":%s,"url":"full"}]}`, r.URL.Query().Get("id"))
	}))
	defer server.Close()
	s := New(&platform.Client{HTTP: server.Client(), MaxBody: 4096}, server.URL, "A")
	s.now = func() time.Time { return time.Unix(100, 0) }
	for i := 1; i <= 257; i++ {
		_, err := s.StreamURL(context.Background(), strconv.Itoa(i))
		require.NoError(t, err)
		require.LessOrEqual(t, len(s.streams), 256)
	}
	require.Len(t, s.streams, 256)
	for i := 1; i <= 257; i++ {
		id := strconv.Itoa(i)
		if _, ok := s.streams[id]; !ok {
			_, err := s.StreamURL(context.Background(), id)
			require.NoError(t, err)
			break
		}
	}
	require.EqualValues(t, 258, calls.Load())
	require.Len(t, s.streams, 256)
}
