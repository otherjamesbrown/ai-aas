// Package tokenusage provides HTTP handlers for token usage queries.
//
// Endpoints:
//   - GET  /v1/usage/tokens                              - Get own usage (user)
//   - GET  /v1/orgs/{orgId}/users/{userId}/usage/tokens  - Get user's usage (admin)
//   - POST /internal/v1/usage/tokens                     - Record usage (M2M)
//   - GET  /internal/v1/users/{userId}/rate-limit        - Check rate limit (M2M)
//
// Spec: 035-token-rate-limits
package tokenusage

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts token usage routes.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}

	// Public endpoints (authenticated users)
	router.Get("/v1/usage/tokens", handler.GetOwnUsage)
	router.Get("/v1/orgs/{orgId}/users/{userId}/usage/tokens", handler.GetUserUsage)

	// Internal M2M endpoints
	router.Post("/internal/v1/usage/tokens", handler.RecordUsage)
	router.Get("/internal/v1/users/{userId}/rate-limit", handler.CheckRateLimit)
}

// Handler serves token usage endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// --- DTOs ---

// TokenUsageDTO represents token usage in API responses.
type TokenUsageDTO struct {
	UserID     string               `json:"user_id,omitempty"`
	OrgID      string               `json:"org_id,omitempty"`
	PolicyID   string               `json:"policy_id,omitempty"`
	PolicyName string               `json:"policy_name"`
	Windows    []TokenUsageWindowDTO `json:"windows"`
}

// TokenUsageWindowDTO represents a usage window.
type TokenUsageWindowDTO struct {
	Window     string  `json:"window"` // "1h", "24h", "7d"
	Limit      int64   `json:"limit"`
	Used       int64   `json:"used"`
	Remaining  int64   `json:"remaining"`
	Percentage float64 `json:"percentage"`
	ResetsAt   string  `json:"resets_at"`
}

// RateLimitStatusDTO represents rate limit status for M2M checks.
type RateLimitStatusDTO struct {
	Allowed        bool                   `json:"allowed"`
	Windows        []TokenUsageWindowDTO  `json:"windows"`
	BlockingWindow *TokenUsageWindowDTO   `json:"blocking_window,omitempty"`
}

// RecordUsageRequest represents the record usage request body.
type RecordUsageRequest struct {
	UserID         string `json:"user_id"`
	Tokens         int64  `json:"tokens"`
	IdempotencyKey string `json:"idempotency_key,omitempty"`
}

// ErrorResponse represents an API error (RFC7807).
type ErrorResponse struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// --- Handlers ---

// GetOwnUsage handles GET /v1/usage/tokens
func (h *Handler) GetOwnUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := middleware.GetUserID(ctx)
	orgID := middleware.GetOrgID(ctx)

	if userID == uuid.Nil {
		h.writeError(w, http.StatusUnauthorized, "unauthorized", "Unauthorized", "authentication required")
		return
	}

	effective, err := h.runtime.Postgres.GetEffectiveTokenPolicy(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found")
			return
		}
		h.logger.Error("failed to get effective token policy", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to get policy")
		return
	}

	usage, err := h.runtime.Postgres.GetTokenUsageForPolicy(ctx, userID, effective.Policy)
	if err != nil {
		h.logger.Error("failed to get token usage", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to get usage")
		return
	}

	dto := TokenUsageDTO{
		UserID:     userID.String(),
		OrgID:      orgID.String(),
		PolicyID:   effective.Policy.ID.String(),
		PolicyName: effective.Policy.Name,
		Windows:    toUsageWindowDTOs(usage),
	}

	h.writeJSON(w, http.StatusOK, dto)
}

// GetUserUsage handles GET /v1/orgs/{orgId}/users/{userId}/usage/tokens
func (h *Handler) GetUserUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "access denied")
		return
	}

	userID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found")
		return
	}

	effective, err := h.runtime.Postgres.GetEffectiveTokenPolicy(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found")
			return
		}
		h.logger.Error("failed to get effective token policy", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to get policy")
		return
	}

	usage, err := h.runtime.Postgres.GetTokenUsageForPolicy(ctx, userID, effective.Policy)
	if err != nil {
		h.logger.Error("failed to get token usage", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to get usage")
		return
	}

	dto := TokenUsageDTO{
		UserID:     userID.String(),
		OrgID:      orgID.String(),
		PolicyID:   effective.Policy.ID.String(),
		PolicyName: effective.Policy.Name,
		Windows:    toUsageWindowDTOs(usage),
	}

	h.writeJSON(w, http.StatusOK, dto)
}

// RecordUsage handles POST /internal/v1/usage/tokens
func (h *Handler) RecordUsage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// M2M auth check - should have internal/service scope
	if !middleware.HasAnyScope(ctx, "internal", "service", "admin", "*") {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "internal access required")
		return
	}

	var req RecordUsageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "invalid JSON payload")
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid User ID", "invalid user ID format")
		return
	}

	if req.Tokens < 0 {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Tokens", "tokens must be non-negative")
		return
	}

	err = h.runtime.Postgres.RecordTokenUsage(ctx, userID, req.Tokens)
	if err != nil {
		h.logger.Error("failed to record token usage",
			zap.Error(err),
			zap.String("user_id", userID.String()),
			zap.Int64("tokens", req.Tokens),
		)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to record usage")
		return
	}

	h.logger.Debug("token usage recorded",
		zap.String("user_id", userID.String()),
		zap.Int64("tokens", req.Tokens),
	)

	w.WriteHeader(http.StatusNoContent)
}

// CheckRateLimit handles GET /internal/v1/users/{userId}/rate-limit
func (h *Handler) CheckRateLimit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userIDParam := chi.URLParam(r, "userId")

	// M2M auth check - should have internal/service scope
	if !middleware.HasAnyScope(ctx, "internal", "service", "admin", "*") {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "internal access required")
		return
	}

	userID, err := uuid.Parse(userIDParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid User ID", "invalid user ID format")
		return
	}

	allowed, windows, blocking, err := h.runtime.Postgres.CheckRateLimitStatus(ctx, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found")
			return
		}
		h.logger.Error("failed to check rate limit status",
			zap.Error(err),
			zap.String("user_id", userID.String()),
		)
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to check rate limit")
		return
	}

	dto := RateLimitStatusDTO{
		Allowed: allowed,
		Windows: toUsageWindowDTOs(windows),
	}

	if blocking != nil {
		blockingDTO := toUsageWindowDTO(*blocking)
		dto.BlockingWindow = &blockingDTO
	}

	h.writeJSON(w, http.StatusOK, dto)
}

// --- Helpers ---

func (h *Handler) resolveOrgID(ctx context.Context, orgIDParam string) (uuid.UUID, error) {
	if orgID, err := uuid.Parse(orgIDParam); err == nil {
		_, err := h.runtime.Postgres.GetOrg(ctx, orgID)
		return orgID, err
	}
	org, err := h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
	if err != nil {
		return uuid.Nil, err
	}
	return org.ID, nil
}

func (h *Handler) resolveUserID(ctx context.Context, orgID uuid.UUID, userIDParam string) (uuid.UUID, error) {
	if userID, err := uuid.Parse(userIDParam); err == nil {
		_, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
		return userID, err
	}
	user, err := h.runtime.Postgres.GetUserByEmail(ctx, orgID, userIDParam)
	if err != nil {
		return uuid.Nil, err
	}
	return user.ID, nil
}

func (h *Handler) requireOrgAccess(ctx context.Context, orgID uuid.UUID) error {
	authOrgID := middleware.GetOrgID(ctx)
	if middleware.HasAnyScope(ctx, "admin", "*") {
		return nil
	}
	if authOrgID == orgID {
		return nil
	}
	userID := middleware.GetUserID(ctx)
	return h.runtime.Postgres.ValidateUserOrgMembership(ctx, userID, orgID)
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("failed to encode response", zap.Error(err))
	}
}

func (h *Handler) writeError(w http.ResponseWriter, status int, errType, title, detail string) {
	resp := ErrorResponse{
		Type:   "https://api.ai-aas.io/errors/" + errType,
		Title:  title,
		Status: status,
		Detail: detail,
	}
	h.writeJSON(w, status, resp)
}

func toUsageWindowDTOs(windows []postgres.TokenUsageStatus) []TokenUsageWindowDTO {
	dtos := make([]TokenUsageWindowDTO, 0, len(windows))
	for _, w := range windows {
		dtos = append(dtos, toUsageWindowDTO(w))
	}
	return dtos
}

func toUsageWindowDTO(w postgres.TokenUsageStatus) TokenUsageWindowDTO {
	return TokenUsageWindowDTO{
		Window:     string(w.Window),
		Limit:      w.Limit,
		Used:       w.Used,
		Remaining:  w.Remaining,
		Percentage: w.Percentage,
		ResetsAt:   w.ResetsAt.Format(time.RFC3339),
	}
}
