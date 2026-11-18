package qq

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestAvailable(t *testing.T) {
	client := platform.NewClient(time.Second)
	require.False(t, New(client, true, "").Available())
	require.False(t, New(client, false, "http://127.0.0.1:3100").Available())
	require.True(t, New(client, true, "http://127.0.0.1:3100").Available())
}

func TestStreamURLUnavailable(t *testing.T) {
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/resolve" || r.URL.Query().Get("id") != "songMid" || r.Header.Get("X-QQ-Cookie") != "session=fixture" {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"error":"no_playable_url"}`)
	}))
	t.Cleanup(sidecar.Close)
	service := New(platform.NewClient(time.Second), true, sidecar.URL)
	service.UpdateCredential("session=fixture")

	_, err := service.StreamURL(t.Context(), "songMid")
	var resolutionErr *platform.StreamResolutionError
	require.ErrorAs(t, err, &resolutionErr)
	require.Equal(t, platform.StreamUnavailable, resolutionErr.Kind)
	require.EqualError(t, resolutionErr, "qq: no playable URL")
}

func TestStreamURLRejectsNonHTTPS(t *testing.T) {
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"url":"http://127.0.0.1/private","expires":0}`)
	}))
	t.Cleanup(sidecar.Close)
	service := New(platform.NewClient(time.Second), true, sidecar.URL)

	stream, err := service.StreamURL(t.Context(), "songMid")
	require.Empty(t, stream)
	require.ErrorContains(t, err, "valid HTTPS stream URL")
}

func TestSearchMapsAndCachesSidecarTracks(t *testing.T) {
	sidecar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			query := r.URL.Query()
			if r.Method != http.MethodGet || query.Get("keyword") != "音乐 & more" || query.Get("offset") != "13" || query.Get("limit") != "20" || r.Header.Get("X-QQ-Cookie") != "session=fixture" {
				http.Error(w, "unexpected search request", http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `[{"id":"songMid","name":"Track","artists":["Singer","Guest"],"duration":123000,"coverUrl":"https://cover.example/art.jpg"}]`)
		case "/resolve":
			fmt.Fprint(w, `{"url":"https://audio.example/song?ticket=fixture","expires":0}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(sidecar.Close)
	service := New(platform.NewClient(time.Second), true, sidecar.URL)
	service.UpdateCredential("session=fixture")
	want := platform.Music{
		ID: "songMid", Name: "Track", Artists: []string{"Singer", "Guest"}, Duration: 123000,
		Platform: "qq", CoverURL: "https://cover.example/art.jpg",
	}

	tracks, err := service.Search(t.Context(), "音乐 & more", 13, 20)
	require.NoError(t, err)
	require.Equal(t, []platform.Music{want}, tracks)

	cover, err := service.CoverURL(t.Context(), want.ID)
	require.NoError(t, err)
	require.Equal(t, want.CoverURL, cover)

	playable, err := service.ResolvePlayable(t.Context(), want.ID)
	require.NoError(t, err)
	require.Equal(t, platform.PlayableMusic{Music: want, URL: "/api/qq/stream/songMid"}, playable)
	require.False(t, service.StreamCacheable())
}
