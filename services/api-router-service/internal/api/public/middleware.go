// Package public provides middleware for rate limiting and budget enforcement.
//
// Purpose:
//
//	This package implements chi middleware for rate limiting and budget checking
//	that runs before request handlers.
package public

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/limiter"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/telemetry"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/usage"
)

// Context key type to avoid collisions with other packages
type contextKey string

// Exported context keys for use by handlers
const (
	AuthContextKey  contextKey = "auth_context"
	BufferedBodyKey contextKey = "buffered_body"
	ModelKey        contextKey = "model"
)

// RateLimitMiddleware creates middleware for rate limiting.
func RateLimitMiddleware(rateLimiter *limiter.RateLimiter, auditLogger *usage.AuditLogger, logger *zap.Logger, tracer trace.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get authenticated context (should be set by auth middleware)
			authCtx := r.Context().Value(AuthContextKey)
			if authCtx == nil {
				// No auth context, skip rate limiting (will fail in auth middleware)
				next.ServeHTTP(w, r)
				return
			}

			authContext, ok := authCtx.(*auth.AuthenticatedContext)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// Check organization rate limit
			orgResult, err := rateLimiter.CheckOrganization(r.Context(), authContext.OrganizationID)
			if err != nil {
				logger.Warn("rate limit check failed, allowing request",
					zap.String("org_id", authContext.OrganizationID),
					zap.Error(err),
				)
				next.ServeHTTP(w, r)
				return
			}

			if !orgResult.Allowed {
				// Emit audit event
				if auditLogger != nil {
					auditLogger.LogDenial(usage.AuditEvent{
						RequestID:      getRequestID(r),
						OrganizationID: authContext.OrganizationID,
						APIKeyID:       authContext.APIKeyID,
						Model:          getModelFromRequest(r),
						Action:         "REQUEST_DENIED",
						DecisionReason: "RATE_LIMIT_EXCEEDED",
						LimitState:     "RATE_LIMITED",
					})
				}
				// Record Prometheus metric
				telemetry.RecordRateLimitDenial("org")
				errorBuilder := api.NewErrorBuilder(tracer)
				writeRateLimitError(w, r, orgResult, logger, errorBuilder)
				return
			}

			// Check API key rate limit (use same limits as org for now)
			keyResult, err := rateLimiter.CheckAPIKey(r.Context(), authContext.APIKeyID, 0, 0)
			if err != nil {
				logger.Warn("API key rate limit check failed, allowing request",
					zap.String("api_key_id", authContext.APIKeyID),
					zap.Error(err),
				)
				next.ServeHTTP(w, r)
				return
			}

			if !keyResult.Allowed {
				// Emit audit event
				if auditLogger != nil {
					auditLogger.LogDenial(usage.AuditEvent{
						RequestID:      getRequestID(r),
						OrganizationID: authContext.OrganizationID,
						APIKeyID:       authContext.APIKeyID,
						Model:          getModelFromRequest(r),
						Action:         "REQUEST_DENIED",
						DecisionReason: "RATE_LIMIT_EXCEEDED",
						LimitState:     "RATE_LIMITED",
					})
				}
				// Record Prometheus metric
				telemetry.RecordRateLimitDenial("key")
				errorBuilder := api.NewErrorBuilder(tracer)
				writeRateLimitError(w, r, keyResult, logger, errorBuilder)
				return
			}

			// Add rate limit headers
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(orgResult.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(orgResult.Remaining))

			next.ServeHTTP(w, r)
		})
	}
}

// isInferencePath returns true if the path is an inference endpoint that should
// have quota enforcement. Admin and user-org proxy routes bypass quota checks.
func isInferencePath(path string) bool {
	// Inference endpoints that SHOULD have quota enforcement
	inferencePathPrefixes := []string{
		"/v1/chat/completions",
		"/v1/completions",
		"/v1/inference",
		"/v1/embeddings",
	}

	for _, prefix := range inferencePathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}

	return false
}

// BudgetMiddleware creates middleware for budget/quota checking.
// Only applies to inference endpoints - admin and user-org operations bypass quota checks.
func BudgetMiddleware(budgetClient *limiter.BudgetClient, auditLogger *usage.AuditLogger, logger *zap.Logger, tracer trace.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip quota checks for non-inference paths (admin, user-org operations)
			if !isInferencePath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Get authenticated context
			authCtx := r.Context().Value(AuthContextKey)
			if authCtx == nil {
				next.ServeHTTP(w, r)
				return
			}

			authContext, ok := authCtx.(*auth.AuthenticatedContext)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// Get API key from header for test scenarios
			apiKey := r.Header.Get("X-API-Key")

			// Check budget/quota
			budgetStatus, err := budgetClient.CheckBudgetWithKey(r.Context(), authContext.OrganizationID, apiKey)
			if err != nil {
				logger.Warn("budget check failed, allowing request",
					zap.String("org_id", authContext.OrganizationID),
					zap.Error(err),
				)
				next.ServeHTTP(w, r)
				return
			}

			if !budgetStatus.Allowed {
				// Emit audit event
				if auditLogger != nil {
					decisionReason := "BUDGET_EXCEEDED"
					if budgetStatus.QuotaType == "daily_quota" || budgetStatus.QuotaType == "monthly_quota" {
						decisionReason = "QUOTA_EXCEEDED"
					}
					auditLogger.LogDenial(usage.AuditEvent{
						RequestID:      getRequestID(r),
						OrganizationID: authContext.OrganizationID,
						APIKeyID:       authContext.APIKeyID,
						Model:          getModelFromRequest(r),
						Action:         "REQUEST_DENIED",
						DecisionReason: decisionReason,
						LimitState:     decisionReason,
					})
				}
				// Record Prometheus metrics
				if budgetStatus.QuotaType == "budget" {
					telemetry.RecordBudgetDenial(budgetStatus.QuotaType)
				} else {
					telemetry.RecordQuotaDenial(budgetStatus.QuotaType)
				}
				errorBuilder := api.NewErrorBuilder(tracer)
				writeBudgetError(w, r, budgetStatus, logger, errorBuilder)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// BodyBufferMiddleware buffers the request body so it can be read multiple times.
// This is needed for HMAC verification and model extraction in middleware.
func BodyBufferMiddleware(maxSize int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only buffer POST/PUT/PATCH requests with bodies
			if r.Method != "POST" && r.Method != "PUT" && r.Method != "PATCH" {
				next.ServeHTTP(w, r)
				return
			}

			if r.Body == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Read the body
			body, err := io.ReadAll(io.LimitReader(r.Body, maxSize))
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}

			// Check if body exceeds max size
			if int64(len(body)) >= maxSize {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}

			// Restore the body for downstream handlers
			r.Body = io.NopCloser(bytes.NewReader(body))

			// Store buffered body in context for HMAC verification
			ctx := context.WithValue(r.Context(), BufferedBodyKey, body)

			// Try to extract model from body and store in context
			var req struct {
				Model string `json:"model"`
			}
			if err := json.Unmarshal(body, &req); err == nil && req.Model != "" {
				ctx = context.WithValue(ctx, ModelKey, req.Model)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AuthContextMiddleware extracts auth context and adds it to request context.
// Public endpoints that don't require authentication will skip auth and proceed without an auth context.
// JWT-authenticated endpoints (using Bearer token) also skip API key auth.
func AuthContextMiddleware(authenticator *auth.Authenticator, logger *zap.Logger, tracer trace.Tracer) func(http.Handler) http.Handler {
	// Public endpoints that don't require API key authentication
	// Note: Some endpoints use JWT (Bearer token) auth instead of API key
	publicEndpoints := map[string]bool{
		"GET /v1/models":        true,
		"POST /v1/auth/login":   true, // Password-based login
		"GET /v1/auth/userinfo": true, // User info endpoint (requires Bearer token, not API key)
		"POST /v1/auth/token":   true, // Token exchange/refresh
		"POST /v1/auth/logout":  true, // Logout endpoint
		"GET /v1/auth/callback": true, // OAuth callback
		// Feature flags endpoint (returns defaults, no auth required)
		"GET /feature-flags": true,
		// Impersonation status (returns 404 if no active session)
		"GET /support/impersonations/current": true,
	}

	// JWT-authenticated endpoints (use Bearer token, skip API key middleware)
	// These endpoints proxy to user-org-service with JWT auth
	jwtAuthPrefixes := []string{
		"/organizations/me",
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if this is a public endpoint
			endpointKey := r.Method + " " + r.URL.Path
			if publicEndpoints[endpointKey] {
				logger.Debug("skipping authentication for public endpoint",
					zap.String("path", r.URL.Path),
					zap.String("method", r.Method))
				next.ServeHTTP(w, r)
				return
			}

			// Check if this is a JWT-authenticated endpoint (uses Bearer token)
			// These endpoints proxy to user-org-service which handles JWT validation
			for _, prefix := range jwtAuthPrefixes {
				if strings.HasPrefix(r.URL.Path, prefix) {
					logger.Debug("skipping API key auth for JWT-authenticated endpoint",
						zap.String("path", r.URL.Path),
						zap.String("method", r.Method))
					next.ServeHTTP(w, r)
					return
				}
			}

			authCtx, err := authenticator.Authenticate(r)
			if err != nil {
				logger.Debug("authentication failed in middleware",
					zap.Error(err),
					zap.String("path", r.URL.Path),
					zap.String("method", r.Method))
				errorBuilder := api.NewErrorBuilder(tracer)
				response := errorBuilder.BuildError(r.Context(), err, api.ErrCodeAuthInvalid)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(api.GetHTTPStatus(api.ErrCodeAuthInvalid))
				_ = json.NewEncoder(w).Encode(response)
				return
			}

			logger.Debug("authentication successful",
				zap.String("org_id", authCtx.OrganizationID),
				zap.String("api_key_id", authCtx.APIKeyID))

			// Add auth context to request context
			ctx := context.WithValue(r.Context(), AuthContextKey, authCtx)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// writeRateLimitError writes a rate limit error response using the error catalog.
func writeRateLimitError(w http.ResponseWriter, r *http.Request, result *limiter.CheckResult, logger *zap.Logger, errorBuilder *api.ErrorBuilder) {
	retryAfterSeconds := int(result.RetryAfter.Seconds())
	if retryAfterSeconds <= 0 {
		retryAfterSeconds = 1
	}

	// Set Retry-After header
	w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))

	limitContext := map[string]interface{}{
		"current_usage": result.Limit - result.Remaining,
		"limit":         result.Limit,
		"remaining":     result.Remaining,
	}

	response := errorBuilder.BuildLimitError(
		r.Context(),
		api.NewError(api.ErrCodeRateLimitExceeded, "Rate limit exceeded"),
		api.ErrCodeRateLimitExceeded,
		&retryAfterSeconds,
		limitContext,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(api.GetHTTPStatus(api.ErrCodeRateLimitExceeded))
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to write rate limit error response", zap.Error(err))
	}
}

// writeBudgetError writes a budget/quota error response using the error catalog.
func writeBudgetError(w http.ResponseWriter, r *http.Request, status *limiter.BudgetStatus, logger *zap.Logger, errorBuilder *api.ErrorBuilder) {
	errorCode := getBudgetErrorCode(status.QuotaType)

	limitContext := map[string]interface{}{
		"current_usage": status.CurrentUsage,
		"limit":         status.Limit,
		"quota_type":    status.QuotaType,
	}

	response := errorBuilder.BuildLimitError(
		r.Context(),
		api.NewError(errorCode, status.Reason),
		errorCode,
		nil, // No retry after for budget errors
		limitContext,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(api.GetHTTPStatus(errorCode))
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.Error("failed to write budget error response", zap.Error(err))
	}
}

// getBudgetErrorCode returns the error code for a quota type using error catalog constants.
func getBudgetErrorCode(quotaType string) string {
	switch quotaType {
	case "budget":
		return api.ErrCodeBudgetExceeded
	case "daily_quota", "monthly_quota":
		return api.ErrCodeQuotaExceeded
	default:
		return api.ErrCodeBudgetExceeded
	}
}

// LimitErrorResponse is now defined in internal/api/errors.go
// This type alias is kept for backward compatibility but should use api.LimitErrorResponse
type LimitErrorResponse = api.LimitErrorResponse

// getRequestID extracts request ID from request context or header.
func getRequestID(r *http.Request) string {
	// Try to get from context (set by handler after parsing request body)
	if reqID := r.Context().Value("request_id"); reqID != nil {
		if id, ok := reqID.(string); ok {
			return id
		}
	}
	// Fallback to header
	if reqID := r.Header.Get("X-Request-ID"); reqID != "" {
		return reqID
	}
	// Fallback to chi request ID middleware
	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		return reqID
	}
	return ""
}

// getModelFromRequest extracts model from request context.
// The model is extracted from the buffered request body by BodyBufferMiddleware.
func getModelFromRequest(r *http.Request) string {
	// Try to get from context (set by BodyBufferMiddleware after parsing request body)
	if model := r.Context().Value("model"); model != nil {
		if m, ok := model.(string); ok {
			return m
		}
	}
	return ""
}

// TokenQuotaMiddleware creates middleware for token-based quota checking.
// Checks token usage quotas (hourly/daily/weekly) before allowing requests.
// Token quotas are configured per organization and checked in real-time using Redis counters.
func TokenQuotaMiddleware(rateLimiter *limiter.RateLimiter, tokenLimits limiter.TokenQuotaLimits, auditLogger *usage.AuditLogger, logger *zap.Logger, tracer trace.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip quota checks for non-inference paths (admin, user-org operations)
			if !isInferencePath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			// Get authenticated context
			authCtx := r.Context().Value(AuthContextKey)
			if authCtx == nil {
				next.ServeHTTP(w, r)
				return
			}

			authContext, ok := authCtx.(*auth.AuthenticatedContext)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// Check token quota (only if limits are configured)
			if tokenLimits.Hourly > 0 || tokenLimits.Daily > 0 || tokenLimits.Weekly > 0 {
				status, err := rateLimiter.CheckTokenQuota(r.Context(), authContext.OrganizationID, tokenLimits)
				if err != nil {
					logger.Warn("token quota check failed, allowing request",
						zap.String("org_id", authContext.OrganizationID),
						zap.Error(err),
					)
					next.ServeHTTP(w, r)
					return
				}

				if status.Exceeded {
					// Emit audit event
					if auditLogger != nil {
						auditLogger.LogDenial(usage.AuditEvent{
							RequestID:      getRequestID(r),
							OrganizationID: authContext.OrganizationID,
							APIKeyID:       authContext.APIKeyID,
							Model:          getModelFromRequest(r),
							Action:         "REQUEST_DENIED",
							DecisionReason: "TOKEN_QUOTA_EXCEEDED",
							LimitState:     "TOKEN_QUOTA_EXCEEDED",
						})
					}

					// Record Prometheus metric
					telemetry.RecordQuotaDenial(status.Period + "_token_quota")

					// Calculate retry-after in seconds
					retryAfterSeconds := int(time.Until(status.ResetAt).Seconds())
					if retryAfterSeconds <= 0 {
						retryAfterSeconds = 60 // Default to 1 minute
					}

					// Set headers
					w.Header().Set("Retry-After", strconv.Itoa(retryAfterSeconds))
					w.Header().Set("X-RateLimit-Limit-Tokens", strconv.Itoa(status.Limit))
					w.Header().Set("X-RateLimit-Remaining-Tokens", strconv.Itoa(status.Limit-status.CurrentUsage))
					w.Header().Set("X-RateLimit-Reset", status.ResetAt.Format(time.RFC3339))

					limitContext := map[string]interface{}{
						"current_usage": status.CurrentUsage,
						"limit":         status.Limit,
						"period":        status.Period,
						"reset_at":      status.ResetAt.Format(time.RFC3339),
					}

					errorBuilder := api.NewErrorBuilder(tracer)
					response := errorBuilder.BuildLimitError(
						r.Context(),
						api.NewError(api.ErrCodeQuotaExceeded, fmt.Sprintf("Token quota exceeded for %s period", status.Period)),
						api.ErrCodeQuotaExceeded,
						&retryAfterSeconds,
						limitContext,
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(api.GetHTTPStatus(api.ErrCodeQuotaExceeded))
					if err := json.NewEncoder(w).Encode(response); err != nil {
						logger.Error("failed to write token quota error response", zap.Error(err))
					}
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ModelAccessMiddleware creates middleware for user-level model access control.
// This middleware checks if the authenticated user has access to the requested model.
// Requires BodyBufferMiddleware to run first to extract the model from the request body.
//
// Spec reference: 022-user-model-access-control
func ModelAccessMiddleware(auditLogger *usage.AuditLogger, logger *zap.Logger, tracer trace.Tracer, featureEnabled bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip if feature is disabled
			if !featureEnabled {
				next.ServeHTTP(w, r)
				return
			}

			// Get authenticated context
			authCtx := r.Context().Value(AuthContextKey)
			if authCtx == nil {
				next.ServeHTTP(w, r)
				return
			}

			authContext, ok := authCtx.(*auth.AuthenticatedContext)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			// Get model from request (set by BodyBufferMiddleware)
			model := getModelFromRequest(r)
			if model == "" {
				// No model specified, skip check (will fail validation in handler)
				next.ServeHTTP(w, r)
				return
			}

			// Check model access
			if !authContext.CanAccessModel(model) {
				logger.Warn("model access denied",
					zap.String("user_id", authContext.PrincipalID),
					zap.String("org_id", authContext.OrganizationID),
					zap.String("model", model),
					zap.String("access_mode", authContext.ModelAccessMode),
				)

				// Emit audit event
				if auditLogger != nil {
					auditLogger.LogDenial(usage.AuditEvent{
						RequestID:      getRequestID(r),
						OrganizationID: authContext.OrganizationID,
						APIKeyID:       authContext.APIKeyID,
						Model:          model,
						Action:         "REQUEST_DENIED",
						DecisionReason: "MODEL_ACCESS_DENIED",
						LimitState:     "MODEL_ACCESS_DENIED",
					})
				}

				// Record metric
				telemetry.RecordModelAccessDenial(model, authContext.OrganizationID)

				// Write error response
				errorBuilder := api.NewErrorBuilder(tracer)
				response := errorBuilder.BuildError(r.Context(),
					api.NewError(api.ErrCodeAccessDenied, "Access to model denied"),
					api.ErrCodeAccessDenied)

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(api.GetHTTPStatus(api.ErrCodeAccessDenied))
				if err := json.NewEncoder(w).Encode(response); err != nil {
					logger.Error("failed to write model access error response", zap.Error(err))
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
