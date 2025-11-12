// Package public provides HTTP handlers for the public API.
//
// Purpose:
//   This package implements HTTP handlers for inference requests, including
//   request validation, authentication, routing, and response formatting.
//
package public

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
)

// Handler handles public API requests.
type Handler struct {
	logger         *zap.Logger
	authenticator  *auth.Authenticator
	configLoader   *config.Loader
	backendClient  *routing.BackendClient
	backendRegistry *config.BackendRegistry
	tracer         trace.Tracer
	errorBuilder   *api.ErrorBuilder
	backendURIs    map[string]string // Map of backend ID to URI (for testing/configuration - overrides registry)
}

// NewHandler creates a new public API handler.
func NewHandler(
	logger *zap.Logger,
	authenticator *auth.Authenticator,
	configLoader *config.Loader,
	backendClient *routing.BackendClient,
	backendRegistry *config.BackendRegistry,
) *Handler {
	tracer := otel.Tracer("api-router-service")
	return &Handler{
		logger:          logger,
		authenticator:   authenticator,
		configLoader:    configLoader,
		backendClient:   backendClient,
		backendRegistry: backendRegistry,
		tracer:          tracer,
		errorBuilder:   api.NewErrorBuilder(tracer),
		backendURIs:     make(map[string]string),
	}
}

// SetBackendURI sets the URI for a backend ID (useful for testing).
// This overrides the backend registry for the given backend ID.
func (h *Handler) SetBackendURI(backendID, uri string) {
	if h.backendURIs == nil {
		h.backendURIs = make(map[string]string)
	}
	h.backendURIs[backendID] = uri
}

// RegisterRoutes registers public API routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/v1/inference", h.HandleInference)
}

// HandleInference handles POST /v1/inference requests.
func (h *Handler) HandleInference(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "inference.request")
	defer span.End()

	startTime := time.Now()

	// Get authenticated context from middleware
	authCtxValue := r.Context().Value("auth_context")
	if authCtxValue == nil {
		h.writeError(w, r, fmt.Errorf("authentication required"), api.ErrCodeAuthInvalid)
		return
	}

	authCtx, ok := authCtxValue.(*auth.AuthenticatedContext)
	if !ok {
		h.writeError(w, r, fmt.Errorf("invalid authentication context"), api.ErrCodeAuthInvalid)
		return
	}

	// Parse request body
	var req InferenceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, fmt.Errorf("invalid request body: %w", err), api.ErrCodeInvalidRequest)
		return
	}

	// Validate request
	if err := req.Validate(); err != nil {
		h.writeError(w, r, err, api.ErrCodeValidationError)
		return
	}

	// Get routing policy
	policy, err := h.configLoader.GetPolicy(authCtx.OrganizationID, req.Model)
	if err != nil {
		h.logger.Warn("no routing policy found, using default",
			zap.String("org_id", authCtx.OrganizationID),
			zap.String("model", req.Model),
		)
		// TODO: Use default routing policy
		h.writeError(w, r, fmt.Errorf("no routing policy configured"), api.ErrCodeRoutingError)
		return
	}

	// Prepare backend request
	backendReq := &routing.BackendRequest{
		Prompt:     req.Payload,
		Parameters: req.Parameters,
	}

	// Try backends with failover
	var backendResp *routing.BackendResponse
	var backend *routing.BackendEndpoint
	var routingDecision string
	var lastErr error

	// Get available backends (excluding degraded)
	availableBackends := h.getAvailableBackends(policy)
	if len(availableBackends) == 0 {
		h.writeError(w, r, fmt.Errorf("no available backend"), api.ErrCodeNoBackendAvailable)
		return
	}

	// Try each backend until one succeeds or all fail
	for i, backendWeight := range availableBackends {
		backend = h.buildBackendEndpoint(backendWeight.BackendID, policy.Model)
		
		if i == 0 {
			routingDecision = "PRIMARY"
		} else {
			routingDecision = "FAILOVER"
		}

		backendResp, lastErr = h.backendClient.ForwardRequest(ctx, backend, backendReq)
		if lastErr == nil {
			// Success - break out of retry loop
			break
		}

		h.logger.Warn("backend request failed, trying failover",
			zap.String("backend_id", backend.ID),
			zap.Int("attempt", i+1),
			zap.Int("total_backends", len(availableBackends)),
			zap.Error(lastErr),
		)

		// If this was the last backend, we'll return the error
		if i == len(availableBackends)-1 {
			h.writeError(w, r, fmt.Errorf("all backends failed, last error: %w", lastErr), api.ErrCodeBackendError)
			return
		}
	}

	if backendResp == nil {
		h.writeError(w, r, fmt.Errorf("backend request failed: %w", lastErr), api.ErrCodeBackendError)
		return
	}

	// Build response
	latency := time.Since(startTime)
	response := InferenceResponse{
		RequestID: req.RequestID,
		Output: map[string]interface{}{
			"text": backendResp.Text,
		},
		Usage: &UsageSummary{
			TokensInput: len(req.Payload), // Simplified token counting
			TokensOutput: backendResp.TokensUsed,
			LatencyMS:    int(latency.Milliseconds()),
			LimitState:   "WITHIN_LIMIT",
		},
		TraceID: span.SpanContext().TraceID().String(),
		SpanID:  span.SpanContext().SpanID().String(),
	}

	// Add routing headers
	w.Header().Set("X-Routing-Backend", backend.ID)
	w.Header().Set("X-Routing-Decision", routingDecision)

	// Write response
	if err := h.writeJSON(w, http.StatusOK, response); err != nil {
		h.logger.Error("failed to write response", zap.Error(err))
	}
}

// getAvailableBackends returns available backends (excluding degraded ones) in weighted order.
func (h *Handler) getAvailableBackends(policy *config.RoutingPolicy) []config.BackendWeight {
	if len(policy.Backends) == 0 {
		return nil
	}

	// Filter out degraded backends
	availableBackends := make([]config.BackendWeight, 0)
	degradedMap := make(map[string]bool)
	for _, degradedID := range policy.DegradedBackends {
		degradedMap[degradedID] = true
	}

	for _, backend := range policy.Backends {
		if !degradedMap[backend.BackendID] {
			availableBackends = append(availableBackends, backend)
		}
	}

	if len(availableBackends) == 0 {
		// All backends are degraded, fall back to all backends
		availableBackends = policy.Backends
	}

	// Sort by weight (descending) for failover order
	// Higher weight backends are tried first
	for i := 0; i < len(availableBackends)-1; i++ {
		for j := i + 1; j < len(availableBackends); j++ {
			if availableBackends[i].Weight < availableBackends[j].Weight {
				availableBackends[i], availableBackends[j] = availableBackends[j], availableBackends[i]
			}
		}
	}

	return availableBackends
}

// selectBackend selects a backend from the routing policy using weighted selection.
// Implements weighted random selection based on backend weights.
// This is used for initial selection; failover logic handles retries.
func (h *Handler) selectBackend(policy *config.RoutingPolicy) *routing.BackendEndpoint {
	availableBackends := h.getAvailableBackends(policy)
	if len(availableBackends) == 0 {
		return nil
	}

	// Calculate total weight
	totalWeight := 0
	for _, backend := range availableBackends {
		totalWeight += backend.Weight
	}

	if totalWeight == 0 {
		// No weights specified, use first backend
		return h.buildBackendEndpoint(availableBackends[0].BackendID, policy.Model)
	}

	// Weighted random selection using crypto/rand
	selectedWeight, err := randomInt64(int64(totalWeight))
	if err != nil {
		// Fallback to time-based if crypto/rand fails
		selectedWeight = time.Now().UnixNano() % int64(totalWeight)
	}

	currentWeight := 0
	var selected config.BackendWeight
	for _, backend := range availableBackends {
		currentWeight += backend.Weight
		if int64(currentWeight) > selectedWeight {
			selected = backend
			break
		}
	}

	// Fallback to first backend if selection didn't work
	if selected.BackendID == "" {
		selected = availableBackends[0]
	}

	return h.buildBackendEndpoint(selected.BackendID, policy.Model)
}

// buildBackendEndpoint constructs a BackendEndpoint from a backend ID.
func (h *Handler) buildBackendEndpoint(backendID, model string) *routing.BackendEndpoint {
	var uri string
	var timeout time.Duration = 30 * time.Second

	// Check test override first (for testing)
	if h.backendURIs != nil {
		if overrideURI := h.backendURIs[backendID]; overrideURI != "" {
			uri = overrideURI
		}
	}

	// If no override, try backend registry
	if uri == "" && h.backendRegistry != nil {
		if backendCfg, err := h.backendRegistry.GetBackend(backendID); err == nil {
			uri = backendCfg.URI
			if backendCfg.Timeout > 0 {
				timeout = backendCfg.Timeout
			}
		}
	}

	// Fallback to default (for backward compatibility)
	if uri == "" {
		h.logger.Warn("backend not found in registry, using default",
			zap.String("backend_id", backendID),
		)
		uri = "http://localhost:8001/v1/completions"
	}

	return &routing.BackendEndpoint{
		ID:          backendID,
		URI:         uri,
		ModelVariant: model,
		Timeout:     timeout,
	}
}

// randomInt64 generates a random int64 in the range [0, max).
func randomInt64(max int64) (int64, error) {
	if max <= 0 {
		return 0, fmt.Errorf("max must be positive")
	}

	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}

	val := binary.BigEndian.Uint64(buf[:])
	return int64(val % uint64(max)), nil
}

// writeError writes an error response using the error catalog.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error, code string) {
	statusCode := api.GetHTTPStatus(code)
	response := h.errorBuilder.BuildError(r.Context(), err, code)

	h.logger.Warn("request error",
		zap.Int("status", statusCode),
		zap.String("code", code),
		zap.Error(err),
	)

	h.writeJSON(w, statusCode, response)
}

// writeJSON writes a JSON response.
func (h *Handler) writeJSON(w http.ResponseWriter, statusCode int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(v)
}

