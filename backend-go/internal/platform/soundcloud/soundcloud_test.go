package soundcloud

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestAvailableForPublicLinksWithoutClientID(t *testing.T) {
	service := New(platform.NewClient(time.Second), true, "", "")
	require.True(t, service.Available())
	require.False(t, New(platform.NewClient(time.Second), false, "", "").Available())
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
				if test.status == http.StatusUnauthorized && r.URL.Path == "/oembed" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
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

func TestPublicPermalinkSurvivesUnavailableAPIAndServiceRestart(t *testing.T) {
	const permalink = "https://soundcloud.com/mystical-complex/mystical-complex-kill-them-1"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/oembed" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		require.Empty(t, r.Header.Get("Authorization"))
		require.Empty(t, r.URL.Query().Get("client_id"))
		require.Empty(t, r.URL.Query().Get("oauth_token"))
		require.Contains(t, []string{permalink, "https://api.soundcloud.com/tracks/54202938"}, r.URL.Query().Get("url"))
		_, _ = w.Write([]byte(`{"title":"Kill them All II by Mystical Complex","author_name":"Mystical Complex","thumbnail_url":"https://i1.sndcdn.com/cover.jpg","html":"<iframe src=\"https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F54202938&visual=true\"></iframe>"}`))
	}))
	t.Cleanup(server.Close)
	client := platform.NewClient(time.Second)
	client.HTTP.Transport = rewriteTransport{target: server.URL, base: server.Client().Transport}
	service := New(client, true, "rejected-client", "")
	service.UpdateCredential("private-token")
	tracks, err := service.Search(t.Context(), permalink, 0, 20)
	require.NoError(t, err)
	require.Len(t, tracks, 1)
	require.Equal(t, "54202938", tracks[0].ID)
	require.Equal(t, "Kill them All II", tracks[0].Name)
	for _, clientID := range []string{"", "rejected-client"} {
		restarted := New(client, true, clientID, "")
		require.True(t, restarted.Available())
		playable, err := restarted.ResolvePlayable(t.Context(), tracks[0].ID)
		require.NoError(t, err)
		require.Equal(t, tracks[0], playable.Music)
		require.Equal(t, "soundcloud-widget", playable.PlayerType)
		require.Equal(t, "54202938", playable.EmbedID)
	}
}

func TestPublicOEmbedRejectsUnexpectedPlayerTargets(t *testing.T) {
	for _, target := range []string{
		"https://evil.example/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F123",
		"https://w.soundcloud.com/player/?url=https%3A%2F%2Fevil.example%2Ftracks%2F123",
		"https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Fplaylists%2F123",
		"https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F-1",
		"https://w.soundcloud.com/player/?url=https%3A%2F%2Fapi.soundcloud.com%2Ftracks%2F123%2Fstream",
	} {
		t.Run(target, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/oembed", r.URL.Path)
				_ = json.NewEncoder(w).Encode(map[string]string{"title": "Track", "html": `<iframe src="` + target + `"></iframe>`})
			}))
			t.Cleanup(server.Close)
			client := platform.NewClient(time.Second)
			client.HTTP.Transport = rewriteTransport{target: server.URL, base: server.Client().Transport}
			tracks, err := New(client, true, "", "").Search(t.Context(), "https://soundcloud.com/artist/track", 0, 20)
			require.ErrorContains(t, err, "did not return a track Widget")
			require.Empty(t, tracks)
		})
	}
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
				if r.URL.Path != "/search/tracks" || query.Get("q") != test.keyword ||
					query.Get("offset") != "0" || query.Get("limit") != "20" {
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
