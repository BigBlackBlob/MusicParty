package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

// Desktop readiness (readinessVersion 1) is the Go counterpart of Banguru's
// GET /api/desktop/readiness. Same rules: the endpoint is credential-free, always
// answers 200 while the process is up, and carries dependency health in
// status/components/diagnostics. Only stable codes cross the wire - never an
// upstream URL, error text, cookie, or token.
//
// Contract invariants (shared with Node; the desktop shell parses one struct for
// both, so these are load-bearing and are asserted in the tests):
//  1. Every timestamp and duration (startedAt, checkedAt, latencyMs, ttlMs,
//     ageMs) is a JSON number of epoch/millisecond units, never an ISO string.
//  2. components.*.code only ever holds a transport/health code. The
//     provisioning code (NETEASE_URL_NOT_CONFIGURED) appears in diagnostics[]
//     and nowhere else - a caller that read it off the component would mark a
//     perfectly healthy backend as unavailable.
//  3. diagnostics[] lists provisioning gaps before the transport code, and the
//     transport code repeats there deliberately, so a caller can act on
//     diagnostics[0] alone.
//  4. New fields are not breaking changes; consumers must tolerate unknown
//     keys. Breaking changes go through readinessVersion.
const (
	desktopReadinessVersion      = 1
	desktopReadinessService      = "musicparty"
	desktopReadinessCacheTTL     = 5 * time.Second
	desktopReadinessProbeTimeout = 2 * time.Second
	desktopReadinessFlightKey    = "core"
)

type readinessComponent struct {
	Status      string `json:"status"`
	Code        string `json:"code"`
	LatencyMS   int64  `json:"latencyMs"`
	Message     string `json:"message,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// readinessCoreComponent is deliberately narrower than readinessComponent:
// "core is up" is proven by the fact that this document was produced at all, so
// there is no code to report and no latency to measure. Node emits the same
// minimal shape, which is what lets one desktop struct read both dialects.
type readinessCoreComponent struct {
	Status string `json:"status"`
}

type readinessComponents struct {
	Core       readinessCoreComponent `json:"core"`
	NeteaseAPI readinessComponent     `json:"neteaseApi"`
}

type readinessDiagnostic struct {
	Severity    string `json:"severity"`
	Code        string `json:"code"`
	Message     string `json:"message"`
	Remediation string `json:"remediation,omitempty"`
}

type readinessCacheInfo struct {
	TTLMS     int64 `json:"ttlMs"`
	AgeMS     int64 `json:"ageMs"`
	FromCache bool  `json:"fromCache"`
}

type readinessListener struct {
	Port int `json:"port"`
}

type readinessConfig struct {
	NeteaseAPI config.NeteaseStatus `json:"neteaseApi"`
}

type readinessSnapshot struct {
	Status               string                `json:"status"`
	Service              string                `json:"service"`
	APIVersion           string                `json:"apiVersion"`
	ServerVersion        string                `json:"serverVersion"`
	MinimumClientVersion string                `json:"minimumClientVersion"`
	ReadinessVersion     int                   `json:"readinessVersion"`
	BootID               string                `json:"bootId"`
	StartedAt            int64                 `json:"startedAt"`
	CheckedAt            int64                 `json:"checkedAt"`
	Listener             readinessListener     `json:"listener"`
	Cache                readinessCacheInfo    `json:"cache"`
	Config               readinessConfig       `json:"config"`
	Components           readinessComponents   `json:"components"`
	Diagnostics          []readinessDiagnostic `json:"diagnostics"`
}

// probedCore is the cached part of a snapshot: everything except per-request
// framing (cache age, listener, boot identity).
type probedCore struct {
	checkedAt   time.Time
	netease     readinessComponent
	diagnostics []readinessDiagnostic
}

// desktopReadinessProbe re-probes the media API at most once per TTL and shares
// one in-flight probe between concurrent callers, so a polling setup wizard
// cannot stampede netease-api.
type desktopReadinessProbe struct {
	client    *http.Client
	ttl       time.Duration
	timeout   time.Duration
	bootID    string
	startedAt time.Time
	group     singleflight.Group
	mu        sync.Mutex
	cached    *probedCore
}

func newDesktopReadinessProbe(client *http.Client) *desktopReadinessProbe {
	if client == nil {
		// One request per TTL window, so keep-alives buy nothing and would only
		// leave idle connections (and their read loops) outliving the probe.
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DisableKeepAlives = true
		client = &http.Client{Transport: transport}
	}
	return &desktopReadinessProbe{
		client:    client,
		ttl:       desktopReadinessCacheTTL,
		timeout:   desktopReadinessProbeTimeout,
		bootID:    uuid.NewString(),
		startedAt: time.Now(),
	}
}

func (p *desktopReadinessProbe) core(cfg config.Config) (*probedCore, bool) {
	p.mu.Lock()
	cached := p.cached
	p.mu.Unlock()
	if cached != nil && time.Since(cached.checkedAt) < p.ttl {
		return cached, true
	}
	value, _, _ := p.group.Do(desktopReadinessFlightKey, func() (any, error) {
		// A sibling may have completed a fresh probe while this call was queued.
		p.mu.Lock()
		fresh := p.cached
		p.mu.Unlock()
		if fresh != nil && time.Since(fresh.checkedAt) < p.ttl {
			return fresh, nil
		}
		core := &probedCore{checkedAt: time.Now(), netease: p.probeNetease(cfg)}
		core.diagnostics = append(core.diagnostics, neteaseConfigDiagnostics(cfg, core.netease)...)
		p.mu.Lock()
		p.cached = core
		p.mu.Unlock()
		return core, nil
	})
	return value.(*probedCore), false
}

// probeNetease answers GET on the configured media API root. Any status below
// 500 counts as reachable: netease-api has no health route, and a 404 on / still
// proves the sidecar process is serving HTTP.
func (p *desktopReadinessProbe) probeNetease(cfg config.Config) readinessComponent {
	base := strings.TrimRight(cfg.Platforms.Netease.BaseURL, "/")
	if base == "" {
		return readinessComponent{
			Status:      "down",
			Code:        "NETEASE_UNREACHABLE",
			Remediation: "CHECK_NETEASE_URL",
			Message:     "the backend has no media API address configured",
		}
	}
	component := readinessComponent{Status: "down", Code: "NETEASE_UNREACHABLE", Remediation: "CHECK_NETEASE_URL"}
	ctx, cancel := context.WithTimeout(context.Background(), p.timeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, base, nil)
	if err != nil {
		component.Message = "the configured media API address could not be requested"
		return component
	}
	started := time.Now()
	response, err := p.client.Do(request)
	component.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		// The SDK error text embeds the URL, so it is never surfaced.
		component.Message = "the media API did not answer"
		if errors.Is(err, context.DeadlineExceeded) {
			component.Code = "NETEASE_TIMEOUT"
			component.Remediation = "CHECK_NETEASE_TIMEOUT"
		}
		return component
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode >= http.StatusInternalServerError {
		component.Code = "NETEASE_BAD_RESPONSE"
		component.Message = "the media API reported a server error"
		return component
	}
	return readinessComponent{Status: "up", Code: "NETEASE_OK", LatencyMS: component.LatencyMS}
}

// neteaseConfigDiagnostics explains an unreachable media source the way the
// setup wizard needs: a defaulted or blank NETEASE_API_URL is a provisioning
// gap, not a network fault. Emitted only on the failure path so a working
// deployment never nags.
func neteaseConfigDiagnostics(cfg config.Config, netease readinessComponent) []readinessDiagnostic {
	if netease.Status == "up" {
		return nil
	}
	var diagnostics []readinessDiagnostic
	switch cfg.Status.Netease.URL {
	case config.ProvisionDefault:
		diagnostics = append(diagnostics, readinessDiagnostic{
			Severity:    "error",
			Code:        "NETEASE_URL_NOT_CONFIGURED",
			Message:     "NETEASE_API_URL is unset; the backend is probing its built-in container-network address",
			Remediation: "NETEASE_URL_SETUP",
		})
	case config.ProvisionMissing:
		diagnostics = append(diagnostics, readinessDiagnostic{
			Severity:    "error",
			Code:        "NETEASE_URL_NOT_CONFIGURED",
			Message:     "NETEASE_API_URL has no value, so the backend has no media API address",
			Remediation: "NETEASE_URL_SETUP",
		})
	}
	return diagnostics
}

func (api *DesktopAPI) readinessSnapshot(w http.ResponseWriter, _ *http.Request) {
	core, fromCache := api.readiness.core(api.cfg)
	status := "ready"
	// Built per request: `core` is shared across callers and must never be
	// appended to in place.
	diagnostics := []readinessDiagnostic{}
	if core.netease.Status != "up" {
		status = "degraded"
		diagnostics = append(diagnostics, core.diagnostics...)
		diagnostics = append(diagnostics, readinessDiagnostic{
			Severity:    "error",
			Code:        core.netease.Code,
			Message:     core.netease.Message,
			Remediation: core.netease.Remediation,
		})
	}
	writeJSON(w, readinessSnapshot{
		Status:               status,
		Service:              desktopReadinessService,
		APIVersion:           desktopAPIVersion,
		ServerVersion:        desktopServerVersion,
		MinimumClientVersion: desktopMinimumClientVersion,
		ReadinessVersion:     desktopReadinessVersion,
		BootID:               api.readiness.bootID,
		StartedAt:            api.readiness.startedAt.UnixMilli(),
		CheckedAt:            core.checkedAt.UnixMilli(),
		Listener:             readinessListener{Port: api.cfg.Server.Port},
		Cache: readinessCacheInfo{
			TTLMS:     api.readiness.ttl.Milliseconds(),
			AgeMS:     time.Since(core.checkedAt).Milliseconds(),
			FromCache: fromCache,
		},
		Config: readinessConfig{NeteaseAPI: api.cfg.Status.Netease},
		Components: readinessComponents{
			Core:       readinessCoreComponent{Status: "up"},
			NeteaseAPI: core.netease,
		},
		Diagnostics: diagnostics,
	})
}
