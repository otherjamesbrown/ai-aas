// Package auth provides HTTP handlers for OAuth2 authentication endpoints.
//
// Purpose:
//   This package implements REST API handlers for user authentication flows:
//   login (resource owner password credentials), token refresh, and logout (token
//   revocation). Handlers translate JSON request payloads into Fosite OAuth2
//   flows and return standard OAuth2 token responses.
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
	"github.com/google/uuid"
	"github.com/ory/fosite"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/audit"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/metrics"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/oauth"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/security"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// RegisterRoutes mounts authentication routes beneath /v1/auth.
// Routes are only registered if runtime and OAuth provider are available.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime, idpRegistry *IdPRegistry) {
	if rt == nil || rt.Provider == nil {
		return
	}
	handler := &Handler{runtime: rt, idpRegistry: idpRegistry}
	router.Route("/v1/auth", func(r chi.Router) {
		r.Post("/login", handler.Login)
		r.Post("/refresh", handler.Refresh)
		r.Post("/logout", handler.Logout)
		
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
	runtime    *bootstrap.Runtime
	idpRegistry *IdPRegistry // IdP registry for OIDC federation (optional, nil if not configured)
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

	var payload loginRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid request payload", http.StatusBadRequest)
		return
	}

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

	req := cloneRequestWithForm(r, form)
	session := &oauth.Session{}

	// Track authentication attempt (before calling NewAccessRequest to catch all failures)
	email := strings.ToLower(strings.TrimSpace(payload.Email))
	var userUUID uuid.UUID // Will be populated if authentication succeeds

	accessRequest, err := h.runtime.Provider.NewAccessRequest(ctx, req, session)
	if err != nil {
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
		h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
		return
	}

	// Validate and set org_id from the authenticated user, then enforce MFA
	if sess, ok := accessRequest.GetSession().(*oauth.Session); ok {
		userID := sess.Subject
		if userID == "" {
			metrics.RecordAuthFailure("password", "invalid_session")
			h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
			return
		}

		userUUID, err = uuid.Parse(userID)
		if err != nil {
			h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
			return
		}

		// Resolve org_id
		var orgID uuid.UUID
		if payload.OrgID != "" {
			orgID, err = uuid.Parse(payload.OrgID)
			if err != nil {
				h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
				return
			}
			// Validate user belongs to org by querying the database
			if err := h.runtime.Postgres.ValidateUserOrgMembership(ctx, userUUID, orgID); err != nil {
				// Return authentication error (don't reveal org membership details)
				h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
				return
			}
		} else {
			// If no org_id provided, look it up from the user
			orgID, err = h.runtime.Postgres.GetUserOrgIDByUserID(ctx, userUUID)
			if err != nil {
				h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
				return
			}
		}
		sess.OrgID = orgID.String()
		sess.UserID = userID

		// MFA Enforcement: Check if MFA is required and verify code
		mfaStart := time.Now()
		mfaVerified, err := h.enforceMFA(ctx, userUUID, orgID, payload.MFACode)
		mfaDuration := time.Since(mfaStart).Seconds()
		if err != nil {
			// Record MFA failure
			metrics.RecordMFAFailure(mfaDuration)
			// Return MFA error (invalid code or MFA required but not provided)
			h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
			return
		}
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
	}

	response, err := h.runtime.Provider.NewAccessResponse(ctx, accessRequest)
	if err != nil {
		h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
		return
	}

	// Clear failed attempt counter on successful authentication
	if h.runtime.LockoutTracker != nil && userUUID != uuid.Nil {
		_ = h.runtime.LockoutTracker.ClearAttempts(ctx, email, userUUID)
	}

	// Record successful authentication and session creation
	metrics.RecordAuthSuccess("password")
	metrics.RecordSessionCreated()

	h.runtime.Provider.WriteAccessResponse(ctx, w, accessRequest, response)
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
	req := r.Clone(r.Context())
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Method = http.MethodPost
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Form = form
	req.PostForm = form
	body := form.Encode()
	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))
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
