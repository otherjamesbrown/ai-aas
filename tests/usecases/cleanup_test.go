package usecases_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// CleanupStaleTestOrgs purges stale test organizations from previous interrupted test runs.
// This is useful when:
// - Test suite was interrupted (Ctrl+C, timeout, panic)
// - Tests failed before cleanup could run
// - Database has stale orgs causing slug conflicts
//
// Call this in TestMain or at the start of test suites that create organizations.
func CleanupStaleTestOrgs(t *testing.T, client *TestClient) error {
	t.Helper()

	// List all organizations
	resp, err := client.GET("/v1/orgs")
	if err != nil {
		return fmt.Errorf("list organizations: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("list organizations failed: status %d, body: %s", resp.StatusCode, string(resp.Body))
	}

	var orgs []Organization
	if err := json.Unmarshal(resp.Body, &orgs); err != nil {
		return fmt.Errorf("unmarshal organizations: %w", err)
	}

	// Filter to test organizations (starting with "test-org-", "test-sa-", etc.)
	var testOrgs []Organization
	for _, org := range orgs {
		if strings.HasPrefix(org.Slug, "test-org-") ||
			strings.HasPrefix(org.Slug, "test-sa-") {
			testOrgs = append(testOrgs, org)
		}
	}

	if len(testOrgs) == 0 {
		t.Logf("No stale test organizations found")
		return nil
	}

	t.Logf("Found %d stale test organizations to clean up", len(testOrgs))

	// Delete each test organization
	deleted := 0
	failed := 0
	for _, org := range testOrgs {
		delResp, err := client.DELETE(fmt.Sprintf("/v1/orgs/%s", org.ID))
		if err != nil {
			t.Logf("Warning: Failed to delete org %s (%s): %v", org.Slug, org.ID, err)
			failed++
			continue
		}

		if delResp.StatusCode != 200 && delResp.StatusCode != 204 {
			t.Logf("Warning: Failed to delete org %s (%s): status %d", org.Slug, org.ID, delResp.StatusCode)
			failed++
			continue
		}

		deleted++
	}

	t.Logf("Cleanup complete: %d deleted, %d failed", deleted, failed)
	return nil
}

// TestCleanupStaleTestOrgs is a manual test to clean up stale test organizations.
// Run with: go test -v -run TestCleanupStaleTestOrgs
//
// This test is skipped by default unless explicitly run. Use it to manually clean up
// test organizations after an interrupted test run.
func TestCleanupStaleTestOrgs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping manual cleanup test in short mode")
	}

	skipIfNoLiveAPI(t)

	client, err := NewTestClientFromEnv()
	if err != nil {
		t.Fatalf("Failed to create test client: %v", err)
	}

	if err := CleanupStaleTestOrgs(t, client); err != nil {
		t.Fatalf("Cleanup failed: %v", err)
	}
}
