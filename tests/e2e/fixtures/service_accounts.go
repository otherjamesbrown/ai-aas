package fixtures

import (
	"fmt"
	"time"

	"github.com/ai-aas/tests/e2e/harness"
)

// ServiceAccount represents a test service account fixture
type ServiceAccount struct {
	ID             string    `json:"serviceAccountId"`
	OrganizationID string    `json:"orgId"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"createdAt"`
}

// ServiceAccountFixture provides methods for creating and managing service account fixtures
type ServiceAccountFixture struct {
	client *harness.Client
	fm     *harness.FixtureManager
}

// NewServiceAccountFixture creates a new service account fixture manager
func NewServiceAccountFixture(client *harness.Client, fm *harness.FixtureManager) *ServiceAccountFixture {
	return &ServiceAccountFixture{
		client: client,
		fm:     fm,
	}
}

// Create creates a new service account for testing
func (saf *ServiceAccountFixture) Create(ctx *harness.Context, orgID string, name string) (*ServiceAccount, error) {
	if name == "" {
		name = ctx.GenerateResourceName("sa")
	}

	reqBody := map[string]interface{}{
		"name":        name,
		"description": "E2E test service account",
	}

	resp, err := saf.client.POST(fmt.Sprintf("/v1/orgs/%s/service-accounts", orgID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("create service account: %w", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("create service account failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var sa ServiceAccount
	if err := resp.UnmarshalJSON(&sa); err != nil {
		return nil, fmt.Errorf("unmarshal service account: %w", err)
	}

	// Register for cleanup
	saf.fm.Register("service_account", sa.ID, map[string]string{
		"name":            sa.Name,
		"organization_id": orgID,
		"test_run_id":     ctx.RunID,
	}, func() error {
		return saf.Delete(orgID, sa.ID)
	})

	return &sa, nil
}

// Delete deletes a service account
func (saf *ServiceAccountFixture) Delete(orgID, id string) error {
	resp, err := saf.client.DELETE(fmt.Sprintf("/v1/orgs/%s/service-accounts/%s", orgID, id))
	if err != nil {
		return fmt.Errorf("delete service account: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete service account failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	return nil
}
