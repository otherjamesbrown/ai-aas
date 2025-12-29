package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
)

// HealthHandler handles health check requests
type HealthHandler struct {
	db      *repository.DB
	version string
}

// NewHealthHandler creates a new health handler
func NewHealthHandler(db *repository.DB, version string) *HealthHandler {
	return &HealthHandler{
		db:      db,
		version: version,
	}
}

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Database  string `json:"database"`
	Timestamp string `json:"timestamp"`
}

// Healthz handles GET /healthz
func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	response := HealthResponse{
		Status:    "healthy",
		Version:   h.version,
		Database:  "connected",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Check database connectivity
	if err := h.db.Ping(ctx); err != nil {
		response.Status = "degraded"
		response.Database = "disconnected"
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Readyz handles GET /readyz (Kubernetes readiness probe)
func (h *HealthHandler) Readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}
