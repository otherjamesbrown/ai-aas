// Package usage provides HTTP handlers for usage statistics endpoints.
package usage

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/ai-aas/shared-go/httputil"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/analytics"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
)

// RegisterRoutes registers the usage endpoints.
func RegisterRoutes(r chi.Router, runtime *bootstrap.Runtime, logger *zap.Logger) {
	h := &Handler{
		runtime: runtime,
		logger:  logger,
		client:  analytics.NewClient(runtime.Config.AnalyticsServiceURL),
	}

	r.Route("/v1/orgs/{orgId}/usage", func(r chi.Router) {
		r.Get("/summary", h.GetUsageSummary)
		r.Get("/by-model", h.GetUsageByModel)
		r.Get("/by-user", h.GetUsageByUser)
	})
}

// Handler handles usage-related requests.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
	client  *analytics.Client
}

// UsageSummaryResponse represents the usage summary response.
type UsageSummaryResponse struct {
	OrgID            string    `json:"org_id"`
	Period           string    `json:"period"`
	StartDate        time.Time `json:"start_date"`
	EndDate          time.Time `json:"end_date"`
	TotalTokens      int64     `json:"total_tokens"`
	PromptTokens     int64     `json:"prompt_tokens"`
	CompletionTokens int64     `json:"completion_tokens"`
	TotalRequests    int64     `json:"total_requests"`
	TotalCost        float64   `json:"total_cost"`
	Currency         string    `json:"currency"`
}

// UsageByModelItem represents usage for a single model.
type UsageByModelItem struct {
	ModelID          string  `json:"model_id"`
	ModelName        string  `json:"model_name"`
	TotalTokens      int64   `json:"total_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalRequests    int64   `json:"total_requests"`
	TotalCost        float64 `json:"total_cost"`
}

// UsageByModelResponse represents the response for usage by model.
type UsageByModelResponse struct {
	Models []UsageByModelItem `json:"models"`
}

// UsageByUserItem represents usage for a single user.
type UsageByUserItem struct {
	UserID           string  `json:"user_id"`
	UserEmail        string  `json:"user_email"`
	UserName         string  `json:"user_name"`
	TotalTokens      int64   `json:"total_tokens"`
	PromptTokens     int64   `json:"prompt_tokens"`
	CompletionTokens int64   `json:"completion_tokens"`
	TotalRequests    int64   `json:"total_requests"`
	TotalCost        float64 `json:"total_cost"`
}

// UsageByUserResponse represents the response for usage by user.
type UsageByUserResponse struct {
	Users []UsageByUserItem `json:"users"`
}

// GetUsageSummary handles GET /v1/orgs/{orgId}/usage/summary
func (h *Handler) GetUsageSummary(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse org ID
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid org_id", err)
		return
	}

	// Parse period parameter
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	// Convert period to start/end times
	start, end, err := parsePeriod(period)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid period", err)
		return
	}

	// Get API key from context (set by auth middleware)
	apiKey := h.getAPIKeyFromContext(r)

	// Query analytics service
	usage, err := h.client.GetUsage(ctx, orgID.String(), start, end, "day", apiKey)
	if err != nil {
		h.logger.Error("failed to get usage from analytics service", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to retrieve usage data", err)
		return
	}

	// Build response
	response := UsageSummaryResponse{
		OrgID:            orgID.String(),
		Period:           period,
		StartDate:        start,
		EndDate:          end,
		TotalTokens:      usage.Totals.InputTokens + usage.Totals.OutputTokens,
		PromptTokens:     usage.Totals.InputTokens,
		CompletionTokens: usage.Totals.OutputTokens,
		TotalRequests:    usage.Totals.Invocations,
		TotalCost:        float64(usage.Totals.CostEstimateCents) / 100.0,
		Currency:         "USD",
	}

	h.respondJSON(w, http.StatusOK, response)
}

// GetUsageByModel handles GET /v1/orgs/{orgId}/usage/by-model
func (h *Handler) GetUsageByModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse org ID
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid org_id", err)
		return
	}

	// Parse period parameter
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	// Convert period to start/end times
	start, end, err := parsePeriod(period)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid period", err)
		return
	}

	// Get API key from context (set by auth middleware)
	apiKey := h.getAPIKeyFromContext(r)

	// Query analytics service
	usage, err := h.client.GetUsage(ctx, orgID.String(), start, end, "day", apiKey)
	if err != nil {
		h.logger.Error("failed to get usage from analytics service", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to retrieve usage data", err)
		return
	}

	// Group by model
	modelMap := make(map[string]*UsageByModelItem)
	for _, point := range usage.Series {
		if point.ModelID == nil {
			continue
		}
		modelID := *point.ModelID
		if _, exists := modelMap[modelID]; !exists {
			modelMap[modelID] = &UsageByModelItem{
				ModelID:   modelID,
				ModelName: modelID, // TODO: resolve model name from model ID
			}
		}
		item := modelMap[modelID]
		item.TotalRequests += point.Invocations
		item.PromptTokens += point.InputTokens
		item.CompletionTokens += point.OutputTokens
		item.TotalTokens += point.InputTokens + point.OutputTokens
		item.TotalCost += float64(point.CostEstimateCents) / 100.0
	}

	// Convert map to slice
	models := make([]UsageByModelItem, 0, len(modelMap))
	for _, item := range modelMap {
		models = append(models, *item)
	}

	response := UsageByModelResponse{
		Models: models,
	}

	h.respondJSON(w, http.StatusOK, response)
}

// GetUsageByUser handles GET /v1/orgs/{orgId}/usage/by-user
func (h *Handler) GetUsageByUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse org ID
	orgIDStr := chi.URLParam(r, "orgId")
	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid org_id", err)
		return
	}

	// Parse period parameter
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}

	// Convert period to start/end times
	start, end, err := parsePeriod(period)
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid period", err)
		return
	}

	// Get API key from context (set by auth middleware)
	apiKey := h.getAPIKeyFromContext(r)

	// Query analytics service
	_, err = h.client.GetUsage(ctx, orgID.String(), start, end, "day", apiKey)
	if err != nil {
		h.logger.Error("failed to get usage from analytics service", zap.Error(err))
		h.respondError(w, http.StatusInternalServerError, "failed to retrieve usage data", err)
		return
	}

	// TODO: The analytics service doesn't currently track usage by individual user.
	// For now, return empty list. This should be enhanced when analytics service
	// adds per-user tracking.
	response := UsageByUserResponse{
		Users: []UsageByUserItem{},
	}

	h.respondJSON(w, http.StatusOK, response)
}

// parsePeriod converts a period string to start/end times
func parsePeriod(period string) (start, end time.Time, err error) {
	now := time.Now()
	end = now

	switch period {
	case "today":
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	case "yesterday":
		yesterday := now.AddDate(0, 0, -1)
		start = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		end = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 0, yesterday.Location())
	case "7d":
		start = now.AddDate(0, 0, -7)
	case "30d":
		start = now.AddDate(0, 0, -30)
	case "90d":
		start = now.AddDate(0, 0, -90)
	case "this-month":
		start = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	case "last-month":
		lastMonth := now.AddDate(0, -1, 0)
		start = time.Date(lastMonth.Year(), lastMonth.Month(), 1, 0, 0, 0, 0, lastMonth.Location())
		end = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).Add(-time.Second)
	default:
		return time.Time{}, time.Time{}, fmt.Errorf("invalid period: must be one of: today, yesterday, 7d, 30d, 90d, this-month, last-month")
	}

	return start, end, nil
}

// getAPIKeyFromContext retrieves the API key from the request context
func (h *Handler) getAPIKeyFromContext(r *http.Request) string {
	// The auth middleware should have set this in the context or headers
	// For now, extract from Authorization header
	authHeader := r.Header.Get("Authorization")
	if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
		return authHeader[7:]
	}
	return ""
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *Handler) respondError(w http.ResponseWriter, status int, message string, err error) {
	h.logger.Warn(message, zap.Error(err), zap.Int("status", status))
	httputil.WriteProblemDetails(w, status, http.StatusText(status), message)
}
