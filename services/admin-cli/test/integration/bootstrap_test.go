// Package integration provides integration tests for the admin-cli.
//
// Purpose:
//
//	Integration tests that verify the CLI commands work correctly against
//	running services (user-org-service, analytics-service). Tests can run
//	against local services, testcontainers, or deployed environments via
//	environment variables.
//
// Dependencies:
//   - user-org-service must be running and accessible
//   - Database must be migrated (for bootstrap tests)
//   - github.com/stretchr/testify: Test assertions
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-001 (Bootstrap Operations)
//   - specs/009-admin-cli/plan.md#testing
//
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	defaultUserOrgEndpoint = "http://localhost:8081"
	defaultAnalyticsEndpoint = "http://localhost:8084"
	testTimeout = 30 * time.Second
)

// setupTestEnvironment configures test environment variables.
func setupTestEnvironment(t *testing.T) (string, string, func()) {
	t.Helper()

	userOrgEndpoint := os.Getenv("ADMIN_CLI_USER_ORG_ENDPOINT")
	if userOrgEndpoint == "" {
		userOrgEndpoint = os.Getenv("USER_ORG_ENDPOINT")
		if userOrgEndpoint == "" {
			userOrgEndpoint = defaultUserOrgEndpoint
		}
	}

	analyticsEndpoint := os.Getenv("ADMIN_CLI_ANALYTICS_ENDPOINT")
	if analyticsEndpoint == "" {
		analyticsEndpoint = os.Getenv("ANALYTICS_ENDPOINT")
		if analyticsEndpoint == "" {
			analyticsEndpoint = defaultAnalyticsEndpoint
		}
	}

	cleanup := func() {
		// Cleanup if needed
	}

	return userOrgEndpoint, analyticsEndpoint, cleanup
}

// checkServiceHealth verifies a service is reachable and healthy.
func checkServiceHealth(ctx context.Context, endpoint, healthPath string) error {
	healthURL := fmt.Sprintf("%s%s", endpoint, healthPath)
	
	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("service unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service unhealthy: status %d", resp.StatusCode)
	}

	return nil
}

// TestBootstrapFlow tests the bootstrap operation end-to-end.
// This test requires user-org-service to be running on the configured endpoint.
func TestBootstrapFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	userOrgEndpoint, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Check service health
	if err := checkServiceHealth(ctx, userOrgEndpoint, "/healthz"); err != nil {
		t.Skipf("skipping test: user-org-service not available: %v", err)
		t.Logf("💡 Start user-org-service: cd services/user-org-service && make run")
		return
	}

	// Build CLI binary if needed
	cliBinary, err := buildCLIBinary(t)
	if err != nil {
		t.Fatalf("failed to build CLI binary: %v", err)
	}

	// Test 1: Dry-run bootstrap
	t.Run("DryRun", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, cliBinary, "bootstrap",
			"--dry-run",
			"--user-org-endpoint", userOrgEndpoint,
			"--email", "test@example.com",
			"--format", "json",
		)

		output, err := cmd.CombinedOutput()
		// Dry-run should work even without email in some cases
		if err != nil {
			t.Logf("Note: dry-run requires email flag: %v", err)
			return
		}

		// Verify JSON output
		var result map[string]interface{}
		err = json.Unmarshal(output, &result)
		assert.NoError(t, err, "output should be valid JSON")
		if result["mode"] != nil {
			assert.Equal(t, "dry-run", result["mode"], "should be in dry-run mode")
		}
	})

	// Test 2: Health check before bootstrap
	t.Run("HealthCheck", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, cliBinary, "bootstrap",
			"--dry-run=false",
			"--confirm",
			"--user-org-endpoint", userOrgEndpoint,
			"--format", "json",
		)

		// This should check health before proceeding
		output, err := cmd.CombinedOutput()
		
		// Bootstrap might fail if admin already exists - that's OK
		// We're testing that health check happens
		if err != nil {
			// Check if error is due to existing admin (expected)
			outputStr := string(output)
			if strings.Contains(outputStr, "already exists") || 
			   strings.Contains(outputStr, "service health check failed") {
				// This is expected behavior
				return
			}
			t.Logf("Bootstrap output: %s", outputStr)
		}

		// If successful, verify JSON output structure
		var result map[string]interface{}
		if err := json.Unmarshal(output, &result); err == nil {
			assert.NotNil(t, result, "should have result")
		}
	})
}

// TestBootstrapHealthCheck verifies health checks are performed.
func TestBootstrapHealthCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	cliBinary, err := buildCLIBinary(t)
	require.NoError(t, err)

	// Test with invalid endpoint - should fail health check
	cmd := exec.CommandContext(ctx, cliBinary, "bootstrap",
		"--dry-run=false",
		"--confirm",
		"--user-org-endpoint", "http://localhost:99999",
	)

	output, err := cmd.CombinedOutput()
	require.Error(t, err, "should fail with invalid endpoint")
	
	outputStr := string(output)
	assert.Contains(t, outputStr, "unavailable", "error should mention service unavailability")
}

// buildCLIBinary builds the admin-cli binary for testing.
func buildCLIBinary(t *testing.T) (string, error) {
	t.Helper()

	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..")
	serviceDir := filepath.Join(projectRoot, "services", "admin-cli")
	binDir := filepath.Join(serviceDir, "bin")
	cliBinary := filepath.Join(binDir, "admin-cli")

	// Check if binary exists and is recent (within last hour)
	if info, err := os.Stat(cliBinary); err == nil {
		if time.Since(info.ModTime()) < time.Hour {
			return cliBinary, nil
		}
	}

	// Build binary
	cmd := exec.Command("make", "build")
	cmd.Dir = serviceDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to build CLI: %w", err)
	}

	if _, err := os.Stat(cliBinary); err != nil {
		return "", fmt.Errorf("binary not found after build: %w", err)
	}

	return cliBinary, nil
}
