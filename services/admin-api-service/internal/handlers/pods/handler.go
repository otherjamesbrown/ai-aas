// Package pods provides HTTP handlers for pod health operations.
package pods

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

// Handler handles pod health HTTP requests
type Handler struct {
	service Service
}

// Service defines the interface for pod health operations
type Service interface {
	GetPodHealth(ctx context.Context, opts ListOptions) (*PodHealthResponse, error)
}

// NewHandler creates a new pods handler
func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

// GetPodHealth handles GET /v1/pods/health
func (h *Handler) GetPodHealth(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	opts := ListOptions{
		Namespace:      r.URL.Query().Get("namespace"),
		ModelName:      r.URL.Query().Get("model_name"),
		Environment:    r.URL.Query().Get("environment"),
		IncludeHealthy: parseBoolParam(r.URL.Query().Get("include_healthy"), true),
	}

	// Default namespace to "system" if not provided
	if opts.Namespace == "" {
		opts.Namespace = "system"
	}

	// Get pod health from service
	response, err := h.service.GetPodHealth(r.Context(), opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve pod health", err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

// parseBoolParam parses a boolean query parameter with a default value
func parseBoolParam(value string, defaultValue bool) bool {
	if value == "" {
		return defaultValue
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return defaultValue
	}

	return parsed
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	errResponse := map[string]interface{}{
		"error": message,
	}

	if err != nil {
		errResponse["details"] = err.Error()
	}

	json.NewEncoder(w).Encode(errResponse)
}

// ErrorResponse represents an API error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// String returns a string representation of the error
func (e ErrorResponse) String() string {
	if e.Details != "" {
		return fmt.Sprintf("%s: %s", e.Error, e.Details)
	}
	return e.Error
}
