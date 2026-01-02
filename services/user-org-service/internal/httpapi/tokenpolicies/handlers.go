// Package tokenpolicies provides HTTP handlers for token rate-limit policy management.
//
// Endpoints:
//   - POST   /v1/orgs/{orgId}/token-policies          - Create policy
//   - GET    /v1/orgs/{orgId}/token-policies          - List policies
//   - GET    /v1/orgs/{orgId}/token-policies/{id}     - Get policy
//   - PUT    /v1/orgs/{orgId}/token-policies/{id}     - Update policy
//   - DELETE /v1/orgs/{orgId}/token-policies/{id}     - Delete policy
//   - GET    /v1/orgs/{orgId}/token-policy            - Get org default
//   - PUT    /v1/orgs/{orgId}/token-policy            - Set org default
//   - GET    /v1/orgs/{orgId}/users/{userId}/token-policy    - Get user's effective policy
//   - PUT    /v1/orgs/{orgId}/users/{userId}/token-policy    - Set user override
//   - DELETE /v1/orgs/{orgId}/users/{userId}/token-policy    - Clear user override
//
// Spec: 035-token-rate-limits
package tokenpolicies

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts token policy routes.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}

	// Policy CRUD
	router.Post("/v1/orgs/{orgId}/token-policies", handler.CreatePolicy)
	router.Get("/v1/orgs/{orgId}/token-policies", handler.ListPolicies)
	router.Get("/v1/orgs/{orgId}/token-policies/{policyId}", handler.GetPolicy)
	router.Put("/v1/orgs/{orgId}/token-policies/{policyId}", handler.UpdatePolicy)
	router.Delete("/v1/orgs/{orgId}/token-policies/{policyId}", handler.DeletePolicy)

	// Org default
	router.Get("/v1/orgs/{orgId}/token-policy", handler.GetOrgDefault)
	router.Put("/v1/orgs/{orgId}/token-policy", handler.SetOrgDefault)

	// User override
	router.Get("/v1/orgs/{orgId}/users/{userId}/token-policy", handler.GetUserPolicy)
	router.Put("/v1/orgs/{orgId}/users/{userId}/token-policy", handler.SetUserPolicy)
	router.Delete("/v1/orgs/{orgId}/users/{userId}/token-policy", handler.ClearUserPolicy)
}

// Handler serves token policy endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// --- DTOs ---

// TokenRateLimitPolicyDTO represents a policy in API responses.
type TokenRateLimitPolicyDTO struct {
	ID          string  `json:"id"`
	OrgID       *string `json:"org_id,omitempty"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Limit1h     *int64  `json:"limit_1h,omitempty"`
	Limit24h    *int64  `json:"limit_24h,omitempty"`
	Limit7d     *int64  `json:"limit_7d,omitempty"`
	IsBuiltin   bool    `json:"is_builtin"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// EffectiveTokenPolicyDTO represents a user's effective policy.
type EffectiveTokenPolicyDTO struct {
	Policy TokenRateLimitPolicyDTO `json:"policy"`
	Source string                  `json:"source"` // "override" or "inherited"
}

// CreatePolicyRequest represents the create policy request body.
type CreatePolicyRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Limit1h     *int64  `json:"limit_1h,omitempty"`
	Limit24h    *int64  `json:"limit_24h,omitempty"`
	Limit7d     *int64  `json:"limit_7d,omitempty"`
}

// UpdatePolicyRequest represents the update policy request body.
type UpdatePolicyRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Limit1h     *int64  `json:"limit_1h,omitempty"`
	Limit24h    *int64  `json:"limit_24h,omitempty"`
	Limit7d     *int64  `json:"limit_7d,omitempty"`
}

// SetPolicyRequest represents the set policy request body.
type SetPolicyRequest struct {
	PolicyID string `json:"policy_id"`
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

// CreatePolicy handles POST /v1/orgs/{orgId}/token-policies
func (h *Handler) CreatePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required")
		return
	}

	var req CreatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "invalid JSON payload")
		return
	}

	if req.Name == "" {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "name is required")
		return
	}

	policy, err := h.runtime.Postgres.CreateTokenPolicy(ctx, postgres.CreateTokenPolicyParams{
		OrgID:       orgID,
		Name:        req.Name,
		Description: req.Description,
		Limit1h:     req.Limit1h,
		Limit24h:    req.Limit24h,
		Limit7d:     req.Limit7d,
	})
	if err != nil {
		if err == postgres.ErrConflict {
			h.writeError(w, http.StatusConflict, "conflict", "Policy Name Conflict", "policy name already exists or is reserved")
			return
		}
		if isNoLimitsSpecifiedError(err) {
			h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "at least one limit (1h, 24h, or 7d) must be specified")
			return
		}
		h.logger.Error("failed to create token policy", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to create policy")
		return
	}

	h.logger.Info("token policy created",
		zap.String("org_id", orgID.String()),
		zap.String("policy_id", policy.ID.String()),
		zap.String("name", policy.Name),
	)

	h.writeJSON(w, http.StatusCreated, toPolicyDTO(policy))
}

// ListPolicies handles GET /v1/orgs/{orgId}/token-policies
func (h *Handler) ListPolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "access denied")
		return
	}

	policies, err := h.runtime.Postgres.ListTokenPolicies(ctx, orgID)
	if err != nil {
		h.logger.Error("failed to list token policies", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to list policies")
		return
	}

	dtos := make([]TokenRateLimitPolicyDTO, 0, len(policies))
	for _, p := range policies {
		dtos = append(dtos, toPolicyDTO(p))
	}

	h.writeJSON(w, http.StatusOK, map[string]any{"policies": dtos})
}

// GetPolicy handles GET /v1/orgs/{orgId}/token-policies/{policyId}
func (h *Handler) GetPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	policyIDParam := chi.URLParam(r, "policyId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "access denied")
		return
	}

	policyID, err := h.resolvePolicyID(policyIDParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Policy ID", "invalid policy ID format")
		return
	}

	policy, err := h.runtime.Postgres.GetTokenPolicy(ctx, policyID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "Policy Not Found", "policy not found")
			return
		}
		h.logger.Error("failed to get token policy", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to get policy")
		return
	}

	// Check policy belongs to org or is builtin
	if policy.OrgID != nil && *policy.OrgID != orgID {
		h.writeError(w, http.StatusNotFound, "not_found", "Policy Not Found", "policy not found")
		return
	}

	h.writeJSON(w, http.StatusOK, toPolicyDTO(policy))
}

// UpdatePolicy handles PUT /v1/orgs/{orgId}/token-policies/{policyId}
func (h *Handler) UpdatePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	policyIDParam := chi.URLParam(r, "policyId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required")
		return
	}

	policyID, err := h.resolvePolicyID(policyIDParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Policy ID", "invalid policy ID format")
		return
	}

	var req UpdatePolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "invalid JSON payload")
		return
	}

	policy, err := h.runtime.Postgres.UpdateTokenPolicy(ctx, postgres.UpdateTokenPolicyParams{
		ID:          policyID,
		Name:        req.Name,
		Description: req.Description,
		Limit1h:     req.Limit1h,
		Limit24h:    req.Limit24h,
		Limit7d:     req.Limit7d,
	})
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "Policy Not Found", "policy not found")
			return
		}
		if err == postgres.ErrForbidden {
			h.writeError(w, http.StatusForbidden, "forbidden", "Cannot Modify Built-in Policy", "built-in policies cannot be modified")
			return
		}
		if err == postgres.ErrConflict {
			h.writeError(w, http.StatusConflict, "conflict", "Policy Name Conflict", "policy name already exists or is reserved")
			return
		}
		h.logger.Error("failed to update token policy", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to update policy")
		return
	}

	h.logger.Info("token policy updated",
		zap.String("org_id", orgID.String()),
		zap.String("policy_id", policy.ID.String()),
	)

	h.writeJSON(w, http.StatusOK, toPolicyDTO(policy))
}

// DeletePolicy handles DELETE /v1/orgs/{orgId}/token-policies/{policyId}
func (h *Handler) DeletePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	policyIDParam := chi.URLParam(r, "policyId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required")
		return
	}

	policyID, err := h.resolvePolicyID(policyIDParam)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Policy ID", "invalid policy ID format")
		return
	}

	err = h.runtime.Postgres.DeleteTokenPolicy(ctx, policyID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "Policy Not Found", "policy not found")
			return
		}
		if err == postgres.ErrForbidden {
			h.writeError(w, http.StatusForbidden, "forbidden", "Cannot Delete Built-in Policy", "built-in policies cannot be deleted")
			return
		}
		if err == postgres.ErrConflict {
			h.writeError(w, http.StatusConflict, "conflict", "Policy In Use", "policy is in use and cannot be deleted")
			return
		}
		h.logger.Error("failed to delete token policy", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to delete policy")
		return
	}

	h.logger.Info("token policy deleted",
		zap.String("org_id", orgID.String()),
		zap.String("policy_id", policyID.String()),
	)

	w.WriteHeader(http.StatusNoContent)
}

// GetOrgDefault handles GET /v1/orgs/{orgId}/token-policy
func (h *Handler) GetOrgDefault(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "access denied")
		return
	}

	policy, err := h.runtime.Postgres.GetOrgDefaultTokenPolicy(ctx, orgID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
			return
		}
		h.logger.Error("failed to get org default token policy", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to get default policy")
		return
	}

	h.writeJSON(w, http.StatusOK, toPolicyDTO(policy))
}

// SetOrgDefault handles PUT /v1/orgs/{orgId}/token-policy
func (h *Handler) SetOrgDefault(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required")
		return
	}

	var req SetPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "invalid JSON payload")
		return
	}

	policyID, err := h.resolvePolicyID(req.PolicyID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Policy ID", "invalid policy ID format")
		return
	}

	err = h.runtime.Postgres.SetOrgDefaultTokenPolicy(ctx, orgID, policyID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "Policy Not Found", "policy not found or not accessible")
			return
		}
		h.logger.Error("failed to set org default token policy", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to set default policy")
		return
	}

	// Return the updated default policy
	policy, err := h.runtime.Postgres.GetTokenPolicy(ctx, policyID)
	if err != nil {
		h.logger.Error("failed to get policy after setting default", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to retrieve policy")
		return
	}

	h.logger.Info("org default token policy set",
		zap.String("org_id", orgID.String()),
		zap.String("policy_id", policyID.String()),
	)

	h.writeJSON(w, http.StatusOK, toPolicyDTO(policy))
}

// GetUserPolicy handles GET /v1/orgs/{orgId}/users/{userId}/token-policy
func (h *Handler) GetUserPolicy(w http.ResponseWriter, r *http.Request) {
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
		h.logger.Error("failed to get user effective token policy", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to get policy")
		return
	}

	h.writeJSON(w, http.StatusOK, EffectiveTokenPolicyDTO{
		Policy: toPolicyDTO(effective.Policy),
		Source: effective.Source,
	})
}

// SetUserPolicy handles PUT /v1/orgs/{orgId}/users/{userId}/token-policy
func (h *Handler) SetUserPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required")
		return
	}

	userID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found")
		return
	}

	var req SetPolicyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Request", "invalid JSON payload")
		return
	}

	policyID, err := h.resolvePolicyID(req.PolicyID)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "bad_request", "Invalid Policy ID", "invalid policy ID format")
		return
	}

	err = h.runtime.Postgres.SetUserTokenPolicyOverride(ctx, orgID, userID, policyID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "Not Found", "user or policy not found")
			return
		}
		h.logger.Error("failed to set user token policy override", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to set policy override")
		return
	}

	// Return the updated effective policy
	effective, err := h.runtime.Postgres.GetEffectiveTokenPolicy(ctx, orgID, userID)
	if err != nil {
		h.logger.Error("failed to get effective policy after setting override", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to retrieve policy")
		return
	}

	h.logger.Info("user token policy override set",
		zap.String("org_id", orgID.String()),
		zap.String("user_id", userID.String()),
		zap.String("policy_id", policyID.String()),
	)

	h.writeJSON(w, http.StatusOK, EffectiveTokenPolicyDTO{
		Policy: toPolicyDTO(effective.Policy),
		Source: effective.Source,
	})
}

// ClearUserPolicy handles DELETE /v1/orgs/{orgId}/users/{userId}/token-policy
func (h *Handler) ClearUserPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	userIDParam := chi.URLParam(r, "userId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	if err := h.requireOrgAdmin(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "admin privileges required")
		return
	}

	userID, err := h.resolveUserID(ctx, orgID, userIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found")
		return
	}

	err = h.runtime.Postgres.ClearUserTokenPolicyOverride(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			h.writeError(w, http.StatusNotFound, "not_found", "User Not Found", "user not found")
			return
		}
		h.logger.Error("failed to clear user token policy override", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to clear policy override")
		return
	}

	// Return the new effective policy (inherited from org)
	effective, err := h.runtime.Postgres.GetEffectiveTokenPolicy(ctx, orgID, userID)
	if err != nil {
		h.logger.Error("failed to get effective policy after clearing override", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to retrieve policy")
		return
	}

	h.logger.Info("user token policy override cleared",
		zap.String("org_id", orgID.String()),
		zap.String("user_id", userID.String()),
	)

	h.writeJSON(w, http.StatusOK, EffectiveTokenPolicyDTO{
		Policy: toPolicyDTO(effective.Policy),
		Source: effective.Source,
	})
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

func (h *Handler) resolvePolicyID(policyIDParam string) (uuid.UUID, error) {
	// Allow "no-limit" as alias for the builtin policy
	if policyIDParam == "no-limit" {
		return postgres.NoTokenRateLimitPolicyID, nil
	}
	return uuid.Parse(policyIDParam)
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

func (h *Handler) requireOrgAdmin(ctx context.Context, orgID uuid.UUID) error {
	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		return err
	}
	if middleware.HasAnyScope(ctx, "org:admin", "admin", "*") {
		return nil
	}
	org, err := h.runtime.Postgres.GetOrg(ctx, orgID)
	if err != nil {
		return fmt.Errorf("failed to get org: %w", err)
	}
	userID := middleware.GetUserID(ctx)
	if org.BillingOwnerUserID != nil && *org.BillingOwnerUserID == userID {
		return nil
	}
	return fmt.Errorf("admin privileges required")
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

func toPolicyDTO(p postgres.TokenRateLimitPolicy) TokenRateLimitPolicyDTO {
	dto := TokenRateLimitPolicyDTO{
		ID:          p.ID.String(),
		Name:        p.Name,
		Description: p.Description,
		Limit1h:     p.Limit1h,
		Limit24h:    p.Limit24h,
		Limit7d:     p.Limit7d,
		IsBuiltin:   p.IsBuiltin,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
	if p.OrgID != nil {
		s := p.OrgID.String()
		dto.OrgID = &s
	}
	return dto
}

func isNoLimitsSpecifiedError(err error) bool {
	return errors.Is(err, postgres.ErrNoLimitsSpecified)
}
