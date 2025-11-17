// Package public provides HTTP handlers for the public API.
//
// Purpose:
//   This package implements HTTP handlers for inference requests, including
//   request validation, authentication, routing, and response formatting.
//
package public

import (
	"context"
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
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/telemetry"
)

// Handler handles public API requests.
type Handler struct {
	logger         *zap.Logger
	authenticator  *auth.Authenticator
	configLoader   *config.Loader
	backendClient  *routing.BackendClient
	backendRegistry *config.BackendRegistry
	routingEngine  *routing.Engine
	routingMetrics *telemetry.RoutingMetrics
	usageHook      *UsageHook
	tracer         trace.Tracer
	errorBuilder   *api.ErrorBuilder
	backendURIs    map[string]string // Map of backend ID to URI (for testing/configuration - overrides registry)
	httpClient     *http.Client      // Shared HTTP client for OpenAI requests (PR#16 Issue#4)
}

// NewHandler creates a new public API handler.
func NewHandler(
	logger *zap.Logger,
	authenticator *auth.Authenticator,
	configLoader *config.Loader,
	backendClient *routing.BackendClient,
	backendRegistry *config.BackendRegistry,
	routingEngine *routing.Engine,
	routingMetrics *telemetry.RoutingMetrics,
	usageHook *UsageHook,
) *Handler {
	tracer := otel.Tracer("api-router-service")
	return &Handler{
		logger:          logger,
		authenticator:   authenticator,
		configLoader:    configLoader,
		backendClient:   backendClient,
		backendRegistry: backendRegistry,
		routingEngine:   routingEngine,
		routingMetrics:  routingMetrics,
		usageHook:       usageHook,
		tracer:          tracer,
		errorBuilder:   api.NewErrorBuilder(tracer),
		backendURIs:     make(map[string]string),
		httpClient: &http.Client{
			// Shared client without timeout - we'll use context for per-request timeouts (PR#16 Issue#4)
			Timeout: 0,
		},
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
	// OpenAI-compatible endpoints
	r.Post("/v1/chat/completions", h.HandleOpenAIChatCompletions)
	r.Post("/v1/completions", h.HandleOpenAICompletions)
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

	// Use routing engine for intelligent routing with failover
	var backendResp *routing.BackendResponse
	var routingDecision *routing.RoutingDecision
	var routingErr error

	if h.routingEngine != nil {
		// Use routing engine for intelligent routing
		backendResp, routingDecision, routingErr = h.routingEngine.RouteWithFailover(ctx, policy, backendReq, h.backendClient)
	} else {
		// Fallback to simple routing if engine not available
		h.logger.Warn("routing engine not available, using fallback routing")
		backendResp, routingDecision, routingErr = h.fallbackRouting(ctx, policy, backendReq)
	}

	if routingErr != nil {
		// Record error metrics
		if routingDecision != nil {
			telemetry.RecordBackendError(
				routingDecision.BackendID,
				authCtx.OrganizationID,
				req.Model,
				"routing_failed",
			)
		}
		h.writeError(w, r, fmt.Errorf("routing failed: %w", routingErr), api.ErrCodeBackendError)
		return
	}

	if backendResp == nil {
		h.writeError(w, r, fmt.Errorf("no backend response"), api.ErrCodeBackendError)
		return
	}

	// Record routing metrics if available
	if h.routingMetrics != nil && routingDecision != nil {
		decisionLatency := time.Since(startTime)
		h.routingMetrics.RecordRoutingDecision(
			routingDecision.BackendID,
			routingDecision.DecisionType,
			true, // success
			decisionLatency,
		)
	}

	// Record per-backend metrics
	if routingDecision != nil {
		requestLatency := time.Since(startTime)
		telemetry.RecordBackendRequest(
			routingDecision.BackendID,
			authCtx.OrganizationID,
			req.Model,
			true, // success
			requestLatency,
		)
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
	if routingDecision != nil {
		w.Header().Set("X-Routing-Backend", routingDecision.BackendID)
		w.Header().Set("X-Routing-Decision", routingDecision.DecisionType)
	}

	// Emit usage record if usage hook is available
	if h.usageHook != nil && routingDecision != nil {
		decisionReason := routingDecision.DecisionType
		if routingDecision.AttemptNumber > 1 {
			decisionReason = "FAILOVER"
		}
		
		_ = h.usageHook.EmitUsage(
			ctx,
			authCtx,
			req.RequestID,
			req.Model,
			routingDecision.BackendID,
			decisionReason,
			response.Usage.TokensInput,
			response.Usage.TokensOutput,
			response.Usage.LatencyMS,
			response.Usage.LimitState,
			span.SpanContext(),
			routingDecision.AttemptNumber-1, // retry count
		)
	}

	// Write response
	if err := h.writeJSON(w, http.StatusOK, response); err != nil {
		h.logger.Error("failed to write response", zap.Error(err))
	}
}

// fallbackRouting provides simple routing when routing engine is not available.
func (h *Handler) fallbackRouting(
	ctx context.Context,
	policy *config.RoutingPolicy,
	backendReq *routing.BackendRequest,
) (*routing.BackendResponse, *routing.RoutingDecision, error) {
	if len(policy.Backends) == 0 {
		return nil, nil, fmt.Errorf("no backends configured")
	}

	// Try backends in order
	for i, backendWeight := range policy.Backends {
		backend := h.buildBackendEndpoint(backendWeight.BackendID, policy.Model)
		decisionType := "PRIMARY"
		if i > 0 {
			decisionType = "FAILOVER"
		}

		response, err := h.backendClient.ForwardRequest(ctx, backend, backendReq)
		if err == nil {
			decision := &routing.RoutingDecision{
				BackendID:     backend.ID,
				DecisionType:  decisionType,
				Reason:        fmt.Sprintf("fallback routing (attempt %d)", i+1),
				Timestamp:     time.Now(),
				AttemptNumber: i + 1,
			}
			return response, decision, nil
		}

		h.logger.Warn("backend request failed in fallback routing",
			zap.String("backend_id", backend.ID),
			zap.Int("attempt", i+1),
			zap.Error(err),
		)
	}

	return nil, nil, fmt.Errorf("all backends failed")
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

