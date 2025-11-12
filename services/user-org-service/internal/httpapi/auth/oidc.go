// Package auth provides OIDC/IdP federation handlers.
//
// Purpose:
//   This package implements OIDC-based identity provider federation,
//   allowing users to authenticate via external providers (Google, GitHub, etc.)
//   and mapping them to internal user accounts via external_idp_id.
//
// Dependencies:
//   - github.com/coreos/go-oidc/v3/oidc: OIDC provider client
//   - github.com/go-chi/chi/v5: HTTP router
//   - internal/bootstrap: Runtime dependencies
//
// Key Responsibilities:
//   - OIDCLogin: Initiates OIDC flow (GET /v1/auth/oidc/{provider}/login)
//   - OIDCCallback: Handles OIDC callback (GET /v1/auth/oidc/{provider}/callback)
//   - Maps external IdP users to internal users via external_idp_id
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-006 (IdP Federation)
//
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/ory/fosite"
	"golang.org/x/oauth2"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/metrics"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/oauth"
	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// IdPProvider represents configuration for an external identity provider.
type IdPProvider struct {
	Name         string
	ClientID     string
	ClientSecret string
	IssuerURL    string
	RedirectURL  string
	Scopes       []string
	Provider     *oidc.Provider
	OAuth2Config *oauth2.Config
}

// IdPRegistry manages configured identity providers.
type IdPRegistry struct {
	providers map[string]*IdPProvider
}

// NewIdPRegistry creates a new IdP registry.
func NewIdPRegistry() *IdPRegistry {
	return &IdPRegistry{
		providers: make(map[string]*IdPProvider),
	}
}

// RegisterProvider adds an IdP provider to the registry.
func (r *IdPRegistry) RegisterProvider(name string, provider *IdPProvider) {
	r.providers[name] = provider
}

// GetProvider retrieves a provider by name.
func (r *IdPRegistry) GetProvider(name string) (*IdPProvider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %s not configured", name)
	}
	return provider, nil
}

// InitializeIdPProviders initializes OIDC providers from configuration.
// Loads provider credentials from config and sets up OIDC clients.
func InitializeIdPProviders(ctx context.Context, baseURL string, cfg *config.Config) (*IdPRegistry, error) {
	registry := NewIdPRegistry()

	// Google provider configuration
	if cfg.OIDCGoogleClientID != "" && cfg.OIDCGoogleClientSecret != "" {
		googleProvider, err := setupOIDCProvider(ctx, "google", cfg.OIDCGoogleClientID, cfg.OIDCGoogleClientSecret,
			"https://accounts.google.com", baseURL+"/v1/auth/oidc/google/callback",
			[]string{"openid", "profile", "email"})
		if err != nil {
			return nil, fmt.Errorf("setup google provider: %w", err)
		}
		registry.RegisterProvider("google", googleProvider)
	}

	// GitHub provider configuration
	// Note: GitHub's OIDC issuer is typically https://token.actions.githubusercontent.com for GitHub Actions
	// For regular GitHub OAuth, use https://github.com/login/oauth/authorize
	// For now, we'll use the standard GitHub OAuth endpoints
	if cfg.OIDCGithubClientID != "" && cfg.OIDCGithubClientSecret != "" {
		// GitHub doesn't support standard OIDC discovery, so we'll use OAuth2 endpoints
		// For OIDC-compliant providers, use: "https://token.actions.githubusercontent.com"
		// For regular GitHub OAuth, we need to handle it differently
		// For now, skip GitHub if not using Actions OIDC
		// TODO: Add support for GitHub OAuth2 (non-OIDC) flow
	}

	return registry, nil
}

func setupOIDCProvider(ctx context.Context, name, clientID, clientSecret, issuerURL, redirectURL string, scopes []string) (*IdPProvider, error) {
	provider, err := oidc.NewProvider(ctx, issuerURL)
	if err != nil {
		return nil, fmt.Errorf("create %s provider: %w", name, err)
	}

	oauth2Config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     provider.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       scopes,
	}

	return &IdPProvider{
		Name:         name,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		IssuerURL:    issuerURL,
		RedirectURL:  redirectURL,
		Scopes:       scopes,
		Provider:     provider,
		OAuth2Config: oauth2Config,
	}, nil
}

// OIDCLogin initiates the OIDC authentication flow.
// GET /v1/auth/oidc/{provider}/login?org_id={orgId}&redirect_uri={redirectUri}
func (h *Handler) OIDCLogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	providerName := chi.URLParam(r, "provider")

	// Get IdP registry from handler
	if h.idpRegistry == nil {
		http.Error(w, "IdP federation not configured", http.StatusServiceUnavailable)
		return
	}

	provider, err := h.idpRegistry.GetProvider(providerName)
	if err != nil {
		http.Error(w, fmt.Sprintf("provider %s not configured", providerName), http.StatusNotFound)
		return
	}

	// Get org_id and redirect_uri from query params
	orgIDParam := r.URL.Query().Get("org_id")
	redirectURI := r.URL.Query().Get("redirect_uri")
	if redirectURI == "" {
		redirectURI = "/"
	}

	// Parse org ID (UUID or slug)
	var orgID uuid.UUID
	if orgIDParam != "" {
		if orgID, err = uuid.Parse(orgIDParam); err != nil {
			// Try as slug
			org, err := h.runtime.Postgres.GetOrgBySlug(ctx, orgIDParam)
			if err != nil {
				http.Error(w, "organization not found", http.StatusNotFound)
				return
			}
			orgID = org.ID
		}
	}

	// Generate state token (CSRF protection)
	stateToken, err := generateStateToken()
	if err != nil {
		http.Error(w, "failed to generate state token", http.StatusInternalServerError)
		return
	}

	// Store state in session/cookie (stub: in production use secure cookie or Redis)
	// For now, encode org_id and redirect_uri in state
	stateData := map[string]string{
		"token":       stateToken,
		"org_id":      orgID.String(),
		"redirect_uri": redirectURI,
	}
	stateJSON, _ := json.Marshal(stateData)
	state := base64.RawURLEncoding.EncodeToString(stateJSON)

	// Build authorization URL
	authURL := provider.OAuth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline)

	// Record OIDC login attempt
	metrics.RecordOIDCLoginAttempt(providerName)

	// Redirect to IdP
	http.Redirect(w, r, authURL, http.StatusFound)
}

// OIDCCallback handles the OIDC callback after user authenticates with IdP.
// GET /v1/auth/oidc/{provider}/callback?code={code}&state={state}
func (h *Handler) OIDCCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	providerName := chi.URLParam(r, "provider")

	// Get IdP registry from handler
	if h.idpRegistry == nil {
		http.Error(w, "IdP federation not configured", http.StatusServiceUnavailable)
		return
	}

	provider, err := h.idpRegistry.GetProvider(providerName)
	if err != nil {
		http.Error(w, fmt.Sprintf("provider %s not configured", providerName), http.StatusNotFound)
		return
	}

	// Verify state token
	stateParam := r.URL.Query().Get("state")
	if stateParam == "" {
		http.Error(w, "missing state parameter", http.StatusBadRequest)
		return
	}

	stateJSON, err := base64.RawURLEncoding.DecodeString(stateParam)
	if err != nil {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)
		return
	}

	var stateData map[string]string
	if err := json.Unmarshal(stateJSON, &stateData); err != nil {
		http.Error(w, "invalid state parameter", http.StatusBadRequest)
		return
	}

	orgID, err := uuid.Parse(stateData["org_id"])
	if err != nil {
		http.Error(w, "invalid organization", http.StatusBadRequest)
		return
	}
	redirectURI := stateData["redirect_uri"]
	if redirectURI == "" {
		redirectURI = "/"
	}

	// Exchange authorization code for token
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing authorization code", http.StatusBadRequest)
		return
	}

	token, err := provider.OAuth2Config.Exchange(ctx, code)
	if err != nil {
		http.Error(w, "failed to exchange authorization code", http.StatusInternalServerError)
		return
	}

	// Extract ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "missing id_token in response", http.StatusInternalServerError)
		return
	}

	// Verify ID token
	verifier := provider.Provider.Verifier(&oidc.Config{ClientID: provider.ClientID})
	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		http.Error(w, "failed to verify ID token", http.StatusInternalServerError)
		return
	}

	// Extract claims
	var claims struct {
		Subject string `json:"sub"`
		Email   string `json:"email"`
		Name    string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		http.Error(w, "failed to extract claims", http.StatusInternalServerError)
		return
	}

	// Map external IdP user to internal user
	externalIDP := fmt.Sprintf("%s:%s", providerName, claims.Subject)
	user, err := h.findOrCreateUserFromIdP(ctx, orgID, externalIDP, claims.Email, claims.Name)
	if err != nil {
		metrics.RecordOIDCCallbackFailure(providerName, "user_creation_failed")
		http.Error(w, "failed to create or find user", http.StatusInternalServerError)
		return
	}

	// Create OAuth2 session (similar to login flow)
	session := &oauth.Session{
		DefaultSession: fosite.DefaultSession{
			Subject: user.ID.String(),
		},
		OrgID:  orgID.String(),
		UserID: user.ID.String(),
	}

	// Create an access request manually (bypassing password validation for IdP users)
	// We create a custom access request with the user session
	// Get the client from the provider's storage
	client, err := h.runtime.OAuthStore.GetClient(ctx, h.runtime.Config.OAuthClientID)
	if err != nil {
		http.Error(w, "failed to get OAuth client", http.StatusInternalServerError)
		return
	}

	// Create access request manually
	ar := fosite.NewAccessRequest(session)
	ar.Client = client
	ar.GrantTypes = fosite.Arguments{"client_credentials"}
	ar.RequestedScope = fosite.Arguments{"openid", "profile", "email"}
	ar.GrantedScope = fosite.Arguments{"openid", "profile", "email"}
	ar.Session = session
	accessRequest := ar

	// Generate access response
	response, err := h.runtime.Provider.NewAccessResponse(ctx, accessRequest)
	if err != nil {
		metrics.RecordOIDCCallbackFailure(providerName, "token_issuance_failed")
		h.runtime.Provider.WriteAccessError(ctx, w, accessRequest, err)
		return
	}

	// Record successful OIDC callback and session creation
	metrics.RecordOIDCCallbackSuccess(providerName)
	metrics.RecordSessionCreated()

	// Write tokens to response
	h.runtime.Provider.WriteAccessResponse(ctx, w, accessRequest, response)

	// Note: Tokens are returned in the response body (JSON format)
	// Frontend should extract tokens from response and store them
	// If redirect is needed, frontend can handle it after receiving tokens
}

// findOrCreateUserFromIdP finds an existing user by external_idp_id or creates a new one.
func (h *Handler) findOrCreateUserFromIdP(ctx context.Context, orgID uuid.UUID, externalIDP, email, displayName string) (postgres.User, error) {
	// Try to find existing user by external_idp_id first
	user, err := h.runtime.Postgres.GetUserByExternalIDP(ctx, orgID, externalIDP)
	if err == nil {
		// User found by external IdP ID
		return user, nil
	}
	if err != postgres.ErrNotFound {
		return postgres.User{}, err
	}

	// Try to find by email (user might exist but not have external_idp_id set)
	user, err = h.runtime.Postgres.GetUserByEmail(ctx, orgID, email)
	if err == nil {
		// User exists - update external_idp_id if not set
		if user.ExternalIDP == nil || *user.ExternalIDP == "" {
			updatedUser, err := h.runtime.Postgres.UpdateUserExternalIDP(ctx, orgID, user.ID, user.Version, externalIDP)
			if err != nil {
				if err == postgres.ErrOptimisticLock {
					// User was modified concurrently - try again
					return h.findOrCreateUserFromIdP(ctx, orgID, externalIDP, email, displayName)
				}
				return postgres.User{}, fmt.Errorf("update external IdP ID: %w", err)
			}
			return updatedUser, nil
		}
		return user, nil
	}

	if err != postgres.ErrNotFound {
		return postgres.User{}, err
	}

	// User doesn't exist - create new user
	// Generate a random password (user won't use password auth)
	passwordHash := "idp_user_no_password" // Placeholder - IdP users don't use passwords
	// In production, generate a secure random password hash that can never be used

	params := postgres.CreateUserParams{
		OrgID:       orgID,
		Email:       email,
		DisplayName: displayName,
		PasswordHash: passwordHash,
		Status:      "active",
		ExternalIDP: &externalIDP,
	}

	user, err = h.runtime.Postgres.CreateUser(ctx, params)
	if err != nil {
		return postgres.User{}, fmt.Errorf("create user from IdP: %w", err)
	}

	return user, nil
}

func generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func getBaseURL(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s", scheme, r.Host)
}

