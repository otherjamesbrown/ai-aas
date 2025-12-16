// Package public provides HTTP handlers for the public API.
package public

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ModelObject represents a single model in the OpenAI format.
type ModelObject struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// ModelsResponse represents the response from GET /v1/models (OpenAI-compatible).
type ModelsResponse struct {
	Object string        `json:"object"`
	Data   []ModelObject `json:"data"`
}

// AdminAPIModelResponse represents a single model from Admin API /v1/models.
// This matches the Model struct returned by admin-api-service.
type AdminAPIModelResponse struct {
	Name             string  `json:"name"`
	HFModelID        string  `json:"hf_model_id"`
	CacheStatus      *string `json:"cache_status"`
	DeploymentStatus *string `json:"deployment_status"`
}

// fetchModelsFromAdminAPI fetches models from the Admin API service.
func (h *Handler) fetchModelsFromAdminAPI(ctx context.Context) ([]AdminAPIModelResponse, error) {
	if h.adminAPIEndpoint == "" {
		return nil, fmt.Errorf("admin API endpoint not configured")
	}

	url := fmt.Sprintf("%s/v1/models", h.adminAPIEndpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Add admin API key for authentication
	if h.adminAPIKey != "" {
		req.Header.Set("Authorization", "Bearer "+h.adminAPIKey)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("admin api returned %d: %s", resp.StatusCode, string(body))
	}

	// Admin API returns an array directly, not wrapped in an object
	var models []AdminAPIModelResponse
	if err := json.NewDecoder(resp.Body).Decode(&models); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return models, nil
}

// HandleModels handles GET /v1/models requests (OpenAI-compatible).
// Returns a list of available models by calling the Admin API service.
// Only returns enabled models for the current environment.
func (h *Handler) HandleModels(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("handling models list request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path))

	var models []ModelObject
	createdTimestamp := time.Now().Unix()

	// Fetch models from Admin API
	adminModels, err := h.fetchModelsFromAdminAPI(r.Context())
	if err != nil {
		h.logger.Error("failed to fetch models from admin api",
			zap.Error(err))
		// Fall back to empty list rather than failing the request
		models = make([]ModelObject, 0)
	} else {
		// Convert all models to OpenAI format
		// Models in the registry are available for routing - actual availability
		// depends on backend configuration in routing policies
		models = make([]ModelObject, 0, len(adminModels))
		for _, adminModel := range adminModels {
			models = append(models, ModelObject{
				ID:      adminModel.Name,
				Object:  "model",
				Created: createdTimestamp,
				OwnedBy: "ai-aas",
			})
		}
	}

	response := ModelsResponse{
		Object: "list",
		Data:   models,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error("failed to encode models response",
			zap.Error(err),
			zap.Int("model_count", len(models)))
		return
	}

	h.logger.Debug("models list request completed",
		zap.Int("model_count", len(models)))
}
