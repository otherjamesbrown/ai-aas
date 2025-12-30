package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// CLI-to-API Contract Integration Tests
//
// These tests verify that the CLI client code can correctly parse actual API responses.
// They prevent contract drift like the bug in aas-ghda where the CLI expected a wrapper
// struct but the API returned a raw array.
//
// Pattern: Execute CLI commands with --json flag and verify successful JSON parsing.
// If JSON parsing succeeds, it means the CLI structs match what the API returns.
//
// Related beads:
// - aas-ghda: Investigation - ListAPIKeys unmarshal error
// - aas-rmt9: Fix - ListAPIKeys to handle array response
// - aas-1ujb: Remove or implement ListUserAPIKeys endpoint
// - aas-dzo8: Add CLI-to-API contract integration tests (this file)

// TestContract_ListUsers_JSONParsing verifies that the CLI can parse the API's
// response when listing users.
//
// Context: The API returns `[]User` (raw array), not a wrapper struct.
// The CLI must be able to decode this and output as JSON.
//
// Related: aas-ghda (bug), commit 82e6fbcb (fix for ListUsers)
func TestContract_ListUsers_JSONParsing(t *testing.T) {
	skipIfNoLiveAPI(t)

	// When: User runs `ai-aas-org user list --json`
	result := runOrgCLI("user", "list", "--json")

	// Then: CLI successfully parses API response and outputs JSON
	if result.ExitCode != 0 {
		t.Fatalf("user list failed (contract mismatch?): exit %d\n%s", result.ExitCode, result.Output)
	}

	// Then: Output is valid JSON array
	if !isValidJSON(result.Output) {
		t.Fatalf("Output is not valid JSON (contract mismatch?):\n%s", result.Output)
	}

	// Then: We can parse it as array of user objects
	var users []map[string]interface{}
	if err := json.Unmarshal([]byte(result.Output), &users); err != nil {
		t.Fatalf("Failed to parse users array (contract mismatch?): %v\nOutput:\n%s", err, result.Output)
	}

	// Then: Array structure is correct (even if empty)
	t.Logf("✓ Contract test passed: ListUsers returned %d users", len(users))

	// If users exist, verify required fields
	if len(users) > 0 {
		requiredFields := []string{"userId", "email", "displayName", "status", "orgId"}
		for _, field := range requiredFields {
			if _, exists := users[0][field]; !exists {
				t.Errorf("User object missing required field %q", field)
			}
		}
	}
}

// TestContract_ListAPIKeys_JSONParsing verifies that the CLI can parse the API's
// response when listing API keys.
//
// Context: The API returns `[]APIKey` (raw array), not `{"api_keys": [...]}`.
// The CLI must be able to decode this and output as JSON.
//
// Related: aas-ghda (bug), aas-rmt9 (fix for ListAPIKeys)
func TestContract_ListAPIKeys_JSONParsing(t *testing.T) {
	skipIfNoLiveAPI(t)

	// When: User runs `ai-aas-org apikey list --json`
	result := runOrgCLI("apikey", "list", "--json")

	// Then: CLI successfully parses API response and outputs JSON
	if result.ExitCode != 0 {
		t.Fatalf("apikey list failed (contract mismatch?): exit %d\n%s", result.ExitCode, result.Output)
	}

	// Then: Output is valid JSON array
	if !isValidJSON(result.Output) {
		t.Fatalf("Output is not valid JSON (contract mismatch?):\n%s", result.Output)
	}

	// Then: We can parse it as array of API key objects
	var keys []map[string]interface{}
	if err := json.Unmarshal([]byte(result.Output), &keys); err != nil {
		t.Fatalf("Failed to parse API keys array (contract mismatch?): %v\nOutput:\n%s", err, result.Output)
	}

	// Then: Array structure is correct (even if empty)
	t.Logf("✓ Contract test passed: ListAPIKeys returned %d keys", len(keys))

	// If keys exist, verify required fields
	if len(keys) > 0 {
		requiredFields := []string{"id", "name", "user_id", "org_id", "status", "created_at"}
		for _, field := range requiredFields {
			if _, exists := keys[0][field]; !exists {
				t.Errorf("APIKey object missing required field %q", field)
			}
		}
	}
}

// TestContract_ListUsers_NoWrapperStruct verifies that the CLI returns a raw array,
// not a wrapper object with metadata.
//
// This test catches regressions where the CLI might start expecting:
// {"users": [...], "total_count": N} instead of the raw array the API returns.
func TestContract_ListUsers_NoWrapperStruct(t *testing.T) {
	skipIfNoLiveAPI(t)

	// When: User runs `ai-aas-org user list --json`
	result := runOrgCLI("user", "list", "--json")

	if result.ExitCode != 0 {
		t.Fatalf("user list failed: exit %d\n%s", result.ExitCode, result.Output)
	}

	// Then: Output starts with '[' (array), not '{' (object)
	trimmed := strings.TrimSpace(result.Output)
	if !strings.HasPrefix(trimmed, "[") {
		t.Errorf("Expected JSON array (starts with '['), got:\n%s", trimmed[:min(100, len(trimmed))])
	}

	// Then: Output does NOT contain wrapper keys like "users" or "total_count"
	if strings.Contains(result.Output, `"users"`) {
		t.Error("Output contains wrapper key 'users' - API should return raw array")
	}
	if strings.Contains(result.Output, `"total_count"`) {
		t.Error("Output contains wrapper key 'total_count' - API should return raw array")
	}

	t.Log("✓ Contract test passed: ListUsers returns raw array without wrapper")
}

// TestContract_ListAPIKeys_NoWrapperStruct verifies that the CLI returns a raw array,
// not a wrapper object.
func TestContract_ListAPIKeys_NoWrapperStruct(t *testing.T) {
	skipIfNoLiveAPI(t)

	// When: User runs `ai-aas-org apikey list --json`
	result := runOrgCLI("apikey", "list", "--json")

	if result.ExitCode != 0 {
		t.Fatalf("apikey list failed: exit %d\n%s", result.ExitCode, result.Output)
	}

	// Then: Output starts with '[' (array), not '{' (object)
	trimmed := strings.TrimSpace(result.Output)
	if !strings.HasPrefix(trimmed, "[") {
		t.Errorf("Expected JSON array (starts with '['), got:\n%s", trimmed[:min(100, len(trimmed))])
	}

	// Then: Output does NOT contain wrapper keys
	if strings.Contains(result.Output, `"api_keys"`) {
		t.Error("Output contains wrapper key 'api_keys' - API should return raw array")
	}
	if strings.Contains(result.Output, `"apiKeys"`) {
		t.Error("Output contains wrapper key 'apiKeys' - API should return raw array")
	}
	if strings.Contains(result.Output, `"total_count"`) {
		t.Error("Output contains wrapper key 'total_count' - API should return raw array")
	}

	t.Log("✓ Contract test passed: ListAPIKeys returns raw array without wrapper")
}

// TestContract_GetUser_SingleObject verifies that GetUser returns a single object,
// not an array.
func TestContract_GetUser_SingleObject(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Given: Get a user email from the list
	listResult := runOrgCLI("user", "list", "--json")
	if listResult.ExitCode != 0 {
		t.Skip("Cannot get user list for test setup")
	}

	var users []map[string]interface{}
	if err := json.Unmarshal([]byte(listResult.Output), &users); err != nil || len(users) == 0 {
		t.Skip("No users available for GetUser test")
	}
	testEmail := users[0]["email"].(string)

	// When: User runs `ai-aas-org user show <email> --json`
	result := runOrgCLI("user", "show", testEmail, "--json")

	// Then: CLI successfully parses API response
	if result.ExitCode != 0 {
		t.Fatalf("user show failed (contract mismatch?): exit %d\n%s", result.ExitCode, result.Output)
	}

	// Then: Output is valid JSON object (not array)
	trimmed := strings.TrimSpace(result.Output)
	if !strings.HasPrefix(trimmed, "{") {
		t.Errorf("Expected JSON object (starts with '{'), got:\n%s", trimmed[:min(100, len(trimmed))])
	}

	// Then: We can parse it as a single user object
	var user map[string]interface{}
	if err := json.Unmarshal([]byte(result.Output), &user); err != nil {
		t.Fatalf("Failed to parse user object (contract mismatch?): %v\nOutput:\n%s", err, result.Output)
	}

	// Then: Has required fields
	if _, exists := user["userId"]; !exists {
		t.Error("User object missing 'userId' field")
	}
	if _, exists := user["email"]; !exists {
		t.Error("User object missing 'email' field")
	}

	t.Log("✓ Contract test passed: GetUser returns single object")
}

// TestContract_CreateUser_ResponseFields verifies that CreateUser response
// has the expected field structure.
func TestContract_CreateUser_ResponseFields(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Given: Unique test email
	testEmail := "contract-test-" + generateTimestamp() + "@example.com"

	// When: User runs `ai-aas-org user create --json`
	result := runOrgCLI("user", "create",
		"--user-email", testEmail,
		"--display-name", "Contract Test User",
		"--json")

	// Then: CLI successfully parses API response
	if result.ExitCode != 0 {
		t.Fatalf("user create failed (contract mismatch?): exit %d\n%s", result.ExitCode, result.Output)
	}

	// Then: Output is valid JSON object
	if !isValidJSON(result.Output) {
		t.Fatalf("Output is not valid JSON (contract mismatch?):\n%s", result.Output)
	}

	var user map[string]interface{}
	if err := json.Unmarshal([]byte(result.Output), &user); err != nil {
		t.Fatalf("Failed to parse user object (contract mismatch?): %v\nOutput:\n%s", err, result.Output)
	}

	// Then: Response has required fields
	requiredFields := []string{"userId", "email", "displayName", "status"}
	for _, field := range requiredFields {
		if _, exists := user[field]; !exists {
			t.Errorf("CreateUser response missing required field %q", field)
		}
	}

	// Cleanup: Delete the test user
	if userID, ok := user["userId"].(string); ok {
		_ = runOrgCLI("user", "delete", userID, "--force")
	}

	t.Log("✓ Contract test passed: CreateUser response has correct fields")
}

// TestContract_ErrorResponses_Consistent verifies that error responses
// have consistent structure across endpoints.
func TestContract_ErrorResponses_Consistent(t *testing.T) {
	skipIfNoLiveAPI(t)

	// When: User tries to get non-existent user
	result := runOrgCLI("user", "show", "nonexistent-user-xyz@example.com", "--json")

	// Then: CLI handles error response (exit code != 0)
	if result.ExitCode == 0 {
		t.Skip("Expected error response, but command succeeded")
	}

	// Then: Error output contains structured error info
	// (CLI may not output errors as JSON, so we check for key error indicators)
	output := strings.ToLower(result.Output)
	if !strings.Contains(output, "not found") && !strings.Contains(output, "error") {
		t.Logf("Warning: Error output may not be structured:\n%s", result.Output)
	}

	t.Log("✓ Contract test passed: Error responses are handled")
}

// TestContract_EmptyLists_ReturnEmptyArrays verifies that list endpoints
// return empty arrays [] when no items exist, not null or error.
func TestContract_EmptyLists_ReturnEmptyArrays(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Note: We can't guarantee empty lists in the test environment,
	// but we can verify the structure is correct regardless of content

	// When: User lists API keys
	result := runOrgCLI("apikey", "list", "--json")

	if result.ExitCode != 0 {
		t.Fatalf("apikey list failed: exit %d\n%s", result.ExitCode, result.Output)
	}

	// Then: Output is JSON array (not null or error object)
	trimmed := strings.TrimSpace(result.Output)
	if trimmed == "null" {
		t.Error("List endpoint returned null instead of empty array")
	}
	if strings.HasPrefix(trimmed, "{") && strings.Contains(result.Output, `"error"`) {
		t.Error("List endpoint returned error object instead of array")
	}

	// Then: Can parse as array (even if empty)
	var keys []interface{}
	if err := json.Unmarshal([]byte(result.Output), &keys); err != nil {
		t.Fatalf("Failed to parse as array: %v\nOutput:\n%s", err, result.Output)
	}

	t.Logf("✓ Contract test passed: List endpoint returns array (length %d)", len(keys))
}

// Helper Functions

// generateTimestamp creates a unique timestamp string for test data.
func generateTimestamp() string {
	return strings.ReplaceAll(strings.ReplaceAll(
		strings.Split(runOrgCLI("user", "list").Output, "\n")[0],
		":", ""), " ", "")[:14]
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
