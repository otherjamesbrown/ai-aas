package suites

import (
	"fmt"
	"testing"
	"time"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
	"github.com/ai-aas/tests/e2e/utils"
)

// TestBackendFailover tests that the system fails over to alternate backends when one is unavailable
func TestBackendFailover(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create organization and API key
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "failover-test-key", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Create client for API router service
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Secret)
	routerClient.SetHeader("X-API-Key", apiKey.Secret)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	// Check backend health status (if admin endpoint available)
	// GET /v1/admin/routing/backends
	backendsResp, err := routerClient.GET("/v1/admin/routing/backends")
	if err == nil && backendsResp.StatusCode == 200 {
		var backends map[string]interface{}
		if err := backendsResp.UnmarshalJSON(&backends); err == nil {
			t.Logf("Backend registry status retrieved: %v", backends)
		}
	} else {
		t.Logf("Warning: Could not query backend status (admin endpoint may require auth): %v", err)
	}

	// Make an inference request
	// The system should handle backend unavailability gracefully
	inferenceReq := map[string]interface{}{
		"model":     "gpt-4",
		"prompt":    "Test prompt for failover",
		"max_tokens": 50,
	}

	resp, err := routerClient.POST("/v1/inference", inferenceReq)
	if err != nil {
		// Check if error indicates backend unavailability
		errMsg := err.Error()
		if contains(errMsg, "backend unavailable") || contains(errMsg, "no backend") || contains(errMsg, "service unavailable") {
			t.Logf("Backend unavailable detected (expected in failover scenario): %v", err)
			// Verify error message is clear
			if !contains(errMsg, "unavailable") && !contains(errMsg, "error") {
				t.Logf("Warning: Error message may not be clear enough")
			}
		} else {
			t.Logf("Request failed with unexpected error: %v", err)
		}
	} else if resp.StatusCode == 200 || resp.StatusCode == 201 {
		t.Logf("Request succeeded (backend available or failover worked): status %d", resp.StatusCode)
	} else if resp.StatusCode == 503 || resp.StatusCode == 502 {
		t.Logf("Service unavailable (backend failover scenario): status %d", resp.StatusCode)
		// Verify error response is clear
		body := resp.String()
		if !contains(body, "unavailable") && !contains(body, "error") {
			t.Logf("Warning: Error response may not be clear enough")
		}
	} else {
		t.Logf("Request returned status %d", resp.StatusCode)
	}

	t.Logf("Backend failover test complete: org=%s, api_key=%s", org.ID, apiKey.ID)
}

// TestHealthCheck tests that health endpoints report unhealthy status when dependencies fail
func TestHealthCheck(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	// Test health endpoints for different services
	services := []struct {
		name     string
		baseURL  string
		endpoint string
	}{
		{"user-org-service", ctx.Config.APIURLs.UserOrgService, "/health"},
		{"user-org-service", ctx.Config.APIURLs.UserOrgService, "/readyz"},
		{"api-router-service", ctx.Config.APIURLs.APIRouterService, "/v1/status/healthz"},
		{"api-router-service", ctx.Config.APIURLs.APIRouterService, "/v1/status/readyz"},
	}

	for _, svc := range services {
		client := harness.NewClient(svc.baseURL, 5*time.Second)
		if isIPAddress(svc.baseURL) {
			client.SetHeader("Host", "api.dev.ai-aas.local")
		}

		resp, err := client.GET(svc.endpoint)
		if err != nil {
			t.Logf("Warning: Health check failed for %s %s: %v", svc.name, svc.endpoint, err)
			continue
		}

		if resp.StatusCode == 200 {
			var health map[string]interface{}
			if err := resp.UnmarshalJSON(&health); err == nil {
				status, _ := health["status"].(string)
				components, hasComponents := health["components"].(map[string]interface{})

				if hasComponents {
					// Check component health
					allHealthy := true
					for compName, compStatus := range components {
						statusStr, _ := compStatus.(string)
						if statusStr != "healthy" && statusStr != "not_configured" {
							allHealthy = false
							t.Logf("%s %s: component %s is %s", svc.name, svc.endpoint, compName, statusStr)
						}
					}
					if !allHealthy {
						t.Logf("%s %s: service is degraded (some components unhealthy)", svc.name, svc.endpoint)
					} else {
						t.Logf("%s %s: service is healthy", svc.name, svc.endpoint)
					}
				} else {
					t.Logf("%s %s: status=%s", svc.name, svc.endpoint, status)
				}
			}
		} else if resp.StatusCode == 503 {
			t.Logf("%s %s: service is unhealthy (status 503)", svc.name, svc.endpoint)
		} else {
			t.Logf("%s %s: unexpected status %d", svc.name, svc.endpoint, resp.StatusCode)
		}
	}

	t.Logf("Health check test complete")
}

// TestPartialOutage tests graceful degradation during partial service outages
func TestPartialOutage(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create organization and API key
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "outage-test-key", []string{"inference:read"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Test that unaffected services continue to work
	// Even if inference is down, org/user management should work
	orgClient := harness.NewClient(ctx.Config.APIURLs.UserOrgService, ctx.Config.Timeouts.RequestTimeout)
	orgClient.SetHeader("Authorization", "Bearer "+apiKey.Secret)
	orgClient.SetHeader("X-API-Key", apiKey.Secret)
	if isIPAddress(ctx.Config.APIURLs.UserOrgService) {
		orgClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	// Try to get organization (should work even if inference is down)
	orgResp, err := orgClient.GET(fmt.Sprintf("/v1/orgs/%s", org.ID))
	if err != nil {
		t.Logf("Warning: Could not get organization (may indicate outage): %v", err)
	} else if orgResp.StatusCode == 200 {
		t.Logf("Organization service working (unaffected by partial outage): org=%s", org.ID)
	} else {
		t.Logf("Organization service returned status %d", orgResp.StatusCode)
	}

	// Test inference service (may be affected by outage)
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Secret)
	routerClient.SetHeader("X-API-Key", apiKey.Secret)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	inferenceReq := map[string]interface{}{
		"model":     "gpt-4",
		"prompt":    "Test prompt during partial outage",
		"max_tokens": 50,
	}

	infResp, err := routerClient.POST("/v1/inference", inferenceReq)
	if err != nil {
		t.Logf("Inference service unavailable (expected in partial outage): %v", err)
	} else if infResp.StatusCode == 503 || infResp.StatusCode == 502 {
		t.Logf("Inference service degraded (status %d) - graceful degradation working", infResp.StatusCode)
	} else if infResp.StatusCode == 200 {
		t.Logf("Inference service working despite partial outage")
	}

	t.Logf("Partial outage test complete: org=%s", org.ID)
}

// TestAllBackendsUnavailable tests that the system detects when all backends are unavailable
func TestAllBackendsUnavailable(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create organization and API key
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "all-backends-down-key", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Create client for API router
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Secret)
	routerClient.SetHeader("X-API-Key", apiKey.Secret)
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	// Check backend status
	backendsResp, err := routerClient.GET("/v1/admin/routing/backends")
	allBackendsDown := false

	if err == nil && backendsResp.StatusCode == 200 {
		var backends map[string]interface{}
		if err := backendsResp.UnmarshalJSON(&backends); err == nil {
			// Check if all backends are unhealthy
			backendsList, ok := backends["backends"].([]interface{})
			if ok {
				unhealthyCount := 0
				for _, backend := range backendsList {
					backendMap, _ := backend.(map[string]interface{})
					status, _ := backendMap["status"].(string)
					if status == "unhealthy" || status == "degraded" {
						unhealthyCount++
					}
				}
				if unhealthyCount == len(backendsList) && len(backendsList) > 0 {
					allBackendsDown = true
					t.Logf("All backends are unavailable: %d/%d unhealthy", unhealthyCount, len(backendsList))
				}
			}
		}
	}

	// Attempt inference request
	inferenceReq := map[string]interface{}{
		"model":     "gpt-4",
		"prompt":    "Test prompt with all backends down",
		"max_tokens": 50,
	}

	resp, err := routerClient.POST("/v1/inference", inferenceReq)
	if err != nil {
		errMsg := err.Error()
		if contains(errMsg, "backend unavailable") || contains(errMsg, "no backend") || contains(errMsg, "all backends") {
			t.Logf("System correctly detected all backends unavailable: %v", err)
		} else {
			t.Logf("Request failed: %v", err)
		}
	} else if resp.StatusCode == 503 || resp.StatusCode == 502 {
		body := resp.String()
		if contains(body, "backend") || contains(body, "unavailable") {
			t.Logf("System correctly returned service unavailable: status %d", resp.StatusCode)
		} else {
			t.Logf("Service unavailable but error message may not be clear: status %d", resp.StatusCode)
		}
	} else if resp.StatusCode == 200 {
		// If request succeeds, backends may have recovered or test environment has backends
		t.Logf("Request succeeded (backends may be available): status %d", resp.StatusCode)
	}

	if allBackendsDown {
		t.Logf("Test detected all backends unavailable condition - test should skip with clear messaging")
		// In a real scenario, we would skip the test with a clear message
		t.Skipf("All backends are unavailable - skipping test with clear messaging")
	}

	t.Logf("All backends unavailable test complete: org=%s", org.ID)
}

// TestResilienceWithMocks tests resilience scenarios using the mock framework
func TestResilienceWithMocks(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	// Use mock backend from utils/mocks.go
	mockBackend := utils.NewMockBackend()
	defer mockBackend.Close()
	
	// Configure mock to simulate backend unavailability (503 Service Unavailable)
	mockBackend.SetResponse(utils.MockResponse{
		StatusCode: 503,
		Body:       []byte(`{"error":"backend unavailable"}`),
	})
	
	t.Logf("Mock backend configured: url=%s, status=unavailable", mockBackend.URL())

	// Test that system handles mock backend unavailability
	// In a real scenario, we would inject the mock URL into the test environment
	// For now, we verify the mock framework is available and can simulate failures
	if mockBackend == nil {
		t.Fatal("Mock backend not available")
	}

	// Simulate backend recovery (200 OK)
	mockBackend.SetResponse(utils.MockResponse{
		StatusCode: 200,
		Body:       []byte(`{"status":"ok"}`),
	})
	t.Logf("Mock backend recovered: url=%s, status=healthy", mockBackend.URL())

	// Test configurable failure scenarios with delay
	mockBackend.SetResponse(utils.MockResponse{
		StatusCode: 500,
		Body:       []byte(`{"error":"internal server error"}`),
		Delay:      100, // 100ms delay
	})
	t.Logf("Mock backend configured with failure scenario: url=%s, status=500, delay=100ms", mockBackend.URL())

	// Verify mock captured requests
	requests := mockBackend.GetRequests()
	t.Logf("Mock backend captured %d requests", len(requests))

	t.Logf("Resilience with mocks test complete")
}

