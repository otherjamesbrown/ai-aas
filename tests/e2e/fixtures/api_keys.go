package fixtures

import (
	"fmt"
	"time"

	"github.com/ai-aas/tests/e2e/harness"
)

// APIKey represents a test API key fixture
type APIKey struct {
	ID             string            `json:"id"`
	Key            string            `json:"key"`
	OrganizationID string            `json:"organization_id"`
	Name           string            `json:"name"`
	Scopes         []string          `json:"scopes"`
	ExpiresAt      *time.Time        `json:"expires_at,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	Metadata       map[string]string `json:"metadata"`
}

// APIKeyFixture provides methods for creating and managing API key fixtures
type APIKeyFixture struct {
	client *harness.Client
	fm     *harness.FixtureManager
}

// NewAPIKeyFixture creates a new API key fixture manager
func NewAPIKeyFixture(client *harness.Client, fm *harness.FixtureManager) *APIKeyFixture {
	return &APIKeyFixture{
		client: client,
		fm:     fm,
	}
}

// Create creates a new API key for testing
func (akf *APIKeyFixture) Create(ctx *harness.Context, orgID string, name string, scopes []string) (*APIKey, error) {
	if name == "" {
		name = ctx.GenerateResourceName("key")
	}
	if scopes == nil {
		scopes = []string{"inference:read", "inference:write"}
	}

	reqBody := map[string]interface{}{
		"name":           name,
		"organization_id": orgID,
		"scopes":         scopes,
	}

	resp, err := akf.client.POST("/v1/api-keys", reqBody)
	if err != nil {
		return nil, fmt.Errorf("create API key: %w", err)
	}

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("create API key failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var apiKey APIKey
	if err := resp.UnmarshalJSON(&apiKey); err != nil {
		return nil, fmt.Errorf("unmarshal API key: %w", err)
	}

	// Register for cleanup
	akf.fm.Register("api_key", apiKey.ID, map[string]string{
		"name": apiKey.Name,
		"organization_id": orgID,
		"test_run_id": ctx.RunID,
	})

	return &apiKey, nil
}

// Validate validates an API key by making an authenticated request
func (akf *APIKeyFixture) Validate(apiKey string, baseURL string) (bool, error) {
	// Create a temporary client with the API key
	tempClient := harness.NewClient(baseURL, 10*time.Second)
	tempClient.SetHeader("Authorization", "Bearer "+apiKey)

	// Try to make a simple authenticated request (e.g., get user info)
	resp, err := tempClient.GET("/v1/me")
	if err != nil {
		return false, fmt.Errorf("validate API key: %w", err)
	}

	return resp.StatusCode == 200, nil
}

// Get retrieves an API key by ID
func (akf *APIKeyFixture) Get(id string) (*APIKey, error) {
	resp, err := akf.client.GET(fmt.Sprintf("/v1/api-keys/%s", id))
	if err != nil {
		return nil, fmt.Errorf("get API key: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get API key failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var apiKey APIKey
	if err := resp.UnmarshalJSON(&apiKey); err != nil {
		return nil, fmt.Errorf("unmarshal API key: %w", err)
	}

	return &apiKey, nil
}

// Delete deletes an API key
func (akf *APIKeyFixture) Delete(id string) error {
	resp, err := akf.client.DELETE(fmt.Sprintf("/v1/api-keys/%s", id))
	if err != nil {
		return fmt.Errorf("delete API key: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete API key failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	return nil
}

