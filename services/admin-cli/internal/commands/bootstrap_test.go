// Package commands provides tests for bootstrap command.
package commands

import (
	"context"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/otherjamesbrown/ai-aas/services/admin-cli/internal/client/userorg"
)

// MockUserOrgClient is a mock implementation of the userorg client for testing
type MockUserOrgClient struct {
	mock.Mock
}

func (m *MockUserOrgClient) Bootstrap(ctx context.Context, req userorg.BootstrapRequest) (*userorg.BootstrapResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*userorg.BootstrapResponse), args.Error(1)
}

func (m *MockUserOrgClient) CheckExistingAdmin(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Bool(0), args.Error(1)
}

func (m *MockUserOrgClient) CreateOrg(ctx context.Context, req userorg.CreateOrgRequest) (*userorg.OrganizationResponse, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*userorg.OrganizationResponse), args.Error(1)
}

func (m *MockUserOrgClient) ListOrgs(ctx context.Context) ([]userorg.OrganizationResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).([]userorg.OrganizationResponse), args.Error(1)
}

func (m *MockUserOrgClient) InviteUser(ctx context.Context, orgID string, req userorg.InviteUserRequest) (*userorg.UserResponse, error) {
	args := m.Called(ctx, orgID, req)
	return args.Get(0).(*userorg.UserResponse), args.Error(1)
}

func (m *MockUserOrgClient) GetUser(ctx context.Context, orgID, userID string) (*userorg.UserResponse, error) {
	args := m.Called(ctx, orgID, userID)
	return args.Get(0).(*userorg.UserResponse), args.Error(1)
}

func (m *MockUserOrgClient) GetUserByEmail(ctx context.Context, orgID, email string) (*userorg.UserResponse, error) {
	args := m.Called(ctx, orgID, email)
	return args.Get(0).(*userorg.UserResponse), args.Error(1)
}

func (m *MockUserOrgClient) ListUsers(ctx context.Context, orgID string) ([]userorg.UserResponse, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).([]userorg.UserResponse), args.Error(1)
}

func (m *MockUserOrgClient) IssueUserAPIKey(ctx context.Context, orgID, userID string, req userorg.IssueAPIKeyRequest) (*userorg.IssuedAPIKeyResponse, error) {
	args := m.Called(ctx, orgID, userID, req)
	return args.Get(0).(*userorg.IssuedAPIKeyResponse), args.Error(1)
}

func (m *MockUserOrgClient) RotateAPIKey(ctx context.Context, orgID, apiKeyID string) (*userorg.RotateAPIKeyResponse, error) {
	args := m.Called(ctx, orgID, apiKeyID)
	return args.Get(0).(*userorg.RotateAPIKeyResponse), args.Error(1)
}

func (m *MockUserOrgClient) ListAPIKeys(ctx context.Context, orgID string) ([]userorg.APIKeyResponse, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).([]userorg.APIKeyResponse), args.Error(1)
}

func (m *MockUserOrgClient) GetOrg(ctx context.Context, orgID string) (*userorg.OrganizationResponse, error) {
	args := m.Called(ctx, orgID)
	return args.Get(0).(*userorg.OrganizationResponse), args.Error(1)
}

func (m *MockUserOrgClient) UpdateOrg(ctx context.Context, orgID string, req userorg.UpdateOrgRequest) (*userorg.OrganizationResponse, error) {
	args := m.Called(ctx, orgID, req)
	return args.Get(0).(*userorg.OrganizationResponse), args.Error(1)
}

func (m *MockUserOrgClient) DeleteOrg(ctx context.Context, orgID string) error {
	args := m.Called(ctx, orgID)
	return args.Error(0)
}

func (m *MockUserOrgClient) UpdateUser(ctx context.Context, orgID, userID string, req userorg.UpdateUserRequest) (*userorg.UserResponse, error) {
	args := m.Called(ctx, orgID, userID, req)
	return args.Get(0).(*userorg.UserResponse), args.Error(1)
}

func (m *MockUserOrgClient) DeleteUser(ctx context.Context, orgID, userID string) error {
	args := m.Called(ctx, orgID, userID)
	return args.Error(0)
}

func (m *MockUserOrgClient) DeleteAPIKey(ctx context.Context, orgID, apiKeyID string) error {
	args := m.Called(ctx, orgID, apiKeyID)
	return args.Error(0)
}

func TestBootstrapCommand(t *testing.T) {
	cmd := BootstrapCommand()
	require.NotNil(t, cmd, "BootstrapCommand() returned nil")

	assert.Equal(t, "bootstrap", cmd.Use)
	assert.NotNil(t, cmd.RunE, "command should have RunE handler")

	// Test flags
	emailFlag := cmd.Flags().Lookup("email")
	assert.NotNil(t, emailFlag, "email flag should exist")

	dryRunFlag := cmd.Flags().Lookup("dry-run")
	assert.NotNil(t, dryRunFlag, "dry-run flag should exist")
}

func TestRunBootstrapDryRun(t *testing.T) {
	mockClient := &MockUserOrgClient{}
	// Note: In a real implementation, we'd inject the mock client, but for now
	// we'll test the basic validation logic

	t.Run("missing email in execution mode", func(t *testing.T) {
		err := runBootstrap(&cobra.Command{}, []string{},
			"", "", "", false, true, "table", false, false, "http://localhost:8080", "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "--email is required")
	})

	t.Run("missing user-org-endpoint", func(t *testing.T) {
		err := runBootstrap(&cobra.Command{}, []string{},
			"admin@example.com", "", "", true, false, "table", false, false, "", "")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user-org-service endpoint is required")
	})

	// Note: Full integration with mocked client would require refactoring to inject dependencies
	// This is a known limitation for Phase 4 - integration tests in test/integration/ provide coverage
}

// Integration tests are in test/integration/bootstrap_test.go (Phase 3)
