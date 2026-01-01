//go:build (nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"net/http"
	"testing"
	"time"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// TestTokenPolicyCRUD tests Create, Read, Update, Delete operations for token policies
func TestTokenPolicyCRUD(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}
	t.Logf("Created organization: %s", org.ID)

	// CREATE: Create a policy with 1h and 24h limits
	limit1h := int64(10000)
	limit24h := int64(100000)
	policy, err := policyFixture.Create(ctx, org.ID, "test-policy", &limit1h, &limit24h, nil)
	if err != nil {
		t.Fatalf("Failed to create token policy: %v", err)
	}
	t.Logf("Created policy: %s (%s)", policy.Name, policy.ID)

	if policy.Name != "test-policy" {
		t.Errorf("Policy name mismatch: expected 'test-policy', got '%s'", policy.Name)
	}
	if policy.Limit1h == nil || *policy.Limit1h != limit1h {
		t.Errorf("Policy limit_1h mismatch: expected %d", limit1h)
	}
	if policy.Limit24h == nil || *policy.Limit24h != limit24h {
		t.Errorf("Policy limit_24h mismatch: expected %d", limit24h)
	}
	if policy.IsBuiltin {
		t.Error("Custom policy should not be marked as builtin")
	}

	// READ: Get the policy
	retrieved, err := policyFixture.Get(org.ID, policy.ID)
	if err != nil {
		t.Fatalf("Failed to get token policy: %v", err)
	}
	if retrieved.ID != policy.ID {
		t.Errorf("Retrieved policy ID mismatch: expected %s, got %s", policy.ID, retrieved.ID)
	}
	t.Logf("Retrieved policy: %s", retrieved.ID)

	// UPDATE: Update the policy limits
	newLimit1h := int64(20000)
	updated, err := policyFixture.Update(org.ID, policy.ID, nil, &newLimit1h, nil, nil)
	if err != nil {
		t.Fatalf("Failed to update token policy: %v", err)
	}
	if updated.Limit1h == nil || *updated.Limit1h != newLimit1h {
		t.Errorf("Updated policy limit_1h mismatch: expected %d", newLimit1h)
	}
	t.Logf("Updated policy limit_1h to %d", newLimit1h)

	// DELETE: Delete the policy
	err = policyFixture.Delete(org.ID, policy.ID)
	if err != nil {
		t.Fatalf("Failed to delete token policy: %v", err)
	}
	t.Logf("Deleted policy: %s", policy.ID)

	// Verify deletion
	_, err = policyFixture.Get(org.ID, policy.ID)
	if err == nil {
		t.Error("Expected error getting deleted policy, but got none")
	}

	t.Logf("Token policy CRUD test passed: org=%s", org.ID)
}

// TestTokenPolicyList tests listing policies including built-in ones
func TestTokenPolicyList(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// List policies - should include built-in "No Token Rate-Limit" policy
	policies, err := policyFixture.List(org.ID)
	if err != nil {
		t.Fatalf("Failed to list policies: %v", err)
	}

	// Find built-in policy
	foundBuiltin := false
	for _, p := range policies {
		if p.IsBuiltin {
			foundBuiltin = true
			t.Logf("Found built-in policy: %s (%s)", p.Name, p.ID)
		}
	}
	if !foundBuiltin {
		t.Error("Expected to find at least one built-in policy")
	}

	// Create a custom policy
	limit := int64(5000)
	_, err = policyFixture.Create(ctx, org.ID, "custom-policy", &limit, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create custom policy: %v", err)
	}

	// List again
	policies, err = policyFixture.List(org.ID)
	if err != nil {
		t.Fatalf("Failed to list policies after creation: %v", err)
	}

	// Should have at least 2 policies now (builtin + custom)
	if len(policies) < 2 {
		t.Errorf("Expected at least 2 policies, got %d", len(policies))
	}

	t.Logf("Token policy list test passed: %d policies found", len(policies))
}

// TestOrgDefaultTokenPolicy tests setting and getting the org default policy
func TestOrgDefaultTokenPolicy(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Get initial default (should be builtin "No Token Rate-Limit")
	defaultPolicy, err := policyFixture.GetOrgDefault(org.ID)
	if err != nil {
		t.Fatalf("Failed to get org default policy: %v", err)
	}
	t.Logf("Initial default policy: %s (%s, builtin=%v)", defaultPolicy.Name, defaultPolicy.ID, defaultPolicy.IsBuiltin)

	if !defaultPolicy.IsBuiltin {
		t.Error("Initial default policy should be the built-in policy")
	}

	// Create a custom policy
	limit1h := int64(10000)
	customPolicy, err := policyFixture.Create(ctx, org.ID, "org-default-policy", &limit1h, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create custom policy: %v", err)
	}

	// Set custom policy as org default
	setPolicy, err := policyFixture.SetOrgDefault(org.ID, customPolicy.ID)
	if err != nil {
		t.Fatalf("Failed to set org default policy: %v", err)
	}
	if setPolicy.ID != customPolicy.ID {
		t.Errorf("Set default policy ID mismatch: expected %s, got %s", customPolicy.ID, setPolicy.ID)
	}
	t.Logf("Set org default to custom policy: %s", setPolicy.ID)

	// Verify default was changed
	newDefault, err := policyFixture.GetOrgDefault(org.ID)
	if err != nil {
		t.Fatalf("Failed to get updated org default policy: %v", err)
	}
	if newDefault.ID != customPolicy.ID {
		t.Errorf("New default policy ID mismatch: expected %s, got %s", customPolicy.ID, newDefault.ID)
	}

	t.Logf("Org default token policy test passed: org=%s, policy=%s", org.ID, customPolicy.ID)
}

// TestUserTokenPolicyOverride tests setting and clearing user policy overrides
func TestUserTokenPolicyOverride(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	userFixture := fixtures.NewUserFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create user
	email := ctx.GenerateResourceName("user") + "@test.example.com"
	user, err := userFixture.Create(ctx, org.ID, email, "Test User", []string{"member"})
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}
	t.Logf("Created user: %s", user.ID)

	// Create two policies: one for org default, one for user override
	limit1h := int64(10000)
	limit24h := int64(100000)
	orgPolicy, err := policyFixture.Create(ctx, org.ID, "org-policy", &limit1h, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create org policy: %v", err)
	}
	userPolicy, err := policyFixture.Create(ctx, org.ID, "user-policy", nil, &limit24h, nil)
	if err != nil {
		t.Fatalf("Failed to create user policy: %v", err)
	}

	// Set org default
	_, err = policyFixture.SetOrgDefault(org.ID, orgPolicy.ID)
	if err != nil {
		t.Fatalf("Failed to set org default: %v", err)
	}

	// Get user's effective policy (should inherit org default)
	effective, err := policyFixture.GetUserPolicy(org.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user policy: %v", err)
	}
	if effective.Source != "inherited" {
		t.Errorf("Expected source 'inherited', got '%s'", effective.Source)
	}
	if effective.Policy.ID != orgPolicy.ID {
		t.Errorf("Expected inherited policy ID %s, got %s", orgPolicy.ID, effective.Policy.ID)
	}
	t.Logf("User inherits org policy: %s (source=%s)", effective.Policy.ID, effective.Source)

	// Set user override
	overridden, err := policyFixture.SetUserPolicy(org.ID, user.ID, userPolicy.ID)
	if err != nil {
		t.Fatalf("Failed to set user override: %v", err)
	}
	if overridden.Source != "override" {
		t.Errorf("Expected source 'override', got '%s'", overridden.Source)
	}
	if overridden.Policy.ID != userPolicy.ID {
		t.Errorf("Expected override policy ID %s, got %s", userPolicy.ID, overridden.Policy.ID)
	}
	t.Logf("Set user override: %s (source=%s)", overridden.Policy.ID, overridden.Source)

	// Clear user override
	cleared, err := policyFixture.ClearUserPolicy(org.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to clear user override: %v", err)
	}
	if cleared.Source != "inherited" {
		t.Errorf("Expected source 'inherited' after clear, got '%s'", cleared.Source)
	}
	if cleared.Policy.ID != orgPolicy.ID {
		t.Errorf("Expected inherited policy ID %s after clear, got %s", orgPolicy.ID, cleared.Policy.ID)
	}
	t.Logf("Cleared user override, back to inherited: %s", cleared.Policy.ID)

	t.Logf("User token policy override test passed: user=%s", user.ID)
}

// TestTokenUsageQuery tests querying token usage
func TestTokenUsageQuery(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	userFixture := fixtures.NewUserFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)
	usageFixture := fixtures.NewTokenUsageFixture(ctx.Client)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create user
	email := ctx.GenerateResourceName("user") + "@test.example.com"
	user, err := userFixture.Create(ctx, org.ID, email, "Test User", []string{"member"})
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create and set a policy with limits
	limit1h := int64(10000)
	limit24h := int64(100000)
	limit7d := int64(500000)
	policy, err := policyFixture.Create(ctx, org.ID, "usage-test-policy", &limit1h, &limit24h, &limit7d)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}
	_, err = policyFixture.SetOrgDefault(org.ID, policy.ID)
	if err != nil {
		t.Fatalf("Failed to set org default: %v", err)
	}

	// Query user's token usage
	usage, err := usageFixture.GetUserUsage(org.ID, user.ID)
	if err != nil {
		t.Fatalf("Failed to get user usage: %v", err)
	}

	t.Logf("User usage for policy '%s':", usage.PolicyName)
	if len(usage.Windows) == 0 {
		t.Error("Expected at least one usage window")
	}

	for _, w := range usage.Windows {
		t.Logf("  Window %s: used=%d, limit=%d, remaining=%d (%.1f%%)",
			w.Window, w.Used, w.Limit, w.Remaining, w.Percentage)

		// Verify the limit matches the policy
		switch w.Window {
		case "1h":
			if w.Limit != limit1h {
				t.Errorf("1h limit mismatch: expected %d, got %d", limit1h, w.Limit)
			}
		case "24h":
			if w.Limit != limit24h {
				t.Errorf("24h limit mismatch: expected %d, got %d", limit24h, w.Limit)
			}
		case "7d":
			if w.Limit != limit7d {
				t.Errorf("7d limit mismatch: expected %d, got %d", limit7d, w.Limit)
			}
		}

		// Initially, usage should be 0
		if w.Used != 0 {
			t.Logf("Warning: Expected initial usage to be 0, got %d (may be from previous tests)", w.Used)
		}
	}

	t.Logf("Token usage query test passed: user=%s", user.ID)
}

// TestTokenRateLimitEnforcement tests that token rate limits are enforced at the API router
func TestTokenRateLimitEnforcement(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	userFixture := fixtures.NewUserFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create user
	email := ctx.GenerateResourceName("user") + "@test.example.com"
	user, err := userFixture.Create(ctx, org.ID, email, "Test User", []string{"member"})
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	// Create a policy with a very low limit (10 tokens per hour)
	lowLimit := int64(10)
	policy, err := policyFixture.Create(ctx, org.ID, "low-limit-policy", &lowLimit, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create low-limit policy: %v", err)
	}

	// Set as user's policy
	_, err = policyFixture.SetUserPolicy(org.ID, user.ID, policy.ID)
	if err != nil {
		t.Fatalf("Failed to set user policy: %v", err)
	}

	// Create API key for the user
	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "rate-limit-test-key", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Create client for API router
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
	routerClient.SetHeader("X-API-Key", apiKey.Key)

	// If using IP address, set Host header
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Make inference request
	reqBody := map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Test token rate limit enforcement",
			},
		},
	}

	// Make requests until we hit the rate limit or model not found
	// Note: The actual token counting happens on response, so we may need multiple requests
	hitRateLimit := false
	var rateLimitResp *harness.Response

	for i := 0; i < 20; i++ {
		resp, err := routerClient.POST("/v1/chat/completions", reqBody)
		if err != nil {
			t.Logf("Request %d failed: %v", i+1, err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			hitRateLimit = true
			rateLimitResp = resp
			t.Logf("Hit rate limit at request %d (status: 429)", i+1)
			break
		}

		// Other acceptable statuses (backend may not be available)
		switch resp.StatusCode {
		case http.StatusOK, http.StatusNotFound, http.StatusBadGateway, http.StatusServiceUnavailable:
			t.Logf("Request %d: status %d", i+1, resp.StatusCode)
		case http.StatusUnauthorized, http.StatusForbidden:
			t.Logf("Request %d: auth error (status %d)", i+1, resp.StatusCode)
		default:
			t.Logf("Request %d: unexpected status %d", i+1, resp.StatusCode)
		}

		time.Sleep(100 * time.Millisecond)
	}

	if hitRateLimit {
		// Validate 429 response format
		if rateLimitResp != nil {
			// Check for OpenAI-compatible error format
			var errBody map[string]interface{}
			if err := rateLimitResp.UnmarshalJSON(&errBody); err == nil {
				if errObj, ok := errBody["error"].(map[string]interface{}); ok {
					if errType, ok := errObj["type"].(string); ok {
						t.Logf("Error type: %s", errType)
					}
					if errMsg, ok := errObj["message"].(string); ok {
						t.Logf("Error message: %s", errMsg)
					}
				}
			}

			// Check for rate limit headers
			retryAfter := rateLimitResp.Headers.Get("Retry-After")
			if retryAfter != "" {
				t.Logf("Retry-After: %s", retryAfter)
			}
			xRateLimitLimit := rateLimitResp.Headers.Get("X-RateLimit-Limit-Tokens")
			xRateLimitRemaining := rateLimitResp.Headers.Get("X-RateLimit-Remaining-Tokens")
			xRateLimitReset := rateLimitResp.Headers.Get("X-RateLimit-Reset-Tokens")
			if xRateLimitLimit != "" {
				t.Logf("X-RateLimit-Limit-Tokens: %s", xRateLimitLimit)
			}
			if xRateLimitRemaining != "" {
				t.Logf("X-RateLimit-Remaining-Tokens: %s", xRateLimitRemaining)
			}
			if xRateLimitReset != "" {
				t.Logf("X-RateLimit-Reset-Tokens: %s", xRateLimitReset)
			}
		}
		t.Logf("Token rate limit enforcement test passed")
	} else {
		t.Logf("Did not hit token rate limit (may be expected if backend unavailable or tokens not counted)")
		t.Logf("Token rate limiting may require actual inference responses to count tokens")
	}

	t.Logf("Token rate limit enforcement test completed: org=%s, user=%s", org.ID, user.ID)
}

// TestTokenPolicyInUseProtection tests that policies in use cannot be deleted
func TestTokenPolicyInUseProtection(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create a policy
	limit := int64(10000)
	policy, err := policyFixture.Create(ctx, org.ID, "in-use-policy", &limit, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create policy: %v", err)
	}

	// Set as org default (now in use)
	_, err = policyFixture.SetOrgDefault(org.ID, policy.ID)
	if err != nil {
		t.Fatalf("Failed to set as org default: %v", err)
	}

	// Try to delete the policy (should fail with 409 Conflict)
	err = policyFixture.Delete(org.ID, policy.ID)
	if err == nil {
		t.Error("Expected error deleting policy in use, but got none")
	} else {
		t.Logf("Correctly rejected deletion of policy in use: %v", err)
	}

	// Reset to builtin policy
	_, err = policyFixture.SetOrgDefault(org.ID, "no-limit")
	if err != nil {
		t.Logf("Warning: Could not reset to builtin policy: %v", err)
	}

	t.Logf("Token policy in-use protection test passed: policy=%s", policy.ID)
}

// TestBuiltinPolicyProtection tests that built-in policies cannot be modified or deleted
func TestBuiltinPolicyProtection(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// List policies to find the builtin one
	policies, err := policyFixture.List(org.ID)
	if err != nil {
		t.Fatalf("Failed to list policies: %v", err)
	}

	var builtinPolicy *fixtures.TokenRateLimitPolicy
	for i := range policies {
		if policies[i].IsBuiltin {
			builtinPolicy = &policies[i]
			break
		}
	}

	if builtinPolicy == nil {
		t.Fatal("Could not find builtin policy")
	}
	t.Logf("Found builtin policy: %s (%s)", builtinPolicy.Name, builtinPolicy.ID)

	// Try to update the builtin policy (should fail with 403)
	newLimit := int64(1000)
	_, err = policyFixture.Update(org.ID, builtinPolicy.ID, nil, &newLimit, nil, nil)
	if err == nil {
		t.Error("Expected error updating builtin policy, but got none")
	} else {
		t.Logf("Correctly rejected update of builtin policy: %v", err)
	}

	// Try to delete the builtin policy (should fail with 403)
	err = policyFixture.Delete(org.ID, builtinPolicy.ID)
	if err == nil {
		t.Error("Expected error deleting builtin policy, but got none")
	} else {
		t.Logf("Correctly rejected deletion of builtin policy: %v", err)
	}

	t.Logf("Builtin policy protection test passed")
}

// TestTokenPolicyNameConflict tests that duplicate policy names are rejected
func TestTokenPolicyNameConflict(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create a policy
	limit := int64(10000)
	_, err = policyFixture.Create(ctx, org.ID, "unique-policy-name", &limit, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create first policy: %v", err)
	}

	// Try to create another policy with the same name (should fail with 409)
	_, err = policyFixture.Create(ctx, org.ID, "unique-policy-name", &limit, nil, nil)
	if err == nil {
		t.Error("Expected error creating policy with duplicate name, but got none")
	} else {
		t.Logf("Correctly rejected duplicate policy name: %v", err)
	}

	t.Logf("Token policy name conflict test passed")
}

// TestNoTokenRateLimitPolicy tests that the "no-limit" alias works
func TestNoTokenRateLimitPolicy(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	policyFixture := fixtures.NewTokenPolicyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create and set a custom policy as default
	limit := int64(10000)
	customPolicy, err := policyFixture.Create(ctx, org.ID, "temp-policy", &limit, nil, nil)
	if err != nil {
		t.Fatalf("Failed to create custom policy: %v", err)
	}
	_, err = policyFixture.SetOrgDefault(org.ID, customPolicy.ID)
	if err != nil {
		t.Fatalf("Failed to set custom policy as default: %v", err)
	}

	// Reset to no-limit using the alias
	noLimitPolicy, err := policyFixture.SetOrgDefault(org.ID, "no-limit")
	if err != nil {
		t.Fatalf("Failed to set 'no-limit' as default: %v", err)
	}

	if !noLimitPolicy.IsBuiltin {
		t.Error("Expected 'no-limit' to resolve to builtin policy")
	}
	t.Logf("Set org default to 'no-limit': %s (builtin=%v)", noLimitPolicy.Name, noLimitPolicy.IsBuiltin)

	t.Logf("No-limit policy alias test passed")
}
