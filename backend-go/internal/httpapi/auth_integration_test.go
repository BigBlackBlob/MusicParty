package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

func TestAccountCookieAndCSRFFlow(t *testing.T) {
	store := httpStore(t)
	service := account.New(store)
	require.NoError(t, service.Bootstrap(context.Background(), "admin", "strong-password"))
	cfg := testConfig(t)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewAuthAPI(cfg, service))
	login := httptest.NewRequest(http.MethodPost, "/api/account/login", strings.NewReader(`{"username":"admin","password":"strong-password"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	require.Equal(t, 200, loginResponse.Code)
	cookies := loginResponse.Result().Cookies()
	require.Len(t, cookies, 2)
	byName := map[string]*http.Cookie{}
	for _, cookie := range cookies {
		byName[cookie.Name] = cookie
	}
	require.True(t, byName[SessionCookieName].HttpOnly)
	require.False(t, byName[CSRFCookieName].HttpOnly)
	me := httptest.NewRequest(http.MethodGet, "/api/account/me", nil)
	me.AddCookie(byName[SessionCookieName])
	meResponse := httptest.NewRecorder()
	handler.ServeHTTP(meResponse, me)
	require.Equal(t, 200, meResponse.Code)
	profile := httptest.NewRequest(http.MethodPut, "/api/account/profile", strings.NewReader(`{"displayName":"新名称"}`))
	profile.AddCookie(byName[SessionCookieName])
	profile.AddCookie(byName[CSRFCookieName])
	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, profile)
	require.Equal(t, 403, missing.Code)
	profile = httptest.NewRequest(http.MethodPut, "/api/account/profile", strings.NewReader(`{"displayName":"新名称"}`))
	profile.AddCookie(byName[SessionCookieName])
	profile.AddCookie(byName[CSRFCookieName])
	profile.Header.Set(CSRFHeaderName, byName[CSRFCookieName].Value)
	updated := httptest.NewRecorder()
	handler.ServeHTTP(updated, profile)
	require.Equal(t, 200, updated.Code)
	require.Contains(t, updated.Body.String(), "新名称")
}

type sessionCloserSpy struct{ tokens []string }

func (s *sessionCloserSpy) CloseSession(token string) { s.tokens = append(s.tokens, token) }

func TestLogoutRevokesRealtimeSession(t *testing.T) {
	store := httpStore(t)
	service := account.New(store)
	require.NoError(t, service.Bootstrap(context.Background(), "admin", "strong-password"))
	cfg := testConfig(t)
	api := NewAuthAPI(cfg, service)
	closer := &sessionCloserSpy{}
	api.SetSessionCloser(closer)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)

	login := httptest.NewRequest(http.MethodPost, "/api/account/login", strings.NewReader(`{"username":"admin","password":"strong-password"}`))
	login.Header.Set("Content-Type", "application/json")
	loginResponse := httptest.NewRecorder()
	handler.ServeHTTP(loginResponse, login)
	require.Equal(t, http.StatusOK, loginResponse.Code)
	cookies := map[string]*http.Cookie{}
	for _, cookie := range loginResponse.Result().Cookies() {
		cookies[cookie.Name] = cookie
	}

	logout := httptest.NewRequest(http.MethodPost, "/api/account/logout", nil)
	logout.AddCookie(cookies[SessionCookieName])
	logout.AddCookie(cookies[CSRFCookieName])
	logout.Header.Set(CSRFHeaderName, cookies[CSRFCookieName].Value)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, logout)

	require.Equal(t, http.StatusNoContent, response.Code)
	require.Equal(t, []string{cookies[SessionCookieName].Value}, closer.tokens)
}
func httpStore(t *testing.T) *storesqlite.Store {
	t.Helper()
	store, err := storesqlite.OpenStore(context.Background(), storesqlite.StoreConfig{Path: filepath.Join(t.TempDir(), "http.db"), BusyTimeout: time.Second, ReadConnections: 2})
	require.NoError(t, err)
	require.NoError(t, storesqlite.EnsureCompatibleSchema(context.Background(), store, true))
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	return store
}
