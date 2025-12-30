//go:build (nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"net/http"
	"testing"
	"time"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
)

// TestAPIKeyRateLimiting tests that rate limiting is enforced at the API router level.
// When an API key exceeds the rate limit, subsequent requests should receive HTTP 429.
func TestAPIKeyRateLimiting(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create API key with inference scopes
	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "ratelimit-test-key", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Clean up Redis rate limit counters before test to ensure clean state
	if err := ctx.CleanupRedisTokenCounters(); err != nil {
		t.Logf("Warning: Could not cleanup Redis counters: %v (continuing anyway)", err)
	}

	// Create client for API router service
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
	routerClient.SetHeader("X-API-Key", apiKey.Key)

	// Make inference request body (minimal valid request)
	// This will fail at the backend level (model not found) but should pass rate limiting
	reqBody := map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Test rate limiting",
			},
		},
	}

	// Development cluster rate limits: 100 RPS with burst of 200
	// We need to exceed the burst limit to trigger rate limiting
	// Send 250 requests rapidly to definitely exceed the limit
	rateLimitExceeded := false
	successfulRequests := 0
	rateLimitedRequests := 0

	t.Logf("Sending rapid requests to trigger rate limit...")

	for i := 0; i < 250; i++ {
		resp, err := routerClient.POST("/v1/chat/completions", reqBody)
		if err != nil {
			// Network errors are not rate limit errors
			t.Logf("Request %d: Network error: %v", i+1, err)
			continue
		}

		// Check response status
		switch resp.StatusCode {
		case http.StatusTooManyRequests: // 429
			rateLimitedRequests++
			if !rateLimitExceeded {
				t.Logf("Rate limit exceeded at request %d (status: 429)", i+1)
				rateLimitExceeded = true

				// Validate Retry-After header is present
				retryAfter := resp.Headers.Get("Retry-After")
				if retryAfter == "" {
					t.Error("Expected Retry-After header in 429 response, but got none")
				} else {
					t.Logf("Retry-After header present: %s seconds", retryAfter)
				}

				// Validate rate limit headers
				limitHeader := resp.Headers.Get("X-RateLimit-Limit")
				remainingHeader := resp.Headers.Get("X-RateLimit-Remaining")
				if limitHeader == "" {
					t.Error("Expected X-RateLimit-Limit header in 429 response")
				}
				if remainingHeader == "" {
					t.Error("Expected X-RateLimit-Remaining header in 429 response")
				}

				t.Logf("Rate limit headers: Limit=%s, Remaining=%s", limitHeader, remainingHeader)

				// Parse response body to check error structure
				var respBody map[string]interface{}
				if err := resp.UnmarshalJSON(&respBody); err == nil {
					if errMsg, ok := respBody["error"].(string); ok {
						t.Logf("Error message: %s", errMsg)
					}
					if errCode, ok := respBody["code"].(string); ok {
						t.Logf("Error code: %s", errCode)
					}
				}
			}
		case http.StatusOK, http.StatusNotFound, http.StatusBadGateway, http.StatusServiceUnavailable:
			// These are acceptable responses before rate limit is hit
			// - 200: Backend responded successfully (unlikely with test-model)
			// - 404: Model not found (expected for test-model)
			// - 502: Backend unavailable
			// - 503: Service temporarily unavailable
			successfulRequests++
		default:
			t.Logf("Request %d: Unexpected status code: %d", i+1, resp.StatusCode)
		}

		// Small delay to simulate rapid requests (not instant)
		time.Sleep(5 * time.Millisecond)

		// Once we've confirmed rate limiting works, no need to continue
		if rateLimitExceeded && rateLimitedRequests >= 5 {
			t.Logf("Rate limiting confirmed after %d total requests (%d successful, %d rate-limited)", i+1, successfulRequests, rateLimitedRequests)
			break
		}
	}

	// Verify that rate limiting was enforced
	if !rateLimitExceeded {
		t.Errorf("FAIL: Expected rate limit to be exceeded (HTTP 429), but all %d requests succeeded. Rate limiting may not be enabled or configured.", successfulRequests)
		t.Logf("Diagnostic info:")
		t.Logf("- API Router Service URL: %s", ctx.Config.APIURLs.APIRouterService)
		t.Logf("- Organization ID: %s", org.ID)
		t.Logf("- API Key ID: %s", apiKey.ID)
		t.Logf("- Expected rate limit: 100 RPS, burst 200 (development cluster config)")
		t.Logf("- Requests sent: 250")
		t.Logf("")
		t.Logf("Possible causes:")
		t.Logf("1. Redis is not running or not accessible from api-router-service")
		t.Logf("2. Rate limiting middleware is not enabled in deployment")
		t.Logf("3. Rate limits are configured too high for this test")
		t.Logf("4. Requests are being rate-limited at a different layer (ingress)")
	} else {
		t.Logf("SUCCESS: Rate limiting is working correctly")
		t.Logf("- Requests before rate limit: %d", successfulRequests)
		t.Logf("- Rate-limited requests: %d", rateLimitedRequests)
	}
}

// TestRateLimitRecovery tests that rate limits reset after the configured window.
func TestRateLimitRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping rate limit recovery test in short mode")
	}

	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Create API key
	apiKey, err := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "recovery-test-key", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Clean up Redis rate limit counters
	if err := ctx.CleanupRedisTokenCounters(); err != nil {
		t.Logf("Warning: Could not cleanup Redis counters: %v", err)
	}

	// Create router client
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
	routerClient.SetHeader("X-API-Key", apiKey.Key)

	reqBody := map[string]interface{}{
		"model": "test-model",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Test",
			},
		},
	}

	// Step 1: Trigger rate limit
	rateLimitTriggered := false
	for i := 0; i < 250; i++ {
		resp, err := routerClient.POST("/v1/chat/completions", reqBody)
		if err == nil && resp.StatusCode == http.StatusTooManyRequests {
			rateLimitTriggered = true
			t.Logf("Rate limit triggered after %d requests", i+1)

			// Get Retry-After value
			retryAfter := resp.Headers.Get("Retry-After")
			t.Logf("Retry-After: %s seconds", retryAfter)
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if !rateLimitTriggered {
		t.Skip("Could not trigger rate limit, skipping recovery test")
	}

	// Step 2: Wait for rate limit window to reset
	// Rate limiter uses token bucket with refill, so we need to wait for bucket to refill
	t.Logf("Waiting 2 seconds for rate limit to reset...")
	time.Sleep(2 * time.Second)

	// Step 3: Verify request succeeds after waiting
	resp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Fatalf("Request failed after rate limit reset: %v", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		t.Errorf("Rate limit still active after waiting for reset (status: 429)")
		t.Logf("This suggests the rate limit window is longer than expected or tokens are not refilling")
	} else {
		t.Logf("SUCCESS: Request succeeded after rate limit reset (status: %d)", resp.StatusCode)
	}
}
