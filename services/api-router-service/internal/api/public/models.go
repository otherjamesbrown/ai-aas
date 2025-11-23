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
// Returns a list of available models based on registered backends.
func (h *Handler) HandleModels(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("handling models list request",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path))

	// Get all registered backend IDs
	backendIDs := h.backendRegistry.ListBackends()

	// Build model list from backends
	models := make([]ModelObject, 0, len(backendIDs))
	createdTimestamp := time.Now().Unix()

	for _, backendID := range backendIDs {
		backend, err := h.backendRegistry.GetBackend(backendID)
		if err != nil {
			h.logger.Warn("failed to get backend",
				zap.String("backend_id", backendID),
				zap.Error(err))
			continue
		}

		// Use backend ID as model ID (e.g., "vllm-gpt-oss-20b")
		models = append(models, ModelObject{
			ID:      backend.ID,
			Object:  "model",
			Created: createdTimestamp,
			OwnedBy: "ai-aas",
		})
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
