package oauth

import (
	"context"
	"time"

	"github.com/ory/fosite"
)

// ClientStore augments the base Store with statically configured clients.
type ClientStore struct {
	*Store
	clients map[string]fosite.Client
}

// NewClientStore creates a storage wrapper that first looks up clients from the provided map.
func NewClientStore(base *Store, clients map[string]fosite.Client) *ClientStore {
	if clients == nil {
		clients = map[string]fosite.Client{}
	}
	return &ClientStore{
		Store:   base,
		clients: clients,
	}
}

// GetClient returns a client from the static registry or delegates to the underlying store.
func (c *ClientStore) GetClient(ctx context.Context, id string) (fosite.Client, error) {
	if client, ok := c.clients[id]; ok {
		return client, nil
	}
	if c.Store != nil {
		return c.Store.GetClient(ctx, id)
	}
	return nil, fosite.ErrNotFound
}

// ClientAssertionJWTValid delegates to the underlying store when available.
func (c *ClientStore) ClientAssertionJWTValid(ctx context.Context, jti string) error {
	if c.Store != nil {
		return c.Store.ClientAssertionJWTValid(ctx, jti)
	}
	return fosite.ErrNotFound
}

// SetClientAssertionJWT delegates to the underlying store when available.
func (c *ClientStore) SetClientAssertionJWT(ctx context.Context, jti string, exp time.Time) error {
	if c.Store != nil {
		return c.Store.SetClientAssertionJWT(ctx, jti, exp)
	}
	return nil
}

// AttachConfig ensures the underlying store is aware of the Fosité configuration.
func (c *ClientStore) AttachConfig(cfg *fosite.Config) {
	if c.Store != nil {
		c.Store.AttachConfig(cfg)
	}
}

// Explicitly forward all storage methods to ensure interface satisfaction
func (c *ClientStore) CreateAuthorizeCodeSession(ctx context.Context, signature string, request fosite.Requester) error {
	return c.Store.CreateAuthorizeCodeSession(ctx, signature, request)
}

func (c *ClientStore) GetAuthorizeCodeSession(ctx context.Context, signature string, session fosite.Session) (fosite.Requester, error) {
	return c.Store.GetAuthorizeCodeSession(ctx, signature, session)
}

func (c *ClientStore) InvalidateAuthorizeCodeSession(ctx context.Context, signature string) error {
	return c.Store.InvalidateAuthorizeCodeSession(ctx, signature)
}

func (c *ClientStore) CreateAccessTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	return c.Store.CreateAccessTokenSession(ctx, signature, request)
}

func (c *ClientStore) GetAccessTokenSession(ctx context.Context, signature string, session fosite.Session) (fosite.Requester, error) {
	return c.Store.GetAccessTokenSession(ctx, signature, session)
}

func (c *ClientStore) DeleteAccessTokenSession(ctx context.Context, signature string) error {
	return c.Store.DeleteAccessTokenSession(ctx, signature)
}

func (c *ClientStore) CreateRefreshTokenSession(ctx context.Context, signature string, request fosite.Requester) error {
	return c.Store.CreateRefreshTokenSession(ctx, signature, request)
}

func (c *ClientStore) GetRefreshTokenSession(ctx context.Context, signature string, session fosite.Session) (fosite.Requester, error) {
	return c.Store.GetRefreshTokenSession(ctx, signature, session)
}

func (c *ClientStore) DeleteRefreshTokenSession(ctx context.Context, signature string) error {
	return c.Store.DeleteRefreshTokenSession(ctx, signature)
}

func (c *ClientStore) RevokeRefreshToken(ctx context.Context, requestID string) error {
	return c.Store.RevokeRefreshToken(ctx, requestID)
}

func (c *ClientStore) RevokeRefreshTokenMaybeGracePeriod(ctx context.Context, signature string, requestID string) error {
	return c.Store.RevokeRefreshTokenMaybeGracePeriod(ctx, signature, requestID)
}

func (c *ClientStore) CreatePKCERequestSession(ctx context.Context, signature string, request fosite.Requester) error {
	return c.Store.CreatePKCERequestSession(ctx, signature, request)
}

func (c *ClientStore) GetPKCERequestSession(ctx context.Context, signature string, session fosite.Session) (fosite.Requester, error) {
	return c.Store.GetPKCERequestSession(ctx, signature, session)
}

func (c *ClientStore) DeletePKCERequestSession(ctx context.Context, signature string) error {
	return c.Store.DeletePKCERequestSession(ctx, signature)
}

func (c *ClientStore) Authenticate(ctx context.Context, username, password string) (string, error) {
	return c.Store.Authenticate(ctx, username, password)
}
