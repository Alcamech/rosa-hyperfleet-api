package handlers

import (
	"log/slog"
	"net/http"
	"sync/atomic"

	"github.com/openshift-online/rosa-hyperfleet-api/platform-api/pkg/api"
)

// HealthHandler handles health check endpoints
type HealthHandler struct {
	ready  *atomic.Bool
	logger *slog.Logger
}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler(logger *slog.Logger) *HealthHandler {
	ready := &atomic.Bool{}
	ready.Store(true)
	return &HealthHandler{
		ready:  ready,
		logger: logger,
	}
}

// SetReady sets the readiness state
func (h *HealthHandler) SetReady(ready bool) {
	h.ready.Store(ready)
}

// Liveness handles GET /live
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	if err := api.Write(w, http.StatusOK, map[string]string{"status": "ok"}); err != nil {
		h.logger.Error("failed to write response", "error", err)
	}
}

// Readiness handles GET /ready
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if !h.ready.Load() {
		if err := api.Write(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable"}); err != nil {
			h.logger.Error("failed to write response", "error", err)
		}
		return
	}

	if err := api.Write(w, http.StatusOK, map[string]string{"status": "ok"}); err != nil {
		h.logger.Error("failed to write response", "error", err)
	}
}
