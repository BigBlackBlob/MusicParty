package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

type fixturePlatform struct{ err error }

type neteaseFixture struct{ err error }

func (neteaseFixture) Name() string    { return "netease" }
func (neteaseFixture) Available() bool { return true }
func (f neteaseFixture) Search(context.Context, string, int, int) ([]platform.Music, error) {
	return []platform.Music{}, f.err
}
func (neteaseFixture) UserPlaylists(context.Context, string) ([]platform.Playlist, error) {
	return []platform.Playlist{}, nil
}
func (f neteaseFixture) PlaylistSongs(context.Context, string, int, int) ([]platform.Music, error) {
	return []platform.Music{}, f.err
}
func (neteaseFixture) SearchAlbums(context.Context, string) ([]platform.Album, error) {
	return []platform.Album{}, nil
}
func (neteaseFixture) AlbumSongs(context.Context, string) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (neteaseFixture) SearchUsers(context.Context, string) ([]platform.User, error) {
	return []platform.User{}, nil
}
func (neteaseFixture) Lyric(context.Context, string) (platform.Lyric, error) {
	return platform.Lyric{}, nil
}

func (fixturePlatform) Name() string    { return "local" }
func (fixturePlatform) Available() bool { return true }
func (f fixturePlatform) Search(context.Context, string, int, int) ([]platform.Music, error) {
	return []platform.Music{}, f.err
}
func (f fixturePlatform) ResolvePlayable(_ context.Context, id string) (platform.PlayableMusic, error) {
	if f.err != nil {
		return platform.PlayableMusic{}, f.err
	}
	return platform.PlayableMusic{Music: platform.Music{ID: id, Name: "Resolved Track", Artists: []string{"Resolved Artist"}, Duration: 60_000, Platform: "local", CoverURL: "/api/local/cover/" + id}, URL: "/api/local/media/" + id}, nil
}
func (fixturePlatform) UserPlaylists(context.Context, string) ([]platform.Playlist, error) {
	return []platform.Playlist{}, nil
}
func (fixturePlatform) PlaylistSongs(context.Context, string, int, int) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (fixturePlatform) SearchAlbums(context.Context, string) ([]platform.Album, error) {
	return []platform.Album{}, nil
}
func (fixturePlatform) AlbumSongs(context.Context, string) ([]platform.Music, error) {
	return []platform.Music{}, nil
}
func (fixturePlatform) SearchUsers(context.Context, string) ([]platform.User, error) {
	return []platform.User{}, nil
}
func (fixturePlatform) Lyric(context.Context, string) (platform.Lyric, error) {
	return platform.Lyric{Lyric: "歌词", TranslatedLyric: "translation"}, nil
}

func TestPlatformRoutesMatchFrozenGoldenStatuses(t *testing.T) {
	cfg := testConfig(t)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewPlatformAPI(cfg, fixturePlatform{}))
	for _, test := range []struct {
		path   string
		status int
		body   string
	}{{"/api/config", 200, `"authorName"`}, {"/api/platforms", 200, `"local"`}, {"/api/search/local/contract?limit=20&offset=0", 200, "[]"}, {"/api/search/local/contract?limit=20&offset=-1", 500, "ResponseStatusException"}, {"/api/music/lyric/local/id", 200, "歌词"}, {"/api/theme/extract-cover-color?url=https://evil.test/x", 400, "Only proxied"}} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, test.status, response.Code, test.path)
		require.Contains(t, response.Body.String(), test.body, test.path)
	}
}
func TestPlatformUpstreamFailureIsBadGateway(t *testing.T) {
	cfg := testConfig(t)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewPlatformAPI(cfg, fixturePlatform{err: errors.New("upstream")}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/search/local/x", nil))
	require.Equal(t, http.StatusBadGateway, response.Code)
	require.Contains(t, response.Body.String(), "Upstream API request failed")
}

func TestNeteaseErrorsAreClassified(t *testing.T) {
	cfg := testConfig(t)
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "missing cookie", err: &platform.CredentialNotConfiguredError{Platform: "netease"}, want: "尚未配置网易云 Cookie"},
		{name: "unreachable API", err: &platform.UpstreamError{Cause: errors.New("connection refused")}, want: "网易云 API 服务不可达"},
		{name: "invalid cookie", err: &platform.UpstreamError{Status: http.StatusUnauthorized}, want: "网易云 Cookie 无效或已过期"},
		{name: "upstream failure", err: &platform.UpstreamError{Status: http.StatusBadGateway}, want: "网易云 API 服务异常"},
	} {
		t.Run(test.name, func(t *testing.T) {
			handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewPlatformAPI(cfg, neteaseFixture{err: test.err}))
			for _, path := range []string{"/api/search/netease/x", "/api/playlist/songs/netease/id"} {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				require.Equal(t, http.StatusBadGateway, response.Code, path)
				require.Contains(t, response.Body.String(), test.want, path)
			}
		})
	}
}
func TestRestrictedPlatformRequiresToken(t *testing.T) {
	cfg := testConfig(t)
	api := NewPlatformAPI(cfg, fixturePlatform{})
	api.SetAccess("local", func(context.Context, string) bool { return false })
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/search/local/x", nil))
	require.Equal(t, http.StatusForbidden, response.Code)
}
func testConfig(t *testing.T) config.Config {
	t.Helper()
	cfg, err := config.Load(func(key string) (string, bool) {
		values := map[string]string{"DB_ENABLED": "false", "AUTH_SECURE_COOKIES": "false", "APP_AUTHOR_NAME": "Author", "APP_BACK_WORDS": "Back"}
		value, ok := values[key]
		return value, ok
	})
	require.NoError(t, err)
	cfg.Application.AllowedOrigins = []string{}
	cfg.Auth.TrustedProxyCIDRs = []string{}
	return cfg
}

var _ = strings.Builder{}
