package fixtures

import (
	"fmt"
	"time"

	"github.com/ai-aas/tests/e2e/harness"
)

// APIKey represents a test API key fixture
type APIKey struct {
	ID          string `json:"keyId"`
	Key         string `json:"token"`
	Fingerprint string `json:"fingerprint"`
	Status      string `json:"status"`
	Name        string `json:"name,omitempty"`
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

// Create creates a new API key for testing.
// Requires orgID and serviceAccountID - use ServiceAccountFixture.Create first.
func (akf *APIKeyFixture) Create(ctx *harness.Context, orgID string, serviceAccountID string, name string, scopes []string) (*APIKey, error) {
	if name == "" {
		name = ctx.GenerateResourceName("key")
	}
	if scopes == nil {
		scopes = []string{"inference:read", "inference:write"}
	}

	reqBody := map[string]interface{}{
		"name":   name,
		"scopes": scopes,
	}

	// Use the correct route: /v1/orgs/{orgId}/service-accounts/{serviceAccountId}/api-keys
	resp, err := akf.client.POST(fmt.Sprintf("/v1/orgs/%s/service-accounts/%s/api-keys", orgID, serviceAccountID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("create API key: %w", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("create API key failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var apiKey APIKey
	if err := resp.UnmarshalJSON(&apiKey); err != nil {
		return nil, fmt.Errorf("unmarshal API key: %w", err)
	}

	// Register for cleanup
	akf.fm.Register("api_key", apiKey.ID, map[string]string{
		"name":               apiKey.Name,
		"organization_id":    orgID,
		"service_account_id": serviceAccountID,
		"test_run_id":        ctx.RunID,
	})

	return &apiKey, nil
}

// CreateWithServiceAccount is a convenience method that creates a service account
// and then creates an API key for it. Use when you don't need the service account separately.
func (akf *APIKeyFixture) CreateWithServiceAccount(ctx *harness.Context, orgID string, name string, scopes []string) (*APIKey, error) {
	// Create service account first
	saFixture := NewServiceAccountFixture(akf.client, akf.fm)
	sa, err := saFixture.Create(ctx, orgID, "")
	if err != nil {
		return nil, fmt.Errorf("create service account for API key: %w", err)
	}

	// Then create API key
	return akf.Create(ctx, orgID, sa.ID, name, scopes)
}

// Validate validates an API key by calling the validate-api-key endpoint
func (akf *APIKeyFixture) Validate(apiKey string, baseURL string) (bool, error) {
	// Create a temporary client (no auth header needed for validation endpoint)
	tempClient := harness.NewClient(baseURL, 10*time.Second)

	// Call the validate-api-key endpoint
	reqBody := map[string]interface{}{
		"apiKeySecret": apiKey,
	}

	resp, err := tempClient.POST("/v1/auth/validate-api-key", reqBody)
	if err != nil {
		return false, fmt.Errorf("validate API key: %w", err)
	}

	// Check response status
	if resp.StatusCode != 200 {
		return false, fmt.Errorf("validate API key failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	// Parse response to get valid field
	var result struct {
		Valid   bool   `json:"valid"`
		Message string `json:"message,omitempty"`
	}
	if err := resp.UnmarshalJSON(&result); err != nil {
		return false, fmt.Errorf("unmarshal validate response: %w", err)
	}

	return result.Valid, nil
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

