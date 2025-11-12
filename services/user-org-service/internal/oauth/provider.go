// Package oauth (provider.go) composes and configures the Fosite OAuth2 provider.
//
// Purpose:
//   This file provides the NewProvider function that wires together Fosite's
//   OAuth2 provider with storage, HMAC strategy, and grant type handlers.
//   It supports static client configuration from environment variables and
//   composes the provider with standard OAuth2 flows (authorization code,
//   refresh token, PKCE, resource owner password credentials, client credentials).
//
// Dependencies:
//   - github.com/ory/fosite: OAuth2 framework and compose utilities
//   - github.com/ory/fosite/compose: Factory functions for grant handlers
//   - internal/storage/postgres: Postgres store (optional, can use custom storage)
//   - internal/oauth: Store, ClientStore, SessionCache from this package
//
// Key Responsibilities:
//   - NewProvider composes a fully configured fosite.OAuth2Provider
//   - Constructs static clients from configuration (hashed secrets)
//   - Applies default token lifetimes and PKCE settings
//   - Wraps storage with ClientStore for static client support
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#FR-005 (OAuth2 Support)
//   - specs/005-user-org-service/spec.md#NFR-003 (Session Management)
//
// Debugging Notes:
//   - HMAC secret must be at least 32 bytes (validated at provider creation)
//   - Static clients are hashed using bcrypt before storage
//   - Default token lifetimes: access=1h, refresh=24h, auth code=10m
//   - PKCE is enforced by default (plain challenge method disabled)
//   - Provider composition uses Fosite's compose.Compose utility
//
// Thread Safety:
//   - Provider creation is not thread-safe (call during initialization)
//   - Created provider is safe for concurrent use by HTTP handlers
//
// Error Handling:
//   - Returns error if HMAC secret is too short (< 32 bytes)
//   - Returns error if storage/PostgresStore not provided
//   - Client secret hashing errors are wrapped with context
package oauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ory/fosite"
	"github.com/ory/fosite/compose"

	"github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/storage/postgres"
)

// ProviderDependencies encapsulates the inputs required to compose a Fosité provider.
type ProviderDependencies struct {
	// Storage allows supplying a custom Fosité storage implementation (mainly for testing).
	// When nil, PostgresStore must be provided.
	Storage fosite.Storage

	// PostgresStore is the concrete store backing OAuth persistence.
	// It is wrapped with caching when Storage is nil.
	PostgresStore *postgres.Store

	// SessionCache enables optional Redis-backed caching of OAuth sessions.
	SessionCache SessionCache

	// Config allows overriding the default Fosité configuration.
	Config *fosite.Config

	// HMACSecret seeds the HMAC strategy (minimum 32 bytes recommended).
	HMACSecret []byte

	// StaticClients registers a collection of pre-configured clients.
	StaticClients []StaticClient

	// Factories allows overriding the default factories used to register handlers.
	Factories []compose.Factory
}

// StaticClient describes a client definition supplied via configuration.
type StaticClient struct {
	ID            string
	Secret        string
	RedirectURIs  []string
	GrantTypes    []string
	ResponseTypes []string
	Scopes        []string
	Audience      []string
}

// NewProvider composes a Fosité OAuth2 provider configured for the user-org service.
func NewProvider(deps ProviderDependencies) (fosite.OAuth2Provider, error) {
	if len(deps.HMACSecret) < 32 {
		return nil, fmt.Errorf("oauth provider: HMAC secret must be at least 32 bytes")
	}

	cfg := deps.Config
	if cfg == nil {
		cfg = defaultConfig()
	}

	// Ensure the global secret is populated.
	var storage fosite.Storage
	switch {
	case deps.Storage != nil:
		storage = deps.Storage
		if len(deps.StaticClients) > 0 {
			clients, err := constructStaticClients(cfg, deps.StaticClients)
			if err != nil {
				return nil, err
			}
			switch typed := storage.(type) {
			case *ClientStore:
				for id, client := range clients {
					typed.clients[id] = client
				}
			case *Store:
				storage = NewClientStore(typed, clients)
			default:
				// Attempt to wrap when possible
				if base, ok := storage.(*Store); ok {
					storage = NewClientStore(base, clients)
				}
			}
		}
	case deps.PostgresStore != nil:
		store := NewStoreWithCache(deps.PostgresStore, deps.SessionCache)
		store.AttachConfig(cfg)
		if len(deps.StaticClients) > 0 {
			staticClients, err := constructStaticClients(cfg, deps.StaticClients)
			if err != nil {
				return nil, err
			}
			storage = NewClientStore(store, staticClients)
		} else {
			storage = store
		}
	default:
		return nil, errors.New("oauth provider: storage or PostgresStore must be supplied")
	}

	if len(cfg.GlobalSecret) == 0 {
		cfg.GlobalSecret = deps.HMACSecret
	}
	if cfg.ClientSecretsHasher == nil {
		cfg.ClientSecretsHasher = &fosite.BCrypt{Config: cfg}
	}

	strategy := compose.NewOAuth2HMACStrategy(cfg)

	factories := deps.Factories
	if len(factories) == 0 {
		factories = defaultFactories()
	}

	switch typed := storage.(type) {
	case *Store:
		typed.AttachConfig(cfg)
	case *ClientStore:
		typed.AttachConfig(cfg)
	}

	return compose.Compose(
		cfg,
		storage,
		strategy,
		factories...,
	), nil
}

func defaultConfig() *fosite.Config {
	return &fosite.Config{
		AccessTokenLifespan:            time.Hour,
		RefreshTokenLifespan:           24 * time.Hour,
		AuthorizeCodeLifespan:          10 * time.Minute,
		IDTokenLifespan:                time.Hour,
		ScopeStrategy:                  fosite.ExactScopeStrategy,
		EnforcePKCE:                    true,
		EnablePKCEPlainChallengeMethod: false,
		SendDebugMessagesToClients:     false,
		TokenEntropy:                   32,
		MinParameterEntropy:            fosite.MinParameterEntropy,
	}
}

func defaultFactories() []compose.Factory {
	return []compose.Factory{
		compose.OAuth2AuthorizeExplicitFactory,
		compose.OAuth2ResourceOwnerPasswordCredentialsFactory,
		compose.OAuth2RefreshTokenGrantFactory,
		compose.OAuth2ClientCredentialsGrantFactory,
		compose.OAuth2TokenIntrospectionFactory,
		compose.OAuth2TokenRevocationFactory,
		compose.OAuth2PKCEFactory,
	}
}

func constructStaticClients(cfg *fosite.Config, defs []StaticClient) (map[string]fosite.Client, error) {
	clients := make(map[string]fosite.Client, len(defs))
	hasher := cfg.ClientSecretsHasher
	if hasher == nil {
		hasher = &fosite.BCrypt{Config: cfg}
		cfg.ClientSecretsHasher = hasher
	}

	for _, def := range defs {
		if def.ID == "" {
			return nil, fmt.Errorf("oauth provider: static client missing id")
		}
		var hashedSecret []byte
		if def.Secret != "" {
			secretHash, err := hasher.Hash(context.Background(), []byte(def.Secret))
			if err != nil {
				return nil, fmt.Errorf("oauth provider: hash client secret: %w", err)
			}
			hashedSecret = secretHash
		}

		client := &fosite.DefaultClient{
			ID:            def.ID,
			Secret:        hashedSecret,
			RedirectURIs:  def.RedirectURIs,
			ResponseTypes: def.ResponseTypes,
			GrantTypes:    def.GrantTypes,
			Scopes:        def.Scopes,
			Audience:      def.Audience,
		}
		if len(client.GrantTypes) == 0 {
			client.GrantTypes = []string{"password", "refresh_token"}
		}
		if len(client.ResponseTypes) == 0 {
			client.ResponseTypes = []string{"token"}
		}
		if len(client.Scopes) == 0 {
			client.Scopes = []string{"openid", "profile", "email"}
		}

		clients[def.ID] = client
	}

	return clients, nil
}
