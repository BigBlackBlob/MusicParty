package spotify

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const trackID = "0123456789abcdefghijkl"
const relinkedID = "0123456789ABCDEFGHIJKL"

func fixture(t *testing.T, catalog http.HandlerFunc) (*Service, *atomic.Int32) {
	t.Helper()
	calls := new(atomic.Int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/token" {
			catalog(w, r)
			return
		}
		assert.Equal(t, http.MethodPost, r.Method)
		id, secret, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "app-id", id)
		assert.Equal(t, "app-secret", secret)
		assert.NoError(t, r.ParseForm())
		assert.Equal(t, "client_credentials", r.PostForm.Get("grant_type"))
		fmt.Fprintf(w, `{"access_token":"bearer-%d","expires_in":3600}`, calls.Add(1))
	}))
	t.Cleanup(server.Close)
	service, ok := New(Config{Enabled: true, ClientID: "app-id", ClientSecret: "app-secret", Market: "jp"})
	require.True(t, ok)
	service.apiBase, service.tokenMgr.baseURL = server.URL, server.URL
	return service, calls
}

func trackFixture(id string) map[string]any {
	return map[string]any{"id": id, "uri": "spotify:track:" + id, "name": "Track", "duration_ms": 12345,
		"artists": []map[string]string{{"name": "Artist"}}, "album": map[string]any{"images": []map[string]string{{"url": "https://example.test/cover.jpg"}}}}
}

func TestAppTokenReuseAndConcurrentRenewal(t *testing.T) {
	service, calls := fixture(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) })
	now := time.Unix(1, 0)
	service.tokenMgr.now = func() time.Time { return now }
	ctx := context.Background()
	for _, elapsed := range []time.Duration{0, 58 * time.Minute} {
		now = time.Unix(1, 0).Add(elapsed)
		token, err := service.tokenMgr.AppToken(ctx)
		require.NoError(t, err)
		require.Equal(t, "bearer-1", token)
	}
	now = time.Unix(1, 0).Add(59 * time.Minute)
	var wg sync.WaitGroup
	for range 12 {
		wg.Go(func() {
			token, err := service.tokenMgr.AppToken(ctx)
			assert.NoError(t, err)
			assert.Equal(t, "bearer-2", token)
		})
	}
	wg.Wait()
	service.tokenMgr.Invalidate("bearer-1")
	token, err := service.tokenMgr.AppToken(ctx)
	require.NoError(t, err)
	require.Equal(t, "bearer-2", token)
	require.EqualValues(t, 2, calls.Load())
}

func TestCatalogRetriesUnauthorizedOnce(t *testing.T) {
	for _, rejectReplacement := range []bool{false, true} {
		t.Run(fmt.Sprintf("reject replacement=%t", rejectReplacement), func(t *testing.T) {
			requestCount := new(atomic.Int32)
			service, calls := fixture(t, func(w http.ResponseWriter, r *http.Request) {
				n := requestCount.Add(1)
				assert.Equal(t, fmt.Sprintf("Bearer bearer-%d", n), r.Header.Get("Authorization"))
				if n == 1 || rejectReplacement {
					http.Error(w, "sensitive upstream payload", http.StatusUnauthorized)
					return
				}
				fmt.Fprint(w, `{"tracks":{"items":[]}}`)
			})
			result, err := service.Search(context.Background(), "song", 0, 20)
			if rejectReplacement {
				require.ErrorContains(t, err, "HTTP 401")
				require.NotContains(t, err.Error(), "sensitive upstream payload")
			} else {
				require.NoError(t, err)
				require.Empty(t, result)
			}
			require.EqualValues(t, 2, requestCount.Load())
			require.EqualValues(t, 2, calls.Load())
		})
	}
}

func TestSearchPaginatesAndMapsTracks(t *testing.T) {
	keyword := "Björk + a&b / 音楽"
	var requests atomic.Int32
	service, calls := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		page := int(requests.Add(1) - 1)
		assert.Equal(t, "/v1/search", r.URL.Path)
		q := r.URL.Query()
		assert.Equal(t, keyword, q.Get("q"))
		assert.Equal(t, "track", q.Get("type"))
		assert.Equal(t, "JP", q.Get("market"))
		assert.Equal(t, "Bearer bearer-1", r.Header.Get("Authorization"))
		assert.Equal(t, strconv.Itoa(3+page*10), q.Get("offset"))
		limit := 10
		if page == 2 {
			limit = 3
		}
		assert.Equal(t, strconv.Itoa(limit), q.Get("limit"))
		items := make([]map[string]any, limit)
		for i := range items {
			items[i] = trackFixture(trackID)
		}
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"tracks": map[string]any{"items": items, "total": 100}}))
	})
	got, err := service.Search(context.Background(), keyword, 3, 23)
	require.NoError(t, err)
	require.Len(t, got, 23)
	require.Equal(t, trackID, got[0].ID)
	require.Equal(t, "Track", got[0].Name)
	require.Equal(t, []string{"Artist"}, got[0].Artists)
	require.EqualValues(t, 12345, got[0].Duration)
	require.Equal(t, "https://example.test/cover.jpg", got[0].CoverURL)
	require.Equal(t, "spotify", got[0].Platform)
	require.EqualValues(t, 3, requests.Load())
	require.EqualValues(t, 1, calls.Load())
}

func TestSearchLimitsAndEmpty(t *testing.T) {
	var requests atomic.Int32
	service, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		switch r.URL.Query().Get("q") {
		case "slow down":
			w.Header().Set("Retry-After", "12")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"reason":"QUOTA_EXCEEDED","message":"private details"}}`)
		case "oversized":
			io.WriteString(w, strings.Repeat(" ", maxCatalogBody+1))
		default:
			assert.Equal(t, "1000", r.URL.Query().Get("offset"))
			assert.Equal(t, "10", r.URL.Query().Get("limit"))
			fmt.Fprint(w, `{"tracks":{"items":[],"total":0}}`)
		}
	})
	ctx := context.Background()
	_, err := service.Search(ctx, "slow down", 0, 20)
	var rateLimit *RateLimitError
	require.ErrorAs(t, err, &rateLimit)
	require.Equal(t, 12*time.Second, rateLimit.RetryAfter)
	require.True(t, rateLimit.QuotaExceeded)
	require.NotContains(t, err.Error(), "private details")
	require.EqualValues(t, 1, requests.Load())
	_, err = service.Search(ctx, "oversized", 0, 20)
	require.ErrorContains(t, err, "response is too large")
	for _, offset := range []int{1000, 1001} {
		got, err := service.Search(ctx, "no matches", offset, 20)
		require.NoError(t, err)
		require.NotNil(t, got)
		require.Empty(t, got)
	}
	require.EqualValues(t, 3, requests.Load())
}

func TestResolvePlayableUsesCanonicalTrack(t *testing.T) {
	service, _ := fixture(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "JP", r.URL.Query().Get("market"))
		switch r.URL.Path {
		case "/v1/tracks/" + trackID:
			item := trackFixture(relinkedID)
			item["linked_from"] = trackFixture(trackID)
			assert.NoError(t, json.NewEncoder(w).Encode(item))
		case "/v1/tracks/" + relinkedID:
			assert.NoError(t, json.NewEncoder(w).Encode(trackFixture(relinkedID)))
		default:
			http.NotFound(w, r)
		}
	})
	for _, id := range []string{trackID, relinkedID} {
		playable, err := service.ResolvePlayable(context.Background(), id)
		require.NoError(t, err)
		require.Equal(t, relinkedID, playable.ID)
		require.Equal(t, "spotify:track:"+relinkedID, playable.EmbedID)
		require.Equal(t, "spotify-embed", playable.PlayerType)
		require.Empty(t, playable.URL)
	}
	_, err := service.ResolvePlayable(context.Background(), "0000000000000000000000")
	require.ErrorContains(t, err, "HTTP 404")
	_, err = service.ResolvePlayable(context.Background(), "../tracks")
	require.ErrorContains(t, err, "invalid track ID")
}

func TestNewRequiresEnabledAndConfigured(t *testing.T) {
	for _, cfg := range []Config{{}, {ClientID: "id", ClientSecret: "secret"}, {Enabled: true, ClientID: "id"}, {Enabled: true, ClientSecret: "secret"}} {
		service, ok := New(cfg)
		require.False(t, ok)
		require.Nil(t, service)
		require.False(t, service.Available())
	}
	service, ok := New(Config{Enabled: true, ClientID: "id", ClientSecret: "secret"})
	require.True(t, ok)
	require.True(t, service.Available())
	require.Equal(t, "US", service.market)
}

func TestRebuiltServiceAcquiresTokenWithRotatedCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, secret, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "app-id", id)
		assert.NoError(t, json.NewEncoder(w).Encode(map[string]any{"access_token": "token-for-" + secret, "expires_in": 3600}))
	}))
	t.Cleanup(server.Close)
	for _, secret := range []string{"old-secret", "new-secret"} {
		service, ok := New(Config{Enabled: true, ClientID: "app-id", ClientSecret: secret})
		require.True(t, ok)
		service.tokenMgr.baseURL = server.URL
		token, err := service.tokenMgr.AppToken(context.Background())
		require.NoError(t, err)
		require.Equal(t, "token-for-"+secret, token)
	}
}
