package observability

import (
	"encoding/json"
	"net/http"
	"sync/atomic"
)

type Health struct {
	ready atomic.Bool
}

func NewHealth() *Health { return &Health{} }

func (h *Health) SetReady(ready bool) { h.ready.Store(ready) }

func (h *Health) Actuator(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/vnd.spring-boot.actuator.v3+json")
	writeJSON(w, http.StatusOK, struct {
		Status string   `json:"status"`
		Groups []string `json:"groups"`
	}{Status: "UP", Groups: []string{"liveness", "readiness"}})
}

func (h *Health) Liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "UP"})
}

func (h *Health) Readiness(w http.ResponseWriter, _ *http.Request) {
	if !h.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "OUT_OF_SERVICE"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "UP"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	if w.Header().Get("Content-Type") == "" {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
