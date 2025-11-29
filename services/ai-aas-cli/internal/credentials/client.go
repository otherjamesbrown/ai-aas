// Package credentials provides client for credentials management via Admin API.
package credentials

import (
	"context"
	"fmt"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
)

// Client provides credentials management operations
type Client struct {
	api *api.Client
}

// NewClient creates a new credentials client
func NewClient(apiClient *api.Client) *Client {
	return &Client{api: apiClient}
}

// Credential represents a stored credential
type Credential struct {
	Type      string    `json:"type"`
	Masked    string    `json:"masked"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SetCredentialRequest represents a request to set a credential
type SetCredentialRequest struct {
	Type     string                 `json:"type"`
	Value    string                 `json:"value"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TestCredentialResult represents the result of testing a credential
type TestCredentialResult struct {
	Valid   bool   `json:"valid"`
	Message string `json:"message"`
}

// List returns all configured credentials
func (c *Client) List(ctx context.Context) ([]Credential, error) {
	var creds []Credential
	if err := c.api.Get(ctx, "/v1/credentials", &creds); err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	return creds, nil
}

// Set stores a credential
func (c *Client) Set(ctx context.Context, credType, value string, metadata map[string]interface{}) error {
	req := SetCredentialRequest{
		Type:     credType,
		Value:    value,
		Metadata: metadata,
	}
	if err := c.api.Post(ctx, "/v1/credentials", req, nil); err != nil {
		return fmt.Errorf("set credential: %w", err)
	}
	return nil
}

// Test validates a credential
func (c *Client) Test(ctx context.Context, credType string) (*TestCredentialResult, error) {
	var result TestCredentialResult
	path := fmt.Sprintf("/v1/credentials/%s/test", credType)
	if err := c.api.Post(ctx, path, nil, &result); err != nil {
		return nil, fmt.Errorf("test credential: %w", err)
	}
	return &result, nil
}

// Delete removes a credential
func (c *Client) Delete(ctx context.Context, credType string) error {
	path := fmt.Sprintf("/v1/credentials/%s", credType)
	if err := c.api.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	return nil
}
