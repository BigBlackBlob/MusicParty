package soundcloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

// v2TrackServer serves a track payload from the public v2 API shape.
func v2TrackServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tracks/123" {
			t.Errorf("request path = %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func v2Service(t *testing.T, server *httptest.Server) *Service {
	t.Helper()
	client := platform.NewClient(time.Second)
	client.HTTP.Transport = rewriteTransport{target: server.URL, base: server.Client().Transport}
	return New(client, true, "client-id", "")
}

func TestResolvePlayableRejectsPreviewOnlyTracks(t *testing.T) {
	server := v2TrackServer(t, `{"id":123,"title":"Preview","duration":30000,"streamable":true,"policy":"SNIP","user":{"username":"Artist"}}`)
	_, err := v2Service(t, server).ResolvePlayable(t.Context(), "123")
	var streamError *platform.StreamResolutionError
	require.ErrorAs(t, err, &streamError)
	require.Equal(t, platform.StreamTrialOnly, streamError.Kind)
}

func TestResolvePlayableKeepsWidgetForProtectedTracks(t *testing.T) {
	server := v2TrackServer(t, `{"id":123,"title":"Protected","duration":180000,"streamable":true,"policy":"ALLOW","user":{"username":"Artist"},
		"media":{"transcodings":[{"preset":"aac_160k","format":{"protocol":"cbc-encrypted-hls"},"url":"https://api-v2.soundcloud.com/media/x"}]}}`)
	playable, err := v2Service(t, server).ResolvePlayable(t.Context(), "123")
	require.NoError(t, err)
	require.Equal(t, "soundcloud-widget", playable.PlayerType)
	require.Equal(t, "123", playable.EmbedID)
}

func TestResolvePlayableKeepsWidgetForStreamableTracks(t *testing.T) {
	server := v2TrackServer(t, `{"id":123,"title":"Track","duration":180000,"streamable":true,"policy":"ALLOW","user":{"username":"Artist"},
		"media":{"transcodings":[{"preset":"aac_160k","format":{"protocol":"hls"},"url":"https://api-v2.soundcloud.com/media/x"}]}}`)
	playable, err := v2Service(t, server).ResolvePlayable(t.Context(), "123")
	require.NoError(t, err)
	require.Equal(t, "soundcloud-widget", playable.PlayerType)
}

func TestStreamURLRejectsPreviewAndProtectedTracks(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		kind string
	}{
		{name: "preview", body: `{"id":123,"policy":"SNIP","duration":30000,"streamable":true,"user":{"username":"Artist"}}`, kind: platform.StreamTrialOnly},
		{name: "protected", body: `{"id":123,"policy":"ALLOW","duration":180000,"streamable":true,"user":{"username":"Artist"},
			"media":{"transcodings":[{"preset":"aac_160k","format":{"protocol":"ctr-encrypted-hls"},"url":"https://api-v2.soundcloud.com/media/x"}]}}`, kind: platform.StreamUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := v2TrackServer(t, test.body)
			_, err := v2Service(t, server).StreamURL(t.Context(), "123")
			var streamError *platform.StreamResolutionError
			require.ErrorAs(t, err, &streamError)
			require.Equal(t, test.kind, streamError.Kind)
		})
	}
}

func TestStreamURLResolvesSignedMediaLocation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/tracks/123":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":123,"policy":"ALLOW","duration":180000,"streamable":true,"user":{"username":"Artist"},
				"media":{"transcodings":[{"preset":"aac_160k","format":{"protocol":"hls"},"url":"https://api-v2.soundcloud.com/media/soundcloud:tracks:123/abc/stream/hls"}]}}`))
		case "/media/soundcloud:tracks:123/abc/stream/hls":
			// The media endpoint only resolves when the client_id is present.
			require.Equal(t, "client-id", r.URL.Query().Get("client_id"))
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"url":"https://playback.example/playlist.m3u8?expires=1"}`))
		default:
			t.Errorf("unexpected request path = %q", r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)

	client := platform.NewClient(time.Second)
	client.HTTP.Transport = rewriteTransport{target: server.URL, base: server.Client().Transport}
	service := New(client, true, "client-id", "")
	target, err := service.StreamURL(t.Context(), "123")
	require.NoError(t, err)
	require.Equal(t, "https://playback.example/playlist.m3u8?expires=1", target)
}

func TestDownloadRejectsPreviewOnlyTracks(t *testing.T) {
	server := v2TrackServer(t, `{"id":123,"policy":"SNIP","duration":30000,"streamable":true,"user":{"username":"Artist"}}`)
	err := v2Service(t, server).Download(t.Context(), "123", t.TempDir()+"/out.m4a", 1<<20)
	var streamError *platform.StreamResolutionError
	require.ErrorAs(t, err, &streamError)
	require.Equal(t, platform.StreamTrialOnly, streamError.Kind)
}

func TestClassifyProtectsEncryptedRenditions(t *testing.T) {
	encrypted := track{Policy: "ALLOW", Media: struct {
		Transcodings []transcoding `json:"transcodings"`
	}{Transcodings: []transcoding{{Format: struct {
		Protocol string `json:"protocol"`
		MimeType string `json:"mime_type"`
	}{Protocol: "cbc-encrypted-hls"}}}}}
	require.Equal(t, accessDRM, encrypted.classify())

	trial := track{Policy: "SNIP"}
	require.Equal(t, accessTrial, trial.classify())

	plain := track{Policy: "ALLOW", Media: struct {
		Transcodings []transcoding `json:"transcodings"`
	}{Transcodings: []transcoding{{Format: struct {
		Protocol string `json:"protocol"`
		MimeType string `json:"mime_type"`
	}{Protocol: "hls"}}}}}
	require.Equal(t, accessNative, plain.classify())
}

func TestArtistPrefersPublisherMetadata(t *testing.T) {
	item := track{User: struct {
		Username string `json:"username"`
	}{Username: "Fallback"}}
	item.Publisher.Artist = "Primary"
	require.Equal(t, "Primary", item.artist())
}

func TestResolvePlayableSurvivesUninspectableTracks(t *testing.T) {
	// A track the API cannot describe must still resolve to the widget, so
	// playback keeps working even when the classification is unavailable.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	client := platform.NewClient(time.Second)
	client.HTTP.Transport = rewriteTransport{target: server.URL, base: server.Client().Transport}
	service := New(client, true, "client-id", "")
	_, err := service.ResolvePlayable(context.Background(), "123")
	require.Error(t, err)
}
