package subsonic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

func TestSearchUsesTokenAuthenticationAndMapsDuration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "search3.view", r.URL.Path[len("/rest/"):])
		require.Equal(t, "json", r.URL.Query().Get("f"))
		require.NotEmpty(t, r.URL.Query().Get("t"))
		_, _ = w.Write([]byte(`{"subsonic-response":{"searchResult3":{"song":[{"id":"s1","title":"Title","artist":"Artist","duration":12,"coverArt":"c1"}]}}}`))
	}))
	defer server.Close()
	service := New(&platform.Client{HTTP: server.Client(), Retries: 0, MaxBody: 2048}, Config{ID: "navidrome", BaseURL: server.URL, Username: "u", Password: "p", Client: "musicparty", Version: "1.16.1", ProxyPrefix: "/api/navidrome/cover", Enabled: true})
	songs, err := service.Search(context.Background(), "x", 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(12000), songs[0].Duration)
	require.Equal(t, "/api/navidrome/cover/c1", songs[0].CoverURL)
}
