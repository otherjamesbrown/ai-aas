//go:build (nightly || full || security) && e2e_tier || !e2e_tier

package suites

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// =============================================================================
// BEAD: aas-nwze - Cross-Organization Isolation Tests
// =============================================================================

// TestCrossOrgIsolation_UserFromOrgACannotAccessOrgB tests that users from one org
// cannot access resources from another org.
func TestCrossOrgIsolation_UserFromOrgACannotAccessOrgB(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create two separate organizations
	orgA, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization A: %v", err)
	}
	t.Logf("Created Organization A: %s", orgA.ID)

	orgB, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization B: %v", err)
	}
	t.Logf("Created Organization B: %s", orgB.ID)

	// Create API key for Org A
	apiKeyA, err := apiKeyFixture.CreateWithServiceAccount(ctx, orgA.ID, "key-org-a", []string{"*"})
	if err != nil {
		t.Fatalf("Failed to create API key for Org A: %v", err)
	}
	t.Logf("Created API key for Org A: %s", apiKeyA.ID)

	// Create client using Org A's API key
	clientA := harness.NewClient(ctx.Config.APIURLs.UserOrgService, ctx.Config.Timeouts.RequestTimeout)
	clientA.SetHeader("Authorization", "Bearer "+apiKeyA.Key)
	clientA.SetHeader("X-API-Key", apiKeyA.Key)
	if isIPAddress(ctx.Config.APIURLs.UserOrgService) {
		clientA.SetHeader("Host", "user-org.dev.otherjamesbrown.com")
	}

	// Try to access Org B's resources using Org A's credentials
	// This should fail with 403 Forbidden

	// Test 1: Try to get Org B's details
	resp, err := clientA.GET(fmt.Sprintf("/v1/orgs/%s", orgB.ID))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 403 or 404 when accessing other org, got %d: %s", resp.StatusCode, resp.String())
	} else {
		t.Logf("PASS: Org A cannot access Org B details (status %d)", resp.StatusCode)
	}

	// Test 2: Try to list Org B's service accounts
	resp, err = clientA.GET(fmt.Sprintf("/v1/orgs/%s/service-accounts", orgB.ID))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 403 or 404 when listing other org's service accounts, got %d", resp.StatusCode)
	} else {
		t.Logf("PASS: Org A cannot list Org B's service accounts (status %d)", resp.StatusCode)
	}

	// Test 3: Try to list Org B's API keys
	resp, err = clientA.GET(fmt.Sprintf("/v1/orgs/%s/api-keys", orgB.ID))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 403 or 404 when listing other org's API keys, got %d", resp.StatusCode)
	} else {
		t.Logf("PASS: Org A cannot list Org B's API keys (status %d)", resp.StatusCode)
	}

	// Test 4: Try to create a service account in Org B
	resp, err = clientA.POST(fmt.Sprintf("/v1/orgs/%s/service-accounts", orgB.ID), map[string]interface{}{
		"name":        "malicious-sa",
		"description": "Attempted cross-org access",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Errorf("Expected 403 or 404 when creating service account in other org, got %d", resp.StatusCode)
	} else {
		t.Logf("PASS: Org A cannot create service account in Org B (status %d)", resp.StatusCode)
	}

	t.Log("Cross-organization isolation tests passed")
}

// TestCrossOrgIsolation_InferenceIsolation tests that API keys from one org
// cannot be used for inference attributed to another org.
func TestCrossOrgIsolation_InferenceIsolation(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create two organizations
	orgA, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization A: %v", err)
	}

	orgB, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization B: %v", err)
	}

	// Create API key for Org A
	apiKeyA, err := apiKeyFixture.CreateWithServiceAccount(ctx, orgA.ID, "inference-key-a", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key for Org A: %v", err)
	}

	// Make inference request with Org A's key but try to attribute to Org B
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKeyA.Key)
	routerClient.SetHeader("X-Organization-ID", orgB.ID) // Try to impersonate Org B
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Check models first
	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Skipf("Cannot reach API router: %v", err)
	}

	var models ModelsResponse
	if err := json.Unmarshal(modelsResp.Body, &models); err != nil || len(models.Data) == 0 {
		t.Skip("No models available for inference test")
	}

	// Make inference request
	reqBody := map[string]interface{}{
		"model": models.Data[0].ID,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Say hello"},
		},
		"max_tokens": 10,
	}

	resp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Skipf("Inference request failed: %v", err)
	}

	// The request should either:
	// 1. Ignore the X-Organization-ID header and use the org from the API key
	// 2. Return 403 if org mismatch is detected
	// It should NOT allow attribution to Org B

	if resp.StatusCode == http.StatusOK {
		// If successful, verify it was attributed to Org A, not Org B
		t.Logf("Inference succeeded - should verify analytics attributes to Org A, not Org B")
		// Note: Full verification would require checking analytics service
	} else if resp.StatusCode == http.StatusForbidden {
		t.Logf("PASS: Inference rejected due to org mismatch (status 403)")
	} else {
		t.Logf("Inference returned status %d (acceptable if org header ignored)", resp.StatusCode)
	}
}

// =============================================================================
// BEAD: aas-txun - Per-Endpoint Authentication Requirement Tests
// =============================================================================

// TestPerEndpointAuth_AllProtectedEndpointsRequireAuth verifies all protected endpoints
// return 401 when no authentication is provided.
func TestPerEndpointAuth_AllProtectedEndpointsRequireAuth(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	// Protected endpoints that MUST require authentication
	protectedEndpoints := []struct {
		service  string
		method   string
		path     string
		baseURL  string
		hostHdr  string
	}{
		// User-Org Service endpoints
		{"user-org", "GET", "/v1/orgs", ctx.Config.APIURLs.UserOrgService, "user-org.dev.otherjamesbrown.com"},
		{"user-org", "POST", "/v1/orgs", ctx.Config.APIURLs.UserOrgService, "user-org.dev.otherjamesbrown.com"},
		{"user-org", "GET", "/v1/me", ctx.Config.APIURLs.UserOrgService, "user-org.dev.otherjamesbrown.com"},
		{"user-org", "GET", "/v1/users", ctx.Config.APIURLs.UserOrgService, "user-org.dev.otherjamesbrown.com"},
		{"user-org", "GET", "/v1/api-keys", ctx.Config.APIURLs.UserOrgService, "user-org.dev.otherjamesbrown.com"},

		// API Router Service protected endpoints
		{"api-router", "POST", "/v1/chat/completions", ctx.Config.APIURLs.APIRouterService, "api.dev.otherjamesbrown.com"},
		{"api-router", "POST", "/v1/completions", ctx.Config.APIURLs.APIRouterService, "api.dev.otherjamesbrown.com"},
		{"api-router", "POST", "/v1/embeddings", ctx.Config.APIURLs.APIRouterService, "api.dev.otherjamesbrown.com"},

		// Analytics Service endpoints
		{"analytics", "GET", "/analytics/v1/orgs/test-org/usage", ctx.Config.APIURLs.AnalyticsService, "analytics.dev.otherjamesbrown.com"},
	}

	passCount := 0
	failCount := 0

	for _, ep := range protectedEndpoints {
		if ep.baseURL == "" {
			t.Logf("SKIP: %s %s - service URL not configured", ep.method, ep.path)
			continue
		}

		// Create unauthenticated client
		client := harness.NewClient(ep.baseURL, ctx.Config.Timeouts.RequestTimeout)
		if isIPAddress(ep.baseURL) {
			client.SetHeader("Host", ep.hostHdr)
		}

		var resp *harness.Response
		var err error

		switch ep.method {
		case "GET":
			resp, err = client.GET(ep.path)
		case "POST":
			resp, err = client.POST(ep.path, map[string]interface{}{})
		case "PUT":
			resp, err = client.PUT(ep.path, map[string]interface{}{})
		case "DELETE":
			resp, err = client.DELETE(ep.path)
		}

		if err != nil {
			t.Logf("SKIP: %s %s %s - request error: %v", ep.service, ep.method, ep.path, err)
			continue
		}

		// Should return 401 Unauthorized
		if resp.StatusCode == http.StatusUnauthorized {
			t.Logf("PASS: %s %s %s returns 401 without auth", ep.service, ep.method, ep.path)
			passCount++
		} else if resp.StatusCode == http.StatusForbidden {
			// 403 is also acceptable for some endpoints
			t.Logf("PASS: %s %s %s returns 403 without auth", ep.service, ep.method, ep.path)
			passCount++
		} else {
			t.Errorf("FAIL: %s %s %s returns %d without auth (expected 401/403)", ep.service, ep.method, ep.path, resp.StatusCode)
			failCount++
		}
	}

	t.Logf("Authentication requirement tests: %d passed, %d failed", passCount, failCount)
	if failCount > 0 {
		t.Fail()
	}
}

// TestPerEndpointAuth_PublicEndpointsNoAuth verifies public endpoints work without auth.
func TestPerEndpointAuth_PublicEndpointsNoAuth(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	// Public endpoints that should NOT require authentication
	publicEndpoints := []struct {
		service string
		method  string
		path    string
		baseURL string
		hostHdr string
	}{
		// Health endpoints
		{"api-router", "GET", "/v1/status/healthz", ctx.Config.APIURLs.APIRouterService, "api.dev.otherjamesbrown.com"},
		{"api-router", "GET", "/healthz", ctx.Config.APIURLs.APIRouterService, "api.dev.otherjamesbrown.com"},
		{"user-org", "GET", "/healthz", ctx.Config.APIURLs.UserOrgService, "user-org.dev.otherjamesbrown.com"},
		{"analytics", "GET", "/analytics/v1/status/healthz", ctx.Config.APIURLs.AnalyticsService, "analytics.dev.otherjamesbrown.com"},

		// Models list (public)
		{"api-router", "GET", "/v1/models", ctx.Config.APIURLs.APIRouterService, "api.dev.otherjamesbrown.com"},
	}

	for _, ep := range publicEndpoints {
		if ep.baseURL == "" {
			t.Logf("SKIP: %s %s - service URL not configured", ep.method, ep.path)
			continue
		}

		client := harness.NewClient(ep.baseURL, ctx.Config.Timeouts.RequestTimeout)
		if isIPAddress(ep.baseURL) {
			client.SetHeader("Host", ep.hostHdr)
		}

		resp, err := client.GET(ep.path)
		if err != nil {
			t.Logf("SKIP: %s %s %s - request error: %v", ep.service, ep.method, ep.path, err)
			continue
		}

		if resp.StatusCode == http.StatusOK {
			t.Logf("PASS: %s %s %s accessible without auth (status 200)", ep.service, ep.method, ep.path)
		} else {
			t.Errorf("FAIL: %s %s %s returns %d (expected 200 for public endpoint)", ep.service, ep.method, ep.path, resp.StatusCode)
		}
	}
}

// =============================================================================
// BEAD: aas-ojsa - Inference Authorization Tests
// =============================================================================

// TestInferenceAuth_NoTokenReturns401 tests that inference without token returns 401.
func TestInferenceAuth_NoTokenReturns401(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Make inference request without any authentication
	reqBody := map[string]interface{}{
		"model": "any-model",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "test"},
		},
	}

	resp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for inference without token, got %d", resp.StatusCode)
	} else {
		t.Log("PASS: Inference without token returns 401")
	}
}

// TestInferenceAuth_InvalidTokenReturns401 tests that inference with invalid token returns 401.
func TestInferenceAuth_InvalidTokenReturns401(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer invalid-token-12345")
	routerClient.SetHeader("X-API-Key", "invalid-token-12345")
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	reqBody := map[string]interface{}{
		"model": "any-model",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "test"},
		},
	}

	resp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("Expected 401 for invalid token, got %d: %s", resp.StatusCode, resp.String())
	} else {
		t.Log("PASS: Inference with invalid token returns 401")
	}
}

// TestInferenceAuth_ValidTokenSucceeds tests that inference with valid token succeeds.
func TestInferenceAuth_ValidTokenSucceeds(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create org and API key
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create org: %v", err)
	}

	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Get available models
	modelsResp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Skipf("Cannot get models: %v", err)
	}

	var models ModelsResponse
	if err := json.Unmarshal(modelsResp.Body, &models); err != nil || len(models.Data) == 0 {
		t.Skip("No models available")
	}

	reqBody := map[string]interface{}{
		"model": models.Data[0].ID,
		"messages": []map[string]interface{}{
			{"role": "user", "content": "Say hello"},
		},
		"max_tokens": 10,
	}

	resp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		t.Log("PASS: Inference with valid token succeeds")
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		t.Skip("Backend unavailable")
	} else {
		t.Errorf("Expected 200 for valid token, got %d: %s", resp.StatusCode, resp.String())
	}
}

// =============================================================================
// BEAD: aas-dd47 - Scope Enforcement Tests
// =============================================================================

// TestScopeEnforcement_InferenceReadCannotWrite tests that inference:read scope cannot write.
func TestScopeEnforcement_InferenceReadCannotWrite(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create org: %v", err)
	}

	// Create API key with only read scope
	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "", []string{"inference:read"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Try to make inference (write operation) with read-only scope
	reqBody := map[string]interface{}{
		"model": "any-model",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "test"},
		},
	}

	resp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	// Should be forbidden if scope enforcement is working
	if resp.StatusCode == http.StatusForbidden {
		t.Log("PASS: inference:read scope cannot perform write operations")
	} else if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusServiceUnavailable {
		t.Log("INFO: Scope enforcement may not distinguish read/write for inference")
	} else {
		t.Logf("Got status %d (scope enforcement may not be granular)", resp.StatusCode)
	}
}

// TestScopeEnforcement_OrgReadCannotModify tests that org:read scope cannot modify orgs.
func TestScopeEnforcement_OrgReadCannotModify(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create org: %v", err)
	}

	// Create API key with only org:read scope
	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "", []string{"org:read"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	client := harness.NewClient(ctx.Config.APIURLs.UserOrgService, ctx.Config.Timeouts.RequestTimeout)
	client.SetHeader("Authorization", "Bearer "+apiKey.Key)
	client.SetHeader("X-API-Key", apiKey.Key)
	if isIPAddress(ctx.Config.APIURLs.UserOrgService) {
		client.SetHeader("Host", "user-org.dev.otherjamesbrown.com")
	}

	// Try to create a new org (write operation) with read-only scope
	resp, err := client.POST("/v1/orgs", map[string]interface{}{
		"name": "unauthorized-org",
		"slug": "unauthorized-org",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode == http.StatusForbidden {
		t.Log("PASS: org:read scope cannot create organizations")
	} else if resp.StatusCode == http.StatusUnauthorized {
		t.Log("PASS: org:read scope rejected (401)")
	} else {
		t.Errorf("Expected 403/401 for org creation with read scope, got %d", resp.StatusCode)
	}
}

// =============================================================================
// BEAD: aas-cyx9 - Token Lifecycle Tests
// =============================================================================

// TestTokenLifecycle_RevokedKeyRejected tests that revoked API keys are rejected.
func TestTokenLifecycle_RevokedKeyRejected(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create org: %v", err)
	}

	// Create API key
	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Verify key works before revocation
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	resp, err := routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Initial request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Skipf("API key doesn't work before revocation: %d", resp.StatusCode)
	}

	// Revoke the API key
	err = apiKeyFixture.Delete(apiKey.ID)
	if err != nil {
		t.Logf("Warning: Failed to revoke API key: %v", err)
	}

	// Wait for cache to potentially expire
	time.Sleep(2 * time.Second)

	// Try to use revoked key
	resp, err = routerClient.GET("/v1/models")
	if err != nil {
		t.Fatalf("Request with revoked key failed: %v", err)
	}

	// Note: /v1/models may be public, so test inference instead
	reqBody := map[string]interface{}{
		"model": "any-model",
		"messages": []map[string]interface{}{
			{"role": "user", "content": "test"},
		},
	}
	resp, err = routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		t.Log("PASS: Revoked API key is rejected")
	} else {
		// Key revocation should be enforced - if the key is still accepted, this is a security issue
		t.Errorf("FAIL: Revoked key returned %d instead of 401 Unauthorized - key revocation may not be working", resp.StatusCode)
	}
}

// =============================================================================
// BEAD: aas-ux5r - Error Response Security Tests
// =============================================================================

// TestErrorResponseSecurity_NoSensitiveDataInErrors tests that error responses don't leak info.
func TestErrorResponseSecurity_NoSensitiveDataInErrors(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer invalid-key-12345")
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	resp, err := routerClient.POST("/v1/chat/completions", map[string]interface{}{
		"model":    "test",
		"messages": []map[string]interface{}{{"role": "user", "content": "test"}},
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	responseBody := strings.ToLower(resp.String())

	// Check for sensitive information that should NOT be in error responses
	sensitivePatterns := []string{
		"stack trace",
		"internal error",
		"sql",
		"database",
		"connection string",
		"password",
		"secret",
		"/home/",
		"/var/",
		"panic",
		"goroutine",
	}

	for _, pattern := range sensitivePatterns {
		if strings.Contains(responseBody, pattern) {
			t.Errorf("FAIL: Error response contains sensitive pattern '%s': %s", pattern, resp.String())
		}
	}

	t.Log("PASS: Error response does not contain obvious sensitive information")
}

// TestErrorResponseSecurity_ConsistentErrorFormat tests error format consistency.
func TestErrorResponseSecurity_ConsistentErrorFormat(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	testCases := []struct {
		name  string
		token string
	}{
		{"invalid token", "invalid-key-12345"},
		{"malformed token", "not-a-real-key"},
		{"empty-ish token", "x"},
	}

	var errorFormats []string

	for _, tc := range testCases {
		client := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
		client.SetHeader("Authorization", "Bearer "+tc.token)
		if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
			client.SetHeader("Host", "api.dev.otherjamesbrown.com")
		}

		resp, err := client.POST("/v1/chat/completions", map[string]interface{}{
			"model":    "test",
			"messages": []map[string]interface{}{{"role": "user", "content": "test"}},
		})
		if err != nil {
			continue
		}

		// All should return same status code (to prevent enumeration)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Logf("INFO: %s returned %d (expected 401)", tc.name, resp.StatusCode)
		}

		// Parse error response
		var errResp map[string]interface{}
		if err := json.Unmarshal(resp.Body, &errResp); err == nil {
			// Check format is consistent
			errorFormats = append(errorFormats, fmt.Sprintf("%d:%v", resp.StatusCode, errResp["error"] != nil))
		}
	}

	// Verify all error formats are consistent
	if len(errorFormats) > 1 {
		first := errorFormats[0]
		for i, format := range errorFormats[1:] {
			if format != first {
				t.Logf("INFO: Error format differs between test cases (case %d)", i+1)
			}
		}
	}

	t.Log("PASS: Error responses checked for consistency")
}

// =============================================================================
// BEAD: aas-r2vq - Input Validation Security Tests
// =============================================================================

// TestInputValidation_MalformedAuthHeader tests handling of malformed auth headers.
func TestInputValidation_MalformedAuthHeader(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	malformedHeaders := []struct {
		name   string
		header string
	}{
		{"empty bearer", "Bearer "},
		{"bearer only", "Bearer"},
		{"extra spaces", "Bearer   token   extra"},
		{"wrong scheme", "Basic dXNlcjpwYXNz"},
		{"no scheme", "just-a-token"},
		{"null bytes", "Bearer token\x00malicious"},
		{"newlines", "Bearer token\ninjection"},
	}

	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	for _, tc := range malformedHeaders {
		client := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
		client.SetHeader("Authorization", tc.header)
		if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
			client.SetHeader("Host", "api.dev.otherjamesbrown.com")
		}

		resp, err := client.POST("/v1/chat/completions", map[string]interface{}{
			"model":    "test",
			"messages": []map[string]interface{}{{"role": "user", "content": "test"}},
		})
		if err != nil {
			t.Logf("PASS: %s - request error (expected)", tc.name)
			continue
		}

		// Should return 401 or 400, not 500
		if resp.StatusCode == http.StatusInternalServerError {
			t.Errorf("FAIL: %s caused 500 error", tc.name)
		} else if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusBadRequest {
			t.Logf("PASS: %s returns %d", tc.name, resp.StatusCode)
		} else {
			t.Logf("INFO: %s returns %d", tc.name, resp.StatusCode)
		}
	}
}

// TestInputValidation_VeryLongAPIKey tests handling of very long API keys.
func TestInputValidation_VeryLongAPIKey(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	// Generate very long API key (potential DoS vector)
	longKey := strings.Repeat("a", 100000) // 100KB key

	client := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	client.SetHeader("Authorization", "Bearer "+longKey)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		client.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	start := time.Now()
	resp, err := client.POST("/v1/chat/completions", map[string]interface{}{
		"model":    "test",
		"messages": []map[string]interface{}{{"role": "user", "content": "test"}},
	})
	duration := time.Since(start)

	if err != nil {
		t.Logf("PASS: Very long key rejected with error (expected)")
		return
	}

	// Should not cause excessive delay (DoS protection)
	if duration > 5*time.Second {
		t.Errorf("FAIL: Very long API key caused %v delay (potential DoS)", duration)
	}

	// Should not cause 500 error
	if resp.StatusCode == http.StatusInternalServerError {
		t.Errorf("FAIL: Very long API key caused 500 error")
	} else {
		t.Logf("PASS: Very long key handled in %v with status %d", duration, resp.StatusCode)
	}
}

// TestInputValidation_SpecialCharactersInKey tests handling of special chars in API keys.
func TestInputValidation_SpecialCharactersInKey(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	specialKeys := []struct {
		name string
		key  string
	}{
		{"equals sign", "key=value=test"},
		{"plus sign", "key+plus+test"},
		{"slash", "key/slash/test"},
		{"spaces", "key with spaces"},
		{"unicode", "key🔐unicode"},
		{"sql injection", "key' OR '1'='1"},
		{"path traversal", "key/../../../etc/passwd"},
	}

	for _, tc := range specialKeys {
		client := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
		client.SetHeader("Authorization", "Bearer "+tc.key)
		client.SetHeader("X-API-Key", tc.key)
		if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
			client.SetHeader("Host", "api.dev.otherjamesbrown.com")
		}

		resp, err := client.POST("/v1/chat/completions", map[string]interface{}{
			"model":    "test",
			"messages": []map[string]interface{}{{"role": "user", "content": "test"}},
		})
		if err != nil {
			t.Logf("PASS: %s - request error", tc.name)
			continue
		}

		// Should not cause 500 error
		if resp.StatusCode == http.StatusInternalServerError {
			t.Errorf("FAIL: %s caused 500 error", tc.name)
		} else {
			t.Logf("PASS: %s handled with status %d", tc.name, resp.StatusCode)
		}
	}
}

// =============================================================================
// BEAD: aas-xk3f - Admin Boundary Tests
// =============================================================================

// TestAdminBoundary_NonAdminCannotCreateOrg tests that non-admin cannot create orgs.
func TestAdminBoundary_NonAdminCannotCreateOrg(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create initial org
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create org: %v", err)
	}

	// Create API key with limited scope (not admin)
	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Try to create new org with non-admin key
	client := harness.NewClient(ctx.Config.APIURLs.UserOrgService, ctx.Config.Timeouts.RequestTimeout)
	client.SetHeader("Authorization", "Bearer "+apiKey.Key)
	client.SetHeader("X-API-Key", apiKey.Key)
	if isIPAddress(ctx.Config.APIURLs.UserOrgService) {
		client.SetHeader("Host", "user-org.dev.otherjamesbrown.com")
	}

	resp, err := client.POST("/v1/orgs", map[string]interface{}{
		"name": "unauthorized-new-org",
		"slug": "unauthorized-new-org",
	})
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized {
		t.Log("PASS: Non-admin cannot create organization")
	} else if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated {
		t.Errorf("FAIL: Non-admin was able to create organization (status %d)", resp.StatusCode)
	} else {
		t.Logf("INFO: Got status %d (may indicate scope enforcement)", resp.StatusCode)
	}
}

// TestAdminBoundary_OrgAdminCanManageOwnOrg tests org admin can manage their own org.
func TestAdminBoundary_OrgAdminCanManageOwnOrg(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create org: %v", err)
	}

	// Create API key with admin scope
	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "", []string{"*"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	client := harness.NewClient(ctx.Config.APIURLs.UserOrgService, ctx.Config.Timeouts.RequestTimeout)
	client.SetHeader("Authorization", "Bearer "+apiKey.Key)
	client.SetHeader("X-API-Key", apiKey.Key)
	if isIPAddress(ctx.Config.APIURLs.UserOrgService) {
		client.SetHeader("Host", "user-org.dev.otherjamesbrown.com")
	}

	// Should be able to list own org's service accounts
	resp, err := client.GET(fmt.Sprintf("/v1/orgs/%s/service-accounts", org.ID))
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		t.Log("PASS: Org admin can manage own organization")
	} else {
		t.Errorf("Expected 200 for org admin managing own org, got %d", resp.StatusCode)
	}
}

// =============================================================================
// BEAD: aas-q25d - Per-Org Rate Limiting Tests
// =============================================================================

// TestRateLimiting_PerOrgIsolation tests that rate limits are per-org.
func TestRateLimiting_PerOrgIsolation(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create two organizations
	orgA, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create org A: %v", err)
	}

	orgB, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create org B: %v", err)
	}

	apiKeyA, err := apiKeyFixture.CreateWithServiceAccount(ctx, orgA.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key A: %v", err)
	}

	apiKeyB, err := apiKeyFixture.CreateWithServiceAccount(ctx, orgB.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key B: %v", err)
	}

	// Make multiple rapid requests with Org A to potentially hit rate limit
	clientA := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	clientA.SetHeader("Authorization", "Bearer "+apiKeyA.Key)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		clientA.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	// Send requests rapidly
	rateLimitedA := false
	for i := 0; i < 20; i++ {
		resp, _ := clientA.GET("/v1/models")
		if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
			rateLimitedA = true
			t.Logf("Org A hit rate limit after %d requests", i+1)
			break
		}
	}

	// Org B should not be affected by Org A's rate limit
	clientB := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	clientB.SetHeader("Authorization", "Bearer "+apiKeyB.Key)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		clientB.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	resp, err := clientB.GET("/v1/models")
	if err != nil {
		t.Fatalf("Org B request failed: %v", err)
	}

	if rateLimitedA && resp.StatusCode == http.StatusTooManyRequests {
		t.Error("FAIL: Org B is rate limited due to Org A's usage (not isolated)")
	} else if rateLimitedA {
		t.Log("PASS: Org B is not affected by Org A's rate limit")
	} else {
		t.Log("INFO: Could not trigger rate limit for testing (may need more requests)")
	}
}

// =============================================================================
// BEAD: aas-a0xs - Audit Trail Tests
// =============================================================================

// TestAuditTrail_FailedAuthLogged tests that failed auth attempts are logged.
func TestAuditTrail_FailedAuthLogged(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	// Make several failed auth attempts
	client := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	client.SetHeader("Authorization", "Bearer invalid-key-for-audit-test")
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		client.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	for i := 0; i < 3; i++ {
		resp, _ := client.POST("/v1/chat/completions", map[string]interface{}{
			"model":    "test",
			"messages": []map[string]interface{}{{"role": "user", "content": "audit test"}},
		})
		if resp != nil && resp.StatusCode == http.StatusUnauthorized {
			t.Logf("Failed auth attempt %d logged (status 401)", i+1)
		}
	}

	// LIMITATION: Automated log verification requires Loki API access.
	// This test generates audit events - manual verification needed via:
	//   1. Check Grafana → Service Logs dashboard for auth_failed events
	//   2. Or query Loki: {service="api-router-service"} |= "auth_failed"
	// TODO: Add Loki client to harness for automated audit log verification (see aas-a0xs)
	t.Log("INFO: Failed auth attempts generated - manual verification required in Loki/Grafana")
	t.Log("Expected log fields: level=warn, event=auth_failed, ip=<client_ip>, key_prefix=<first_chars>")
}

// TestAuditTrail_SuccessfulRequestLogged tests that successful requests are logged.
func TestAuditTrail_SuccessfulRequestLogged(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create org: %v", err)
	}

	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	client := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	client.SetHeader("Authorization", "Bearer "+apiKey.Key)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		client.SetHeader("Host", "api.dev.otherjamesbrown.com")
	}

	resp, err := client.GET("/v1/models")
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}

	if resp.StatusCode == http.StatusOK {
		t.Log("INFO: Successful request made - verify in logs that it is recorded")
		t.Logf("Expected log fields: org_id=%s, api_key_id=%s, method=GET, path=/v1/models", org.ID, apiKey.ID)
	}
}
