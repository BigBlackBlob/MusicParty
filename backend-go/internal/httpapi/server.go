package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/BigBlackBlob/MusicParty/backend-go/internal/config"
	"github.com/BigBlackBlob/MusicParty/backend-go/internal/observability"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewHandler(cfg config.Config, logger *slog.Logger, health *observability.Health, metrics *observability.Metrics, routeAPIs ...interface{ Routes(chi.Router) }) http.Handler {
	router := chi.NewRouter()
	router.Use(RequestID)
	router.Use(TrustedProxyClientIP(cfg.Auth.TrustedProxyCIDRs))
	router.Use(Recover(logger))
	router.Use(AccessLog(logger, metrics))
	router.Use(CORS(cfg.Application.AllowedOrigins))
	router.Use(CSRF(map[string]struct{}{"/api/account/login": {}, "/api/admin/auth/login": {}, "/api/invites/redeem": {}}))

	router.Get("/actuator/health", health.Actuator)
	router.Get("/actuator/health/liveness", health.Liveness)
	router.Get("/actuator/health/readiness", health.Readiness)
	router.Get("/actuator/prometheus", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}).ServeHTTP)
	router.Get("/actuator/metrics", metrics.ActuatorNames)
	router.Get("/actuator/metrics/{metricName}", metrics.ActuatorMetric)
	router.Get("/actuator/info", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{})
	})
	for _, api := range routeAPIs {
		if api != nil {
			api.Routes(router)
		}
	}
	router.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "NOT_FOUND", "Not found")
	})
	return router
}
