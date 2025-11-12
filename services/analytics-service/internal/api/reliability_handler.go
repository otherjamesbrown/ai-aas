// Package api provides HTTP handlers for reliability endpoints.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

// ReliabilityHandler handles reliability-related API requests.
type ReliabilityHandler struct {
	store  *postgres.Store
	logger *zap.Logger
}

// NewReliabilityHandler creates a new reliability handler.
func NewReliabilityHandler(store *postgres.Store, logger *zap.Logger) *ReliabilityHandler {
	return &ReliabilityHandler{
		store:  store,
		logger: logger,
	}
}

// GetOrgReliability handles GET /analytics/v1/orgs/{orgId}/reliability
func (h *ReliabilityHandler) GetOrgReliability(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse org ID
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid org_id", err)
		return
	}

	// Parse query parameters
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	granularity := r.URL.Query().Get("granularity")
	if granularity == "" {
		granularity = "day"
	}
	modelIDStr := r.URL.Query().Get("modelId")
	percentile := r.URL.Query().Get("percentile") // Optional: p50, p95, p99

	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid start parameter", err)
		return
	}

	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid end parameter", err)
		return
	}

	if end.Before(start) {
		h.respondError(w, http.StatusBadRequest, "end must be after start", nil)
		return
	}

	// Validate granularity
	if granularity != "hour" && granularity != "day" {
		h.respondError(w, http.StatusBadRequest, "granularity must be 'hour' or 'day'", nil)
		return
	}

	// Validate percentile if provided
	if percentile != "" && percentile != "p50" && percentile != "p95" && percentile != "p99" {
		h.respondError(w, http.StatusBadRequest, "percentile must be 'p50', 'p95', or 'p99'", nil)
		return
	}

	var modelID *uuid.UUID
	if modelIDStr != "" {
		parsed, err := uuid.Parse(modelIDStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid model_id", err)
			return
		}
		modelID = &parsed
	}

	// Query reliability series
	points, err := h.store.GetReliabilitySeries(ctx, orgID, start, end, granularity, modelID)
	if err != nil {
		h.logger.Error("failed to get reliability series", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to retrieve reliability data", err)
		return
	}

	// Build response
	response := ReliabilitySeriesResponse{
		OrgID:       orgID.String(),
		Granularity: granularity,
		Series:      convertReliabilityPoints(points, percentile),
	}

	h.respondJSON(w, http.StatusOK, response)
}

// ReliabilitySeriesResponse matches the OpenAPI schema.
type ReliabilitySeriesResponse struct {
	OrgID       string                `json:"orgId"`
	Granularity string                `json:"granularity"`
	Series      []ReliabilityPointResp `json:"series"`
}

// ReliabilityPointResp matches the OpenAPI schema.
type ReliabilityPointResp struct {
	BucketStart string              `json:"bucketStart"`
	ModelID     *string              `json:"modelId,omitempty"`
	ErrorRate   float64              `json:"errorRate"`
	LatencyMs   LatencyPercentiles   `json:"latencyMs"`
}

// LatencyPercentiles represents latency percentiles.
type LatencyPercentiles struct {
	P50 int `json:"p50"`
	P95 int `json:"p95"`
	P99 int `json:"p99"`
}

func convertReliabilityPoints(points []postgres.ReliabilityPoint, highlightPercentile string) []ReliabilityPointResp {
	result := make([]ReliabilityPointResp, len(points))
	for i, p := range points {
		r := ReliabilityPointResp{
			BucketStart: p.BucketStart.Format(time.RFC3339),
			ErrorRate:   p.ErrorRate,
			LatencyMs: LatencyPercentiles{
				P50: p.LatencyP50,
				P95: p.LatencyP95,
				P99: p.LatencyP99,
			},
		}
		if p.ModelID != nil {
			id := p.ModelID.String()
			r.ModelID = &id
		}
		result[i] = r
	}
	return result
}

func (h *ReliabilityHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *ReliabilityHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Warn(message, zap.Error(err), zap.Int("status", status))
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"title":  http.StatusText(status),
		"detail": message,
	})
}

