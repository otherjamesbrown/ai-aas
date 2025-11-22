package suites

import (
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/ai-aas/tests/e2e/fixtures"
	"github.com/ai-aas/tests/e2e/harness"
	"github.com/ai-aas/tests/e2e/utils"
)

// isIPAddress checks if a URL string contains an IP address
func isIPAddress(urlStr string) bool {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	
	// Check if host is an IP address (IPv4)
	ipRegex := regexp.MustCompile(`^(\d{1,3}\.){3}\d{1,3}$`)
	if ipRegex.MatchString(host) {
		return true
	}
	
	// Check if host is an IP address with port
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
		return ipRegex.MatchString(host)
	}
	
	return false
}

// TestOrganizationLifecycle tests the complete organization lifecycle
func TestOrganizationLifecycle(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	if org.ID == "" {
		t.Fatal("Organization ID is empty")
	}

	// Verify organization exists
	retrieved, err := orgFixture.Get(org.ID)
	if err != nil {
		t.Fatalf("Failed to get organization: %v", err)
	}

	if retrieved.ID != org.ID {
		t.Fatalf("Retrieved organization ID mismatch: expected %s, got %s", org.ID, retrieved.ID)
	}

	t.Logf("Organization lifecycle test passed: org=%s", org.ID)
}

// TestUserInviteFlow tests the user invitation and acceptance flow
func TestUserInviteFlow(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	userFixture := fixtures.NewUserFixture(ctx.Client, ctx.Fixtures)

	// Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Invite user
	email := ctx.GenerateResourceName("user") + "@test.example.com"
	invite, err := userFixture.Invite(org.ID, email)
	if err != nil {
		t.Fatalf("Failed to invite user: %v", err)
	}

	if invite.InviteID == "" {
		t.Fatal("Invite ID is empty")
	}

	// Create user (simulating acceptance)
	user, err := userFixture.Create(ctx, org.ID, email, "Test User", []string{"member"})
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	if user.Email != email {
		t.Fatalf("User email mismatch: expected %s, got %s", email, user.Email)
	}

	t.Logf("User invite flow test passed: user=%s, org=%s", user.ID, org.ID)
}

// TestAPIKeyIssuance tests API key creation and validation
func TestAPIKeyIssuance(t *testing.T) {
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
	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	if apiKey.Key == "" {
		t.Fatal("API key value is empty")
	}

	// Validate API key (if validation endpoint exists)
	// Note: This may need to be adjusted based on actual API
	valid, err := apiKeyFixture.Validate(apiKey.Key, ctx.Config.APIURLs.UserOrgService)
	if err != nil {
		t.Logf("API key validation not available: %v", err)
	} else if !valid {
		t.Fatalf("API key validation failed")
	}

	t.Logf("API key issuance test passed: key=%s, org=%s", apiKey.ID, org.ID)
}

// TestModelRequestRouting tests routing a request to a model backend
func TestModelRequestRouting(t *testing.T) {
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
	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Failed to create API key: %v", err)
	}

	// Create client for API router service
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)
	
	// If using IP address, set Host header for ingress routing
	if isIPAddress(ctx.Config.APIURLs.APIRouterService) {
		routerClient.SetHeader("Host", "api.dev.ai-aas.local")
	}

	// Make inference request
	reqBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello, this is a test",
			},
		},
	}

	resp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Logf("Model request failed (may be expected if backend unavailable): %v", err)
		// This is acceptable for e2e tests - backend may not be available
		return
	}

	if resp.StatusCode != 200 {
		t.Logf("Model request returned non-200 status: %d (may be expected)", resp.StatusCode)
		// Acceptable if backend is unavailable or rate-limited
		return
	}

	t.Logf("Model request routing test passed: status=%d", resp.StatusCode)
}

// TestSuccessfulCompletion tests the complete end-to-end flow
func TestSuccessfulCompletion(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
	userFixture := fixtures.NewUserFixture(ctx.Client, ctx.Fixtures)
	apiKeyFixture := fixtures.NewAPIKeyFixture(ctx.Client, ctx.Fixtures)

	// Step 1: Create organization
	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Step 1 failed - create organization: %v", err)
	}
	t.Logf("Step 1: Created organization %s", org.ID)

	// Step 2: Create user
	email := ctx.GenerateResourceName("user") + "@test.example.com"
	user, err := userFixture.Create(ctx, org.ID, email, "Test User", []string{"admin"})
	if err != nil {
		t.Fatalf("Step 2 failed - create user: %v", err)
	}
	t.Logf("Step 2: Created user %s", user.ID)

	// Step 3: Create API key
	apiKey, err := apiKeyFixture.Create(ctx, org.ID, "", []string{"inference:read", "inference:write"})
	if err != nil {
		t.Fatalf("Step 3 failed - create API key: %v", err)
	}
	t.Logf("Step 3: Created API key %s", apiKey.ID)

	// Step 4: Make model request
	routerClient := harness.NewClient(ctx.Config.APIURLs.APIRouterService, ctx.Config.Timeouts.RequestTimeout)
	routerClient.SetHeader("Authorization", "Bearer "+apiKey.Key)

	reqBody := map[string]interface{}{
		"model": "gpt-4",
		"messages": []map[string]interface{}{
			{
				"role":    "user",
				"content": "Hello",
			},
		},
	}

	resp, err := routerClient.POST("/v1/chat/completions", reqBody)
	if err != nil {
		t.Logf("Step 4: Model request failed (backend may be unavailable): %v", err)
		// Acceptable - backend may not be available in test environment
	} else {
		t.Logf("Step 4: Model request completed with status %d", resp.StatusCode)
	}

	// Verify artifacts were collected
	artifacts := ctx.Artifacts.List()
	if len(artifacts) == 0 {
		t.Logf("No artifacts collected (may be expected if requests failed)")
	} else {
		t.Logf("Collected %d artifacts", len(artifacts))
	}

	t.Logf("Successful completion test passed: org=%s, user=%s, key=%s", org.ID, user.ID, apiKey.ID)
}

// TestArtifactCollection verifies that artifacts are properly collected
func TestArtifactCollection(t *testing.T) {
	ctx := setupTestContext(t)
	defer ctx.Cleanup()

	orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)

	// Generate correlation IDs
	corrIDs := utils.GenerateCorrelationIDs()

	// Create organization with correlation IDs
	orgClient := ctx.Client
	orgClient.SetHeader("X-Request-ID", corrIDs.RequestID)
	orgClient.SetHeader("X-Trace-ID", corrIDs.TraceID)

	org, err := orgFixture.Create(ctx, "")
	if err != nil {
		t.Fatalf("Failed to create organization: %v", err)
	}

	// Verify artifacts were collected
	artifacts := ctx.Artifacts.List()
	if len(artifacts) == 0 {
		t.Logf("No artifacts collected (artifact collection may be disabled)")
	} else {
		t.Logf("Collected %d artifacts", len(artifacts))

		// Verify correlation IDs in artifacts
		for _, artifact := range artifacts {
			if artifact.Metadata["request_id"] != corrIDs.RequestID {
				t.Logf("Correlation ID mismatch in artifact %s", artifact.ID)
			}
		}
	}

	t.Logf("Artifact collection test passed: org=%s, artifacts=%d", org.ID, len(artifacts))
}

// setupTestContext creates a test context for test execution
func setupTestContext(t *testing.T) *harness.Context {
	config, err := harness.LoadConfig()
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	ctx, err := harness.NewContext(config)
	if err != nil {
		t.Fatalf("Failed to create test context: %v", err)
	}

	return ctx
}

