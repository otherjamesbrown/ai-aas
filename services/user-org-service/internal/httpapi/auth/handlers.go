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
//   - OrgID validation is basic (parsing only); full membership check TODO
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
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/bootstrap"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/oauth"
)

// RegisterRoutes mounts authentication routes beneath /v1/auth.
// Routes are only registered if runtime and OAuth provider are available.
func RegisterRoutes(router chi.Router, rt *bootstrap.Runtime) {
	if rt == nil || rt.Provider == nil {
		return
	}
	handler := &Handler{runtime: rt}
	router.Route("/v1/auth", func(r chi.Router) {
		r.Post("/login", handler.Login)
		r.Post("/refresh", handler.Refresh)
		r.Post("/logout", handler.Logout)
	})
}

// Handler serves authentication endpoints backed by Fosité.
type Handler struct {
	runtime *bootstrap.Runtime
}

type loginRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
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

	accessRequest, err := h.runtime.Provider.NewAccessRequest(ctx, req, session)
	if err != nil {
		h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
		return
	}

	// Validate and set org_id from the authenticated user
	if sess, ok := accessRequest.GetSession().(*oauth.Session); ok {
		userID := sess.Subject
		if userID == "" {
			h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
			return
		}

		// If org_id is provided, validate the user belongs to that org
		if payload.OrgID != "" {
			orgID, err := uuid.Parse(payload.OrgID)
			if err != nil {
				h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
				return
			}
			// TODO: Validate user belongs to org by querying the database
			// For now, we'll set it and rely on RLS policies to enforce isolation
			sess.OrgID = orgID.String()
		} else {
			// If no org_id provided, we need to look it up from the user
			// TODO: Lookup user's org_id from database using userID
		}
		sess.UserID = userID
	}

	response, err := h.runtime.Provider.NewAccessResponse(ctx, accessRequest)
	if err != nil {
		h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
		return
	}

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
	h.runtime.Provider.WriteRevocationResponse(ctx, w, err)
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
