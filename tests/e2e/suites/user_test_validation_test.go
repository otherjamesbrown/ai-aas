//go:build (admin || nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestEnvironmentValidation verifies that TestMain properly validates ADMIN_API_KEY
func TestEnvironmentValidation(t *testing.T) {
	// This test verifies the TestMain validation logic by running a subprocess
	// We cannot test TestMain directly as it runs before all tests

	t.Run("CI_MissingAPIKey_FailsFast", func(t *testing.T) {
		// Run a subprocess with TEST_ENV=ci and no ADMIN_API_KEY
		cmd := exec.Command("go", "test", "-v", "-count=1", "-run", "TestUserCLIAvailable", ".")
		cmd.Env = append(os.Environ(),
			"TEST_ENV=ci",
			"GOWORK=off",
		)
		// Remove ADMIN_API_KEY if present
		var filteredEnv []string
		for _, env := range cmd.Env {
			if !strings.HasPrefix(env, "ADMIN_API_KEY=") {
				filteredEnv = append(filteredEnv, env)
			}
		}
		cmd.Env = filteredEnv

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		// Verify test fails
		if err == nil {
			t.Errorf("Expected test to fail without ADMIN_API_KEY in CI mode, but it succeeded")
		}

		// Verify error message contains expected guidance
		expectedPhrases := []string{
			"FATAL: ADMIN_API_KEY environment variable must be set",
			"export ADMIN_API_KEY",
			"source tests/e2e/.admin-key.env",
		}

		for _, phrase := range expectedPhrases {
			if !strings.Contains(outputStr, phrase) {
				t.Errorf("Expected error message to contain %q, but it was missing.\nOutput:\n%s", phrase, outputStr)
			}
		}
	})

	t.Run("Development_MissingAPIKey_Warns", func(t *testing.T) {
		// Run a subprocess with TEST_ENV=development and no ADMIN_API_KEY
		cmd := exec.Command("go", "test", "-v", "-count=1", "-run", "TestUserCLIAvailable", ".")
		cmd.Env = append(os.Environ(),
			"TEST_ENV=development",
			"GOWORK=off",
		)
		// Remove ADMIN_API_KEY if present
		var filteredEnv []string
		for _, env := range cmd.Env {
			if !strings.HasPrefix(env, "ADMIN_API_KEY=") {
				filteredEnv = append(filteredEnv, env)
			}
		}
		cmd.Env = filteredEnv

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		// Verify test passes (warns but doesn't fail)
		if err != nil {
			t.Logf("Test output:\n%s", outputStr)
			t.Errorf("Expected test to pass with warning in development mode, but it failed: %v", err)
		}

		// Verify warning message is present
		if !strings.Contains(outputStr, "WARNING: ADMIN_API_KEY not set") {
			t.Errorf("Expected warning about ADMIN_API_KEY, but it was missing.\nOutput:\n%s", outputStr)
		}
	})

	t.Run("Development_WithAPIKey_NoWarning", func(t *testing.T) {
		// Run a subprocess with TEST_ENV=development and ADMIN_API_KEY set
		cmd := exec.Command("go", "test", "-v", "-count=1", "-run", "TestUserCLIAvailable", ".")
		cmd.Env = append(os.Environ(),
			"TEST_ENV=development",
			"ADMIN_API_KEY=test-key",
			"GOWORK=off",
		)

		output, err := cmd.CombinedOutput()
		outputStr := string(output)

		// Verify test passes
		if err != nil {
			t.Logf("Test output:\n%s", outputStr)
			t.Errorf("Expected test to pass with ADMIN_API_KEY set, but it failed: %v", err)
		}

		// Verify no warning message
		if strings.Contains(outputStr, "WARNING: ADMIN_API_KEY not set") {
			t.Errorf("Did not expect warning when ADMIN_API_KEY is set.\nOutput:\n%s", outputStr)
		}
	})
}
