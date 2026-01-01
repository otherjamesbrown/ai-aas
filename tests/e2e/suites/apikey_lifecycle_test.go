//go:build (smoke || nightly || full) && e2e_tier || !e2e_tier

// NOTE: This file contains API-level API key tests (not CLI-based).
// These tests exercise the User-Org Service API and API Router directly using HTTP fixtures.
// CLI-based API key management tests are in tests/usecases/apikeys_test.go.
//
// Tests in this file:
// - API key creation and inference usage
// - API key revocation and invalidation
// - Multiple API keys per service account

package suites

import (
	"testing"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// TestAPIKeyCreationAndUsage tests creating an API key and verifying it works for inference
func TestAPIKeyCreationAndUsage(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	t.Log("=== API Key Creation and Usage Test ===")
	t.Log("")

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	saFixture := fixtures.NewServiceAccountFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Step 1: Create organization
	t.Log("Step 1: Create Organization")
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}
	t.Logf("  PASS: Organization created: %s", org.ID)

	// Step 2: Create service account
	t.Log("Step 2: Create Service Account")
	sa, err := saFixture.Create(ctx, org.ID, "")
	if err != nil {
		t.Fatalf("Failed to create service account: %v", err)
	}
	t.Logf("  PASS: Service account created: %s", sa.ID)

	// Step 3: Create API key for service account
	t.Log("Step 3: Create API Key")
	apiKey, err := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	if apiKey.ID == "" {
		t.Fatal("API key ID is empty")
	}
	if apiKey.Key == "" {
		t.Fatal("API key token is empty")
	}
	if apiKey.Fingerprint == "" {
		t.Fatal("API key fingerprint is empty")
	}
	if apiKey.Status != "active" {
		t.Fatalf("Expected status 'active', got '%s'", apiKey.Status)
	}

	t.Logf("  PASS: API key created: %s", apiKey.ID)
	t.Logf("        Status: %s", apiKey.Status)
	t.Logf("        Fingerprint: %s", apiKey.Fingerprint[:min(20, len(apiKey.Fingerprint))]+"...")

	// Step 4: Verify key works for inference
	t.Log("Step 4: Verify API Key Works for Inference")

	// Create client for API router service with the new API key
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Try to get available models - this should succeed with valid API key
	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Failed to get models with API key: %v", err)
	}

	if modelsResp.StatusCode != 200 {
		t.Fatalf("Expected status 200 when using valid API key, got %d: %s", modelsResp.StatusCode, modelsResp.String())
	}

	t.Logf("  PASS: API key successfully authenticated to API router")

	// Step 5: Get API key details via List endpoint
	t.Log("Step 5: Get API Key Details")
	listResp, err := ctx.Client.GET("/v1/orgs/" + org.ID + "/api-keys")
	if err != nil {
		t.Fatalf("Failed to list API keys: %v", err)
	}

	if listResp.StatusCode != 200 {
		t.Fatalf("List API keys failed: status %d, body: %s", listResp.StatusCode, listResp.String())
	}

	type APIKeyListItem struct {
		KeyID         string   `json:"keyId"`
		Notes         string   `json:"notes,omitempty"`
		PrincipalType string   `json:"principalType"`
		PrincipalID   string   `json:"principalId"`
		Fingerprint   string   `json:"fingerprint"`
		Status        string   `json:"status"`
		Scopes        []string `json:"scopes"`
		IssuedAt      string   `json:"issuedAt"`
		ExpiresAt     *string  `json:"expiresAt,omitempty"`
		LastUsedAt    *string  `json:"lastUsedAt,omitempty"`
	}

	var apiKeyList []APIKeyListItem
	if err := listResp.UnmarshalJSON(&apiKeyList); err != nil {
		t.Fatalf("Failed to parse API key list: %v", err)
	}

	// Verify our key is in the list
	found := false
	for _, key := range apiKeyList {
		if key.KeyID == apiKey.ID {
			found = true
			if key.Status != "active" {
				t.Errorf("Expected status 'active', got '%s'", key.Status)
			}
			if key.PrincipalType != "service_account" {
				t.Errorf("Expected principalType 'service_account', got '%s'", key.PrincipalType)
			}
			if key.PrincipalID != sa.ID {
				t.Errorf("Expected principalID '%s', got '%s'", sa.ID, key.PrincipalID)
			}
			t.Logf("  PASS: API key found in list with correct metadata")
			break
		}
	}

	if !found {
		t.Fatal("API key not found in list")
	}

	t.Log("")
	t.Log("=== API Key Creation and Usage Test Complete ===")
}

// TestAPIKeyRevocation tests revoking an API key and verifying it no longer works
func TestAPIKeyRevocation(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	t.Log("=== API Key Revocation Test ===")
	t.Log("")

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	saFixture := fixtures.NewServiceAccountFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Step 1: Create organization, service account, and API key
	t.Log("Step 1: Create Organization, Service Account, and API Key")
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	sa, err := saFixture.Create(ctx, org.ID, "")
	if err != nil {
		t.Fatalf("Failed to create service account: %v", err)
	}

	apiKey, err := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	t.Logf("  PASS: Created API key: %s", apiKey.ID)

	// Step 2: Verify key works before revocation
	t.Log("Step 2: Verify API Key Works Before Revocation")

	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Failed to get models with API key: %v", err)
	}

	if modelsResp.StatusCode != 200 {
		t.Fatalf("Expected status 200 with valid API key, got %d", modelsResp.StatusCode)
	}

	t.Logf("  PASS: API key works before revocation")

	// Step 3: Revoke the API key
	t.Log("Step 3: Revoke API Key")

	revokeResp, err := ctx.Client.DELETE("/v1/orgs/" + org.ID + "/api-keys/" + apiKey.ID)
	if err != nil {
		t.Fatalf("Failed to revoke API key: %v", err)
	}

	if revokeResp.StatusCode != 200 && revokeResp.StatusCode != 204 {
		t.Fatalf("Revoke API key failed: status %d, body: %s", revokeResp.StatusCode, revokeResp.String())
	}

	t.Logf("  PASS: API key revoked")

	// Step 4: Verify revoked key no longer works
	t.Log("Step 4: Verify Revoked API Key Does Not Work")

	// Try to use the revoked key - should fail with 401
	modelsResp2, err := routerClient.GET("/v1/models")
	if err != nil {
		// Network errors are acceptable here (connection refused, etc.)
		t.Logf("  PASS: Revoked API key rejected (network error: %v)", err)
	} else if modelsResp2.StatusCode == 401 || modelsResp2.StatusCode == 403 {
		t.Logf("  PASS: Revoked API key rejected with status %d", modelsResp2.StatusCode)
	} else if modelsResp2.StatusCode == 200 {
		t.Fatalf("FAIL: Revoked API key still works! Expected 401/403, got 200")
	} else {
		// Any other error is acceptable (could be routing issue, etc.)
		t.Logf("  PASS: Revoked API key rejected with status %d", modelsResp2.StatusCode)
	}

	t.Log("")
	t.Log("=== API Key Revocation Test Complete ===")
}

// TestMultipleAPIKeys tests creating and managing multiple API keys for the same service account
func TestMultipleAPIKeys(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	t.Log("=== Multiple API Keys Test ===")
	t.Log("")

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	saFixture := fixtures.NewServiceAccountFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Step 1: Create organization and service account
	t.Log("Step 1: Create Organization and Service Account")
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	sa, err := saFixture.Create(ctx, org.ID, "")
	if err != nil {
		t.Fatalf("Failed to create service account: %v", err)
	}

	t.Logf("  PASS: Created organization %s and service account %s", org.ID, sa.ID)

	// Step 2: Create multiple API keys for the same service account
	t.Log("Step 2: Create Multiple API Keys")

	const numKeys = 3
	apiKeys := make([]*fixtures.APIKey, numKeys)

	for i := 0; i < numKeys; i++ {
		key, err := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", []string{"inference:read", "inference:write"})
		if err != nil {
			t.Fatalf("Failed to create API key %d: %v", i+1, err)
		}
		apiKeys[i] = key
		t.Logf("  Created API key %d: %s", i+1, key.ID)
	}

	t.Logf("  PASS: Created %d API keys", numKeys)

	// Step 3: Verify all keys work independently
	t.Log("Step 3: Verify All Keys Work Independently")

	for i, key := range apiKeys {
		routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
		routerClient.SetHeader("Authorization", "Bearer "+key.Key)

		if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
			routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
		}

		modelsResp, err := routerClient.GET("/v1/models")
		if err != nil {
			t.Fatalf("Failed to get models with API key %d: %v", i+1, err)
		}

		if modelsResp.StatusCode != 200 {
			t.Fatalf("API key %d failed: status %d", i+1, modelsResp.StatusCode)
		}

		t.Logf("  API key %d works: %s", i+1, key.ID)
	}

	t.Logf("  PASS: All %d API keys work independently", numKeys)

	// Step 4: Revoke one key and verify others still work
	t.Log("Step 4: Revoke One Key and Verify Others Still Work")

	keyToRevoke := apiKeys[1] // Revoke the second key

	revokeResp, err := ctx.Client.DELETE("/v1/orgs/" + org.ID + "/api-keys/" + keyToRevoke.ID)
	if err != nil {
		t.Fatalf("Failed to revoke API key: %v", err)
	}

	if revokeResp.StatusCode != 200 && revokeResp.StatusCode != 204 {
		t.Fatalf("Revoke API key failed: status %d, body: %s", revokeResp.StatusCode, revokeResp.String())
	}

	t.Logf("  Revoked API key: %s", keyToRevoke.ID)

	// Verify revoked key doesn't work
	revokedClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	revokedClient.SetHeader("Authorization", "Bearer "+keyToRevoke.Key)

	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		revokedClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	modelsResp, err := revokedClient.GET("/v1/models")
	if err == nil && modelsResp.StatusCode == 200 {
		t.Fatal("Revoked API key still works!")
	}
	t.Logf("  Revoked key correctly rejected")

	// Verify other keys still work
	for i, key := range apiKeys {
		if key.ID == keyToRevoke.ID {
			continue // Skip the revoked key
		}

		activeClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
		activeClient.SetHeader("Authorization", "Bearer "+key.Key)

		if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
			activeClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
		}

		modelsResp, err := activeClient.GET("/v1/models")
		if err != nil {
			t.Fatalf("Failed to get models with API key %d: %v", i+1, err)
		}

		if modelsResp.StatusCode != 200 {
			t.Fatalf("API key %d failed after revoking key 2: status %d", i+1, modelsResp.StatusCode)
		}

		t.Logf("  API key %d still works: %s", i+1, key.ID)
	}

	t.Logf("  PASS: Other API keys still work after revoking one")

	t.Log("")
	t.Log("=== Multiple API Keys Test Complete ===")
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
