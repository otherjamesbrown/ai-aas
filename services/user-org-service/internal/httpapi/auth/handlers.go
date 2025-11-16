// Package auth provides HTTP handlers for OAuth2 authentication endpoints.
//
// Purpose:
//
//	This package implements REST API handlers for user authentication flows:
//	login (resource owner password credentials), token refresh, and logout (token
//	revocation). Handlers translate JSON request payloads into Fosite OAuth2
//	flows and return standard OAuth2 token responses.
//
// Dependencies:
//   - github.com/go-chi/chi/v5: HTTP router for route registration
//   - github.com/google/uuid: UUID parsing for org_id validation
//   - internal/bootstrap: Runtime dependencies (OAuth provider, config)
//   - internal/oauth: OAuth session types
//
// Key Responsibilities:
//   - Login: Resource owner password credentials grant (POST /v1/auth/login)
//   - Refresh: Refresh token exchange (POST /v1/auth/refresh)
//   - Logout: Token revocation (POST /v1/auth/logout)
//   - Request transformation: JSON → form-urlencoded for Fosite compatibility
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-001 (User Authentication)
//   - specs/005-user-org-service/spec.md#FR-005 (OAuth2 Support)
//   - specs/005-user-org-service/contracts/user-org-service.openapi.yaml (API contract)
//
// Debugging Notes:
//   - All handlers accept JSON but convert to form-urlencoded for Fosite
//   - Default client credentials used if not provided in request
//   - OrgID validation checks user membership in the specified org via database query
//   - If org_id is not provided, it is looked up from the user's record
//   - Errors are written via Fosite's WriteAccessError/WriteRevocationResponse
//   - Session org_id and user_id are set from request payload and authenticated user
//
// Thread Safety:
//   - Handler methods are safe for concurrent use (stateless, uses runtime dependencies)
//
// Error Handling:
//   - Invalid JSON returns 400 Bad Request
//   - OAuth errors are written via Fosite's error writers (proper OAuth2 error format)
//   - Context errors propagate from Fosite provider
package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/ory/fosite"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/httpapi/middleware"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/metrics"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/oauth"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts authentication routes beneath /v1/auth.
// Routes are only registered if runtime and OAuth provider are available.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, idpRegistry *IdPRegistry, logger *zap.Logger) {
	if rt == nil || rt.Provider == nil {
		return
	}
	handler := &Handler{runtime: rt, idpRegistry: idpRegistry, logger: logger}
	router.Route("/v1/auth", func(r chi.Router) {
		r.Post("/login", handler.Login)
		r.Post("/refresh", handler.Refresh)
		r.Post("/logout", handler.Logout)

		// User info endpoint (requires authentication)
		// Register as a route group with auth middleware
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAuth(rt, logger))
			r.Get("/userinfo", handler.UserInfo)
		})

		// OIDC/IdP federation routes
		r.Get("/oidc/{provider}/login", handler.OIDCLogin)
		r.Get("/oidc/{provider}/callback", handler.OIDCCallback)

		// Recovery routes
		r.Post("/recover", handler.InitiateRecovery)
		r.Post("/recover/verify", handler.VerifyRecoveryToken)
		r.Post("/recover/reset", handler.ResetPassword)

		// Admin recovery approval routes (require authentication)
		r.Post("/recover/approve", handler.ApproveRecovery)
		r.Post("/recover/reject", handler.RejectRecovery)

		// API key validation (public endpoint for service-to-service auth)
		r.Post("/validate-api-key", handler.ValidateAPIKey)
	})
}

// Handler serves authentication endpoints backed by Fosité.
type Handler struct {
	runtime     *bootstrap.Runtime
	idpRegistry *IdPRegistry // IdP registry for OIDC federation (optional, nil if not configured)
	logger      *zap.Logger
}

type loginRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	MFACode      string `json:"mfaCode,omitempty"` // TOTP code for MFA verification
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scope        string `json:"scope"`
	OrgID        string `json:"org_id"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Scope        string `json:"scope"`
}

type logoutRequest struct {
	Token         string `json:"token"`
	TokenTypeHint string `json:"token_type_hint"`
	ClientID      string `json:"client_id"`
	ClientSecret  string `json:"client_secret"`
}

// Login handles the resource-owner password credentials flow.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	requestID := chimiddleware.GetReqID(ctx)
	logger := h.logger.With(zap.String("request_id", requestID), zap.String("handler", "Login"))

	logger.Info("=== LOGIN REQUEST START ===",
		zap.String("origin", r.Header.Get("Origin")),
		zap.String("user_agent", r.Header.Get("User-Agent")),
		zap.String("content_type", r.Header.Get("Content-Type")),
		zap.String("content_length", r.Header.Get("Content-Length")))

	var payload loginRequest
	logger.Debug("about to decode JSON payload")
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		logger.Error("failed to decode login request payload", zap.Error(err))
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}
	logger.Info("login payload decoded successfully",
		zap.String("email", payload.Email),
		zap.String("org_id", payload.OrgID),
		zap.String("client_id", payload.ClientID),
		zap.Bool("has_password", payload.Password != ""),
		zap.String("payload_client_secret", payload.ClientSecret))

	logger.Debug("creating form data from payload")
	form := url.Values{}
	form.Set("grant_type", "password")
	form.Set("username", strings.TrimSpace(payload.Email))
	form.Set("password", payload.Password)
	clientID := payload.ClientID
	if clientID == "" {
		clientID = h.runtime.Config.OAuthClientID
	}
	clientSecret := payload.ClientSecret
	if clientSecret == "" {
		clientSecret = h.runtime.Config.OAuthClientSecret
	}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	scope := payload.Scope
	if scope == "" {
		scope = strings.Join([]string{"openid", "profile", "email"}, " ")
	}
	form.Set("scope", scope)
	logger.Info("form data created",
		zap.String("grant_type", "password"),
		zap.String("username", strings.TrimSpace(payload.Email)),
		zap.Bool("has_password", payload.Password != ""),
		zap.String("client_id", clientID),
		zap.String("client_id_source", func() string {
			if payload.ClientID != "" {
				return "payload"
			} else {
				return "config"
			}
		}()),
		zap.String("client_secret_source", func() string {
			if payload.ClientSecret != "" {
				return "payload"
			} else {
				return "config"
			}
		}()),
		zap.Bool("has_client_secret", clientSecret != ""),
		zap.String("scope", scope))

	logger.Debug("cloning request with form data")
	req := cloneRequestWithForm(r, form)
	logger.Info("request cloned successfully",
		zap.String("req_method", req.Method),
		zap.String("req_url", req.URL.String()),
		zap.String("req_host", req.Host),
		zap.String("content_type", req.Header.Get("Content-Type")),
		zap.String("content_length", fmt.Sprintf("%d", req.ContentLength)),
		zap.String("user_agent", req.Header.Get("User-Agent")),
		zap.String("origin", req.Header.Get("Origin")))

	session := &oauth.Session{}
	logger.Debug("oauth session created")

	// Track authentication attempt (before calling NewAccessRequest to catch all failures)
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	var userUUID uuid.UUID // Will be populated if authentication succeeds

	// Log form data that will be parsed by Fosite
	logger.Info("form data to be parsed by Fosite",
		zap.String("grant_type", form.Get("grant_type")),
		zap.String("username", form.Get("username")),
		zap.String("client_id", form.Get("client_id")),
		zap.Bool("has_password", form.Get("password") != ""),
		zap.Bool("has_client_secret", form.Get("client_secret") != ""),
		zap.String("scope", form.Get("scope")))

	logger.Info("=== CALLING NEWACCESSREQUEST ===",
		zap.String("email", email),
		zap.String("runtime_provider_nil", fmt.Sprintf("%v", h.runtime.Provider == nil)),
		zap.String("ctx_nil", fmt.Sprintf("%v", ctx == nil)),
		zap.String("ctx_done", fmt.Sprintf("%v", ctx.Err())),
		zap.String("req_body_readable", fmt.Sprintf("%v", req.Body != nil)))
	// Use context.Background() instead of the request context to prevent cancellation
	// when Playwright or other clients close the connection prematurely
	// The cloned request already uses context.Background(), so this ensures consistency
	accessRequest, err := h.runtime.Provider.NewAccessRequest(context.Background(), req, session)
	logger.Info("NewAccessRequest completed", zap.Error(err))
	if err != nil {
		logger.Error("NewAccessRequest failed",
			zap.Error(err),
			zap.String("email", email),
			zap.String("error_type", fmt.Sprintf("%T", err)))
		// Track failed attempt by email
		if h.runtime.LockoutTracker != nil {
			count, shouldLockout, trackErr := h.runtime.LockoutTracker.TrackFailedAttempt(ctx, email)
			if trackErr == nil && shouldLockout {
				// Look up user by email to enforce lockout
				// We need to find the user to set lockout_until
				if orgIDParam := payload.OrgID; orgIDParam != "" {
					var orgID uuid.UUID
					if orgID, err = uuid.Parse(orgIDParam); err == nil {
						if user, lookupErr := h.runtime.Postgres.GetUserByEmail(ctx, orgID, email); lookupErr == nil {
							lockoutUntil := h.runtime.LockoutTracker.CalculateLockoutUntil()
							_, _ = h.runtime.Postgres.UpdateUserStatus(ctx, postgres.UpdateUserStatusParams{
								ID:           user.ID,
								OrgID:        orgID,
								Status:       user.Status,
								LockoutUntil: &lockoutUntil,
								Version:      user.Version,
							})
							// Emit audit event for lockout
							event := audit.BuildEvent(orgID, user.ID, audit.ActorTypeSystem, audit.ActionAccountLockout, audit.TargetTypeUser, &user.ID)
							event = audit.BuildEventFromRequest(event, r)
							event.Metadata = map[string]any{
								"failed_attempts": count,
								"lockout_until":   lockoutUntil.Format(time.RFC3339),
							}
							_ = h.runtime.Audit.Emit(ctx, event)
						}
					}
				}
			}
		}
		// Record authentication failure
		metrics.RecordAuthFailure("password", extractErrorReason(err))
		logger.Warn("authentication failed, writing error response",
			zap.Error(err),
			zap.String("email", email))
		h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
		return
	}

	logger.Debug("NewAccessRequest succeeded, validating session")

	// Use background context for all database operations after authentication
	// The original request context may be canceled (e.g., Playwright closing connection)
	// but we need to complete authentication and return the token regardless
	bgCtx := context.Background()

	// Validate and set org_id from the authenticated user, then enforce MFA
	if sess, ok := accessRequest.GetSession().(*oauth.Session); ok {
		logger.Debug("session type assertion succeeded", zap.String("session_type", "oauth.Session"))
		userID := sess.Subject
		logger.Debug("extracted user ID from session", zap.String("user_id", userID))
		if userID == "" {
			logger.Error("session subject is empty")
			metrics.RecordAuthFailure("password", "invalid_session")
			// Create a proper error for invalid session
			invalidSessionErr := fmt.Errorf("invalid session: missing subject")
			h.runtime.Provider.WriteAccessError(bgCtx, w, accessRequest, invalidSessionErr)
			return
		}

		logger.Debug("parsing user ID UUID", zap.String("user_id", userID))
		userUUID, err = uuid.Parse(userID)
		if err != nil {
			logger.Error("failed to parse user ID UUID",
				zap.Error(err),
				zap.String("user_id", userID))
			h.runtime.Provider.WriteAccessError(bgCtx, w, accessRequest, err)
			return
		}
		logger.Debug("user UUID parsed successfully", zap.String("user_uuid", userUUID.String()))

		// Resolve org_id
		var orgID uuid.UUID
		if payload.OrgID != "" {
			logger.Debug("org_id provided in payload, parsing", zap.String("org_id_param", payload.OrgID))
			orgID, err = uuid.Parse(payload.OrgID)
			if err != nil {
				logger.Error("failed to parse org_id UUID",
					zap.Error(err),
					zap.String("org_id_param", payload.OrgID))
				h.runtime.Provider.WriteAccessError(bgCtx, w, accessRequest, err)
				return
			}
			// Validate user belongs to org by querying the database
			logger.Debug("validating user org membership",
				zap.String("org_id", orgID.String()),
				zap.String("user_uuid", userUUID.String()))
			if err := h.runtime.Postgres.ValidateUserOrgMembership(bgCtx, userUUID, orgID); err != nil {
				logger.Error("user org membership validation failed",
					zap.Error(err),
					zap.String("org_id", orgID.String()),
					zap.String("user_uuid", userUUID.String()))
				// Return authentication error (don't reveal org membership details)
				h.runtime.Provider.WriteAccessError(bgCtx, w, accessRequest, err)
				return
			}
			logger.Debug("user org membership validated", zap.String("org_id", orgID.String()))
		} else {
			// If no org_id provided, look it up from the user
			logger.Debug("no org_id provided, looking up from user", zap.String("user_uuid", userUUID.String()))
			orgID, err = h.runtime.Postgres.GetUserOrgIDByUserID(bgCtx, userUUID)
			if err != nil {
				logger.Error("failed to get user org ID",
					zap.Error(err),
					zap.String("user_uuid", userUUID.String()))
				h.runtime.Provider.WriteAccessError(bgCtx, w, accessRequest, err)
				return
			}
			logger.Debug("user org ID retrieved", zap.String("org_id", orgID.String()))
		}
		sess.OrgID = orgID.String()
		sess.UserID = userID
		logger.Debug("session org_id and user_id set",
			zap.String("org_id", orgID.String()),
			zap.String("user_id", userID))

		// MFA Enforcement: Check if MFA is required and verify code
		logger.Debug("enforcing MFA", zap.Bool("has_mfa_code", payload.MFACode != ""))
		mfaStart := time.Now()
		mfaVerified, err := h.enforceMFA(bgCtx, userUUID, orgID, payload.MFACode)
		mfaDuration := time.Since(mfaStart).Seconds()
		if err != nil {
			logger.Error("MFA enforcement failed",
				zap.Error(err),
				zap.Duration("duration_seconds", time.Duration(mfaDuration*float64(time.Second))))
			// Record MFA failure
			metrics.RecordMFAFailure(mfaDuration)
			// Return MFA error (invalid code or MFA required but not provided)
			h.runtime.Provider.WriteAccessError(bgCtx, w, accessRequest, err)
			return
		}
		logger.Debug("MFA enforcement completed", zap.Bool("mfa_verified", mfaVerified))
		if mfaVerified {
			// Record MFA success
			metrics.RecordMFASuccess(mfaDuration)
			// Store MFA verification timestamp in session metadata
			// This will be serialized in the oauth_sessions.session_data JSONB field
			if sess.Extra == nil {
				sess.Extra = make(map[string]interface{})
			}
			sess.Extra["mfa_verified_at"] = time.Now().UTC().Format(time.RFC3339)
		}
	} else {
		// Session is not an oauth.Session - this should not happen if NewAccessRequest succeeded
		// but handle it gracefully
		sessionType := fmt.Sprintf("%T", accessRequest.GetSession())
		logger.Error("session type assertion failed: expected oauth.Session",
			zap.String("session_type", sessionType))
		metrics.RecordAuthFailure("password", "invalid_session_type")
		invalidSessionErr := fmt.Errorf("invalid session type: expected oauth.Session, got %s", sessionType)
		h.runtime.Provider.WriteAccessError(bgCtx, w, accessRequest, invalidSessionErr)
		return
	}

	logger.Debug("creating access response")
	response, err := h.runtime.Provider.NewAccessResponse(bgCtx, accessRequest)
	if err != nil {
		logger.Error("failed to create access response", zap.Error(err))
		h.runtime.Provider.WriteAccessError(bgCtx, w, accessRequest, err)
		return
	}
	logger.Debug("access response created successfully")

	// Clear failed attempt counter on successful authentication
	if h.runtime.LockoutTracker != nil && userUUID != uuid.Nil {
		logger.Debug("clearing lockout attempts",
			zap.String("user_uuid", userUUID.String()))
		_ = h.runtime.LockoutTracker.ClearAttempts(bgCtx, email, userUUID)
	}

	// Record successful authentication and session creation
	metrics.RecordAuthSuccess("password")
	metrics.RecordSessionCreated()

	logger.Info("login successful, writing access response",
		zap.String("email", email),
		zap.String("user_uuid", userUUID.String()))

	logger.Info("=== LOGIN REQUEST END - SUCCESS ===")
	h.runtime.Provider.WriteAccessResponse(bgCtx, w, accessRequest, response)

	// Explicitly flush the response to ensure it's sent to the client immediately
	// This is particularly important for Playwright/browser-based clients that may
	// close connections early or have different buffering behavior
	if flusher, ok := w.(http.Flusher); ok {
		logger.Debug("flushing HTTP response")
		flusher.Flush()
	}
	logger.Debug("login handler complete, response sent")
}

// Refresh exchanges a refresh token for a new access token.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var payload refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	form := url.Values{}
	form.Set("grant_type", "refresh_token")
	form.Set("refresh_token", payload.RefreshToken)
	clientID := payload.ClientID
	if clientID == "" {
		clientID = h.runtime.Config.OAuthClientID
	}
	clientSecret := payload.ClientSecret
	if clientSecret == "" {
		clientSecret = h.runtime.Config.OAuthClientSecret
	}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	if payload.Scope != "" {
		form.Set("scope", payload.Scope)
	}

	req := cloneRequestWithForm(r, form)
	session := &oauth.Session{}

	accessRequest, err := h.runtime.Provider.NewAccessRequest(ctx, req, session)
	if err != nil {
		h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
		return
	}

	response, err := h.runtime.Provider.NewAccessResponse(ctx, accessRequest)
	if err != nil {
		h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
		return
	}

	h.runtime.Provider.WriteAccessResponse(ctx, w, accessRequest, response)
}

// Logout revokes refresh or access tokens.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var payload logoutRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	form := url.Values{}
	form.Set("token", payload.Token)
	if payload.TokenTypeHint != "" {
		form.Set("token_type_hint", payload.TokenTypeHint)
	}
	clientID := payload.ClientID
	if clientID == "" {
		clientID = h.runtime.Config.OAuthClientID
	}
	clientSecret := payload.ClientSecret
	if clientSecret == "" {
		clientSecret = h.runtime.Config.OAuthClientSecret
	}
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req := cloneRequestWithForm(r, form)

	err := h.runtime.Provider.NewRevocationRequest(ctx, req)
	if err == nil {
		// Record session revocation
		metrics.RecordSessionRevoked()
	}
	h.runtime.Provider.WriteRevocationResponse(ctx, w, err)
}

// enforceMFA checks if MFA is required for the user and verifies the provided code.
// Returns (true, nil) if MFA is verified or not required.
// Returns (false, error) if MFA is required but code is invalid or missing.
func (h *Handler) enforceMFA(ctx context.Context, userID, orgID uuid.UUID, mfaCode string) (bool, error) {
	// Get user details to check MFA enrollment
	user, err := h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	if err != nil {
		if err == postgres.ErrNotFound {
			return false, fosite.ErrNotFound
		}
		return false, fmt.Errorf("get user: %w", err)
	}

	// Get org details to check MFA requirements
	org, err := h.runtime.Postgres.GetOrg(ctx, orgID)
	if err != nil {
		return false, fmt.Errorf("get org: %w", err)
	}

	// Check if MFA is required:
	// 1. User must be enrolled (mfa_enrolled = true)
	// 2. User must have TOTP method configured (mfa_methods includes "totp")
	// 3. Org may require MFA for specific roles (mfa_required_roles)
	mfaRequired := user.MFAEnrolled && contains(user.MFAMethods, "totp")

	// If org has MFA required roles, check if user's role requires MFA
	// TODO: Check user's actual roles once role system is implemented
	// For now, if org has any mfa_required_roles, we require MFA for all enrolled users
	if len(org.MFARequiredRoles) > 0 && user.MFAEnrolled {
		mfaRequired = true
	}

	if !mfaRequired {
		// MFA not required, proceed
		return false, nil
	}

	// MFA is required - verify code
	if mfaCode == "" {
		// Return error indicating MFA code is required
		return false, &fosite.RFC6749Error{
			ErrorField:       "mfa_required",
			DescriptionField: "MFA code is required for this account",
			HintField:        "Please provide a TOTP code",
			CodeField:        http.StatusBadRequest,
			DebugField:       "MFA is required but no code provided",
		}
	}

	// Verify TOTP code
	if user.MFASecret == nil || *user.MFASecret == "" {
		return false, fmt.Errorf("MFA secret not configured for user")
	}

	valid, err := security.VerifyTOTP(*user.MFASecret, mfaCode)
	if err != nil {
		return false, fmt.Errorf("verify TOTP: %w", err)
	}
	if !valid {
		// Invalid MFA code
		return false, &fosite.RFC6749Error{
			ErrorField:       "invalid_grant",
			DescriptionField: "Invalid MFA code",
			HintField:        "The provided TOTP code is invalid or expired",
			CodeField:        http.StatusBadRequest,
			DebugField:       "MFA code verification failed",
		}
	}

	// MFA verified successfully
	return true, nil
}

// contains checks if a string slice contains a specific value.
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

func cloneRequestWithForm(r *http.Request, form url.Values) *http.Request {
	// Create a new request with a background context to avoid cancellation issues
	// The original request context may be canceled (e.g., when Playwright closes connection),
	// but we need the cloned request to complete authentication regardless
	// Use the original URL directly to preserve scheme, host, path, and query
	body := form.Encode()
	
	// Build full URL string preserving scheme and host
	// r.URL might be relative, so we need to construct the full URL
	var fullURL string
	if r.URL.IsAbs() {
		fullURL = r.URL.String()
	} else {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		host := r.Host
		if host == "" {
			host = r.Header.Get("Host")
		}
		if host == "" {
			host = "localhost"
		}
		fullURL = fmt.Sprintf("%s://%s%s", scheme, host, r.URL.RequestURI())
	}
	
	// Use context.Background() instead of r.Context() to prevent cancellation
	// when the original request context is canceled (e.g., Playwright closing connection)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, fullURL, strings.NewReader(body))
	if err != nil {
		// Fallback to Clone if NewRequestWithContext fails (shouldn't happen)
		req = r.Clone(r.Context())
		req.Method = http.MethodPost
		req.Body = io.NopCloser(strings.NewReader(body))
		req.ContentLength = int64(len(body))
	} else {
		// Ensure URL is properly set
		if req.URL.Host == "" {
			req.URL.Host = r.Host
		}
		if req.URL.Scheme == "" {
			if r.TLS != nil {
				req.URL.Scheme = "https"
			} else {
				req.URL.Scheme = "http"
			}
		}
		req.Host = r.Host
	}
	
	// Preserve important headers from original request
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	// Copy all headers from original request (preserves Origin, User-Agent, etc.)
	for k, v := range r.Header {
		req.Header[k] = v
	}
	// Override Content-Type for form data
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	
	// Set content length
	req.ContentLength = int64(len(body))
	
	// Preserve RemoteAddr and other connection info (for logging/debugging)
	req.RemoteAddr = r.RemoteAddr
	
	// Don't set Form/PostForm directly - let Fosite parse from body
	// This ensures compatibility with Fosite's internal parsing logic
	// Fosite will call ParseForm() internally, which reads from req.Body
	
	return req
}

// extractErrorReason extracts a human-readable reason from an error for metrics labeling.
func extractErrorReason(err error) string {
	if err == nil {
		return "unknown"
	}

	// Check for Fosite errors
	if fositeErr, ok := err.(*fosite.RFC6749Error); ok {
		return fositeErr.ErrorField
	}

	// Check for common error patterns
	errStr := err.Error()
	if strings.Contains(errStr, "invalid") {
		return "invalid_credentials"
	}
	if strings.Contains(errStr, "not found") || strings.Contains(errStr, "ErrNotFound") {
		return "user_not_found"
	}
	if strings.Contains(errStr, "locked") {
		return "account_locked"
	}
	if strings.Contains(errStr, "mfa") {
		return "mfa_required"
	}

	return "unknown"
}

// UserInfo returns user information for the authenticated user.
// GET /v1/auth/userinfo
// Requires: Bearer token in Authorization header
// Returns: User information in OIDC userinfo format
func (h *Handler) UserInfo(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get user ID from authenticated context (set by RequireAuth middleware)
	userID := middleware.GetUserID(ctx)
	if userID == uuid.Nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get org ID from context
	orgID := middleware.GetOrgID(ctx)

	// Fetch user from database
	var user postgres.User
	var err error
	if orgID != uuid.Nil {
		// If org ID is available, fetch user from org
		user, err = h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	} else {
		// Fallback: get org ID from user ID first, then fetch user
		orgID, err = h.runtime.Postgres.GetUserOrgIDByUserID(ctx, userID)
		if err != nil {
			if err == postgres.ErrNotFound {
				http.Error(w, "user not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		user, err = h.runtime.Postgres.GetUserByID(ctx, orgID, userID)
	}

	if err != nil {
		if err == postgres.ErrNotFound {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Get user's organization if orgID is available
	var orgIDStr string
	if orgID != uuid.Nil {
		orgIDStr = orgID.String()
	} else if user.OrgID != uuid.Nil {
		orgIDStr = user.OrgID.String()
	}

	// Build userinfo response in OIDC format
	userInfo := map[string]interface{}{
		"sub":            user.ID.String(),
		"id":             user.ID.String(),
		"email":          user.Email,
		"name":           user.DisplayName,
		"organization_id": orgIDStr,
		"scopes":         []string{"openid", "profile", "email"},
	}

	// Add roles if available (TODO: implement role system)
	// For now, return empty array
	userInfo["roles"] = []string{}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(userInfo); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
}
