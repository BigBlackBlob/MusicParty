package httpapi

import (
	"context"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/domain/account"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBatch5LimiterDoesNotAdmitUntrackedIPsAtCapacity(t *testing.T) {
	cfg := testConfig(t)
	cfg.Auth.MaxTrackedIPs = 1
	cfg.Auth.MaxAttempts = 1
	limiter := newLoginLimiter(cfg)
	_, _ = limiter.admit("first")
	_, _ = limiter.admit("second")
	_, blocked := limiter.admit("second")
	require.True(t, blocked)
}

func TestBatch5ConcurrentAttemptsReserveBeforeWork(t *testing.T) {
	cfg := testConfig(t)
	cfg.Auth.MaxAttempts = 3
	limiter := newLoginLimiter(cfg)
	results := make(chan bool, 20)
	var workers sync.WaitGroup
	for range 20 {
		workers.Go(func() { _, blocked := limiter.admit("same-ip"); results <- blocked })
	}
	workers.Wait()
	close(results)
	admitted := 0
	for blocked := range results {
		if !blocked {
			admitted++
		}
	}
	require.Equal(t, 3, admitted)
}

func TestBatch5GuestAdmissionExpiresAndBoundsTrackedSources(t *testing.T) {
	store := httpStore(t)
	cfg := testConfig(t)
	cfg.Auth.MaxAttempts = 2
	cfg.Auth.MaxTrackedIPs = 1
	api := NewAuthAPI(cfg, account.New(store))
	now := time.Now()
	api.guestLimiter.now = func() time.Time { return now }
	handler := NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api)
	create := func(ip string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodPost, "/api/account/guest", strings.NewReader(`{"displayName":"Guest"}`))
		request.RemoteAddr = ip + ":1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	require.Equal(t, 200, create("192.0.2.1").Code)
	require.Equal(t, 200, create("192.0.2.1").Code)
	blocked := create("192.0.2.1")
	require.Equal(t, 429, blocked.Code)
	require.NotEmpty(t, blocked.Header().Get("Retry-After"))
	require.Equal(t, 429, create("192.0.2.2").Code)
	now = now.Add(cfg.Auth.BlockDuration + cfg.Auth.Window)
	require.Equal(t, 200, create("192.0.2.2").Code)
	var count int
	require.NoError(t, store.Reader().QueryRowContext(context.Background(), "select count(*) from user_profile").Scan(&count))
	require.Equal(t, 3, count)
	require.Len(t, api.guestLimiter.items, 1)
}

func TestBatch5ProxyChainDropsOnlyTrustedHops(t *testing.T) {
	handler := TrustedProxyClientIP([]string{"10.0.0.0/8"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(ClientIP(r))) }))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Set("X-Forwarded-For", "spoofed, 203.0.113.9, 10.0.0.1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	require.Equal(t, "203.0.113.9", response.Body.String())
}
