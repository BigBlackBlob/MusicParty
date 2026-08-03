package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/platform"
	storesqlite "github.com/BigBlackBlob/MusicParty/backend-go/internal/store/sqlite"
	"github.com/stretchr/testify/require"
)

type credentialFixture struct {
	fixturePlatform
	credential string
}

func (*credentialFixture) Name() string                    { return "netease" }
func (f *credentialFixture) UpdateCredential(value string) { f.credential = value }

func TestPlatformCredentialUpdatePersistsAndUpdatesRuntime(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	require.NoError(t, accounts.Bootstrap(context.Background(), "admin", "strong-password"))
	admin, err := accounts.Login(context.Background(), "admin", "strong-password")
	require.NoError(t, err)
	cfg := testConfig(t)
	service := &credentialFixture{}
	platformAPI := NewPlatformAPI(cfg, service)
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewAdminAPI(store, accounts, platform.NewClient(0), platformAPI))

	secret := "sensitive-cookie-value"
	request := httptest.NewRequest(http.MethodPost, "/api/admin/platform-credentials", strings.NewReader(`{"platform":"netease","credential":"`+secret+`"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(CSRFHeaderName, "csrf")
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: admin.SessionToken})
	request.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "csrf"})
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code)
	require.NotContains(t, response.Body.String(), secret)
	require.Equal(t, secret, service.credential)

	setting, err := storesqlite.NewSiteSettingRepository(store).Find(context.Background(), "netease.cookie")
	require.NoError(t, err)
	require.True(t, setting.Secret)
	require.NotNil(t, setting.Value)
	require.Equal(t, secret, *setting.Value)
}

func TestPlatformCredentialUpdateRejectsInvalidRequests(t *testing.T) {
	store := httpStore(t)
	accounts := account.New(store)
	cfg := testConfig(t)
	platformAPI := NewPlatformAPI(cfg, &credentialFixture{})
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), NewAdminAPI(store, accounts, platform.NewClient(0), platformAPI))

	for _, body := range []string{
		`{"platform":"unsupported","credential":"secret"}`,
		`{"platform":"netease","credential":""}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/admin/platform-credentials", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		require.Equal(t, http.StatusForbidden, response.Code)
	}
}
