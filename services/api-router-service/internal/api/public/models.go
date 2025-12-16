// Package public provides HTTP handlers for the public API.
package public

import (
	"encoding/json"
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

// HandleModels handles GET /v1/models requests (OpenAI-compatible).
// Returns a list of available models based on the model registry.
// Only returns models with deployment_status = 'ready' and a valid deployment_endpoint.
func (h *Handler) HandleModels(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("handling models list request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path))

	// Get ready models from model registry
	var models []ModelObject
	createdTimestamp := time.Now().Unix()

	// If ModelRegistry is available, query it
	if h.modelRegistry != nil {
		entries, err := h.modelRegistry.ListReadyModels(r.Context())
		if err != nil {
			h.logger.Error("failed to list ready models from registry",
				zap.Error(err))
			// Fall back to empty list rather than failing the request
			models = make([]ModelObject, 0)
		} else {
			models = make([]ModelObject, 0, len(entries))
			for _, entry := range entries {
				models = append(models, ModelObject{
					ID:      entry.ModelName,
					Object:  "model",
					Created: createdTimestamp,
					OwnedBy: "ai-aas",
				})
			}
		}
	} else {
		// Fallback: If ModelRegistry is not configured, return empty list
		h.logger.Warn("model registry not configured, returning empty model list")
		models = make([]ModelObject, 0)
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
