package usecases_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

// FixtureManager manages test fixtures and cleanup for UC tests
type FixtureManager struct {
	t        *testing.T
	client   *TestClient
	fixtures []TestFixture
}

// TestFixture represents a test resource that needs cleanup
type TestFixture struct {
	Type      string
	ID        string
	CreatedAt time.Time
	Metadata  map[string]string
	CleanupFn func() error
}

// NewFixtureManager creates a new fixture manager for a test
func NewFixtureManager(t *testing.T, client *TestClient) *FixtureManager {
	t.Helper()
	fm := &FixtureManager{
		t:        t,
		client:   client,
		fixtures: []TestFixture{},
	}

	// Register cleanup to run at test end
	t.Cleanup(func() {
		if err := fm.Cleanup(); err != nil {
			t.Logf("Warning: Cleanup failed: %v", err)
		}
	})

	return fm
}

// Register registers a fixture for cleanup
func (fm *FixtureManager) Register(fixtureType, id string, metadata map[string]string, cleanupFn func() error) {
	fm.fixtures = append(fm.fixtures, TestFixture{
		Type:      fixtureType,
		ID:        id,
		CreatedAt: time.Now(),
		Metadata:  metadata,
		CleanupFn: cleanupFn,
	})
}

// Cleanup performs cleanup of all registered fixtures in reverse order
func (fm *FixtureManager) Cleanup() error {
	// Cleanup in reverse order (last created, first deleted)
	for i := len(fm.fixtures) - 1; i >= 0; i-- {
		fixture := fm.fixtures[i]
		if fixture.CleanupFn != nil {
			if err := fixture.CleanupFn(); err != nil {
				fm.t.Logf("Warning: Failed to cleanup %s %s: %v", fixture.Type, fixture.ID, err)
			}
		}
	}
	return nil
}

// OrganizationFixture provides organization CRUD for tests
type OrganizationFixture struct {
	fm     *FixtureManager
	client *TestClient
}

// NewOrganizationFixture creates a new organization fixture manager
func NewOrganizationFixture(fm *FixtureManager, client *TestClient) *OrganizationFixture {
	return &OrganizationFixture{
		fm:     fm,
		client: client,
	}
}

// Organization represents a test organization
type Organization struct {
	ID        string    `json:"orgId"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"createdAt"`
}

// Create creates a test organization and registers it for cleanup
func (of *OrganizationFixture) Create(name string) (*Organization, error) {
	if name == "" {
		name = fmt.Sprintf("test-org-%s", generateUniqueID())
	}

	slug := name // For simplicity, use name as slug
	reqBody := map[string]interface{}{
		"name": name,
		"slug": slug,
	}

	resp, err := of.client.POST("/v1/orgs", reqBody)
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("create organization failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	var org Organization
	if err := json.Unmarshal(resp.Body, &org); err != nil {
		return nil, fmt.Errorf("unmarshal organization: %w", err)
	}

	// Register for cleanup
	of.fm.Register("organization", org.ID, map[string]string{
		"name": org.Name,
	}, func() error {
		return of.Delete(org.ID)
	})

	return &org, nil
}

// Delete deletes an organization
func (of *OrganizationFixture) Delete(id string) error {
	resp, err := of.client.DELETE(fmt.Sprintf("/v1/orgs/%s", id))
	if err != nil {
		return fmt.Errorf("delete organization: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete organization failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	return nil
}

// ServiceAccountFixture provides service account CRUD for tests
type ServiceAccountFixture struct {
	fm     *FixtureManager
	client *TestClient
}

// NewServiceAccountFixture creates a new service account fixture manager
func NewServiceAccountFixture(fm *FixtureManager, client *TestClient) *ServiceAccountFixture {
	return &ServiceAccountFixture{
		fm:     fm,
		client: client,
	}
}

// ServiceAccount represents a test service account
type ServiceAccount struct {
	ID             string    `json:"serviceAccountId"`
	Name           string    `json:"name"`
	OrganizationID string    `json:"orgId"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
}

// Create creates a test service account and registers it for cleanup
func (saf *ServiceAccountFixture) Create(orgID, name string) (*ServiceAccount, error) {
	if name == "" {
		name = fmt.Sprintf("test-sa-%s", generateUniqueID())
	}

	reqBody := map[string]interface{}{
		"name": name,
	}

	resp, err := saf.client.POST(fmt.Sprintf("/v1/orgs/%s/service-accounts", orgID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("create service account: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("create service account failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	var sa ServiceAccount
	if err := json.Unmarshal(resp.Body, &sa); err != nil {
		return nil, fmt.Errorf("unmarshal service account: %w", err)
	}

	// Register for cleanup
	saf.fm.Register("service_account", sa.ID, map[string]string{
		"name":  sa.Name,
		"orgId": orgID,
	}, func() error {
		return saf.Delete(orgID, sa.ID)
	})

	return &sa, nil
}

// Delete deletes a service account
func (saf *ServiceAccountFixture) Delete(orgID, serviceAccountID string) error {
	resp, err := saf.client.DELETE(fmt.Sprintf("/v1/orgs/%s/service-accounts/%s", orgID, serviceAccountID))
	if err != nil {
		return fmt.Errorf("delete service account: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete service account failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	return nil
}

// APIKeyFixture provides API key CRUD for tests
type APIKeyFixture struct {
	fm     *FixtureManager
	client *TestClient
}

// NewAPIKeyFixture creates a new API key fixture manager
func NewAPIKeyFixture(fm *FixtureManager, client *TestClient) *APIKeyFixture {
	return &APIKeyFixture{
		fm:     fm,
		client: client,
	}
}

// APIKey represents a test API key
type APIKey struct {
	ID          string `json:"keyId"`
	Key         string `json:"token"`
	Fingerprint string `json:"fingerprint"`
	Status      string `json:"status"`
	Name        string `json:"name"`
}

// Create creates a test API key and registers it for cleanup
// Requires orgID and serviceAccountID - use ServiceAccountFixture.Create first
func (akf *APIKeyFixture) Create(orgID, serviceAccountID, name string, scopes []string) (*APIKey, error) {
	if name == "" {
		name = fmt.Sprintf("test-key-%s", generateUniqueID())
	}
	if scopes == nil {
		scopes = []string{"inference:read", "inference:write"}
	}

	reqBody := map[string]interface{}{
		"name":   name,
		"scopes": scopes,
	}

	resp, err := akf.client.POST(fmt.Sprintf("/v1/orgs/%s/service-accounts/%s/api-keys", orgID, serviceAccountID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("create API key: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		return nil, fmt.Errorf("create API key failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	var apiKey APIKey
	if err := json.Unmarshal(resp.Body, &apiKey); err != nil {
		return nil, fmt.Errorf("unmarshal API key: %w", err)
	}

	// Register for cleanup
	akf.fm.Register("api_key", apiKey.ID, map[string]string{
		"name":  apiKey.Name,
		"orgId": orgID,
	}, func() error {
		return akf.Delete(orgID, apiKey.ID)
	})

	return &apiKey, nil
}

// CreateWithServiceAccount is a convenience method that creates a service account
// and then creates an API key for it
func (akf *APIKeyFixture) CreateWithServiceAccount(orgID, name string, scopes []string) (*APIKey, error) {
	// Create service account first
	saFixture := NewServiceAccountFixture(akf.fm, akf.client)
	sa, err := saFixture.Create(orgID, "")
	if err != nil {
		return nil, fmt.Errorf("create service account for API key: %w", err)
	}

	// Then create API key
	return akf.Create(orgID, sa.ID, name, scopes)
}

// Delete deletes an API key
func (akf *APIKeyFixture) Delete(orgID, keyID string) error {
	resp, err := akf.client.DELETE(fmt.Sprintf("/v1/orgs/%s/api-keys/%s", orgID, keyID))
	if err != nil {
		return fmt.Errorf("delete API key: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete API key failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	return nil
}

// ModelDeploymentFixture provides model deployment operations for tests
type ModelDeploymentFixture struct {
	fm     *FixtureManager
	client *TestClient
}

// NewModelDeploymentFixture creates a new model deployment fixture manager
func NewModelDeploymentFixture(fm *FixtureManager, client *TestClient) *ModelDeploymentFixture {
	return &ModelDeploymentFixture{
		fm:     fm,
		client: client,
	}
}

// ModelDeployment represents a deployed model
type ModelDeployment struct {
	ID          string `json:"deploymentId"`
	Name        string `json:"name"`
	Model       string `json:"model"`
	Environment string `json:"environment"`
	Status      string `json:"status"`
}

// Create creates a model deployment and registers it for cleanup
// Note: This is a placeholder - actual implementation depends on Admin API endpoints
func (mdf *ModelDeploymentFixture) Create(modelName, environment string) (*ModelDeployment, error) {
	// TODO: Implement once Admin API model deployment endpoints are available
	return nil, fmt.Errorf("model deployment fixture not yet implemented")
}

// Delete deletes a model deployment
func (mdf *ModelDeploymentFixture) Delete(deploymentID string) error {
	// TODO: Implement once Admin API model deployment endpoints are available
	return fmt.Errorf("model deployment fixture not yet implemented")
}
