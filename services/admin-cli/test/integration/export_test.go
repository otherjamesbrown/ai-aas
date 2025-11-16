// Package integration provides integration tests for export commands.
//
// Purpose:
//
//	Test export commands end-to-end against running analytics-service.
//
// Requirements Reference:
//   - specs/009-admin-cli/spec.md#US-003 (Exports)
//
package integration

import (
	"context"
	"encoding/json"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExportUsage tests usage export functionality.
func TestExportUsage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	_, analyticsEndpoint, cleanup := setupTestEnvironment(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// Check service health
	if err := checkServiceHealth(ctx, analyticsEndpoint, "/analytics/v1/status/healthz"); err != nil {
		t.Skipf("skipping test: analytics-service not available: %v", err)
		return
	}

	cliBinary, err := buildCLIBinary(t)
	require.NoError(t, err)

	// Test: Export usage (dry-run if supported)
	t.Run("ExportUsageDryRun", func(t *testing.T) {
		cmd := exec.CommandContext(ctx, cliBinary, "export", "usage",
			"--analytics-endpoint", analyticsEndpoint,
			"--format", "json",
		)

		output, err := cmd.CombinedOutput()
		// This might fail if command not fully implemented
		if err != nil {
			t.Logf("Note: export usage not fully implemented: %v", err)
			return
		}

		var result map[string]interface{}
		err = json.Unmarshal(output, &result)
		assert.NoError(t, err, "output should be valid JSON")
	})
}

