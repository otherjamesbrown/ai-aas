// Package models provides HTTP handlers for org-level model management.
//
// Purpose:
//
//	This package implements REST API handlers for organization-level model
//	operations. Organization admins can list available models and view
//	model details.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router for route parameters
//   - github.com/google/uuid: UUID parsing and validation
//   - internal/bootstrap: Runtime dependencies (Postgres store)
//   - internal/storage/postgres: Data access layer
//
// Key Responsibilities:
//   - ListModels: GET /v1/orgs/{orgId}/models
//   - GetModel: GET /v1/orgs/{orgId}/models/{modelName}
//
// Requirements Reference:
//   - usecases/models.yaml (UC-MDL-001, UC-MDL-002)
package models

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
)

// RegisterRoutes mounts model routes beneath /v1/orgs/{orgId}.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, logger *zap.Logger) {
	if rt == nil || rt.Postgres == nil {
		return
	}
	handler := &Handler{
		runtime: rt,
		logger:  logger,
	}

	router.Get("/v1/orgs/{orgId}/models", handler.ListModels)
	router.Get("/v1/orgs/{orgId}/models/{modelName}", handler.GetModel)
}

// Handler serves org-level model management endpoints.
type Handler struct {
	runtime *bootstrap.Runtime
	logger  *zap.Logger
}

// Model represents an AI model available to the organization.
type Model struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Description string `json:"description"`
	Status      string `json:"status"`
	ContextSize int    `json:"context_size"`
}

// ListModelsResponse represents the response for listing models.
type ListModelsResponse struct {
	Models []Model `json:"models"`
}

// ErrorResponse represents an API error.
type ErrorResponse struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail,omitempty"`
	Instance string `json:"instance,omitempty"`
}

// OpenAIModelsResponse represents the response from /v1/models
type OpenAIModelsResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIModel represents a model in the OpenAI API response
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ListModels handles GET /v1/orgs/{orgId}/models
func (h *Handler) ListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Verify caller has access to this org
	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "you do not have access to this organization")
		return
	}

	// Fetch models from inference endpoint
	models, err := h.fetchModelsFromInferenceService(ctx)
	if err != nil {
		h.logger.Error("failed to fetch models from inference service", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to retrieve models")
		return
	}

	resp := ListModelsResponse{
		Models: models,
	}

	h.writeJSON(w, http.StatusOK, resp)
}

// GetModel handles GET /v1/orgs/{orgId}/models/{modelName}
func (h *Handler) GetModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orgIDParam := chi.URLParam(r, "orgId")
	modelName := chi.URLParam(r, "modelName")

	orgID, err := h.resolveOrgID(ctx, orgIDParam)
	if err != nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Organization Not Found", "organization not found")
		return
	}

	// Verify caller has access to this org
	if err := h.requireOrgAccess(ctx, orgID); err != nil {
		h.writeError(w, http.StatusForbidden, "forbidden", "Access Denied", "you do not have access to this organization")
		return
	}

	// Fetch models from inference endpoint
	models, err := h.fetchModelsFromInferenceService(ctx)
	if err != nil {
		h.logger.Error("failed to fetch models from inference service", zap.Error(err))
		h.writeError(w, http.StatusInternalServerError, "internal_error", "Internal Error", "failed to retrieve models")
		return
	}

	// Find the requested model
	var model *Model
	for i := range models {
		if models[i].Name == modelName || models[i].ID == modelName {
			model = &models[i]
			break
		}
	}

	if model == nil {
		h.writeError(w, http.StatusNotFound, "not_found", "Model Not Found", "model not found or not accessible")
		return
	}

	h.writeJSON(w, http.StatusOK, model)
}

// Helper methods

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

func (h *Handler) requireOrgAccess(ctx context.Context, orgID uuid.UUID) error {
	authOrgID := middleware.GetOrgID(ctx)
	userID := middleware.GetUserID(ctx)

	// Service accounts with admin scope (or wildcard "*" scope) can access any org
	if middleware.HasAnyScope(ctx, "admin", "*") {
		return nil
	}

	// If user is in the same org, allow access
	if authOrgID == orgID {
		return nil
	}

	// Otherwise, verify user is a member of the target org
	return h.runtime.Postgres.ValidateUserOrgMembership(ctx, userID, orgID)
}

func (h *Handler) fetchModelsFromInferenceService(ctx context.Context) ([]Model, error) {
	// Get inference endpoint from config (has default value)
	inferenceEndpoint := h.runtime.Config.InferenceEndpoint
	modelsURL := inferenceEndpoint + "/v1/models"

	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Allow insecure TLS for development environments
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.InsecureSkipVerify = true
	httpClient.Transport = transport

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("inference service returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var modelsResp OpenAIModelsResponse
	if err := json.Unmarshal(body, &modelsResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to our Model format
	models := make([]Model, 0, len(modelsResp.Data))
	for _, m := range modelsResp.Data {
		provider := m.OwnedBy
		if provider == "" {
			provider = "platform"
		}

		models = append(models, Model{
			ID:          m.ID,
			Name:        m.ID,
			Provider:    provider,
			Description: "", // OpenAI models endpoint doesn't provide description
			Status:      "available",
			ContextSize: 4096, // Default value - would need to be looked up from model metadata
		})
	}

	return models, nil
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
