package usecases_test

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

// Security use case tests
// See: usecases/security.yaml
// Bead: aas-1iffu (UC-SEC-001), aas-v3klh (UC-SEC-002)

// TestUC_SEC_001_APIKeyRequiredOnAllEndpoints validates that all public API
// endpoints require authentication.
//
// Use Case: UC-SEC-001 - API Key Required on All Endpoints
// See: usecases/security.yaml
func TestUC_SEC_001_APIKeyRequiredOnAllEndpoints(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Get admin API endpoint for admin-api-service tests
	adminAPIEndpoint := getAdminAPIEndpoint()
	if adminAPIEndpoint == "" {
		t.Skip("Admin API endpoint not configured (AI_AAS_ADMIN_API_ENDPOINT)")
	}

	// Create client WITHOUT API key to test unauthenticated access
	noAuthClient := NewTestClient(adminAPIEndpoint, "")

	t.Run("AC-01: Admin API endpoints require authentication", func(t *testing.T) {
		// List of admin API endpoints that should require authentication
		// Using actual admin-api-service routes
		adminEndpoints := []struct {
			method   string
			path     string
			needBody bool
		}{
			{"GET", "/v1/organizations", false},
			{"GET", "/v1/registry/models", false},
			{"GET", "/v1/routing/policies", false},
			{"GET", "/v1/audit-logs", false},
			{"GET", "/v1/models", false},
			{"GET", "/v1/benchmarks/scenarios", false},
			{"GET", "/v1/benchmarks/targets", false},
			{"POST", "/v1/organizations", true},
			{"POST", "/v1/registry/models", true},
			{"POST", "/v1/routing/policies", true},
		}

		for _, ep := range adminEndpoints {
			t.Run(ep.method+" "+ep.path, func(t *testing.T) {
				var resp *TestResponse
				var err error

				if ep.needBody {
					resp, err = noAuthClient.POST(ep.path, map[string]string{})
				} else {
					resp, err = noAuthClient.GET(ep.path)
				}

				if err != nil {
					t.Fatalf("Request failed: %v", err)
				}

				// Then: Response status is 401 Unauthorized
				if resp.StatusCode != 401 {
					t.Errorf("Expected 401 Unauthorized for %s %s, got %d: %s",
						ep.method, ep.path, resp.StatusCode, resp.String())
				}

				// Then: Response body contains error message about missing/invalid key
				bodyLower := strings.ToLower(resp.String())
				if !strings.Contains(bodyLower, "unauthorized") &&
					!strings.Contains(bodyLower, "authentication") &&
					!strings.Contains(bodyLower, "api key") &&
					!strings.Contains(bodyLower, "missing") &&
					!strings.Contains(bodyLower, "invalid") {
					t.Logf("Warning: 401 response should mention auth issue, got: %s", resp.String())
				}

				// Then: No sensitive data is leaked in error response
				// Check that response doesn't contain stack traces or internal paths
				if strings.Contains(resp.String(), "/home/") ||
					strings.Contains(resp.String(), "goroutine") ||
					strings.Contains(resp.String(), "panic") {
					t.Error("Error response should not contain internal details")
				}
			})
		}
	})

	t.Run("AC-02: API Router endpoints require authentication", func(t *testing.T) {
		// Get API Router URL
		routerURL := getAPIRouterURL()
		if routerURL == "" {
			t.Skip("API Router URL not configured")
		}

		routerClient := NewTestClient(routerURL, "")

		// When: Request is made to /v1/chat/completions without auth
		resp, err := routerClient.POST("/v1/chat/completions", map[string]interface{}{
			"model": "test-model",
			"messages": []map[string]string{
				{"role": "user", "content": "test"},
			},
		})
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Then: Response status is 401 Unauthorized
		if resp.StatusCode != 401 {
			t.Errorf("Expected 401 for /v1/chat/completions, got %d: %s",
				resp.StatusCode, resp.String())
		}

		// Then: Response body indicates authentication required
		bodyLower := strings.ToLower(resp.String())
		if !strings.Contains(bodyLower, "unauthorized") &&
			!strings.Contains(bodyLower, "authentication") &&
			!strings.Contains(bodyLower, "api key") {
			t.Logf("Warning: 401 response should mention auth, got: %s", resp.String())
		}
	})

	t.Run("AC-03: Invalid API key is rejected", func(t *testing.T) {
		// Create client with invalid API key format
		invalidKeyClient := NewTestClient(adminAPIEndpoint, "invalid_key_format_123")

		// When: Request is made with Authorization header containing invalid key
		resp, err := invalidKeyClient.GET("/v1/organizations")
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Then: Response status is 401 Unauthorized
		if resp.StatusCode != 401 {
			t.Errorf("Expected 401 for invalid API key, got %d: %s",
				resp.StatusCode, resp.String())
		}

		// Then: Response body indicates invalid credentials
		bodyLower := strings.ToLower(resp.String())
		if !strings.Contains(bodyLower, "unauthorized") &&
			!strings.Contains(bodyLower, "invalid") &&
			!strings.Contains(bodyLower, "authentication") {
			t.Logf("Warning: 401 response should mention invalid credentials, got: %s", resp.String())
		}
	})
}

// TestUC_SEC_002_OrganizationResourceIsolation validates that organizations
// cannot access each other's resources.
//
// Use Case: UC-SEC-002 - Organization Resource Isolation
// See: usecases/security.yaml
func TestUC_SEC_002_OrganizationResourceIsolation(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Create two separate organizations for isolation testing
	orgACtx := NewTestOrgContext(t)
	orgBCtx := NewTestOrgContext(t)

	// Create some resources in Org A
	t.Logf("Setting up test resources in Org A...")

	// Create a user in Org A
	createUserResult := orgACtx.RunCLI("user", "create",
		"--user-email", "isolation-test-"+randomSuffix()+"@example.com",
		"--user-display-name", "Isolation Test User",
		"--role", "user")
	if createUserResult.ExitCode != 0 {
		t.Logf("Warning: Could not create test user in Org A: %s", createUserResult.Output)
	}

	// Extract the user ID if created
	var orgAUserID string
	if createUserResult.ExitCode == 0 {
		// Try to parse user ID from output (JSON format expected)
		var userResp map[string]interface{}
		if err := json.Unmarshal([]byte(createUserResult.Output), &userResp); err == nil {
			if id, ok := userResp["id"].(string); ok {
				orgAUserID = id
			}
		}
		// Also try to get from "user_id" key
		if orgAUserID == "" {
			if id, ok := userResp["user_id"].(string); ok {
				orgAUserID = id
			}
		}
	}

	t.Run("AC-01: Cannot list other organization's users", func(t *testing.T) {
		// Given: Org A has created users, Org B has valid credentials
		// When: Org B requests user list via CLI
		result := orgBCtx.RunCLI("user", "list", "--json")

		// Then: Exit code is 0 (request succeeds)
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Org A's users are not visible in Org B's list
		if strings.Contains(result.Output, "isolation-test-") {
			t.Error("Org B should not see Org A's isolation test user")
		}

		// Verify the output doesn't contain Org A's org ID
		if strings.Contains(result.Output, orgACtx.OrgID) {
			t.Error("Org B's user list should not contain Org A's org ID")
		}
	})

	t.Run("AC-02: Cannot view other organization's user details", func(t *testing.T) {
		if orgAUserID == "" {
			t.Skip("Could not create test user in Org A")
		}

		// Given: Org A has user with known ID
		// When: Org B attempts to get user details for Org A's user ID
		result := orgBCtx.RunCLI("user", "show", orgAUserID)

		// Then: Response is 404 Not Found (not 403 Forbidden)
		// The CLI should fail to find the user, not show access denied
		if result.ExitCode == 0 {
			t.Error("Org B should not be able to view Org A's user details")
		}

		// Then: No information about user existence is leaked
		outputLower := strings.ToLower(result.Output)
		// 404 "not found" is acceptable, 403 "forbidden" leaks existence
		if strings.Contains(outputLower, "forbidden") || strings.Contains(outputLower, "access denied") {
			t.Error("Should return 404 not 403 to avoid leaking resource existence")
		}
	})

	t.Run("AC-03: Cannot list other organization's API keys", func(t *testing.T) {
		// Given: Org A may have API keys
		// When: Org B requests API key list
		result := orgBCtx.RunCLI("apikey", "list", "--json")

		// Then: Exit code is 0 (request succeeds)
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Org A's keys are not visible
		// Verify the output doesn't contain Org A's org ID
		if strings.Contains(result.Output, orgACtx.OrgID) {
			t.Error("Org B's API key list should not contain Org A's org ID")
		}
	})

	t.Run("AC-04: Cannot view other organization's usage data", func(t *testing.T) {
		// When: Org B requests usage report
		result := orgBCtx.RunCLI("usage", "report", "--json")

		// The CLI might not support this command yet
		if result.ExitCode != 0 && strings.Contains(result.Output, "unknown command") {
			t.Skip("Usage report command not implemented")
		}

		// If it works, verify no cross-org data
		if result.ExitCode == 0 {
			// Then: Only Org B's usage data is returned
			if strings.Contains(result.Output, orgACtx.OrgID) {
				t.Error("Org B's usage data should not contain Org A's org ID")
			}
		}
	})

	t.Run("AC-05: Cannot view other organization's audit logs", func(t *testing.T) {
		// When: Org B requests audit logs
		result := orgBCtx.RunCLI("audit", "list", "--json")

		// The CLI might not support this command yet
		if result.ExitCode != 0 && strings.Contains(result.Output, "unknown command") {
			t.Skip("Audit list command not implemented")
		}

		// If it works, verify no cross-org data
		if result.ExitCode == 0 {
			// Then: Only Org B's audit events are returned
			if strings.Contains(result.Output, orgACtx.OrgID) {
				t.Error("Org B's audit logs should not contain Org A's org ID")
			}
		}
	})
}

// randomSuffix generates a random suffix for unique resource names
func randomSuffix() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
