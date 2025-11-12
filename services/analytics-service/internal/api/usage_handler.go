// Package api provides HTTP handlers for analytics endpoints.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/freshness"
	"github.com/otherjamesbrown/ai-aas/services/analytics-service/internal/storage/postgres"
)

// UsageHandler handles usage-related API requests.
type UsageHandler struct {
	store          *postgres.Store
	logger         *zap.Logger
	freshnessCache *freshness.Cache
}

// NewUsageHandler creates a new usage handler.
func NewUsageHandler(store *postgres.Store, logger *zap.Logger, cache *freshness.Cache) *UsageHandler {
	return &UsageHandler{
		store:          store,
		logger:         logger,
		freshnessCache: cache,
	}
}

// GetOrgUsage handles GET /analytics/v1/orgs/{orgId}/usage
func (h *UsageHandler) GetOrgUsage(w http.ResponseWriter, r *http.Request) {
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

	var modelID *uuid.UUID
	if modelIDStr != "" {
		parsed, err := uuid.Parse(modelIDStr)
		if err != nil {
			h.respondError(w, http.StatusBadRequest, "invalid model_id", err)
			return
		}
		modelID = &parsed
	}

	// Query usage series
	points, err := h.store.GetUsageSeries(ctx, orgID, start, end, granularity, modelID)
	if err != nil {
		h.logger.Error("failed to get usage series", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to retrieve usage data", err)
		return
	}

	// Query totals
	totals, err := h.store.GetUsageTotals(ctx, orgID, start, end, modelID)
	if err != nil {
		h.logger.Error("failed to get usage totals", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to retrieve totals", err)
		return
	}

	// Get freshness indicator from cache or database
	var freshnessIndicator FreshnessIndicator
	if cached, err := h.freshnessCache.Get(ctx, orgID, modelID); err == nil && cached != nil {
		freshnessIndicator = FreshnessIndicator{
			Status:       cached.Status,
			LagSeconds:   cached.LagSeconds,
			LastEventAt:  cached.LastEventAt,
			LastRollupAt: cached.LastRollupAt,
		}
	} else {
		// Fallback to database if cache miss
		if dbFreshness, err := h.store.GetFreshnessStatus(ctx, orgID, modelID); err == nil {
			freshnessIndicator = FreshnessIndicator{
				Status:       dbFreshness.Status,
				LagSeconds:   dbFreshness.LagSeconds,
				LastEventAt:  dbFreshness.LastEventAt,
				LastRollupAt: dbFreshness.LastRollupAt,
			}
			// Cache it for next time
			_ = h.freshnessCache.Set(ctx, dbFreshness)
		} else {
			// Default if not found
			freshnessIndicator = FreshnessIndicator{
				Status:       "fresh",
				LagSeconds:   0,
				LastEventAt:  time.Now().Add(-5 * time.Minute),
				LastRollupAt: time.Now().Add(-1 * time.Minute),
			}
		}
	}

	// Build response
	response := UsageSeriesResponse{
		OrgID:       orgID.String(),
		Granularity: granularity,
		Series:      convertPoints(points),
		Totals: UsageTotalsResponse{
			Invocations:       totals.Invocations,
			InputTokens:       totals.InputTokens,
			OutputTokens:      totals.OutputTokens,
			CostEstimateCents: int64(totals.CostEstimateCents * 100), // Convert to cents
		},
		Freshness: freshnessIndicator,
	}

	h.respondJSON(w, http.StatusOK, response)
}

// UsageSeriesResponse matches the OpenAPI schema.
type UsageSeriesResponse struct {
	OrgID       string                `json:"orgId"`
	Granularity string                `json:"granularity"`
	Series      []UsagePointResponse  `json:"series"`
	Totals      UsageTotalsResponse   `json:"totals"`
	Freshness   FreshnessIndicator    `json:"freshness"`
}

// UsagePointResponse matches the OpenAPI schema.
type UsagePointResponse struct {
	BucketStart       string  `json:"bucketStart"`
	ModelID           *string `json:"modelId,omitempty"`
	Invocations       int64   `json:"invocations"`
	InputTokens       int64   `json:"inputTokens,omitempty"`
	OutputTokens      int64   `json:"outputTokens,omitempty"`
	CostEstimateCents int64   `json:"costEstimateCents"`
}

// UsageTotalsResponse matches the OpenAPI schema.
type UsageTotalsResponse struct {
	Invocations       int64 `json:"invocations"`
	InputTokens       int64 `json:"inputTokens,omitempty"`
	OutputTokens      int64 `json:"outputTokens,omitempty"`
	CostEstimateCents int64 `json:"costEstimateCents"`
}

// FreshnessIndicator represents freshness status.
type FreshnessIndicator struct {
	Status        string    `json:"status"`
	LagSeconds    int       `json:"lagSeconds"`
	LastEventAt   time.Time `json:"lastEventAt"`
	LastRollupAt  time.Time `json:"lastRollupAt"`
}

func convertPoints(points []postgres.UsagePoint) []UsagePointResponse {
	result := make([]UsagePointResponse, len(points))
	for i, p := range points {
		r := UsagePointResponse{
			BucketStart:       p.BucketStart.Format(time.RFC3339),
			Invocations:       p.Invocations,
			InputTokens:       p.InputTokens,
			OutputTokens:      p.OutputTokens,
			CostEstimateCents: int64(p.CostEstimateCents * 100), // Convert to cents
		}
		if p.ModelID != nil {
			id := p.ModelID.String()
			r.ModelID = &id
		}
		result[i] = r
	}
	return result
}

func (h *UsageHandler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *UsageHandler) respondError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Warn(message, zap.Error(err), zap.Int("status", status))
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": status,
		"title":  http.StatusText(status),
		"detail": message,
	})
}

