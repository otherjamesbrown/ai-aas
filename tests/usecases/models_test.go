package usecases_test

import (
	"encoding/json"
	"testing"
)

// Model access use case tests
// See: usecases/models.yaml
// CLI helpers defined in helpers_test.go

// TestUC_MDL_001_ListAvailableModels validates that users can list models
// available to their organization.
//
// Use Case: UC-MDL-001 - List Available Models
// See: usecases/models.yaml
func TestUC_MDL_001_ListAvailableModels(t *testing.T) {
	t.Run("AC-01: list all available models", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org model list`
		result := runOrgCLI("model", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: All models available to the organization are listed
		// Then: Each model shows name, provider, status, and context window
		// Then: Output is formatted as a table
		if result.Output == "" {
			t.Error("expected non-empty output")
		}
	})

	t.Run("AC-02: list models with JSON output", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org model list --json`
		result := runOrgCLI("model", "list", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is valid JSON array
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: Each model object includes name, provider, status, context_window
		var models []map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &models); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
	})

	t.Run("AC-03: handle no models available", func(t *testing.T) {
		t.Skip("requires live API with empty org")

		// Given: Organization has no models assigned
		// When: User runs `ai-aas-org model list`
		result := runOrgCLI("model", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message indicates no models are available
		// Then: Suggestion to contact administrator is shown
	})
}

// TestUC_MDL_002_ShowModelDetails validates that users can retrieve detailed
// information about a specific model.
//
// Use Case: UC-MDL-002 - Show Model Details
// See: usecases/models.yaml
func TestUC_MDL_002_ShowModelDetails(t *testing.T) {
	t.Run("AC-01: show details for valid model", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated and model "gpt-4" is available
		// When: User runs `ai-aas-org model show gpt-4`
		result := runOrgCLI("model", "show", "gpt-4")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Model name is displayed
		// Then: Provider is displayed
		// Then: Status is displayed (online/offline)
		// Then: Context window size is displayed
	})

	t.Run("AC-02: show details with JSON output", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User is authenticated and model "gpt-4" is available
		// When: User runs `ai-aas-org model show gpt-4 --json`
		result := runOrgCLI("model", "show", "gpt-4", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is valid JSON object
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: JSON includes all model properties
		var model map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &model); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
	})

	t.Run("AC-03: reject request for non-existent model", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User specifies a model that doesn't exist
		// When: User runs `ai-aas-org model show nonexistent-model`
		result := runOrgCLI("model", "show", "nonexistent-model")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Fatalf("expected exit code 5, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates model not found
		// Then: Suggestion to run model list is shown
	})

	t.Run("AC-04: reject request for unauthorized model", func(t *testing.T) {
		t.Skip("requires live API")

		// Given: User specifies a model they don't have access to
		// When: User runs `ai-aas-org model show restricted-model`
		result := runOrgCLI("model", "show", "restricted-model")

		// Then: Command fails with exit code 4 (auth error)
		if result.ExitCode != 4 {
			t.Fatalf("expected exit code 4, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates lack of access
		// Then: No model details are shown
	})
}
