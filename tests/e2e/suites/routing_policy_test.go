//go:build (smoke || nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"testing"

	"github.com/ai-aas/tests/e2e/fixtures"
)

// TestRoutingPolicyLifecycle tests the complete routing policy lifecycle
// This test covers policy creation, validation, activation, deactivation, and deletion
func TestRoutingPolicyLifecycle(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	t.Log("=== Routing Policy Lifecycle Test ===")
	t.Log("")

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)

	// Step 1: Create organization
	t.Log("Step 1: Create Organization")
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}
	t.Logf("  PASS: Organization created: %s", org.ID)

	// Step 2: Create routing policy
	t.Log("Step 2: Create Routing Policy")

	policyReq := map[string]interface{}{
		"organization_id": org.ID,
		"model":           "test-model",
		"backend_type":    "openai",
		"backends": []map[string]interface{}{
			{
				"backend_id": "backend-1",
				"weight":     60,
			},
			{
				"backend_id": "backend-2",
				"weight":     40,
			},
		},
		"failover_threshold": 3,
		"enabled":            false, // Start disabled
		"metadata": map[string]interface{}{
			"description": "E2E test policy",
		},
	}

	createResp, err := ctx.Client.POST("/v1/routing/policies", policyReq)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if createResp.StatusCode != 201 {
		t.Fatalf("Create policy failed: status %d, body: %s", createResp.StatusCode, createResp.String())
	}

	type PolicyResponse struct {
		PolicyID          string                   `json:"policy_id"`
		OrganizationID    string                   `json:"organization_id"`
		Model             string                   `json:"model"`
		BackendType       string                   `json:"backend_type"`
		Backends          []map[string]interface{} `json:"backends"`
		FailoverThreshold int                      `json:"failover_threshold"`
		Enabled           bool                     `json:"enabled"`
		Version           int                      `json:"version"`
	}

	var createdPolicy PolicyResponse
	if err := createResp.UnmarshalJSON(&createdPolicy); err != nil {
		t.Fatalf("Failed to parse created policy: %v", err)
	}

	if createdPolicy.PolicyID == "" {
		t.Fatal("Policy ID is empty")
	}
	if createdPolicy.Model != "test-model" {
		t.Fatalf("Expected model 'test-model', got '%s'", createdPolicy.Model)
	}
	if createdPolicy.Enabled {
		t.Fatal("Policy should be disabled on creation")
	}

	t.Logf("  PASS: Policy created: %s", createdPolicy.PolicyID)
	t.Logf("        Model: %s", createdPolicy.Model)
	t.Logf("        Backend Type: %s", createdPolicy.BackendType)
	t.Logf("        Backends: %d", len(createdPolicy.Backends))
	t.Logf("        Enabled: %v", createdPolicy.Enabled)

	// Step 3: Validate policy (using validate endpoint)
	t.Log("Step 3: Validate Policy Configuration")

	validateReq := map[string]interface{}{
		"organization_id": org.ID,
		"model":           "validation-test",
		"backend_type":    "openai",
		"backends": []map[string]interface{}{
			{
				"backend_id": "backend-1",
				"weight":     100,
			},
		},
	}

	validateResp, err := ctx.Client.POST("/v1/routing/policies/validate", validateReq)
	if err != nil {
		t.Fatalf("Failed to validate policy: %v", err)
	}

	if validateResp.StatusCode != 200 {
		t.Fatalf("Validate policy failed: status %d, body: %s", validateResp.StatusCode, validateResp.String())
	}

	type ValidationResponse struct {
		Valid    bool                     `json:"valid"`
		Errors   []map[string]interface{} `json:"errors,omitempty"`
		Warnings []string                 `json:"warnings,omitempty"`
	}

	var validation ValidationResponse
	if err := validateResp.UnmarshalJSON(&validation); err != nil {
		t.Fatalf("Failed to parse validation response: %v", err)
	}

	if !validation.Valid {
		t.Fatalf("Policy validation failed: %v", validation.Errors)
	}

	t.Logf("  PASS: Policy configuration validated successfully")

	// Step 4: List policies and verify our policy appears
	t.Log("Step 4: List Policies")

	listResp, err := ctx.Client.GET("/v1/routing/policies?organization_id=" + org.ID)
	if err != nil {
		t.Fatalf("Failed to list policies: %v", err)
	}

	if listResp.StatusCode != 200 {
		t.Fatalf("List policies failed: status %d, body: %s", listResp.StatusCode, listResp.String())
	}

	type PolicyListResponse struct {
		Policies   []PolicyResponse `json:"policies"`
		Pagination map[string]interface{} `json:"pagination"`
	}

	var listResult PolicyListResponse
	if err := listResp.UnmarshalJSON(&listResult); err != nil {
		t.Fatalf("Failed to parse policy list: %v", err)
	}

	// Verify our policy is in the list
	found := false
	for _, policy := range listResult.Policies {
		if policy.PolicyID == createdPolicy.PolicyID {
			found = true
			if policy.Model != "test-model" {
				t.Errorf("Expected model 'test-model', got '%s'", policy.Model)
			}
			t.Logf("  PASS: Policy found in list")
			break
		}
	}

	if !found {
		t.Fatalf("Policy %s not found in list", createdPolicy.PolicyID)
	}

	// Step 5: Get policy details
	t.Log("Step 5: Get Policy Details")

	getResp, err := ctx.Client.GET("/v1/routing/policies/" + createdPolicy.PolicyID)
	if err != nil {
		t.Fatalf("Failed to get policy: %v", err)
	}

	if getResp.StatusCode != 200 {
		t.Fatalf("Get policy failed: status %d, body: %s", getResp.StatusCode, getResp.String())
	}

	var retrievedPolicy PolicyResponse
	if err := getResp.UnmarshalJSON(&retrievedPolicy); err != nil {
		t.Fatalf("Failed to parse retrieved policy: %v", err)
	}

	if retrievedPolicy.PolicyID != createdPolicy.PolicyID {
		t.Fatalf("Policy ID mismatch: expected %s, got %s", createdPolicy.PolicyID, retrievedPolicy.PolicyID)
	}

	t.Logf("  PASS: Policy details retrieved")

	// Step 6: Activate policy
	t.Log("Step 6: Activate Policy")

	activateResp, err := ctx.Client.POST("/v1/routing/policies/"+createdPolicy.PolicyID+"/activate", nil)
	if err != nil {
		t.Fatalf("Failed to activate policy: %v", err)
	}

	if activateResp.StatusCode != 200 {
		t.Fatalf("Activate policy failed: status %d, body: %s", activateResp.StatusCode, activateResp.String())
	}

	type ActivationResponse struct {
		PolicyID string `json:"policy_id"`
		Enabled  bool   `json:"enabled"`
		Message  string `json:"message"`
	}

	var activateResult ActivationResponse
	if err := activateResp.UnmarshalJSON(&activateResult); err != nil {
		t.Fatalf("Failed to parse activation response: %v", err)
	}

	if !activateResult.Enabled {
		t.Fatal("Policy should be enabled after activation")
	}

	t.Logf("  PASS: Policy activated")
	t.Logf("        Message: %s", activateResult.Message)

	// Verify activation by getting policy again
	getResp2, err := ctx.Client.GET("/v1/routing/policies/" + createdPolicy.PolicyID)
	if err != nil {
		t.Fatalf("Failed to get policy after activation: %v", err)
	}

	var activatedPolicy PolicyResponse
	if err := getResp2.UnmarshalJSON(&activatedPolicy); err != nil {
		t.Fatalf("Failed to parse activated policy: %v", err)
	}

	if !activatedPolicy.Enabled {
		t.Fatal("Policy enabled flag not updated after activation")
	}

	t.Logf("  PASS: Policy enabled flag verified")

	// Step 7: Deactivate policy
	t.Log("Step 7: Deactivate Policy")

	deactivateResp, err := ctx.Client.POST("/v1/routing/policies/"+createdPolicy.PolicyID+"/deactivate", nil)
	if err != nil {
		t.Fatalf("Failed to deactivate policy: %v", err)
	}

	if deactivateResp.StatusCode != 200 {
		t.Fatalf("Deactivate policy failed: status %d, body: %s", deactivateResp.StatusCode, deactivateResp.String())
	}

	var deactivateResult ActivationResponse
	if err := deactivateResp.UnmarshalJSON(&deactivateResult); err != nil {
		t.Fatalf("Failed to parse deactivation response: %v", err)
	}

	if deactivateResult.Enabled {
		t.Fatal("Policy should be disabled after deactivation")
	}

	t.Logf("  PASS: Policy deactivated")

	// Step 8: Delete policy
	t.Log("Step 8: Delete Policy")

	deleteResp, err := ctx.Client.DELETE("/v1/routing/policies/" + createdPolicy.PolicyID)
	if err != nil {
		t.Fatalf("Failed to delete policy: %v", err)
	}

	if deleteResp.StatusCode != 204 && deleteResp.StatusCode != 200 {
		t.Fatalf("Delete policy failed: status %d, body: %s", deleteResp.StatusCode, deleteResp.String())
	}

	t.Logf("  PASS: Policy deleted")

	// Verify deletion by trying to get the policy
	getResp3, err := ctx.Client.GET("/v1/routing/policies/" + createdPolicy.PolicyID)
	if err == nil && getResp3.StatusCode == 200 {
		t.Fatal("Policy still exists after deletion")
	}

	t.Logf("  PASS: Policy deletion verified")

	t.Log("")
	t.Log("=== Routing Policy Lifecycle Test Complete ===")
}

// TestRoutingPolicyValidation tests various policy validation scenarios
func TestRoutingPolicyValidation(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	t.Log("=== Routing Policy Validation Test ===")
	t.Log("")

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	t.Log("Step 1: Create Organization")
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}
	t.Logf("  PASS: Organization created: %s", org.ID)

	// Test Case 1: Valid policy with correct weight distribution
	t.Log("Step 2: Test Valid Policy (Weights sum to 100)")

	validPolicy := map[string]interface{}{
		"organization_id": org.ID,
		"model":           "valid-test",
		"backend_type":    "openai",
		"backends": []map[string]interface{}{
			{"backend_id": "backend-1", "weight": 50},
			{"backend_id": "backend-2", "weight": 30},
			{"backend_id": "backend-3", "weight": 20},
		},
	}

	createResp, err := ctx.Client.POST("/v1/routing/policies", validPolicy)
	if err != nil {
		t.Fatalf("Failed to create valid policy: %v", err)
	}

	if createResp.StatusCode != 201 {
		t.Fatalf("Valid policy creation failed: status %d, body: %s", createResp.StatusCode, createResp.String())
	}

	type PolicyResponse struct {
		PolicyID string `json:"policy_id"`
	}

	var validPolicyResult PolicyResponse
	if err := createResp.UnmarshalJSON(&validPolicyResult); err != nil {
		t.Fatalf("Failed to parse valid policy: %v", err)
	}

	t.Logf("  PASS: Valid policy created: %s", validPolicyResult.PolicyID)

	// Test Case 2: Invalid policy with incorrect weight distribution
	t.Log("Step 3: Test Invalid Policy (Weights don't sum to 100)")

	invalidPolicy := map[string]interface{}{
		"organization_id": org.ID,
		"model":           "invalid-test",
		"backend_type":    "openai",
		"backends": []map[string]interface{}{
			{"backend_id": "backend-1", "weight": 50},
			{"backend_id": "backend-2", "weight": 30},
			// Total: 80, not 100
		},
	}

	createResp2, err := ctx.Client.POST("/v1/routing/policies", invalidPolicy)
	if err != nil {
		t.Fatalf("Failed to call create endpoint: %v", err)
	}

	// Should fail validation
	if createResp2.StatusCode == 201 {
		t.Fatal("Invalid policy was accepted (weights should sum to 100)")
	}

	if createResp2.StatusCode != 400 {
		t.Logf("  WARNING: Expected status 400 for invalid weights, got %d", createResp2.StatusCode)
	} else {
		t.Logf("  PASS: Invalid policy rejected with status 400")
	}

	// Test Case 3: Policy with missing required field
	t.Log("Step 4: Test Invalid Policy (Missing Model Field)")

	missingModelPolicy := map[string]interface{}{
		"organization_id": org.ID,
		// Missing "model" field
		"backend_type": "openai",
		"backends": []map[string]interface{}{
			{"backend_id": "backend-1", "weight": 100},
		},
	}

	createResp3, err := ctx.Client.POST("/v1/routing/policies", missingModelPolicy)
	if err != nil {
		t.Fatalf("Failed to call create endpoint: %v", err)
	}

	// Should fail validation
	if createResp3.StatusCode == 201 {
		t.Fatal("Policy without model field was accepted")
	}

	if createResp3.StatusCode != 400 {
		t.Logf("  WARNING: Expected status 400 for missing model, got %d", createResp3.StatusCode)
	} else {
		t.Logf("  PASS: Policy without model rejected with status 400")
	}

	// Test Case 4: Policy with invalid backend type
	t.Log("Step 5: Test Invalid Policy (Invalid Backend Type)")

	invalidBackendType := map[string]interface{}{
		"organization_id": org.ID,
		"model":           "test-model",
		"backend_type":    "invalid-backend-type",
		"backends": []map[string]interface{}{
			{"backend_id": "backend-1", "weight": 100},
		},
	}

	createResp4, err := ctx.Client.POST("/v1/routing/policies", invalidBackendType)
	if err != nil {
		t.Fatalf("Failed to call create endpoint: %v", err)
	}

	// Should fail validation
	if createResp4.StatusCode == 201 {
		t.Fatal("Policy with invalid backend_type was accepted")
	}

	if createResp4.StatusCode != 400 {
		t.Logf("  WARNING: Expected status 400 for invalid backend_type, got %d", createResp4.StatusCode)
	} else {
		t.Logf("  PASS: Policy with invalid backend_type rejected with status 400")
	}

	// Test Case 5: Triton policy with tokenizer requirement
	t.Log("Step 6: Test Triton Policy (With Tokenizer)")

	tritonPolicy := map[string]interface{}{
		"organization_id": org.ID,
		"model":           "triton-test",
		"backend_type":    "triton-grpc",
		"tokenizer":       "llama3",
		"backends": []map[string]interface{}{
			{"backend_id": "triton-backend", "weight": 100},
		},
	}

	createResp5, err := ctx.Client.POST("/v1/routing/policies", tritonPolicy)
	if err != nil {
		t.Fatalf("Failed to create triton policy: %v", err)
	}

	if createResp5.StatusCode != 201 {
		t.Fatalf("Triton policy creation failed: status %d, body: %s", createResp5.StatusCode, createResp5.String())
	}

	t.Logf("  PASS: Triton policy with tokenizer created")

	// Cleanup - delete the created policies
	if validPolicyResult.PolicyID != "" {
		ctx.Client.DELETE("/v1/routing/policies/" + validPolicyResult.PolicyID)
	}

	t.Log("")
	t.Log("=== Routing Policy Validation Test Complete ===")
}

// TestRoutingPolicyUpdate tests updating routing policy configuration
func TestRoutingPolicyUpdate(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	t.Log("=== Routing Policy Update Test ===")
	t.Log("")

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)

	// Step 1: Create organization
	t.Log("Step 1: Create Organization")
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}
	t.Logf("  PASS: Organization created: %s", org.ID)

	// Step 2: Create initial policy
	t.Log("Step 2: Create Initial Policy")

	initialPolicy := map[string]interface{}{
		"organization_id": org.ID,
		"model":           "update-test",
		"backend_type":    "openai",
		"backends": []map[string]interface{}{
			{"backend_id": "backend-1", "weight": 100},
		},
		"failover_threshold": 3,
	}

	createResp, err := ctx.Client.POST("/v1/routing/policies", initialPolicy)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	if createResp.StatusCode != 201 {
		t.Fatalf("Create policy failed: status %d, body: %s", createResp.StatusCode, createResp.String())
	}

	type PolicyResponse struct {
		PolicyID          string                   `json:"policy_id"`
		Backends          []map[string]interface{} `json:"backends"`
		FailoverThreshold int                      `json:"failover_threshold"`
		Version           int                      `json:"version"`
	}

	var createdPolicy PolicyResponse
	if err := createResp.UnmarshalJSON(&createdPolicy); err != nil {
		t.Fatalf("Failed to parse created policy: %v", err)
	}

	t.Logf("  PASS: Policy created: %s (version %d)", createdPolicy.PolicyID, createdPolicy.Version)

	// Step 3: Update policy backends
	t.Log("Step 3: Update Policy Backends")

	updateReq := map[string]interface{}{
		"backends": []map[string]interface{}{
			{"backend_id": "backend-1", "weight": 60},
			{"backend_id": "backend-2", "weight": 40},
		},
	}

	updateResp, err := ctx.Client.PATCH("/v1/routing/policies/"+createdPolicy.PolicyID, updateReq)
	if err != nil {
		t.Fatalf("Failed to update policy: %v", err)
	}

	if updateResp.StatusCode != 200 {
		t.Fatalf("Update policy failed: status %d, body: %s", updateResp.StatusCode, updateResp.String())
	}

	var updatedPolicy PolicyResponse
	if err := updateResp.UnmarshalJSON(&updatedPolicy); err != nil {
		t.Fatalf("Failed to parse updated policy: %v", err)
	}

	if len(updatedPolicy.Backends) != 2 {
		t.Fatalf("Expected 2 backends after update, got %d", len(updatedPolicy.Backends))
	}

	if updatedPolicy.Version <= createdPolicy.Version {
		t.Fatalf("Policy version should increment after update: before=%d, after=%d",
			createdPolicy.Version, updatedPolicy.Version)
	}

	t.Logf("  PASS: Policy backends updated (version %d)", updatedPolicy.Version)
	t.Logf("        Backends: %d", len(updatedPolicy.Backends))

	// Step 4: Update failover threshold
	t.Log("Step 4: Update Failover Threshold")

	failoverUpdate := map[string]interface{}{
		"failover_threshold": 5,
	}

	updateResp2, err := ctx.Client.PATCH("/v1/routing/policies/"+createdPolicy.PolicyID, failoverUpdate)
	if err != nil {
		t.Fatalf("Failed to update failover threshold: %v", err)
	}

	if updateResp2.StatusCode != 200 {
		t.Fatalf("Update failover threshold failed: status %d, body: %s", updateResp2.StatusCode, updateResp2.String())
	}

	var updatedPolicy2 PolicyResponse
	if err := updateResp2.UnmarshalJSON(&updatedPolicy2); err != nil {
		t.Fatalf("Failed to parse updated policy: %v", err)
	}

	if updatedPolicy2.FailoverThreshold != 5 {
		t.Fatalf("Expected failover_threshold 5, got %d", updatedPolicy2.FailoverThreshold)
	}

	t.Logf("  PASS: Failover threshold updated to %d", updatedPolicy2.FailoverThreshold)

	// Cleanup
	ctx.Client.DELETE("/v1/routing/policies/" + createdPolicy.PolicyID)

	t.Log("")
	t.Log("=== Routing Policy Update Test Complete ===")
}
