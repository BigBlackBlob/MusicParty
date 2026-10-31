package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/stretchr/testify/require"
)

type fixturePlatform struct {
	err              error
	playablePlatform string
}

type namedFixturePlatform struct {
	fixturePlatform
	name string
}

type searchCapturePlatform struct {
	fixturePlatform
	name    string
	keyword string
}

func (f *searchCapturePlatform) Name() string { return f.name }

func (f *searchCapturePlatform) Search(_ context.Context, keyword string, _, _ int) ([]platform.Music, error) {
	f.keyword = keyword
	return []platform.Music{}, nil
}

func (f namedFixturePlatform) Name() string { return f.name }

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
func (neteaseFixture) SearchAlbums(context.Context, string, int, int) (platform.AlbumSearchResult, error) {
	return platform.AlbumSearchResult{Items: []platform.Album{}}, nil
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
	name := f.playablePlatform
	if name == "" {
		name = "local"
	}
	return platform.PlayableMusic{Music: platform.Music{ID: id, Name: "Resolved Track", Artists: []string{"Resolved Artist"}, Duration: 60_000, Platform: name, CoverURL: "/api/" + name + "/cover/" + id}, URL: "/api/" + name + "/media/" + id}, nil
}
func (fixturePlatform) UserPlaylists(context.Context, string) ([]platform.Playlist, error) {
	return []platform.Playlist{}, nil
}
func (fixturePlatform) PlaylistSongs(context.Context, string, int, int) ([]platform.Music, error) {
	return []platform.Music{}, nil
}

// SearchAlbums echoes the page it was asked for, so route tests can prove offset/limit reach the
// provider and that the provider's own total is passed through untouched. A keyword of "no-total"
// stands in for a platform that reports no count at all, which must surface as 0 rather than the
// page length.
func (fixturePlatform) SearchAlbums(_ context.Context, keyword string, offset, limit int) (platform.AlbumSearchResult, error) {
	result := platform.AlbumSearchResult{Items: []platform.Album{{ID: fmt.Sprintf("album-%d", offset), Name: "Album", Platform: "local"}}, Total: 42}
	if strings.Contains(keyword, "no-total") {
		result.Total = 0
	}
	return result, nil
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
	}{{"/api/config", 200, `"authorName"`}, {"/api/platforms", 200, `"local"`}, {"/api/search/local/contract?limit=20&offset=0", 200, "[]"}, {"/api/search/local/contract?limit=20&offset=-1", 400, "bad-request"}, {"/api/music/lyric/local/id", 200, "歌词"}, {"/api/theme/extract-cover-color?url=https://evil.test/x", 400, "Only proxied"}} {
		request := httptest.NewRequest(http.MethodGet, test.path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, test.status, response.Code, test.path)
		require.Contains(t, response.Body.String(), test.body, test.path)
	}
}

func TestPlatformRouteDecodesEncodedPlatformNames(t *testing.T) {
	cfg := testConfig(t)
	service := namedFixturePlatform{fixturePlatform: fixturePlatform{}, name: "subsonic-local-navi@lounge"}
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewPlatformAPI(cfg, service))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/search/subsonic-local-navi%40lounge/song", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
}

func TestPlatformSearchDecodesEncodedKeywordWithEncodedPlatform(t *testing.T) {
	cfg := testConfig(t)
	service := &searchCapturePlatform{name: "subsonic-local-navi@lounge"}
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewPlatformAPI(cfg, service))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/search/subsonic-local-navi%40lounge/%E5%B9%BB%E6%83%B3", nil))
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.Equal(t, "幻想", service.keyword)
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

func TestRestrictedPlatformsUseSessionCookie(t *testing.T) {
	cfg := testConfig(t)
	api := NewPlatformAPI(cfg, fixturePlatform{})
	api.SetAccess("local", func(_ context.Context, token string) bool { return token == "session-token" })
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)

	for _, path := range []string{"/api/platforms?token=ignored", "/api/search/local/query?token=ignored"} {
		t.Run(path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, path, nil)
			request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-token"})
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code)
			if strings.HasPrefix(path, "/api/platforms") {
				require.Contains(t, response.Body.String(), `"local"`)
			}
		})
	}
	for _, path := range []string{"/api/platforms?token=session-token", "/api/search/local/query?token=session-token"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if strings.HasPrefix(path, "/api/platforms") {
			require.NotContains(t, response.Body.String(), `"local"`)
		} else {
			require.Equal(t, http.StatusForbidden, response.Code)
		}
	}

}

func TestPlatformAccessPolicyCanChangeConcurrentlyWithChecks(t *testing.T) {
	api := NewPlatformAPI(config.Config{})
	check := func(context.Context, string) bool { return true }
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				api.SetAccess("local", check)
				api.PlatformAllowed(context.Background(), "local", "session")
			}
		}()
	}
	wg.Wait()
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
