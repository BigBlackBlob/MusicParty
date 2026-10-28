package httpapi

import (
	"image"
	"image/color"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/subsonic"
	"github.com/stretchr/testify/require"
)

func TestBatch7CoverColorUsesCurrentAuthorization(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		require.Empty(t, r.Header.Get("Cookie"), "browser credentials must stay within the app")
		require.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "image/png")
		img := image.NewRGBA(image.Rect(0, 0, 1, 1))
		img.Set(0, 0, color.RGBA{R: 32, G: 64, B: 128, A: 255})
		require.NoError(t, png.Encode(w, img))
	}))
	defer upstream.Close()
	store := httpStore(t)
	accounts := account.New(store)
	insertStage5User(t, store, "listener-token", "listener", "MEMBER")
	cfg := testConfig(t)
	service := subsonic.New(platform.NewClient(time.Second), subsonic.Config{ID: "navidrome", BaseURL: upstream.URL, Username: "u", Password: "p", Enabled: true})
	platforms := NewPlatformAPI(cfg, service)
	platforms.SetAccess("navidrome", PlatformAccess(accounts, "listener"))
	mediaAPI := NewMediaAPI(cfg, accounts, store, nil, platforms, http.DefaultClient, nil)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), platforms, mediaAPI)
	self := httptest.NewServer(handler)
	defer self.Close()
	platforms.cfg.Application.BaseURL = self.URL
	for _, cover := range []struct{ name, path string }{
		{"navidrome", "/api/navidrome/cover/art"},
		{"subsonic-source@lounge", "/api/subsonic/source/cover/art"},
		{"subsonic-source@room", "/api/subsonic/room/source/cover/art"},
	} {
		platforms.Register(subsonic.New(platform.NewClient(time.Second), subsonic.Config{ID: cover.name, BaseURL: upstream.URL, Username: "u", Password: "p", Enabled: true}))
		platforms.SetAccess(cover.name, PlatformAccess(accounts, "listener"))
		before := calls.Load()
		path := "/api/theme/extract-cover-color?url=" + url.QueryEscape(cover.path)
		w := batch2Request(handler, "GET", path, "", "listener-token", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		require.Contains(t, w.Body.String(), "#204080")
		require.EqualValues(t, before+1, calls.Load())
		platforms.SetAccess(cover.name, PlatformAccess(accounts, "nobody"))
		w = batch2Request(handler, "GET", path, "", "listener-token", "")
		require.Equal(t, http.StatusForbidden, w.Code)
		require.EqualValues(t, before+1, calls.Load(), "denied requests must not fetch upstream")
	}
	for _, invalid := range []string{"https://evil.test/image", "//evil.test/image", "/api/navidrome/cover/../art", "/api/subsonic/source/cover/art/extra", "/api/navidrome/cover/art?url=https://evil.test", "/media/../../secret"} {
		w := batch2Request(handler, "GET", "/api/theme/extract-cover-color?url="+url.QueryEscape(invalid), "", "listener-token", "")
		require.Equal(t, http.StatusBadRequest, w.Code, invalid)
	}
	require.EqualValues(t, 3, calls.Load())
}
