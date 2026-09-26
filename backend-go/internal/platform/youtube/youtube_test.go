package youtube

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	value, err := parseDuration("P1DT2H3M4S")
	require.NoError(t, err)
	require.Equal(t, 26*time.Hour+3*time.Minute+4*time.Second, value)
}

// rewriteToFixture sends the service's absolute Google URLs to a local server, because the client
// builds those URLs itself. It exists so paging is testable at all without an API key.
type rewriteToFixture struct{ base string }

func (t rewriteToFixture) RoundTrip(request *http.Request) (*http.Response, error) {
	forked := request.Clone(request.Context())
	forked.URL = mustParse(t.base + request.URL.Path + "?" + request.URL.RawQuery)
	forked.Host = ""
	return http.DefaultTransport.RoundTrip(forked)
}

func mustParse(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return parsed
}

// The upstream search endpoint is paged by over-fetching one window and cutting the head off, so
// the regression this pins is "page 2 silently repeated page 1".
func TestSearchPagesWithoutRepeating(t *testing.T) {
	var asked []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/youtube/v3/search" {
			asked = append(asked, r.URL.Query().Get("maxResults"))
		}
		switch r.URL.Path {
		case "/youtube/v3/search":
			_, _ = w.Write([]byte(`{"items":[
				{"id":{"videoId":"v1"},"snippet":{"title":"One","channelTitle":"A","thumbnails":{"high":{"url":"http://img/1"}}}},
				{"id":{"videoId":"v2"},"snippet":{"title":"Two","channelTitle":"A","thumbnails":{"high":{"url":"http://img/2"}}}},
				{"id":{"videoId":"v3"},"snippet":{"title":"Three","channelTitle":"A","thumbnails":{"high":{"url":"http://img/3"}}}},
				{"id":{"videoId":"v4"},"snippet":{"title":"Four","channelTitle":"A","thumbnails":{"high":{"url":"http://img/4"}}}},
				{"id":{"videoId":"v5"},"snippet":{"title":"Five","channelTitle":"A","thumbnails":{"high":{"url":"http://img/5"}}}},
				{"id":{"videoId":"v6"},"snippet":{"title":"Six","channelTitle":"A","thumbnails":{"high":{"url":"http://img/6"}}}}]}`))
		case "/youtube/v3/videos":
			_, _ = w.Write([]byte(`{"items":[{"id":"v1","contentDetails":{"duration":"PT1M"}},{"id":"v2","contentDetails":{"duration":"PT1M"}},{"id":"v3","contentDetails":{"duration":"PT1M"}},{"id":"v4","contentDetails":{"duration":"PT1M"}},{"id":"v5","contentDetails":{"duration":"PT1M"}},{"id":"v6","contentDetails":{"duration":"PT1M"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: &http.Client{Transport: rewriteToFixture{base: server.URL}}, Retries: 0, MaxBody: 1 << 16}, true, "key", 6)

	first, err := service.Search(context.Background(), "song", 0, 3)
	require.NoError(t, err)
	require.Equal(t, []string{"v1", "v2", "v3"}, ids(first))
	second, err := service.Search(context.Background(), "song", 3, 3)
	require.NoError(t, err)
	require.Equal(t, []string{"v4", "v5", "v6"}, ids(second), "the second page must continue, not restart")
	require.Equal(t, []string{"3", "6"}, asked, "each page fetches exactly its own window")

	beyond, err := service.Search(context.Background(), "song", 6, 3)
	require.NoError(t, err)
	require.Empty(t, beyond, "past the window is an empty page, never a repeat of page 1")
}

func ids(items []platform.Music) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}
