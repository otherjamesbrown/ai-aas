package fixtures

import (
	"fmt"
	"strings"
	"time"

	"github.com/ai-aas/tests/e2e/harness"
)

// Organization represents a test organization fixture
type Organization struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Slug      string            `json:"slug"`
	CreatedAt time.Time         `json:"created_at"`
	Metadata   map[string]string `json:"metadata"`
}

// OrganizationFixture provides methods for creating and managing organization fixtures
type OrganizationFixture struct {
	client *harness.Client
	fm     *harness.FixtureManager
}

// NewOrganizationFixture creates a new organization fixture manager
func NewOrganizationFixture(client *harness.Client, fm *harness.FixtureManager) *OrganizationFixture {
	return &OrganizationFixture{
		client: client,
		fm:     fm,
	}
}

// Create creates a new organization for testing
func (of *OrganizationFixture) Create(ctx *harness.Context, name string) (*Organization, error) {
	if name == "" {
		name = ctx.GenerateResourceName("org")
	}

	// Generate a unique slug from the name
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	if slug == "" {
		slug = ctx.GenerateResourceName("org")
	}

	reqBody := map[string]interface{}{
		"name": name,
		"slug": slug,
	}

	resp, err := of.client.POST("/v1/orgs", reqBody)
	if err != nil {
		return nil, fmt.Errorf("create organization: %w", err)
	}

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("create organization failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var org Organization
	if err := resp.UnmarshalJSON(&org); err != nil {
		return nil, fmt.Errorf("unmarshal organization: %w", err)
	}

	// Register for cleanup
	of.fm.Register("organization", org.ID, map[string]string{
		"name": org.Name,
		"test_run_id": ctx.RunID,
	})

	return &org, nil
}

// Get retrieves an organization by ID
func (of *OrganizationFixture) Get(id string) (*Organization, error) {
	resp, err := of.client.GET(fmt.Sprintf("/v1/orgs/%s", id))
	if err != nil {
		return nil, fmt.Errorf("get organization: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get organization failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var org Organization
	if err := resp.UnmarshalJSON(&org); err != nil {
		return nil, fmt.Errorf("unmarshal organization: %w", err)
	}

	return &org, nil
}

// Delete deletes an organization
func (of *OrganizationFixture) Delete(id string) error {
	resp, err := of.client.DELETE(fmt.Sprintf("/v1/orgs/%s", id))
	if err != nil {
		return fmt.Errorf("delete organization: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete organization failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	return nil
}

