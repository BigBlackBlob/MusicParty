package platform_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/bilibili"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/local"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/netease"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/subsonic"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/youtube"
	"github.com/stretchr/testify/require"
)

func TestBatch9ProvidersRejectInputBeforeIO(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(500) }))
	defer server.Close()
	client := &platform.Client{HTTP: server.Client(), MaxBody: 4096}
	providers := []platform.Service{
		netease.New(client, server.URL, "cookie"), bilibili.New(client, server.URL, "cookie"),
		subsonic.New(client, subsonic.Config{ID: "subsonic-test", BaseURL: server.URL}),
		youtube.New(client, true, "key", 50), local.New(nil),
	}
	for _, service := range providers {
		t.Run(service.Name(), func(t *testing.T) {
			for _, page := range [][2]int{{-1, 20}, {10001, 20}, {0, -1}, {0, 0}, {0, 101}} {
				_, err := service.Search(context.Background(), "song", page[0], page[1])
				var input *platform.InputError
				require.ErrorAs(t, err, &input)
			}
			_, err := service.Search(context.Background(), strings.Repeat("x", 129), 0, 20)
			var input *platform.InputError
			require.ErrorAs(t, err, &input)
		})
	}
	for _, service := range providers[:2] {
		for _, page := range [][2]int{{-1, 20}, {10001, 20}, {0, -1}, {0, 0}, {0, 501}} {
			_, err := service.PlaylistSongs(context.Background(), "list", page[0], page[1])
			var input *platform.InputError
			require.ErrorAs(t, err, &input)
		}
	}
	for _, service := range []platform.Service{providers[0], providers[2]} {
		_, err := service.SearchAlbums(context.Background(), "album", -1, 0)
		var input *platform.InputError
		require.ErrorAs(t, err, &input)
	}
	require.Zero(t, calls.Load(), "invalid internal requests must not reach an upstream")
}
