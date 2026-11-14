package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	roomdomain "github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/room"
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
	api := NewDesktopAPI(cfg, account.New(store), nil, nil, NewPlatformAPI(cfg, fixturePlatform{}))
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
	api := NewDesktopAPI(cfg, account.New(store), nil, nil, NewPlatformAPI(cfg, service))
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
	api := NewDesktopAPI(cfg, nil, nil, nil, NewPlatformAPI(cfg, fixturePlatform{}))
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

type recordingBroadcaster struct{ kinds []string }

func (b *recordingBroadcaster) BroadcastRoom(string, string, any) {}
func (b *recordingBroadcaster) BroadcastAll(kind string, _ any)   { b.kinds = append(b.kinds, kind) }

// The desktop creates rooms over HTTP so a browser page is never required;
// private rooms must come back with the proof that lets the creator connect.
func TestDesktopRoomCreation(t *testing.T) {
	store := httpStore(t)
	insertStage5User(t, store, "creator-token", "creator", "MEMBER")
	cfg := testConfig(t)
	accounts := account.New(store)
	broadcaster := &recordingBroadcaster{}
	api := NewDesktopAPI(cfg, accounts, roomdomain.New(store, accounts), broadcaster, NewPlatformAPI(cfg, fixturePlatform{}))
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)

	post := func(token, csrf string, body string, withVersion bool) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/desktop/v1/rooms", strings.NewReader(body))
		if withVersion {
			request.Header.Set("X-Desktop-API-Version", desktopAPIVersion)
			request.Header.Set("X-Desktop-Client-Version", "0.2.0")
		}
		request.Header.Set("Content-Type", "application/json")
		if token != "" {
			request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
		}
		if csrf != "" {
			request.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
			request.Header.Set(CSRFHeaderName, csrf)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	created := func(body string) (roomdomain.Info, *httptest.ResponseRecorder) {
		t.Helper()
		response := post("creator-token", "creator-csrf", body, true)
		require.Equal(t, http.StatusOK, response.Code, response.Body.String())
		var room roomdomain.Info
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &room))
		require.True(t, strings.HasPrefix(room.RoomID, "room-"))
		require.Equal(t, "creator", room.CreatorPublicID)
		return room, response
	}

	require.Equal(t, http.StatusForbidden, post("creator-token", "", `{}`, true).Code)
	require.Equal(t, http.StatusUnauthorized, post("", "any-csrf", `{}`, true).Code)
	require.Equal(t, http.StatusUpgradeRequired, post("creator-token", "creator-csrf", `{}`, false).Code)

	publicRoom, publicResponse := created(`{"name":"Desktop Lounge"}`)
	require.False(t, publicRoom.PrivateRoom)
	require.True(t, publicRoom.AccessGranted)
	require.Empty(t, publicResponse.Result().Cookies())
	var role string
	require.NoError(t, store.Reader().QueryRowContext(context.Background(), "select role from room_membership where room_id=? and public_id='creator'", publicRoom.RoomID).Scan(&role))
	require.Equal(t, "OWNER", role)
	require.Equal(t, []string{"rooms.list"}, broadcaster.kinds)

	privateRoom, privateResponse := created(`{"name":"Desktop Private","isPrivate":true,"password":"hunter2"}`)
	require.True(t, privateRoom.PrivateRoom)
	proof := privateResponse.Result().Cookies()[0]
	require.Equal(t, RoomAccessCookieName, proof.Name)
	require.True(t, proof.HttpOnly)
	require.True(t, validRoomAccessToken(roomAccessSecret(cfg.Auth.RoomAccessTokenSecret), proof.Value, privateRoom.RoomID, "creator", 1, time.Now()))
	metadata, err := api.rooms.Access(context.Background(), privateRoom.RoomID)
	require.NoError(t, err)
	require.True(t, metadata.Private)
	require.Equal(t, 1, metadata.PasswordVersion)
	require.True(t, strings.HasPrefix(metadata.PasswordHash, "$2a$"))

	require.Equal(t, http.StatusConflict, post("creator-token", "creator-csrf", `{"name":"desktop lounge"}`, true).Code)
	require.Equal(t, http.StatusBadRequest, post("creator-token", "creator-csrf", `{"name":"   "}`, true).Code)
	require.Equal(t, http.StatusBadRequest, post("creator-token", "creator-csrf", `{"name":"Missing Secret","isPrivate":true}`, true).Code)
	require.Equal(t, broadcaster.kinds, []string{"rooms.list", "rooms.list"})
}

// R5-1 裁决 ②: opening a room takes a registered account, not just any session. Redeeming an
// invite is the only way a desktop user ever gets one, so that path has to stay open, and the
// refusal needs a code the shell can name instead of guessing from prose.
func TestDesktopRoomCreationRequiresRegisteredAccount(t *testing.T) {
	store := httpStore(t)
	insertStage5User(t, store, "admin-token", "seed-admin", "ADMIN")
	cfg := testConfig(t)
	accounts := account.New(store)
	rooms := roomdomain.New(store, accounts)
	api := NewDesktopAPI(cfg, accounts, rooms, &recordingBroadcaster{}, NewPlatformAPI(cfg, fixturePlatform{}))
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)

	post := func(token string, body string, withVersion bool) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/desktop/v1/rooms", strings.NewReader(body))
		request.Header.Set("X-Desktop-API-Version", desktopAPIVersion)
		request.Header.Set("X-Desktop-Client-Version", "0.2.0")
		if !withVersion {
			request.Header.Del("X-Desktop-API-Version")
		}
		request.Header.Set("Content-Type", "application/json")
		request.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "member-csrf"})
		request.Header.Set(CSRFHeaderName, "member-csrf")
		if token != "" {
			request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	seed, err := rooms.Create(context.Background(), "admin-token", roomdomain.CreateInput{Name: "Seed Room"})
	require.NoError(t, err)
	invite, err := rooms.CreateInvite(context.Background(), seed.RoomID, "admin-token", "desktop")
	require.NoError(t, err)
	member, err := accounts.RedeemInvite(context.Background(), invite.Secret, "受邀成员")
	require.NoError(t, err)
	require.False(t, member.Guest)

	memberResponse := post(member.SessionToken, `{"name":"Member Owned"}`, true)
	require.Equal(t, http.StatusOK, memberResponse.Code, memberResponse.Body.String())
	var created roomdomain.Info
	require.NoError(t, json.Unmarshal(memberResponse.Body.Bytes(), &created))
	require.Equal(t, member.PublicID, created.CreatorPublicID)

	// An administrator keeps the same privilege they always had.
	require.Equal(t, http.StatusOK, post("admin-token", `{"name":"Admin Owned"}`, true).Code)

	guest, err := accounts.CreateGuestSession(context.Background(), "访客建房")
	require.NoError(t, err)
	require.True(t, guest.Guest)
	refused := post(guest.SessionToken, `{"name":"Member Owned"}`, true)
	require.Equal(t, http.StatusForbidden, refused.Code)
	require.Contains(t, refused.Body.String(), `"code":"guest-not-allowed"`)

	// The version gate and the anonymous refusal run in front of the account check, unchanged.
	require.Equal(t, http.StatusUpgradeRequired, post(guest.SessionToken, `{}`, false).Code)
	anonymous := post("", `{"name":"Nobody"}`, true)
	require.Equal(t, http.StatusUnauthorized, anonymous.Code)
	require.Contains(t, anonymous.Body.String(), `"code":"unauthorized"`)

	var attempts int
	require.NoError(t, store.Reader().QueryRowContext(context.Background(),
		"select count(1) from room where name in ('Member Owned','Nobody') and deleted_at is null").Scan(&attempts))
	require.Equal(t, 1, attempts)
}

// The album view needs a page and the provider's own total, and it must sit behind the same gate
// as every other desktop media route: version first, session before any upstream call.
func TestDesktopAlbumRoutes(t *testing.T) {
	store := httpStore(t)
	insertStage5User(t, store, "album-token", "album-member", "MEMBER")
	cfg := testConfig(t)
	api := NewDesktopAPI(cfg, account.New(store), nil, nil, NewPlatformAPI(cfg, fixturePlatform{}))
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	get := func(path string, withVersion bool, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		if withVersion {
			request.Header.Set("X-Desktop-API-Version", desktopAPIVersion)
			request.Header.Set("X-Desktop-Client-Version", "0.2.0")
		}
		if token != "" {
			request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: token})
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}

	require.Equal(t, http.StatusUpgradeRequired, get("/api/desktop/v1/albums/local?q=zhou", false, "album-token").Code)
	require.Equal(t, http.StatusUnauthorized, get("/api/desktop/v1/albums/local?q=zhou", true, "").Code)
	require.Equal(t, http.StatusBadRequest, get("/api/desktop/v1/albums/local", true, "album-token").Code, "an empty keyword never reaches a provider")
	require.Equal(t, http.StatusBadRequest, get("/api/desktop/v1/albums/local?q=zhou&limit=0", true, "album-token").Code)
	require.Equal(t, http.StatusBadRequest, get("/api/desktop/v1/albums/local?q=zhou&offset=-1", true, "album-token").Code)
	require.Equal(t, http.StatusBadRequest, get("/api/desktop/v1/albums/local/"+strings.Repeat("a", 65)+"/songs", true, "album-token").Code)

	page := get("/api/desktop/v1/albums/local?q=zhou&offset=10&limit=5", true, "album-token")
	require.Equal(t, http.StatusOK, page.Code, page.Body.String())
	var albums struct {
		Items  []platform.Album `json:"items"`
		Total  int              `json:"total"`
		Offset int              `json:"offset"`
		Limit  int              `json:"limit"`
	}
	require.NoError(t, json.Unmarshal(page.Body.Bytes(), &albums))
	require.Equal(t, "album-10", albums.Items[0].ID, "the offset has to reach the provider")
	require.Equal(t, 10, albums.Offset)
	require.Equal(t, 5, albums.Limit)
	require.Equal(t, 42, albums.Total, "the provider's total passes through untouched")

	// The client's paging fallback keys off 0 meaning "this platform gives no count", so the route
	// must not quietly substitute the page length for a missing total.
	unknown := get("/api/desktop/v1/albums/local?q=no-total&offset=0&limit=5", true, "album-token")
	require.Equal(t, http.StatusOK, unknown.Code, unknown.Body.String())
	var uncounted struct {
		Items []platform.Album `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(unknown.Body.Bytes(), &uncounted))
	require.NotEmpty(t, uncounted.Items, "a page can hold albums while the platform reports no count")
	require.Zero(t, uncounted.Total, "0 means unknown and must survive the round trip")

	tracks := get("/api/desktop/v1/albums/local/album-10/songs", true, "album-token")
	require.Equal(t, http.StatusOK, tracks.Code, tracks.Body.String())
	var songs struct {
		Items []platform.Music `json:"items"`
		Total int              `json:"total"`
	}
	require.NoError(t, json.Unmarshal(tracks.Body.Bytes(), &songs))
	require.Equal(t, len(songs.Items), songs.Total, "an unpaged album reports its whole size")

	capabilities := httptest.NewRecorder()
	handler.ServeHTTP(capabilities, httptest.NewRequest(http.MethodGet, "/api/desktop/v1/capabilities", nil))
	var advertised struct {
		Features             map[string]bool `json:"features"`
		Providers            map[string]bool `json:"providers"`
		AlbumSearchProviders map[string]bool `json:"albumSearchProviders"`
	}
	require.NoError(t, json.Unmarshal(capabilities.Body.Bytes(), &advertised))
	require.True(t, advertised.Features["albumSearch"], "the shell must be able to see the endpoint exists")
	require.Empty(t, advertised.AlbumSearchProviders, "only configured providers are listed, and local is not one of the three")

	// A configured netease provider is the one that answers album queries.
	neteaseHandler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(),
		NewDesktopAPI(cfg, account.New(store), nil, nil, NewPlatformAPI(cfg, neteaseFixture{})))
	neteaseCapabilities := httptest.NewRecorder()
	neteaseHandler.ServeHTTP(neteaseCapabilities, httptest.NewRequest(http.MethodGet, "/api/desktop/v1/capabilities", nil))
	require.NoError(t, json.Unmarshal(neteaseCapabilities.Body.Bytes(), &advertised))
	require.Equal(t, map[string]bool{"netease": true}, advertised.AlbumSearchProviders)
	require.True(t, advertised.Providers["netease"])
	require.False(t, advertised.Providers["bilibili"], "an unconfigured provider stays listed but off")
}
