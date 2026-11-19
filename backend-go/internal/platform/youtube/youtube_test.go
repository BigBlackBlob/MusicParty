package youtube

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDuration(t *testing.T) {
	value, err := parseDuration("P1DT2H3M4S")
	require.NoError(t, err)
	require.Equal(t, 26*time.Hour+3*time.Minute+4*time.Second, value)
}

// rewriteToFixture sends the service's absolute Google URLs to a local server, because the client
// builds those URLs itself.
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

func TestSearchTrackURLs(t *testing.T) {
	const id = "Ab-12_cdE34"
	const watch = "https://www.youtube.com/watch?v=" + id
	const video = `{"items":[{"id":"` + id + `","snippet":{"title":"Track","channelTitle":"Artist","thumbnails":{"high":{"url":"https://img.test/cover.jpg"}}},"contentDetails":{"duration":"PT1M2S"}}]}`
	want := []platform.Music{{ID: id, Name: "Track", Artists: []string{"Artist"}, Duration: 62000, Platform: "youtube", CoverURL: "https://img.test/cover.jpg"}}
	for _, test := range []struct {
		name, keyword string
		status        int
		body          string
		offset        int
		fallback      bool
	}{
		{name: "watch", keyword: watch + "&t=42", body: video},
		{name: "short link", keyword: "https://youtu.be/" + id, body: video},
		{name: "embed", keyword: "https://www.youtube.com/embed/" + id, body: video},
		{name: "shorts", keyword: "https://www.youtube.com/shorts/" + id, body: video},
		{name: "mobile", keyword: "https://m.youtube.com/watch?v=" + id, body: video},
		{name: "music", keyword: "https://music.youtube.com/watch?v=" + id, body: video},
		{name: "later page", keyword: watch, offset: 1},
		{name: "empty items", keyword: watch, body: `{"items":[]}`},
		{name: "not found", keyword: watch, status: http.StatusNotFound},
		{name: "unauthorized", keyword: watch, status: http.StatusUnauthorized},
		{name: "upstream failure", keyword: watch, status: http.StatusInternalServerError},
		{name: "profile", keyword: "https://www.youtube.com/@artist?v=" + id, fallback: true},
		{name: "playlist", keyword: "https://www.youtube.com/playlist?list=list-id", fallback: true},
		{name: "wrong host", keyword: "https://www.youtube.com.other.test/watch?v=" + id, fallback: true},
		{name: "http scheme", keyword: "http://www.youtube.com/watch?v=" + id, fallback: true},
		{name: "invalid ID length", keyword: "https://youtu.be/short", fallback: true},
		{name: "invalid ID characters", keyword: "https://www.youtube.com/embed/Ab-12.cdE34", fallback: true},
		{name: "extra path segment", keyword: "https://youtu.be/" + id + "/other", fallback: true},
		{name: "missing v", keyword: "https://www.youtube.com/watch?list=list-id", fallback: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				query := r.URL.Query()
				assert.Equal(t, http.MethodGet, r.Method)
				assert.Equal(t, "key", query.Get("key"))
				if test.fallback {
					assert.Equal(t, "/youtube/v3/search", r.URL.Path)
					assert.Equal(t, test.keyword, query.Get("q"))
					_, _ = w.Write([]byte(`{"items":[]}`))
					return
				}
				assert.Equal(t, "/youtube/v3/videos", r.URL.Path)
				assert.Equal(t, id, query.Get("id"))
				assert.Equal(t, "snippet,contentDetails", query.Get("part"))
				if test.status != 0 {
					w.WriteHeader(test.status)
				}
				_, _ = w.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)
			client := platform.NewClient(time.Second)
			client.HTTP.Transport = rewriteToFixture{base: server.URL}
			client.Retries = 0
			service := New(client, true, "key", 20)
			tracks, err := service.Search(t.Context(), test.keyword, test.offset, 20)
			switch {
			case test.offset != 0:
				require.NoError(t, err)
				require.Equal(t, []platform.Music{}, tracks)
				require.Zero(t, calls.Load())
			case test.status == http.StatusUnauthorized || test.status == http.StatusInternalServerError:
				var upstream *platform.UpstreamError
				require.ErrorAs(t, err, &upstream)
				require.Equal(t, test.status, upstream.Status)
				require.Empty(t, tracks)
			case test.body == video:
				require.NoError(t, err)
				require.Equal(t, want, tracks)
			default:
				require.NoError(t, err)
				require.Equal(t, []platform.Music{}, tracks)
			}
			if test.offset == 0 {
				require.EqualValues(t, 1, calls.Load())
			}
		})
	}
}

func ids(items []platform.Music) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.ID)
	}
	return out
}
