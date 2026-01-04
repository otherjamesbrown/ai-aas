// Package public provides HTTP handlers for the public API.
//
// Purpose:
//
//	This package implements HTTP handlers for inference requests, including
//	request validation, authentication, routing, and response formatting.
package public

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/preprocessor"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/triton"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/limiter"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/telemetry"
)

// Handler handles public API requests.
type Handler struct {
	logger            *zap.Logger
	authenticator     *auth.Authenticator
	configLoader      *config.Loader
	backendClient     *routing.BackendClient
	backendRegistry   *config.BackendRegistry
	routingEngine     *routing.Engine
	routingMetrics    *telemetry.RoutingMetrics
	tokenMetrics      *telemetry.TokenMetrics
	usageHook         *UsageHook
	rateLimiter       *limiter.RateLimiter        // Rate limiter for org-level token quota tracking
	tokenLimiter      *limiter.CachedTokenLimiter // Token limiter for per-user rate limits (spec035)
	tracer            trace.Tracer
	errorBuilder      *api.ErrorBuilder
	backendURIs       map[string]string // Map of backend ID to URI (for testing/configuration - overrides registry)
	httpClient        *http.Client      // Shared HTTP client for OpenAI requests (PR#16 Issue#4)
	userOrgServiceURL string            // URL for user-org-service (for auth proxy)
	adminAPIEndpoint  string            // URL for admin-api-service (for models endpoint)
	adminAPIKey       string            // API key for admin-api-service
	adminAPIClient    config.AdminAPIClient // Admin API client for policy fallback (aas-q34ls)
	defaultTimeout    time.Duration     // Default backend timeout

	// Triton protocol translation support (spec032)
	tritonTranslators map[string]*triton.Translator // keyed by tokenizer encoding
	translatorMu      sync.RWMutex

	// Triton gRPC streaming support (spec030-grpc)
	grpcClients     *gRPCClientManager     // gRPC client connections
	grpcTranslators *gRPCTranslatorManager // gRPC protocol translators

	// Preprocessor service support (for model-specific chat templates)
	preprocessorClients *preprocessor.ClientManager

	// Models cache (to avoid hitting Admin API rate limits)
	modelsCache     []AdminAPIModelResponse
	modelsCacheTime time.Time
	modelsCacheMu   sync.RWMutex
	modelsCacheTTL  time.Duration
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
	tokenMetrics *telemetry.TokenMetrics,
	usageHook *UsageHook,
	rateLimiter *limiter.RateLimiter,
	adminAPIEndpoint string,
	adminAPIKey string,
	defaultTimeout time.Duration,
	modelsCacheTTL time.Duration,
) *Handler {
	tracer := otel.Tracer("api-router-service")

	// Initialize admin API client for policy fallback (aas-q34ls)
	var adminAPIClient config.AdminAPIClient
	if adminAPIEndpoint != "" {
		adminAPIClient = config.NewHTTPAdminAPIClient(adminAPIEndpoint, adminAPIKey, logger)
	}

	return &Handler{
		logger:              logger,
		authenticator:       authenticator,
		configLoader:        configLoader,
		backendClient:       backendClient,
		backendRegistry:     backendRegistry,
		routingEngine:       routingEngine,
		routingMetrics:      routingMetrics,
		tokenMetrics:        tokenMetrics,
		usageHook:           usageHook,
		rateLimiter:         rateLimiter,
		tracer:              tracer,
		errorBuilder:        api.NewErrorBuilder(tracer),
		backendURIs:         make(map[string]string),
		adminAPIEndpoint:    adminAPIEndpoint,
		adminAPIKey:         adminAPIKey,
		adminAPIClient:      adminAPIClient,
		defaultTimeout:      defaultTimeout,
		tritonTranslators:   make(map[string]*triton.Translator),
		grpcClients:         newGRPCClientManager(logger),
		grpcTranslators:     newGRPCTranslatorManager(),
		preprocessorClients: preprocessor.NewClientManager(logger),
		modelsCacheTTL:      modelsCacheTTL,
		httpClient: &http.Client{
			// Shared client without timeout - we'll use context for per-request timeouts (PR#16 Issue#4)
			Timeout: 0,
		},
	}
}

// Close cleans up handler resources, including gRPC client connections.
func (h *Handler) Close() {
	if h.grpcClients != nil {
		h.grpcClients.close()
	}
	if h.preprocessorClients != nil {
		_ = h.preprocessorClients.Close()
	}
}

// getOrCreateTranslator returns a cached Triton translator or creates one for the encoding.
// Uses double-checked locking for thread-safe lazy initialization.
func (h *Handler) getOrCreateTranslator(encoding string) (*triton.Translator, error) {
	// Fast path: check with read lock
	h.translatorMu.RLock()
	if t, ok := h.tritonTranslators[encoding]; ok {
		h.translatorMu.RUnlock()
		return t, nil
	}
	h.translatorMu.RUnlock()

	// Slow path: acquire write lock and create translator
	h.translatorMu.Lock()
	defer h.translatorMu.Unlock()

	// Double-check after acquiring write lock
	if t, ok := h.tritonTranslators[encoding]; ok {
		return t, nil
	}

	t, err := triton.NewTranslator(encoding)
	if err != nil {
		return nil, fmt.Errorf("create translator for encoding %q: %w", encoding, err)
	}

	h.tritonTranslators[encoding] = t
	h.logger.Info("created triton translator",
		zap.String("encoding", encoding),
	)

	return t, nil
}

// SetBackendURI sets the URI for a backend ID (useful for testing).
// This overrides the backend registry for the given backend ID.
func (h *Handler) SetBackendURI(backendID, uri string) {
	if h.backendURIs == nil {
		h.backendURIs = make(map[string]string)
	}
	h.backendURIs[backendID] = uri
}

// SetUserOrgServiceURL sets the URL for user-org-service (for auth proxy).
func (h *Handler) SetUserOrgServiceURL(url string) {
	h.userOrgServiceURL = url
}

// SetTokenLimiter sets the per-user token limiter for rate limiting (spec035).
func (h *Handler) SetTokenLimiter(tokenLimiter *limiter.CachedTokenLimiter) {
	h.tokenLimiter = tokenLimiter
}

// RegisterRoutes registers public API routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/v1/inference", h.HandleInference)
	// OpenAI-compatible endpoints
	r.Get("/v1/models", h.HandleModels)
	r.Post("/v1/chat/completions", h.HandleOpenAIChatCompletions)
	r.Post("/v1/completions", h.HandleOpenAICompletions)
	// Authentication endpoints (proxied to user-org-service)
	r.Post("/v1/auth/login", h.HandleAuthProxy)
	r.Get("/v1/auth/userinfo", h.HandleAuthProxy)
	r.Post("/v1/auth/token", h.HandleAuthProxy)
	r.Post("/v1/auth/logout", h.HandleAuthProxy)
	r.Get("/v1/auth/callback", h.HandleAuthProxy)

	// Admin portal endpoints (proxied to user-org-service with JWT auth)
	// Organization and API key management
	r.Get("/organizations/me", h.HandleAdminProxy)
	r.Patch("/organizations/me", h.HandleAdminProxy)
	r.Get("/organizations/me/api-keys", h.HandleAdminProxy)
	r.Post("/organizations/me/api-keys", h.HandleAdminProxy)
	r.Get("/organizations/me/api-keys/{apiKeyId}", h.HandleAdminProxy)
	r.Patch("/organizations/me/api-keys/{apiKeyId}", h.HandleAdminProxy)
	r.Post("/organizations/me/api-keys/{apiKeyId}/rotate", h.HandleAdminProxy)
	r.Post("/organizations/me/api-keys/{apiKeyId}/revoke", h.HandleAdminProxy)
	r.Delete("/organizations/me/api-keys/{apiKeyId}", h.HandleAdminProxy)

	// Member management
	r.Get("/organizations/me/members", h.HandleAdminProxy)
	r.Post("/organizations/me/members", h.HandleAdminProxy)
	r.Patch("/organizations/me/members/{memberId}", h.HandleAdminProxy)
	r.Delete("/organizations/me/members/{memberId}", h.HandleAdminProxy)

	// Usage and budgets
	r.Get("/organizations/me/usage", h.HandleAdminProxy)
	r.Get("/organizations/me/budgets", h.HandleAdminProxy)
	r.Patch("/organizations/me/budgets", h.HandleAdminProxy)

	// Models (admin view)
	r.Get("/organizations/me/models", h.HandleAdminProxy)

	// Feature flags (local stub - returns sensible defaults)
	r.Get("/feature-flags", h.HandleFeatureFlags)

	// Support/impersonation status
	r.Get("/support/impersonations/current", h.HandleImpersonationStatus)
}

// HandleChatCompletionsHealth handles GET /v1/chat/completions/health requests.
// This endpoint is unauthenticated and designed for validation tools like guidellm
// that need to verify the endpoint is reachable without requiring an API key.
func (h *Handler) HandleChatCompletionsHealth(w http.ResponseWriter, r *http.Request) {
	_, span := h.tracer.Start(r.Context(), "health.chat_completions")
	defer span.End()

	h.logger.Debug("chat completions health check requested",
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("user_agent", r.UserAgent()),
	)

	// Return a simple success response
	response := map[string]interface{}{
		"status":    "ok",
		"service":   "api-router-service",
		"endpoint":  "/v1/chat/completions",
		"timestamp": time.Now().Unix(),
	}

	if err := h.writeJSON(w, http.StatusOK, response); err != nil {
		h.logger.Error("failed to write health response", zap.Error(err))
	}
}

// HandleInference handles POST /v1/inference requests.
func (h *Handler) HandleInference(w http.ResponseWriter, r *http.Request) {
	ctx, span := h.tracer.Start(r.Context(), "inference.request")
	defer span.End()

	startTime := time.Now()

	// Get authenticated context from middleware
	authCtxValue := r.Context().Value(AuthContextKey)
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

	// Get routing policy with intelligent fallback (aas-q34ls)
	policy, source, err := h.configLoader.GetPolicyWithFallback(ctx, authCtx.OrganizationID, req.Model, h.adminAPIClient)
	if err != nil {
		h.logger.Warn("no routing policy or deployment found",
			zap.String("org_id", authCtx.OrganizationID),
			zap.String("model", req.Model),
			zap.Error(err),
		)
		h.writeError(w, r, err, api.ErrCodeRoutingError)
		return
	}

	h.logger.Debug("routing policy resolved",
		zap.String("org_id", authCtx.OrganizationID),
		zap.String("model", req.Model),
		zap.String("source", source),
		zap.String("policy_id", policy.PolicyID),
	)

	// Record policy source metric (aas-q34ls)
	if h.routingMetrics != nil {
		h.routingMetrics.RecordPolicySource(authCtx.OrganizationID, req.Model, source)
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
			TokensInput:  len(req.Payload), // Simplified token counting
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
	timeout := h.defaultTimeout

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
		ID:           backendID,
		URI:          uri,
		ModelVariant: model,
		Timeout:      timeout,
	}
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

	_ = h.writeJSON(w, statusCode, response)
}

// writeJSON writes a JSON response.
func (h *Handler) writeJSON(w http.ResponseWriter, statusCode int, v interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	return json.NewEncoder(w).Encode(v)
}

// recordUserTokenUsage records token usage for per-user rate limiting (spec035).
// This is called asynchronously after a successful inference request.
// Service accounts are skipped since they bypass rate limits.
func (h *Handler) recordUserTokenUsage(ctx context.Context, authCtx *auth.AuthenticatedContext, totalTokens int) {
	if h.tokenLimiter == nil || authCtx == nil {
		return
	}

	// Service accounts bypass rate limits
	if authCtx.PrincipalType == "service_account" {
		return
	}

	// Record usage asynchronously to avoid blocking the response
	go func() {
		// Use a background context since the request context may be cancelled
		bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := h.tokenLimiter.RecordUsage(bgCtx, authCtx.PrincipalID, int64(totalTokens)); err != nil {
			h.logger.Warn("failed to record per-user token usage",
				zap.String("user_id", authCtx.PrincipalID),
				zap.Int("tokens", totalTokens),
				zap.Error(err),
			)
		}
	}()
}
