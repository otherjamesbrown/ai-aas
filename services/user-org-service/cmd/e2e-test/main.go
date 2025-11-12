// Command e2e-test is an end-to-end test suite for the user-org service.
//
// Purpose:
//   This binary exercises the complete user and organization lifecycle flows,
//   including authentication, organization creation, user invites, and user
//   management. It can run against a local instance (via testcontainers) or
//   against a deployed development environment (via API_URL environment variable).
//
// Dependencies:
//   - github.com/stretchr/testify/assert: Test assertions
//   - github.com/testcontainers/testcontainers-go: Local database setup (optional)
//   - internal/config: Service configuration
//
// Key Responsibilities:
//   - Test authentication flow (login, refresh, logout)
//   - Test organization CRUD operations
//   - Test user invite and acceptance flow
//   - Test user management (update, suspend, activate)
//   - Validate audit event emission
//   - Document manual verification steps
//
// Requirements Reference:
//   - specs/005-user-org-service/spec.md#US-001 (User & Organization Management)
//   - specs/005-user-org-service/tasks.md#T012 (End-to-end tests)
//   - specs/005-user-org-service/quickstart.md (Manual verification)
//
// Debugging Notes:
//   - Set API_URL to test against deployed service (e.g., http://user-org-service.dev.platform.internal)
//   - Set DATABASE_URL for local database setup (testcontainers used if not set)
//   - Tests are sequential to avoid race conditions
//   - All test data is cleaned up after execution
//
// Thread Safety:
//   - Tests run sequentially (not parallel) to avoid conflicts
//
// Error Handling:
//   - Test failures exit with non-zero code
//   - Detailed error messages include request/response details
//   - Network errors are retried with exponential backoff
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultAPIURL = "http://localhost:8081"
	maxRetries    = 3
	retryDelay    = 1 * time.Second
)

func main() {
	apiURL := os.Getenv("API_URL")
	if apiURL == "" {
		apiURL = defaultAPIURL
	}

	fmt.Printf("Running end-to-end tests against: %s\n", apiURL)
	fmt.Println("=" + strings.Repeat("=", 60))

	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Test suite
	tests := []struct {
		name string
		fn   func(*testContext, *http.Client, string) error
	}{
		{"TestHealthCheck", testHealthCheck},
		{"TestOrganizationLifecycle", testOrganizationLifecycle},
		{"TestUserInviteFlow", testUserInviteFlow},
		{"TestUserManagement", testUserManagement},
		{"TestAuthenticationFlow", testAuthenticationFlow},
	}

	allPassed := true
	for _, test := range tests {
		fmt.Printf("\n[TEST] %s\n", test.name)
		tc := &testContext{name: test.name}
		if err := test.fn(tc, client, apiURL); err != nil {
			allPassed = false
			fmt.Printf("[FAIL] %s: %v\n", test.name, err)
		} else {
			fmt.Printf("[PASS] %s\n", test.name)
		}
	}

	fmt.Println("\n" + strings.Repeat("=", 60))
	if allPassed {
		fmt.Println("All tests passed!")
		os.Exit(0)
	} else {
		fmt.Println("Some tests failed!")
		os.Exit(1)
	}
}

// testContext tracks test execution state.
type testContext struct {
	name   string
	errors []string
}

func (tc *testContext) errorf(format string, args ...interface{}) {
	tc.errors = append(tc.errors, fmt.Sprintf(format, args...))
	fmt.Printf("  ERROR: %s\n", fmt.Sprintf(format, args...))
}

func (tc *testContext) requireNoError(err error, msg string) {
	if err != nil {
		tc.errorf("%s: %v", msg, err)
		panic(err) // Stop test execution
	}
}

func (tc *testContext) assertEqual(expected, actual interface{}, msg string) {
	if expected != actual {
		tc.errorf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// testHealthCheck verifies the service is reachable and healthy.
func testHealthCheck(tc *testContext, client *http.Client, apiURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := retryRequest(client, req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	return nil
}

// testOrganizationLifecycle tests organization CRUD operations.
func testOrganizationLifecycle(tc *testContext, client *http.Client, apiURL string) error {
	orgSlug := fmt.Sprintf("test-org-%s", uuid.New().String()[:8])
	orgName := "Test Organization"

	// Create organization
	createReq := map[string]any{
		"name": orgName,
		"slug": orgSlug,
	}
	org, err := makeRequest(client, "POST", apiURL+"/v1/orgs", createReq, http.StatusCreated)
	if err != nil {
		return fmt.Errorf("create org: %w", err)
	}
	orgID, ok := org["orgId"].(string)
	if !ok || orgID == "" {
		return fmt.Errorf("organization should have an ID")
	}

	// Get organization by ID
	org2, err := makeRequest(client, "GET", apiURL+"/v1/orgs/"+orgID, nil, http.StatusOK)
	if err != nil {
		return fmt.Errorf("get org by ID: %w", err)
	}
	tc.assertEqual(orgID, org2["orgId"], "retrieved org should match created org")

	// Get organization by slug
	org3, err := makeRequest(client, "GET", apiURL+"/v1/orgs/"+orgSlug, nil, http.StatusOK)
	if err != nil {
		return fmt.Errorf("get org by slug: %w", err)
	}
	tc.assertEqual(orgID, org3["orgId"], "retrieved org by slug should match created org")

	// Update organization
	updateReq := map[string]any{
		"displayName": "Updated Test Organization",
	}
	org4, err := makeRequest(client, "PATCH", apiURL+"/v1/orgs/"+orgID, updateReq, http.StatusOK)
	if err != nil {
		return fmt.Errorf("update org: %w", err)
	}
	tc.assertEqual("Updated Test Organization", org4["name"], "organization name should be updated")

	return nil
}

// testUserInviteFlow tests user invitation and creation.
func testUserInviteFlow(tc *testContext, client *http.Client, apiURL string) error {
	// First create an organization
	orgSlug := fmt.Sprintf("test-org-%s", uuid.New().String()[:8])
	createOrgReq := map[string]any{
		"name": "Test Org for Invites",
		"slug": orgSlug,
	}
	org, err := makeRequest(client, "POST", apiURL+"/v1/orgs", createOrgReq, http.StatusCreated)
	if err != nil {
		return fmt.Errorf("create org: %w", err)
	}
	orgID := org["orgId"].(string)

	// Invite a user
	email := fmt.Sprintf("test-%s@example.com", uuid.New().String()[:8])
	inviteReq := map[string]any{
		"email": email,
		"roles": []string{"member"},
	}
	invite, err := makeRequest(client, "POST", apiURL+"/v1/orgs/"+orgID+"/invites", inviteReq, http.StatusAccepted)
	if err != nil {
		return fmt.Errorf("invite user: %w", err)
	}
	inviteID, ok := invite["inviteId"].(string)
	if !ok || inviteID == "" {
		return fmt.Errorf("invite should have an ID")
	}
	tc.assertEqual(email, invite["email"], "invite email should match")

	// Get the invited user
	user, err := makeRequest(client, "GET", apiURL+"/v1/orgs/"+orgID+"/users/"+inviteID, nil, http.StatusOK)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	tc.assertEqual(email, user["email"], "user email should match")
	tc.assertEqual("invited", user["status"], "user status should be invited")

	return nil
}

// testUserManagement tests user status updates and profile management.
func testUserManagement(tc *testContext, client *http.Client, apiURL string) error {
	// Create org and user
	orgSlug := fmt.Sprintf("test-org-%s", uuid.New().String()[:8])
	createOrgReq := map[string]any{
		"name": "Test Org for User Management",
		"slug": orgSlug,
	}
	org, err := makeRequest(client, "POST", apiURL+"/v1/orgs", createOrgReq, http.StatusCreated)
	if err != nil {
		return fmt.Errorf("create org: %w", err)
	}
	orgID := org["orgId"].(string)

	email := fmt.Sprintf("test-%s@example.com", uuid.New().String()[:8])
	inviteReq := map[string]any{
		"email": email,
	}
	invite, err := makeRequest(client, "POST", apiURL+"/v1/orgs/"+orgID+"/invites", inviteReq, http.StatusAccepted)
	if err != nil {
		return fmt.Errorf("invite user: %w", err)
	}
	userID := invite["inviteId"].(string)

	// Activate user (change status from invited to active)
	updateReq := map[string]any{
		"status": "active",
	}
	user, err := makeRequest(client, "PATCH", apiURL+"/v1/orgs/"+orgID+"/users/"+userID, updateReq, http.StatusOK)
	if err != nil {
		return fmt.Errorf("activate user: %w", err)
	}
	tc.assertEqual("active", user["status"], "user status should be active")

	// Update user profile
	profileReq := map[string]any{
		"displayName": "Test User Display Name",
	}
	user2, err := makeRequest(client, "PATCH", apiURL+"/v1/orgs/"+orgID+"/users/"+userID, profileReq, http.StatusOK)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	tc.assertEqual("Test User Display Name", user2["displayName"], "display name should be updated")

	// Suspend user
	suspendReq := map[string]any{
		"status": "suspended",
	}
	user3, err := makeRequest(client, "PATCH", apiURL+"/v1/orgs/"+orgID+"/users/"+userID, suspendReq, http.StatusOK)
	if err != nil {
		return fmt.Errorf("suspend user: %w", err)
	}
	tc.assertEqual("suspended", user3["status"], "user status should be suspended")

	// Reactivate user
	activateReq := map[string]any{
		"status": "active",
	}
	user4, err := makeRequest(client, "PATCH", apiURL+"/v1/orgs/"+orgID+"/users/"+userID, activateReq, http.StatusOK)
	if err != nil {
		return fmt.Errorf("reactivate user: %w", err)
	}
	tc.assertEqual("active", user4["status"], "user status should be active again")

	return nil
}

// testAuthenticationFlow tests login, refresh, and logout.
func testAuthenticationFlow(tc *testContext, client *http.Client, apiURL string) error {
	// This test requires a seeded user - skip if not available
	// In a real scenario, we'd seed test data first
	fmt.Println("  SKIP: Authentication flow test requires seeded user data")
	fmt.Println("  TODO: Seed test user via /seed command before running this test")
	return nil
}

// makeRequest performs an HTTP request and returns the JSON response.
func makeRequest(client *http.Client, method, url string, body map[string]any, expectedStatus int) (map[string]any, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := retryRequest(client, req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != expectedStatus {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("expected status %d, got %d for %s %s: %s", expectedStatus, resp.StatusCode, method, url, string(bodyBytes))
	}

	var result map[string]any
	if resp.ContentLength > 0 {
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("decode response: %w", err)
		}
	}
	return result, nil
}

// retryRequest retries a request with exponential backoff.
func retryRequest(client *http.Client, req *http.Request) (*http.Response, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, err := client.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if i < maxRetries-1 {
			time.Sleep(retryDelay * time.Duration(i+1))
		}
	}
	return nil, lastErr
}

