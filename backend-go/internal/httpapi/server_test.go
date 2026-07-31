package httpapi

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
)

func testHandler(t *testing.T, origins ...string) (*observability.Health, http.Handler) {
	t.Helper()
	cfg, err := config.Load(func(name string) (string, bool) {
		if name == "ALLOWED_ORIGINS" {
			return strings.Join(origins, ","), len(origins) > 0
		}
		if name == "BASE_URL" && len(origins) > 0 {
			return origins[0], true
		}
		return "", false
	})
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	health := observability.NewHealth()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return health, NewHandler(cfg, logger, health, observability.NewMetrics())
}

func TestHealthCompatibility(t *testing.T) {
	health, handler := testHandler(t)
	health.SetReady(true)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/actuator/health", nil))
	if response.Code != http.StatusOK || strings.TrimSpace(response.Body.String()) != `{"status":"UP","groups":["liveness","readiness"]}` {
		t.Fatalf("health response = %d %s", response.Code, response.Body.String())
	}
	if response.Header().Get(requestIDHeader) == "" {
		t.Fatal("health response missing request ID")
	}
}

func TestReadinessStartsUnavailable(t *testing.T) {
	_, handler := testHandler(t)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/actuator/health/readiness", nil))
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("readiness status = %d, want 503", response.Code)
	}
}

func TestActuatorMetricsDiscoveryAndDetail(t *testing.T) {
	health, handler := testHandler(t)
	health.SetReady(true)
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/actuator/health", nil))

	discovery := httptest.NewRecorder()
	handler.ServeHTTP(discovery, httptest.NewRequest(http.MethodGet, "/actuator/metrics", nil))
	if discovery.Code != http.StatusOK || !strings.Contains(discovery.Body.String(), "http.server.requests") {
		t.Fatalf("metrics discovery = %d %s", discovery.Code, discovery.Body.String())
	}

	detail := httptest.NewRecorder()
	handler.ServeHTTP(detail, httptest.NewRequest(http.MethodGet, "/actuator/metrics/http.server.requests", nil))
	if detail.Code != http.StatusOK || !strings.Contains(detail.Body.String(), `"name":"http.server.requests"`) {
		t.Fatalf("metric detail = %d %s", detail.Code, detail.Body.String())
	}
}

func TestCORSMatchesJavaMediaMapping(t *testing.T) {
	_, handler := testHandler(t, "https://music.example")
	request := httptest.NewRequest(http.MethodOptions, "/media/example.ogg", nil)
	request.Header.Set("Origin", "https://any.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || response.Header().Get("Access-Control-Allow-Origin") != "*" || response.Header().Get("Access-Control-Allow-Credentials") != "" {
		t.Fatalf("CORS response = %d headers=%v", response.Code, response.Header())
	}
}

func TestAPIDoesNotGainCrossOriginAccess(t *testing.T) {
	_, handler := testHandler(t, "https://music.example")
	request := httptest.NewRequest(http.MethodOptions, "/api/example", nil)
	request.Header.Set("Origin", "https://music.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatalf("API unexpectedly returned CORS headers: %v", response.Header())
	}
}

func TestCSRFRejectsMismatchedToken(t *testing.T) {
	_, handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/example", nil)
	request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session"})
	request.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: "cookie-token"})
	request.Header.Set("X-CSRF-Token", "header-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("CSRF status = %d, want 403", response.Code)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("CSRF body = %q, want empty", response.Body.String())
	}
}

func TestTrustedProxyClientIP(t *testing.T) {
	middleware := TrustedProxyClientIP([]string{"10.0.0.0/8"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(ClientIP(r)))
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.2")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Body.String() != "203.0.113.9" {
		t.Fatalf("client IP = %q", response.Body.String())
	}
}

func TestTrustedProxyFallsBackToRealIP(t *testing.T) {
	middleware := TrustedProxyClientIP([]string{"10.0.0.0/8"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(ClientIP(r)))
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "10.0.0.2:1234"
	request.Header.Set("X-Forwarded-For", "unknown")
	request.Header.Set("X-Real-IP", "203.0.113.10")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Body.String() != "203.0.113.10" {
		t.Fatalf("client IP = %q", response.Body.String())
	}
}

func TestUntrustedPeerCannotSpoofClientIP(t *testing.T) {
	middleware := TrustedProxyClientIP([]string{"10.0.0.0/8"})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(ClientIP(r)))
	}))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.RemoteAddr = "192.0.2.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.9")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Body.String() != "192.0.2.1" {
		t.Fatalf("client IP = %q", response.Body.String())
	}
}
