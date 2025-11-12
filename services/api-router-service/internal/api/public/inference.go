// Package public provides public API handlers for inference requests.
//
// Purpose:
//   This package implements the public-facing API endpoints for the router service,
//   including request validation, DTOs, and response formatting.
//
// Dependencies:
//   - internal/auth: Authentication middleware
//   - internal/routing: Backend routing logic
//
package public

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// InferenceRequest represents an inbound inference request.
type InferenceRequest struct {
	RequestID      string                 `json:"request_id"`
	Model          string                 `json:"model"`
	Payload        string                 `json:"payload"`
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
	ContentType    string                 `json:"content_type,omitempty"`
	Metadata       map[string]string      `json:"metadata,omitempty"`
	HMACSignature  string                 `json:"hmac_signature,omitempty"`
}

// Validate validates the inference request and returns an error if invalid.
func (r *InferenceRequest) Validate() error {
	if r.RequestID == "" {
		return fmt.Errorf("request_id is required")
	}
	if _, err := uuid.Parse(r.RequestID); err != nil {
		return fmt.Errorf("request_id must be a valid UUID: %w", err)
	}

	if r.Model == "" {
		return fmt.Errorf("model is required")
	}

	if r.Payload == "" {
		return fmt.Errorf("payload is required")
	}

	if len(r.Payload) > 65536 { // 64 KB limit
		return fmt.Errorf("payload exceeds maximum size of 64 KB")
	}

	if r.ContentType != "" && r.ContentType != "text/plain" && r.ContentType != "application/json" {
		return fmt.Errorf("content_type must be 'text/plain' or 'application/json'")
	}

	return nil
}

// InferenceResponse represents the response from an inference request.
type InferenceResponse struct {
	RequestID string                 `json:"request_id"`
	Output    map[string]interface{} `json:"output"`
	Usage     *UsageSummary          `json:"usage,omitempty"`
	TraceID   string                 `json:"trace_id,omitempty"`
	SpanID    string                 `json:"span_id,omitempty"`
}

// UsageSummary contains usage metrics for the inference request.
type UsageSummary struct {
	TokensInput  int     `json:"tokens_input"`
	TokensOutput int     `json:"tokens_output"`
	LatencyMS    int     `json:"latency_ms"`
	CostUSD      float64 `json:"cost_usd,omitempty"`
	LimitState   string  `json:"limit_state,omitempty"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error    string `json:"error"`
	Code     string `json:"code"`
	TraceID  string `json:"trace_id,omitempty"`
}

// NewErrorResponse creates a new error response.
func NewErrorResponse(err error, code string) *ErrorResponse {
	return &ErrorResponse{
		Error: err.Error(),
		Code:  code,
	}
}

// WriteJSON writes a JSON response with the given status code.
// This is a helper function for writing JSON responses.
func WriteJSON(w http.ResponseWriter, statusCode int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(v)
}

// RoutingContext contains context for routing decisions.
type RoutingContext struct {
	RequestID      string
	OrganizationID string
	APIKeyID       string
	Model          string
	ReceivedAt     time.Time
}

