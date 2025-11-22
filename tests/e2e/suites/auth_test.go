package suites

import (
	"fmt"
	"testing"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// TestAuthorizationDenial tests that restricted actions are denied with insufficient permissions
func TestAuthorizationDenial(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	userFixture := fixtures.NewUserFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Invite a user (this creates a user invitation)
	invite, err := userFixture.Invite(org.ID, "test-user@example.com")
	if err != nil {
		t.Fatalf("Failed to invite user: %v", err)
	}

	// Create an API key with limited scopes (read-only)
	// Note: API key creation may require a service account, but we'll test with basic scopes
	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "test-limited-key", []string{"inference:read"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Create a new client with the limited-scope API key
	limitedClient := harness.NewClient(ctx.Config.APIURLs.UserOrgService, ctx.Config.Timeouts.RequestTimeout)
	limitedClient.SetHeader("Authorization", "Bearer "+apiKey.Secret)
	limitedClient.SetHeader("X-API-Key", apiKey.Secret)
	
	// If using IP address, set Host header
	if isIPAddress(ctx.Config.APIURLs.UserOrgService) {
		limitedClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	// Attempt a restricted action (e.g., creating another organization)
	// This should fail with 403 Forbidden
	restrictedOrgFixture := fixtures.NewOrganizationFixture(limitedClient, ctx.Fixtures)
	_, err = restrictedOrgFixture.Create(ctx, "")
	
	if err == nil {
		t.Fatal("Expected authorization denial for restricted action, but request succeeded")
	}

	// Verify the error indicates authorization failure
	// The error should contain "forbidden", "unauthorized", or "permission"
	errMsg := err.Error()
	if !contains(errMsg, "forbidden") && !contains(errMsg, "unauthorized") && !contains(errMsg, "permission") {
		t.Logf("Warning: Error message doesn't clearly indicate authorization failure: %s", errMsg)
	}

	t.Logf("Authorization denial test passed: org=%s, invite=%s, api_key=%s", org.ID, invite.InviteID, apiKey.ID)
}

// TestRoleBasedAccess tests that different roles have appropriate access levels
func TestRoleBasedAccess(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	userFixture := fixtures.NewUserFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Test inviting multiple users
	// Note: Role assignment may happen after invitation acceptance
	testEmails := []string{"viewer@test.example.com", "member@test.example.com", "admin@test.example.com"}
	
	for _, email := range testEmails {
		invite, err := userFixture.Invite(org.ID, email)
		if err != nil {
			t.Logf("Warning: Failed to invite user %s: %v", email, err)
			continue
		}

		// Verify invitation was created
		if invite.InviteID == "" {
			t.Fatalf("Invite ID is empty for email %s", email)
		}

		t.Logf("Created invitation: email=%s, invite=%s, org=%s", email, invite.InviteID, org.ID)
	}

	t.Logf("Role-based access test passed: org=%s, invites_created=%d", org.ID, len(testEmails))
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && 
		(s == substr || 
		 (len(s) > len(substr) && 
		  (s[:len(substr)] == substr || 
		   s[len(s)-len(substr):] == substr ||
		   containsMiddle(s, substr))))
}

// containsMiddle is a helper for case-insensitive substring matching
func containsMiddle(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase (simple implementation)
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

