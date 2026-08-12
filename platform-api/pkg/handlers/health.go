package handlers

import (
	"net/http"
	"sync/atomic"

	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/api"
)

// HealthHandler handles health check endpoints
// TODO: add a logger field so write errors can be logged.
type HealthHandler struct {
	ready *atomic.Bool
}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler() *HealthHandler {
	ready := &atomic.Bool{}
	ready.Store(true)
	return &HealthHandler{
		ready: ready,
	}
}

// SetReady sets the readiness state
func (h *HealthHandler) SetReady(ready bool) {
	h.ready.Store(ready)
}

// Liveness handles GET /live
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	_ = api.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readiness handles GET /ready
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if !h.ready.Load() {
		_ = api.Write(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"})
		return
	}

	_ = api.Write(w, http.StatusOK, map[string]string{"status": "ok"})
}
