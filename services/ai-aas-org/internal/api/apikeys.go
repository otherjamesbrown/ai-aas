package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// APIKey represents an API key.
type APIKey struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Prefix      string     `json:"prefix"`
	UserID      string     `json:"user_id"`
	UserEmail   string     `json:"user_email"`
	OrgID       string     `json:"org_id"`
	Status      string     `json:"status"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CreatedBy   string     `json:"created_by"`
}

// CreateAPIKeyRequest is the request to create an API key.
type CreateAPIKeyRequest struct {
	Name      string `json:"notes,omitempty"`          // API uses 'notes' for description
	UserID    string `json:"-"`                        // Used in URL path, not body
	ExpiresIn string `json:"expiresInDays,omitempty"`  // e.g., "30", "90"
	Scopes    string `json:"scopes,omitempty"`         // Comma-separated scopes
}

// CreateAPIKeyResponse is the response from creating an API key.
type CreateAPIKeyResponse struct {
	// Flat response fields from API
	KeyID       string `json:"keyId"`
	Token       string `json:"token"`       // Only returned once at creation
	Fingerprint string `json:"fingerprint"`
	Status      string `json:"status"`
	// Populated for backwards compatibility
	RawKey string `json:"-"` // Populated from Token
}

// ListAPIKeysResponse is the response from listing API keys.
type ListAPIKeysResponse struct {
	APIKeys    []APIKey `json:"api_keys"`
	TotalCount int      `json:"total_count"`
}

// ListAPIKeys lists all API keys in the organization.
func (c *Client) ListAPIKeys(ctx context.Context, orgID string) (*ListAPIKeysResponse, error) {
	path := fmt.Sprintf("/v1/orgs/%s/api-keys", url.PathEscape(orgID))

	// API returns raw array of API keys, not a wrapper object
	var apiKeys []APIKey
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &apiKeys); err != nil {
		return nil, err
	}
	return &ListAPIKeysResponse{
		APIKeys:    apiKeys,
		TotalCount: len(apiKeys),
	}, nil
}

// ListUserAPIKeys lists API keys for a specific user.
func (c *Client) ListUserAPIKeys(ctx context.Context, orgID, userID string) (*ListAPIKeysResponse, error) {
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/api-keys", url.PathEscape(orgID), url.PathEscape(userID))

	// API returns raw array of API keys, not a wrapper object
	var apiKeys []APIKey
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &apiKeys); err != nil {
		return nil, err
	}
	return &ListAPIKeysResponse{
		APIKeys:    apiKeys,
		TotalCount: len(apiKeys),
	}, nil
}

// GetAPIKey gets an API key by ID.
func (c *Client) GetAPIKey(ctx context.Context, orgID, keyID string) (*APIKey, error) {
	path := fmt.Sprintf("/v1/orgs/%s/api-keys/%s", url.PathEscape(orgID), url.PathEscape(keyID))

	var result APIKey
	if err := c.doRequest(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateAPIKey creates a new API key for a user.
func (c *Client) CreateAPIKey(ctx context.Context, orgID string, req *CreateAPIKeyRequest) (*CreateAPIKeyResponse, error) {
	// API requires user-specific endpoint
	path := fmt.Sprintf("/v1/orgs/%s/users/%s/api-keys", url.PathEscape(orgID), url.PathEscape(req.UserID))

	var result CreateAPIKeyResponse
	if err := c.doRequest(ctx, http.MethodPost, path, req, &result); err != nil {
		return nil, err
	}
	// Populate backwards-compatible field
	result.RawKey = result.Token
	return &result, nil
}

// DeleteAPIKey deletes (revokes) an API key.
func (c *Client) DeleteAPIKey(ctx context.Context, orgID, keyID string) error {
	path := fmt.Sprintf("/v1/orgs/%s/api-keys/%s", url.PathEscape(orgID), url.PathEscape(keyID))
	return c.doRequest(ctx, http.MethodDelete, path, nil, nil)
}

// RotateAPIKeyResponse is the response from rotating an API key.
type RotateAPIKeyResponse struct {
	OldKeyID string `json:"old_key_id"`
	NewAPIKey CreateAPIKeyResponse `json:"new_api_key"`
}

// RotateAPIKey rotates an API key (creates new, revokes old).
func (c *Client) RotateAPIKey(ctx context.Context, orgID, keyID string) (*RotateAPIKeyResponse, error) {
	path := fmt.Sprintf("/v1/orgs/%s/api-keys/%s/rotate", url.PathEscape(orgID), url.PathEscape(keyID))

	var result RotateAPIKeyResponse
	if err := c.doRequest(ctx, http.MethodPost, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
