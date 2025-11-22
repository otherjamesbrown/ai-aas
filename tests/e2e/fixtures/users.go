package fixtures

import (
	"fmt"
	"time"

	"github.com/ai-aas/tests/e2e/harness"
)

// User represents a test user fixture
type User struct {
	ID             string            `json:"id"`
	Email          string            `json:"email"`
	Name           string            `json:"name"`
	OrganizationID string            `json:"organization_id"`
	Roles          []string           `json:"roles"`
	Status         string            `json:"status"`
	CreatedAt      time.Time         `json:"created_at"`
	Metadata       map[string]string `json:"metadata"`
}

// UserFixture provides methods for creating and managing user fixtures
type UserFixture struct {
	client *harness.Client
	fm     *harness.FixtureManager
}

// NewUserFixture creates a new user fixture manager
func NewUserFixture(client *harness.Client, fm *harness.FixtureManager) *UserFixture {
	return &UserFixture{
		client: client,
		fm:     fm,
	}
}

// Create creates a new user for testing
func (uf *UserFixture) Create(ctx *harness.Context, orgID string, email string, name string, roles []string) (*User, error) {
	if email == "" {
		email = ctx.GenerateResourceName("user") + "@test.example.com"
	}
	if name == "" {
		name = "Test User"
	}
	if roles == nil {
		roles = []string{"member"}
	}

	reqBody := map[string]interface{}{
		"email":          email,
		"name":           name,
		"organization_id": orgID,
		"roles":          roles,
	}

	resp, err := uf.client.POST("/v1/users", reqBody)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	if resp.StatusCode != 201 {
		return nil, fmt.Errorf("create user failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var user User
	if err := resp.UnmarshalJSON(&user); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}

	// Register for cleanup
	uf.fm.Register("user", user.ID, map[string]string{
		"email": user.Email,
		"organization_id": orgID,
		"test_run_id": ctx.RunID,
	})

	return &user, nil
}

// Invite sends an invitation to a user
func (uf *UserFixture) Invite(orgID string, email string) (*InviteResponse, error) {
	reqBody := map[string]interface{}{
		"email": email,
	}

	resp, err := uf.client.POST(fmt.Sprintf("/v1/orgs/%s/invites", orgID), reqBody)
	if err != nil {
		return nil, fmt.Errorf("invite user: %w", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		return nil, fmt.Errorf("invite user failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var invite InviteResponse
	if err := resp.UnmarshalJSON(&invite); err != nil {
		return nil, fmt.Errorf("unmarshal invite: %w", err)
	}

	return &invite, nil
}

// Get retrieves a user by ID
func (uf *UserFixture) Get(id string) (*User, error) {
	resp, err := uf.client.GET(fmt.Sprintf("/v1/users/%s", id))
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("get user failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var user User
	if err := resp.UnmarshalJSON(&user); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}

	return &user, nil
}

// Delete deletes a user
func (uf *UserFixture) Delete(id string) error {
	resp, err := uf.client.DELETE(fmt.Sprintf("/v1/users/%s", id))
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("delete user failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	return nil
}

// InviteResponse represents an invitation response
type InviteResponse struct {
	InviteID string    `json:"invite_id"`
	Email    string    `json:"email"`
	ExpiresAt time.Time `json:"expires_at"`
}

