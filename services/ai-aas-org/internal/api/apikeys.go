package api

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// APIKey represents an API key as returned by the user-org-service API.
// JSON tags match the API response format for parsing.
type APIKey struct {
	ID            string     `json:"keyId"`
	Name          string     `json:"notes"`
	Prefix        string     `json:"prefix,omitempty"` // Derived from fingerprint if needed
	UserID        string     `json:"principalId"`
	PrincipalType string     `json:"principalType,omitempty"`
	UserEmail     string     `json:"user_email,omitempty"` // Not returned by API, populated separately
	OrgID         string     `json:"org_id,omitempty"`     // Not returned by API, known from request context
	Fingerprint   string     `json:"fingerprint,omitempty"`
	Status        string     `json:"status"`
	Scopes        []string   `json:"scopes,omitempty"`
	LastUsedAt    *time.Time `json:"lastUsedAt,omitempty"`
	ExpiresAt     *time.Time `json:"expiresAt,omitempty"`
	CreatedAt     *time.Time `json:"issuedAt,omitempty"`
	CreatedBy     string     `json:"created_by,omitempty"` // Not returned by API
}

// APIKeyDisplay is the user-facing representation of an API key for JSON output.
// Field names match user expectations and CLI documentation.
type APIKeyDisplay struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	UserID    string `json:"user_id"`
	UserEmail string `json:"user_email,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
	LastUsed  string `json:"last_used,omitempty"`
}

// ToDisplay converts an APIKey to its display representation.
func (k *APIKey) ToDisplay() APIKeyDisplay {
	display := APIKeyDisplay{
		ID:        k.ID,
		Name:      k.Name,
		UserID:    k.UserID,
		UserEmail: k.UserEmail,
		Status:    k.Status,
	}
	if k.CreatedAt != nil {
		display.CreatedAt = k.CreatedAt.Format(time.RFC3339)
	}
	if k.ExpiresAt != nil {
		display.ExpiresAt = k.ExpiresAt.Format(time.RFC3339)
	}
	if k.LastUsedAt != nil {
		display.LastUsed = k.LastUsedAt.Format(time.RFC3339)
	}
	return display
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
