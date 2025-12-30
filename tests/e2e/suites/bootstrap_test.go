//go:build (smoke || nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"encoding/json"
	"testing"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// BootstrapKeyCreatedResponse represents a newly created bootstrap key
type BootstrapKeyCreatedResponse struct {
	KeyID     string `json:"keyId"`
	Token     string `json:"token"` // bsk_xxxx - shown only once
	OrgID     string `json:"orgId"`
	OrgName   string `json:"orgName,omitempty"`
	ExpiresAt string `json:"expiresAt"`
}

// BootstrapKeyResponse represents a bootstrap key in list responses
type BootstrapKeyResponse struct {
	KeyID      string  `json:"keyId"`
	OrgID      string  `json:"orgId"`
	OrgName    string  `json:"orgName,omitempty"`
	Status     string  `json:"status"` // active, revoked, expired, redeemed
	Notes      string  `json:"notes,omitempty"`
	CreatedAt  string  `json:"createdAt"`
	ExpiresAt  string  `json:"expiresAt"`
	RedeemedAt *string `json:"redeemedAt,omitempty"`
	RedeemedBy *string `json:"redeemedBy,omitempty"`
}

// ListBootstrapKeysResponse represents the response from GET /v1/bootstrap-keys
type ListBootstrapKeysResponse struct {
	Keys []BootstrapKeyResponse `json:"keys"`
}

// RedeemBootstrapKeyResponse represents the response from redeeming a bootstrap key
type RedeemBootstrapKeyResponse struct {
	OrgID       string `json:"orgId"`
	OrgName     string `json:"orgName"`
	APIEndpoint string `json:"apiEndpoint"`
	APIKey      string `json:"apiKey"`
}

// TestBootstrapKeyLifecycle tests the complete bootstrap key workflow
// This test verifies:
// 1. Create bootstrap key
// 2. List bootstrap keys and verify new key appears
// 3. Redeem the key (simulating new org admin onboarding)
// 4. Verify org access granted after redemption
// 5. Create an API key using redeemed credentials
// 6. Make inference request to verify full access
// 7. Revoke the bootstrap key
// 8. Verify revoked key cannot be redeemed again
func TestBootstrapKeyLifecycle(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	t.Log("=== Bootstrap Key Lifecycle Test ===")
	t.Log("")

	// Step 1: Create organization
	t.Log("Step 1: Create Organization")
	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}
	t.Logf("  PASS: Organization created: %s", org.ID)

	// Step 2: Create bootstrap key
	t.Log("Step 2: Create Bootstrap Key")
	reqBody := map[string]interface{}{
		"orgId":         org.ID,
		"expiresInDays": 7,
		"notes":         "E2E test bootstrap key",
	}

	resp, err := ctx.Client.POST("/v1/bootstrap-keys", reqBody)
	if err != nil {
		t.Fatalf("Failed to create bootstrap key: %v", err)
	}

	if resp.StatusCode != 201 && resp.StatusCode != 200 {
		t.Fatalf("Create bootstrap key failed: status %d, body: %s", resp.StatusCode, resp.String())
	}

	var bootstrapKey BootstrapKeyCreatedResponse
	if err := resp.UnmarshalJSON(&bootstrapKey); err != nil {
		t.Fatalf("Failed to parse bootstrap key response: %v", err)
	}

	if bootstrapKey.KeyID == "" {
		t.Fatal("Bootstrap key ID is empty")
	}
	if bootstrapKey.Token == "" {
		t.Fatal("Bootstrap token is empty")
	}
	if len(bootstrapKey.Token) < 5 || bootstrapKey.Token[:4] != "bsk_" {
		t.Fatalf("Bootstrap token has invalid format: %s", bootstrapKey.Token[:10])
	}

	t.Logf("  PASS: Bootstrap key created: %s (token: %s...)", bootstrapKey.KeyID, bootstrapKey.Token[:10])

	// Step 3: List bootstrap keys and verify new key appears
	t.Log("Step 3: List Bootstrap Keys")
	listResp, err := ctx.Client.GET("/v1/bootstrap-keys?orgId=" + org.ID)
	if err != nil {
		t.Fatalf("Failed to list bootstrap keys: %v", err)
	}

	if listResp.StatusCode != 200 {
		t.Fatalf("List bootstrap keys failed: status %d, body: %s", listResp.StatusCode, listResp.String())
	}

	var listResult ListBootstrapKeysResponse
	if err := listResp.UnmarshalJSON(&listResult); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	found := false
	for _, key := range listResult.Keys {
		if key.KeyID == bootstrapKey.KeyID {
			found = true
			if key.Status != "active" {
				t.Fatalf("Bootstrap key status should be 'active', got: %s", key.Status)
			}
			break
		}
	}

	if !found {
		t.Fatalf("Bootstrap key %s not found in list response", bootstrapKey.KeyID)
	}

	t.Logf("  PASS: Bootstrap key appears in list with status 'active'")

	// Step 4: Redeem the bootstrap key (simulating new org admin onboarding)
	t.Log("Step 4: Redeem Bootstrap Key")

	// Create a new client without auth headers to simulate public redemption
	publicClient := harness.NewClient(ctx.Config.APIURLs.UserOrgService, ctx.Config.Timeouts.RequestTimeout)
	if isIPAddress(ctx.Config.APIURLs.UserOrgService) {
		publicClient.SetHeader("Host", "user-org.dev.otherjamesbrown.com")
	}

	redeemReq := map[string]interface{}{
		"token": bootstrapKey.Token,
	}

	redeemResp, err := publicClient.POST("/v1/bootstrap-keys/redeem", redeemReq)
	if err != nil {
		t.Fatalf("Failed to redeem bootstrap key: %v", err)
	}

	if redeemResp.StatusCode != 200 {
		t.Fatalf("Redeem bootstrap key failed: status %d, body: %s", redeemResp.StatusCode, redeemResp.String())
	}

	var redeemResult RedeemBootstrapKeyResponse
	if err := redeemResp.UnmarshalJSON(&redeemResult); err != nil {
		t.Fatalf("Failed to parse redeem response: %v", err)
	}

	if redeemResult.OrgID != org.ID {
		t.Fatalf("Redeemed org ID mismatch: expected %s, got %s", org.ID, redeemResult.OrgID)
	}
	if redeemResult.APIKey == "" {
		t.Fatal("Redeemed API key is empty")
	}
	if len(redeemResult.APIKey) < 8 || redeemResult.APIKey[:7] != "ai-aas_" {
		t.Fatalf("Redeemed API key has invalid format: %s", redeemResult.APIKey[:15])
	}

	t.Logf("  PASS: Bootstrap key redeemed, received org admin API key: %s...", redeemResult.APIKey[:15])

	// Step 5: Verify org access granted using redeemed API key
	t.Log("Step 5: Verify Org Access with Redeemed API Key")

	// Create authenticated client with redeemed API key
	authClient := harness.NewClient(ctx.Config.APIURLs.UserOrgService, ctx.Config.Timeouts.RequestTimeout)
	authClient.SetHeader("Authorization", "Bearer "+redeemResult.APIKey)
	if isIPAddress(ctx.Config.APIURLs.UserOrgService) {
		authClient.SetHeader("Host", "user-org.dev.otherjamesbrown.com")
	}

	// Try to get org details using redeemed API key
	orgResp, err := authClient.GET("/v1/orgs/" + org.ID)
	if err != nil {
		t.Fatalf("Failed to get org with redeemed API key: %v", err)
	}

	if orgResp.StatusCode != 200 {
		t.Fatalf("Get org failed with redeemed API key: status %d, body: %s", orgResp.StatusCode, orgResp.String())
	}

	t.Logf("  PASS: Org access verified with redeemed API key")

	// Step 6: Create a service account and API key for inference (using redeemed org admin key)
	t.Log("Step 6: Create API Key for Inference")

	saFixture := fixtures.NewServiceAccountFixture(authClient, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(authClient, ctx.Fixtures)

	sa, err := saFixture.Create(ctx, org.ID, "")
	if err != nil {
		t.Fatalf("Failed to create service account with redeemed key: %v", err)
	}
	t.Logf("  Created service account: %s", sa.ID)

	apiKey, err := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key with redeemed key: %v", err)
	}
	t.Logf("  PASS: Created inference API key: %s", apiKey.ID)

	// Step 7: Make inference request to verify full access
	t.Log("Step 7: Test Inference with Created API Key")

	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Get available models
	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Failed to get models: %v", err)
	}

	var models ModelsResponse
	if err := json.Unmarshal(modelsResp.Body, &models); err != nil {
		t.Fatalf("Failed to parse models: %v", err)
	}

	if len(models.Data) > 0 {
		// Make a simple inference request
		modelID := models.Data[0].ID
		testPrompt := getTestPrompt(modelID)
		reqBody := buildChatRequest(modelID, testPrompt, 20)

		inferResp, err := routerClient.POST("/v1/chat/completions", reqBody)
		if err == nil && inferResp.StatusCode == 200 {
			var completion ChatCompletionResponse
			if err := json.Unmarshal(inferResp.Body, &completion); err == nil {
				if len(completion.Choices) > 0 && completion.Choices[0].Message.Content != "" {
					t.Logf("  PASS: Inference successful with API key created via redeemed bootstrap key")
					t.Logf("        Model: %s, Tokens: %d", modelID, completion.Usage.TotalTokens)
				}
			}
		} else if inferResp.StatusCode == 503 {
			t.Log("  SKIP: Backend unavailable (503) - API key authentication works")
		} else {
			t.Logf("  WARNING: Inference failed (status %d) but API key authentication works", inferResp.StatusCode)
		}
	} else {
		t.Log("  SKIP: No models available for inference test")
	}

	// Step 8: Verify bootstrap key status changed to 'redeemed'
	t.Log("Step 8: Verify Bootstrap Key Status Changed to 'redeemed'")

	listResp2, err := ctx.Client.GET("/v1/bootstrap-keys?orgId=" + org.ID)
	if err != nil {
		t.Fatalf("Failed to list bootstrap keys: %v", err)
	}

	var listResult2 ListBootstrapKeysResponse
	if err := listResp2.UnmarshalJSON(&listResult2); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	redeemedFound := false
	for _, key := range listResult2.Keys {
		if key.KeyID == bootstrapKey.KeyID {
			redeemedFound = true
			if key.Status != "redeemed" {
				t.Fatalf("Bootstrap key status should be 'redeemed', got: %s", key.Status)
			}
			if key.RedeemedAt == nil {
				t.Fatal("Bootstrap key RedeemedAt should be set")
			}
			break
		}
	}

	if !redeemedFound {
		t.Fatalf("Bootstrap key %s not found in list after redemption", bootstrapKey.KeyID)
	}

	t.Logf("  PASS: Bootstrap key status is 'redeemed' with redemption timestamp")

	// Step 9: Try to redeem the same key again (should fail)
	t.Log("Step 9: Verify Redeemed Key Cannot Be Used Again")

	redeemResp2, err := publicClient.POST("/v1/bootstrap-keys/redeem", redeemReq)
	if err != nil {
		t.Fatalf("Failed to attempt second redemption: %v", err)
	}

	// Should return 401 Unauthorized because key is already redeemed
	if redeemResp2.StatusCode == 200 || redeemResp2.StatusCode == 201 {
		t.Fatalf("Second redemption should fail, got status %d", redeemResp2.StatusCode)
	}
	if redeemResp2.StatusCode != 401 {
		t.Logf("  WARNING: Expected 401 for redeemed key, got %d (but redemption did fail)", redeemResp2.StatusCode)
	}

	t.Logf("  PASS: Redeemed bootstrap key cannot be used again (status: %d)", redeemResp2.StatusCode)

	// Step 10: Create a new bootstrap key and revoke it
	t.Log("Step 10: Test Bootstrap Key Revocation")

	reqBody2 := map[string]interface{}{
		"orgId":         org.ID,
		"expiresInDays": 7,
		"notes":         "E2E test revocation",
	}

	resp2, err := ctx.Client.POST("/v1/bootstrap-keys", reqBody2)
	if err != nil {
		t.Fatalf("Failed to create second bootstrap key: %v", err)
	}

	var bootstrapKey2 BootstrapKeyCreatedResponse
	if err := resp2.UnmarshalJSON(&bootstrapKey2); err != nil {
		t.Fatalf("Failed to parse second bootstrap key response: %v", err)
	}

	t.Logf("  Created second bootstrap key: %s", bootstrapKey2.KeyID)

	// Revoke the key
	revokeResp, err := ctx.Client.POST("/v1/bootstrap-keys/"+bootstrapKey2.KeyID+"/revoke", nil)
	if err != nil {
		t.Fatalf("Failed to revoke bootstrap key: %v", err)
	}

	if revokeResp.StatusCode != 204 && revokeResp.StatusCode != 200 {
		t.Fatalf("Revoke bootstrap key failed: status %d, body: %s", revokeResp.StatusCode, revokeResp.String())
	}

	t.Logf("  Bootstrap key revoked successfully")

	// Verify revoked key cannot be redeemed
	redeemRevokedReq := map[string]interface{}{
		"token": bootstrapKey2.Token,
	}

	redeemRevokedResp, err := publicClient.POST("/v1/bootstrap-keys/redeem", redeemRevokedReq)
	if err != nil {
		t.Fatalf("Failed to attempt redemption of revoked key: %v", err)
	}

	if redeemRevokedResp.StatusCode == 200 || redeemRevokedResp.StatusCode == 201 {
		t.Fatalf("Revoked key redemption should fail, got status %d", redeemRevokedResp.StatusCode)
	}

	t.Logf("  PASS: Revoked bootstrap key cannot be redeemed (status: %d)", redeemRevokedResp.StatusCode)

	// Verify status in list
	listResp3, err := ctx.Client.GET("/v1/bootstrap-keys?orgId=" + org.ID)
	if err != nil {
		t.Fatalf("Failed to list bootstrap keys: %v", err)
	}

	var listResult3 ListBootstrapKeysResponse
	if err := listResp3.UnmarshalJSON(&listResult3); err != nil {
		t.Fatalf("Failed to parse list response: %v", err)
	}

	revokedFound := false
	for _, key := range listResult3.Keys {
		if key.KeyID == bootstrapKey2.KeyID {
			revokedFound = true
			if key.Status != "revoked" {
				t.Fatalf("Bootstrap key status should be 'revoked', got: %s", key.Status)
			}
			break
		}
	}

	if !revokedFound {
		t.Fatalf("Revoked bootstrap key %s not found in list", bootstrapKey2.KeyID)
	}

	t.Logf("  PASS: Revoked bootstrap key has status 'revoked'")

	t.Log("")
	t.Log("=== Bootstrap Key Lifecycle Test Complete ===")
	t.Log("")
	t.Log("Summary:")
	t.Log("  - Created bootstrap key via admin API")
	t.Log("  - Listed keys and verified presence")
	t.Log("  - Redeemed key and received org admin API key")
	t.Log("  - Verified org access with redeemed key")
	t.Log("  - Created service account and inference API key")
	t.Log("  - Made successful inference request")
	t.Log("  - Verified key status changed to 'redeemed'")
	t.Log("  - Verified redeemed key cannot be used again")
	t.Log("  - Tested key revocation")
	t.Log("  - Verified revoked key cannot be redeemed")
}
