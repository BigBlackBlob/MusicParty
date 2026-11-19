package soundcloud

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

func TestAvailableRequiresClientID(t *testing.T) {
	service := New(platform.NewClient(time.Second), true, "", "")
	if service.Available() {
		t.Fatal("Available() = true with an empty client ID")
	}
}

func TestResolvePlayableUsesSoundCloudWidget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tracks/123" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("client_id"); got != "client-id" {
			t.Errorf("client_id = %q", got)
		}
		if got := r.URL.Query().Get("oauth_token"); got != "test-token" {
			t.Errorf("oauth_token = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "OAuth test-token" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":123,"title":"Track","duration":180000,"artwork_url":"https://img.example/cover-large.jpg","streamable":true,"user":{"username":"Artist"}}`))
	}))
	defer server.Close()

	client := platform.NewClient(time.Second)
	client.HTTP = &http.Client{Transport: rewriteTransport{target: server.URL, base: server.Client().Transport}}
	service := New(client, true, "client-id", "")
	service.UpdateCredential("test-token")
	playable, err := service.ResolvePlayable(context.Background(), "123")
	if err != nil {
		t.Fatal(err)
	}
	if playable.PlayerType != "soundcloud-widget" || playable.EmbedID != "123" || playable.URL != "" {
		t.Fatalf("unexpected playable target: %+v", playable)
	}
	if playable.ID != "123" || playable.Platform != "soundcloud" || playable.Duration != 180000 {
		t.Fatalf("unexpected track metadata: %+v", playable.Music)
	}
}

func TestSearchResolvesPermalink(t *testing.T) {
	const permalink = "https://SoundCloud.com/artist/track?in=artist%2Fset"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/tracks/123" {
			_, _ = w.Write([]byte(`{"id":123,"title":"Track","duration":180000,"artwork_url":"https://img.example/cover-large.jpg","streamable":true,"user":{"username":"Artist"}}`))
			return
		}
		query := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/resolve" || query.Get("url") != permalink ||
			query.Get("client_id") != "client-id" || query.Get("oauth_token") != "test-token" {
			t.Errorf("unexpected resolve request: %s %s", r.Method, r.URL)
			http.Error(w, "unexpected resolve request", http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, "/tracks/123", http.StatusFound)
	}))
	t.Cleanup(server.Close)

	client := platform.NewClient(time.Second)
	client.HTTP.Transport = rewriteTransport{target: server.URL, base: server.Client().Transport}
	service := New(client, true, "client-id", "")
	service.UpdateCredential("test-token")
	tracks, err := service.Search(t.Context(), permalink, 0, 20)
	require.NoError(t, err)
	require.Equal(t, []platform.Music{{
		ID: "123", Name: "Track", Artists: []string{"Artist"}, Duration: 180000,
		Platform: "soundcloud", CoverURL: "https://img.example/cover-large.jpg",
	}}, tracks)
}

func TestSearchPermalinkResolveResponses(t *testing.T) {
	const permalink = "https://soundcloud.com/artist/track"
	for _, test := range []struct {
		name   string
		status int
		body   string
	}{
		{name: "non-streamable", status: http.StatusOK, body: `{"id":123,"streamable":false}`},
		{name: "not found", status: http.StatusNotFound},
		{name: "missing track ID", status: http.StatusOK, body: `{"streamable":true}`},
		{name: "wrong JSON shape", status: http.StatusOK, body: `[]`},
		{name: "malformed JSON", status: http.StatusOK, body: `{"id":`},
		{name: "unauthorized", status: http.StatusUnauthorized},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/resolve" || r.URL.Query().Get("url") != permalink {
					t.Errorf("unexpected resolve request: %s", r.URL)
					http.Error(w, "unexpected resolve request", http.StatusBadRequest)
					return
				}
				w.WriteHeader(test.status)
				_, _ = w.Write([]byte(test.body))
			}))
			t.Cleanup(server.Close)

			client := platform.NewClient(time.Second)
			client.HTTP.Transport = rewriteTransport{target: server.URL, base: server.Client().Transport}
			service := New(client, true, "client-id", "")
			tracks, err := service.Search(t.Context(), permalink, 0, 20)
			if test.status == http.StatusUnauthorized {
				var upstream *platform.UpstreamError
				require.ErrorAs(t, err, &upstream)
				require.Equal(t, http.StatusUnauthorized, upstream.Status)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, tracks)
		})
	}
}

func TestSearchPermalinkLaterPageIsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request: %s", r.URL)
		http.Error(w, "unexpected request", http.StatusBadRequest)
	}))
	t.Cleanup(server.Close)

	client := platform.NewClient(time.Second)
	client.HTTP.Transport = rewriteTransport{target: server.URL, base: server.Client().Transport}
	service := New(client, true, "client-id", "")
	tracks, err := service.Search(t.Context(), "https://soundcloud.com/artist/track", 20, 20)
	require.NoError(t, err)
	require.Empty(t, tracks)
}

func TestSearchKeywordsAndNonTrackURLs(t *testing.T) {
	for _, test := range []struct {
		name    string
		keyword string
	}{
		{name: "set", keyword: "https://soundcloud.com/artist/sets/collection"},
		{name: "profile", keyword: "https://soundcloud.com/artist"},
		{name: "www host", keyword: "https://www.soundcloud.com/artist/track"},
		{name: "mobile host", keyword: "https://m.soundcloud.com/artist/track"},
		{name: "short link host", keyword: "https://on.soundcloud.com/artist/track"},
		{name: "API host", keyword: "https://api.soundcloud.com/artist/track"},
		{name: "API v2 host", keyword: "https://api-v2.soundcloud.com/artist/track"},
		{name: "HTTP scheme", keyword: "http://soundcloud.com/artist/track"},
		{name: "invalid URL", keyword: "https://soundcloud.com/artist/%zz"},
		{name: "empty artist", keyword: "https://soundcloud.com//track"},
		{name: "empty track", keyword: "https://soundcloud.com/artist/"},
		{name: "keyword", keyword: "音乐 & more"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				if r.URL.Path != "/tracks" || query.Get("q") != test.keyword ||
					query.Get("offset") != "0" || query.Get("limit") != "20" || query.Get("linked_partitioning") != "1" {
					t.Errorf("unexpected keyword search request: %s", r.URL)
					http.Error(w, "unexpected keyword search request", http.StatusBadRequest)
					return
				}
				_, _ = w.Write([]byte(`{"collection":[{"id":42,"title":"Search result","streamable":true,"user":{"username":"Artist"}}]}`))
			}))
			t.Cleanup(server.Close)

			client := platform.NewClient(time.Second)
			client.HTTP.Transport = rewriteTransport{target: server.URL, base: server.Client().Transport}
			service := New(client, true, "client-id", "")
			tracks, err := service.Search(t.Context(), test.keyword, 0, 20)
			require.NoError(t, err)
			require.Equal(t, []platform.Music{{
				ID: "42", Name: "Search result", Artists: []string{"Artist"}, Platform: "soundcloud",
			}}, tracks)
		})
	}
}

type rewriteTransport struct {
	target string
	base   http.RoundTripper
}

func (r rewriteTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	target, err := url.Parse(r.target)
	if err != nil {
		return nil, err
	}
	copy := request.Clone(request.Context())
	copy.URL.Scheme = target.Scheme
	copy.URL.Host = target.Host
	copy.Host = target.Host
	return r.base.RoundTrip(copy)
}
