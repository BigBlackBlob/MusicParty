package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform/netease"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/stretchr/testify/require"
)

func TestDesktopLyricsFormatsAndAuthentication(t *testing.T) {
	store := httpStore(t)
	insertStage5User(t, store, "lyrics-token", "member", "MEMBER")
	cfg := testConfig(t)
	api := NewDesktopAPI(cfg, account.New(store), nil, NewPlatformAPI(cfg, fixturePlatform{}))
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	for _, namespace := range []string{"music", "media"} {
		for _, authenticated := range []bool{false, true} {
			request := httptest.NewRequest(http.MethodGet, "/api/desktop/v1/"+namespace+"/local/track/lyrics", nil)
			request.Header.Set("X-Desktop-API-Version", desktopAPIVersion)
			request.Header.Set("X-Desktop-Client-Version", "0.2.0")
			if authenticated {
				request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "lyrics-token"})
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if !authenticated {
				require.Equal(t, http.StatusUnauthorized, response.Code)
			} else if namespace == "music" {
				require.Equal(t, http.StatusOK, response.Code)
				require.Equal(t, "text/plain; charset=utf-8", response.Header().Get("Content-Type"))
				require.Equal(t, "歌词", response.Body.String())
			} else {
				require.Equal(t, http.StatusOK, response.Code)
				require.Contains(t, response.Header().Get("Content-Type"), "application/json")
				require.Contains(t, response.Body.String(), "translation")
			}
		}
	}
}

// The media JSON route must carry verbatim YRC while the music compatibility
// route stays pure LRC text (desktop adapter consumes it as plain text).
func TestDesktopNeteaseWordLyricsRoutes(t *testing.T) {
	const yrc = "[0,4440](0,1320,0)Lately(1740,0,0), I've"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/lyric":
			_, _ = w.Write([]byte(`{"lrc":{"lyric":"[00:00.00]最近"},"tlyric":{},"romalrc":{}}`))
		case "/lyric/new":
			_, _ = w.Write([]byte(`{"lrc":{"lyric":"{\"t\":0}drift"},"yrc":{"lyric":"` + yrc + `"},"ytlrc":{},"yromalrc":{}}`))
		default:
			http.Error(w, `{"code":404,"message":"unexpected"}`, http.StatusNotFound)
		}
	}))
	defer upstream.Close()
	service := netease.New(&platform.Client{HTTP: upstream.Client(), Retries: 0, MaxBody: 4096}, upstream.URL, "secret")
	store := httpStore(t)
	insertStage5User(t, store, "word-lyrics-token", "member", "MEMBER")
	cfg := testConfig(t)
	api := NewDesktopAPI(cfg, account.New(store), nil, NewPlatformAPI(cfg, service))
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	for _, tc := range []struct{ namespace, wantContentType string }{{"media", "application/json"}, {"music", "text/plain; charset=utf-8"}} {
		request := httptest.NewRequest(http.MethodGet, "/api/desktop/v1/"+tc.namespace+"/netease/42/lyrics", nil)
		request.Header.Set("X-Desktop-API-Version", desktopAPIVersion)
		request.Header.Set("X-Desktop-Client-Version", "0.2.0")
		request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "word-lyrics-token"})
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
		require.Contains(t, response.Header().Get("Content-Type"), tc.wantContentType)
		if tc.namespace == "media" {
			require.Contains(t, response.Body.String(), `"wordLyric":"`+yrc+`"`)
			require.Contains(t, response.Body.String(), `"lyric":"[00:00.00]最近"`)
			require.NotContains(t, response.Body.String(), `"wordTranslatedLyric"`)
			require.NotContains(t, response.Body.String(), `"wordRomanizedLyric"`)
		} else {
			require.Equal(t, "[00:00.00]最近", response.Body.String())
			require.False(t, strings.Contains(response.Body.String(), "Lately"))
		}
	}
}

// Reject incompatible clients before authentication or invite consumption.
func TestDesktopVersionGate(t *testing.T) {
	for _, version := range []string{"", "0.1.99", "0.2", "0.02.0", "0.2.0-beta", "-1.2.0", "0.2.0", "1.0.0"} {
		t.Run(version, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.Header.Set("X-Desktop-API-Version", desktopAPIVersion)
			request.Header.Set("X-Desktop-Client-Version", version)
			response := httptest.NewRecorder()
			called := false
			desktopGuard(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })).ServeHTTP(response, request)
			compatible := version == "0.2.0" || version == "1.0.0"
			require.Equal(t, compatible, called)
			if !compatible {
				require.Equal(t, http.StatusUpgradeRequired, response.Code)
				require.Contains(t, response.Body.String(), "version-incompatible")
			}
		})
	}
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set("X-Desktop-Client-Version", "0.2.0")
	response := httptest.NewRecorder()
	require.False(t, desktopVersion(response, request))
	require.Equal(t, http.StatusUpgradeRequired, response.Code)
}

func TestDesktopDiscoveryAndInviteVersionBeforeCSRF(t *testing.T) {
	cfg := testConfig(t)
	api := NewDesktopAPI(cfg, nil, nil, NewPlatformAPI(cfg, fixturePlatform{}))
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	for _, path := range []string{"health", "capabilities"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/desktop/v1/"+path, nil))
		require.Equal(t, http.StatusOK, response.Code)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/desktop/v1/invites/redeem", nil))
	require.Equal(t, http.StatusUpgradeRequired, response.Code)
	request := httptest.NewRequest(http.MethodPost, "/api/desktop/v1/invites/redeem", nil)
	request.Header.Set("X-Desktop-API-Version", desktopAPIVersion)
	request.Header.Set("X-Desktop-Client-Version", "0.2.0")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "invite-invalid")
}
