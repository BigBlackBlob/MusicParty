package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/stretchr/testify/require"
)

// readinessFixture mirrors the wire contract so the tests assert on JSON keys,
// not on Go field names.
type readinessFixture struct {
	Status               string `json:"status"`
	Service              string `json:"service"`
	APIVersion           string `json:"apiVersion"`
	ServerVersion        string `json:"serverVersion"`
	MinimumClientVersion string `json:"minimumClientVersion"`
	ReadinessVersion     int    `json:"readinessVersion"`
	BootID               string `json:"bootId"`
	StartedAt            int64  `json:"startedAt"`
	CheckedAt            int64  `json:"checkedAt"`
	Listener             struct {
		Port int `json:"port"`
	} `json:"listener"`
	Cache struct {
		TTLMS     int64 `json:"ttlMs"`
		AgeMS     int64 `json:"ageMs"`
		FromCache bool  `json:"fromCache"`
	} `json:"cache"`
	Config struct {
		NeteaseAPI struct {
			URL string `json:"url"`
		} `json:"neteaseApi"`
	} `json:"config"`
	Components struct {
		Core       readinessComponent `json:"core"`
		NeteaseAPI readinessComponent `json:"neteaseApi"`
	} `json:"components"`
	Diagnostics []readinessDiagnostic `json:"diagnostics"`
}

func loadReadinessConfig(t *testing.T, values map[string]string) config.Config {
	t.Helper()
	if values == nil {
		values = map[string]string{}
	}
	values["DB_ENABLED"] = "false"
	values["AUTH_SECURE_COOKIES"] = "false"
	cfg, err := config.Load(func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	})
	require.NoError(t, err)
	cfg.Application.AllowedOrigins = []string{}
	cfg.Auth.TrustedProxyCIDRs = []string{}
	return cfg
}

func newReadinessHandler(t *testing.T, cfg config.Config) (http.Handler, *DesktopAPI) {
	t.Helper()
	api := NewDesktopAPI(cfg, nil, nil, nil, NewPlatformAPI(cfg, fixturePlatform{}))
	return NewHandler(cfg, slog.Default(), observability.NewHealth(), observability.NewMetrics(), api), api
}

func requestReadiness(t *testing.T, handler http.Handler) (int, readinessFixture, string) {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/desktop/v1/readiness", nil))
	var body readinessFixture
	raw := response.Body.String()
	require.NoError(t, json.Unmarshal([]byte(raw), &body), raw)
	return response.Code, body, raw
}

// readReadiness is the non-asserting form: testify's FailNow cannot run on a
// helper goroutine, so concurrent probes collect their body and assert later.
func readReadiness(handler http.Handler) (readinessFixture, error) {
	var body readinessFixture
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/desktop/v1/readiness", nil))
	if response.Code != http.StatusOK {
		return body, errStatus(response.Code)
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		return body, err
	}
	return body, nil
}

type errStatus int

func (e errStatus) Error() string { return "unexpected readiness status " + http.StatusText(int(e)) }

func TestDesktopReadinessIsPublicAndReportsReachableMediaAPI(t *testing.T) {
	var hits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	cfg := loadReadinessConfig(t, map[string]string{"NETEASE_API_URL": upstream.URL})
	handler, _ := newReadinessHandler(t, cfg)
	status, body, raw := requestReadiness(t, handler)

	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "ready", body.Status)
	require.Equal(t, "musicparty", body.Service)
	require.Equal(t, 1, body.ReadinessVersion)
	require.Equal(t, desktopAPIVersion, body.APIVersion)
	require.Equal(t, desktopServerVersion, body.ServerVersion)
	require.NotEmpty(t, body.BootID)
	require.Positive(t, body.StartedAt)
	require.Equal(t, cfg.Server.Port, body.Listener.Port)
	require.Equal(t, int64(5000), body.Cache.TTLMS)
	require.False(t, body.Cache.FromCache)
	require.Equal(t, "explicit", body.Config.NeteaseAPI.URL)
	require.Equal(t, int64(1), hits.Load())
	require.Equal(t, "up", body.Components.NeteaseAPI.Status)
	require.Equal(t, "NETEASE_OK", body.Components.NeteaseAPI.Code)
	require.Empty(t, body.Diagnostics)
	require.Equal(t, "up", body.Components.Core.Status)
	// core is deliberately {status:"up"} only, matching Node field for field.
	require.NotContains(t, raw, "CORE_OK")
	require.Regexp(t, `"core":\{"status":"up"\}`, raw)
	// Units are part of the contract: epoch/elapsed milliseconds as JSON
	// numbers. A string here silently fails serde on the shell and the banner
	// falls back to its single line, so it is pinned on the raw text.
	require.Regexp(t, `"startedAt":[0-9]+`, raw)
	require.Regexp(t, `"checkedAt":[0-9]+`, raw)
	require.Regexp(t, `"ttlMs":[0-9]+`, raw)
	require.Regexp(t, `"latencyMs":[0-9]+`, raw)
	require.Equal(t, int64(5000), body.Cache.TTLMS)
	// Pinned on the raw text: Go decodes JSON keys case-insensitively, so only
	// the wire form can catch a field name that drifted to "URL".
	require.Contains(t, raw, `"config":{"neteaseApi":{"url":"explicit"}}`)
	// The upstream address is configuration, not something to echo back.
	require.NotContains(t, raw, upstream.URL)
}

func TestDesktopReadinessDegradesWhenReachableMediaAPIFails(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer upstream.Close()

	cfg := loadReadinessConfig(t, map[string]string{"NETEASE_API_URL": upstream.URL})
	handler, _ := newReadinessHandler(t, cfg)
	status, body, _ := requestReadiness(t, handler)

	require.Equal(t, http.StatusOK, status)
	require.Equal(t, "degraded", body.Status)
	require.Equal(t, "down", body.Components.NeteaseAPI.Status)
	require.Equal(t, "NETEASE_BAD_RESPONSE", body.Components.NeteaseAPI.Code)
	require.Equal(t, "CHECK_NETEASE_URL", body.Components.NeteaseAPI.Remediation)
	require.Equal(t, []readinessDiagnostic{{
		Severity:    "error",
		Code:        "NETEASE_BAD_RESPONSE",
		Message:     "the media API reported a server error",
		Remediation: "CHECK_NETEASE_URL",
	}}, body.Diagnostics)
	// A configured url must never be reported as a provisioning gap.
	require.Equal(t, "explicit", body.Config.NeteaseAPI.URL)
	for _, diagnostic := range body.Diagnostics {
		require.NotEqual(t, "NETEASE_URL_NOT_CONFIGURED", diagnostic.Code)
	}
}

func TestDesktopReadinessUnreachableMediaAPIKeepsTransportDiagnosticOnly(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	cfg := loadReadinessConfig(t, map[string]string{"NETEASE_API_URL": upstream.URL})
	upstream.Close()

	handler, _ := newReadinessHandler(t, cfg)
	_, body, _ := requestReadiness(t, handler)

	require.Equal(t, "degraded", body.Status)
	require.Equal(t, "NETEASE_UNREACHABLE", body.Components.NeteaseAPI.Code)
	require.Equal(t, []readinessDiagnostic{{
		Severity:    "error",
		Code:        "NETEASE_UNREACHABLE",
		Message:     "the media API did not answer",
		Remediation: "CHECK_NETEASE_URL",
	}}, body.Diagnostics)
}

// A defaulted NETEASE_API_URL is the compose-only container address, which
// cannot resolve on a desktop host: the wizard needs to be told to set it.
func TestDesktopReadinessNamesProvisioningGapBeforeTransportFault(t *testing.T) {
	cfg := loadReadinessConfig(t, nil)
	require.Equal(t, config.ProvisionDefault, cfg.Status.Netease.URL)
	handler, _ := newReadinessHandler(t, cfg)
	_, body, raw := requestReadiness(t, handler)

	require.Equal(t, "degraded", body.Status)
	require.Equal(t, "default", body.Config.NeteaseAPI.URL)
	require.Len(t, body.Diagnostics, 2)
	// Invariant: a provisioning code lives in diagnostics[] only. Surfacing it
	// through components.*.code would make a healthy backend look unreachable.
	require.Equal(t, 1, strings.Count(raw, "NOT_CONFIGURED"), `NETEASE_URL_NOT_CONFIGURED must appear once, inside diagnostics`)
	require.Equal(t, readinessDiagnostic{
		Severity:    "error",
		Code:        "NETEASE_URL_NOT_CONFIGURED",
		Message:     "NETEASE_API_URL is unset; the backend is probing its built-in container-network address",
		Remediation: "NETEASE_URL_SETUP",
	}, body.Diagnostics[0])
	// The transport half depends on the host network (the container hostname may
	// refuse, never resolve, or be answered by a proxy), so only the pairing and
	// ordering is asserted here.
	require.Contains(t, []string{"NETEASE_UNREACHABLE", "NETEASE_TIMEOUT", "NETEASE_BAD_RESPONSE"}, body.Diagnostics[1].Code)
	require.NotContains(t, raw, "netease-api:3000")
}

func TestDesktopReadinessBlankMediaAPIURLIsReportedAsMissing(t *testing.T) {
	cfg := loadReadinessConfig(t, map[string]string{"NETEASE_API_URL": ""})
	require.Equal(t, config.ProvisionMissing, cfg.Status.Netease.URL)
	handler, _ := newReadinessHandler(t, cfg)
	_, body, _ := requestReadiness(t, handler)

	require.Equal(t, "degraded", body.Status)
	require.Equal(t, "missing", body.Config.NeteaseAPI.URL)
	require.Equal(t, "NETEASE_UNREACHABLE", body.Components.NeteaseAPI.Code)
	require.Equal(t, "NETEASE_URL_NOT_CONFIGURED", body.Diagnostics[0].Code)
	require.Contains(t, body.Diagnostics[0].Message, "no value")
}

func TestDesktopReadinessSlowMediaAPITimesOut(t *testing.T) {
	release := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	defer func() {
		close(release)
		upstream.Close()
	}()

	cfg := loadReadinessConfig(t, map[string]string{"NETEASE_API_URL": upstream.URL})
	handler, api := newReadinessHandler(t, cfg)
	api.readiness.timeout = 5 * time.Millisecond
	_, body, _ := requestReadiness(t, handler)

	require.Equal(t, "degraded", body.Status)
	require.Equal(t, "NETEASE_TIMEOUT", body.Components.NeteaseAPI.Code)
	require.Equal(t, "CHECK_NETEASE_TIMEOUT", body.Components.NeteaseAPI.Remediation)
}

func TestDesktopReadinessCachesAndSharesOneProbe(t *testing.T) {
	var hits atomic.Int64
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	cfg := loadReadinessConfig(t, map[string]string{"NETEASE_API_URL": upstream.URL})
	handler, api := newReadinessHandler(t, cfg)

	first := make([]readinessFixture, 6)
	errs := make([]error, 6)
	var group sync.WaitGroup
	for i := range first {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			first[index], errs[index] = readReadiness(handler)
		}(i)
	}
	group.Wait()

	for index, err := range errs {
		require.NoError(t, err, "concurrent probe %d", index)
	}
	for _, body := range first {
		require.Equal(t, "ready", body.Status)
		require.Equal(t, first[0].CheckedAt, body.CheckedAt)
		require.Equal(t, first[0].BootID, body.BootID)
	}
	require.Equal(t, int64(1), hits.Load(), "concurrent probes must share one upstream request")

	_, cached, _ := requestReadiness(t, handler)
	require.True(t, cached.Cache.FromCache)
	require.Equal(t, first[0].CheckedAt, cached.CheckedAt)
	require.Equal(t, int64(1), hits.Load())

	// After the TTL expires the next call re-probes.
	api.readiness.ttl = time.Millisecond
	time.Sleep(5 * time.Millisecond)
	_, refreshed, _ := requestReadiness(t, handler)
	require.False(t, refreshed.Cache.FromCache)
	require.Equal(t, int64(2), hits.Load())
	require.Greater(t, refreshed.CheckedAt, first[0].CheckedAt)
}

func TestDesktopCapabilitiesAdvertiseReadiness(t *testing.T) {
	cfg := loadReadinessConfig(t, nil)
	handler, _ := newReadinessHandler(t, cfg)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/desktop/v1/capabilities", nil))
	require.Equal(t, http.StatusOK, response.Code)
	var body struct {
		Features map[string]bool `json:"features"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
	require.True(t, body.Features["readiness"])
}
