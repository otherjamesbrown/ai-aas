package usecases_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// CLIResult holds the result of a CLI command execution
type CLIResult struct {
	Output   string
	Error    string
	ExitCode int
}

// runOrgCLI executes an ai-aas-org command and returns the result
func runOrgCLI(args ...string) CLIResult {
	cmd := exec.Command("ai-aas-org", args...)
	output, err := cmd.CombinedOutput()

	result := CLIResult{
		Output:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = err.Error()
	}

	return result
}

// runOrgCLIWithEnv executes an ai-aas-org command with custom environment variables
func runOrgCLIWithEnv(env map[string]string, args ...string) CLIResult {
	cmd := exec.Command("ai-aas-org", args...)

	// Copy current environment and add custom vars
	cmd.Env = os.Environ()
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	output, err := cmd.CombinedOutput()

	result := CLIResult{
		Output:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = err.Error()
	}

	return result
}

// tempConfigFile creates a temporary config file path and returns it
// The temporary directory is automatically cleaned up when the test ends
func tempConfigFile(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	return filepath.Join(tmpDir, ".ai-aas-org.yaml")
}

// isValidJSON checks if a string is valid JSON
func isValidJSON(s string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(s), &js) == nil
}
