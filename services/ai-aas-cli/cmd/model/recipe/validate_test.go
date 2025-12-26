package recipe

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestNewValidateCommand verifies that the validate command is created correctly
func TestNewValidateCommand(t *testing.T) {
	cmd := NewValidateCommand()

	require.NotNil(t, cmd, "NewValidateCommand() returned nil")
	assert.Equal(t, "validate <file>", cmd.Use, "command Use should be 'validate <file>'")
	assert.NotEmpty(t, cmd.Short, "command Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "command Long description should not be empty")
	assert.NotNil(t, cmd.RunE, "command RunE should be set")

	// Verify format flag exists
	formatFlag := cmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
	assert.Equal(t, "text", formatFlag.DefValue, "format flag default should be 'text'")
}

// TestValidateCommand_ValidFile verifies validation of a valid recipe file
func TestValidateCommand_ValidFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	recipeFile := filepath.Join(tmpDir, "valid-recipe.yaml")

	// Create a valid recipe file
	validRecipe := `
name: mistral-7b-instruct-v03
display_name: Mistral 7B Instruct v0.3
description: Mistral 7B Instruct model optimized for chat
model_id: mistralai/Mistral-7B-Instruct-v0.3
runtime: vllm
spec:
  resources:
    gpu:
      count: 1
      vendor: nvidia
  parameters:
    max_model_len: 8192
    dtype: float16
`
	err := os.WriteFile(recipeFile, []byte(validRecipe), 0644)
	require.NoError(t, err, "failed to create test recipe file")

	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{recipeFile})
	err = cmd.Execute()

	require.NoError(t, err, "command should execute successfully for valid recipe")
	outputStr := output.String()
	assert.Contains(t, outputStr, "valid", "output should indicate recipe is valid")
	assert.NotContains(t, outputStr, "error", "output should not contain errors")
}

// TestValidateCommand_InvalidFile verifies validation reports errors for invalid recipe
func TestValidateCommand_InvalidFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	recipeFile := filepath.Join(tmpDir, "invalid-recipe.yaml")

	// Create an invalid recipe file (missing required fields)
	invalidRecipe := `
name: incomplete-recipe
# Missing: display_name, model_id, runtime, spec
`
	err := os.WriteFile(recipeFile, []byte(invalidRecipe), 0644)
	require.NoError(t, err, "failed to create test recipe file")

	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{recipeFile})
	err = cmd.Execute()

	require.Error(t, err, "command should return error for invalid recipe")
	outputStr := output.String()
	assert.Contains(t, outputStr, "validation", "output should mention validation")
}

// TestValidateCommand_FileNotFound verifies error when file doesn't exist
func TestValidateCommand_FileNotFound(t *testing.T) {
	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{"/nonexistent/path/recipe.yaml"})
	err := cmd.Execute()

	require.Error(t, err, "command should return error when file doesn't exist")
	assert.Contains(t, err.Error(), "no such file", "error should mention file not found")
}

// TestValidateCommand_InvalidYAML verifies error for malformed YAML
func TestValidateCommand_InvalidYAML(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	recipeFile := filepath.Join(tmpDir, "malformed.yaml")

	// Create a malformed YAML file
	malformedYAML := `
name: test-recipe
  invalid indentation here
runtime: vllm
	tabs and spaces mixed
`
	err := os.WriteFile(recipeFile, []byte(malformedYAML), 0644)
	require.NoError(t, err, "failed to create test file")

	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{recipeFile})
	err = cmd.Execute()

	require.Error(t, err, "command should return error for malformed YAML")
	errorMsg := err.Error()
	assert.True(t,
		contains(errorMsg, "yaml") || contains(errorMsg, "parse"),
		"error should mention YAML or parsing: %s", errorMsg)
}

// TestValidateCommand_NoFile verifies error when no file argument is provided
func TestValidateCommand_NoFile(t *testing.T) {
	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	// Don't provide any args
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	require.Error(t, err, "command should return error when no file is provided")
	// The error should indicate that an argument is required or received 0
	errorMsg := err.Error()
	assert.True(t,
		contains(errorMsg, "required") || contains(errorMsg, "received 0"),
		"error should mention required argument or no args received: %s", errorMsg)
}

// TestValidateCommand_JSONOutput verifies JSON output format
func TestValidateCommand_JSONOutput(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	recipeFile := filepath.Join(tmpDir, "recipe.yaml")

	// Create a valid recipe file
	validRecipe := `
name: test-recipe
display_name: Test Recipe
description: A test recipe
model_id: test/model
runtime: vllm
spec:
  resources:
    gpu:
      count: 1
`
	err := os.WriteFile(recipeFile, []byte(validRecipe), 0644)
	require.NoError(t, err, "failed to create test recipe file")

	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{recipeFile, "--format", "json"})
	err = cmd.Execute()

	require.NoError(t, err, "command should execute successfully with JSON format")

	// Verify output is valid JSON
	var result map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err, "output should be valid JSON")

	// Verify expected fields are present
	assert.Contains(t, result, "valid", "JSON should contain 'valid' field")
	assert.Contains(t, result, "file", "JSON should contain 'file' field")
	// If valid, there should be no errors
	if valid, ok := result["valid"].(bool); ok && valid {
		errors, hasErrors := result["errors"]
		if hasErrors {
			// Errors field should be empty or nil
			if errList, isList := errors.([]interface{}); isList {
				assert.Empty(t, errList, "errors list should be empty for valid recipe")
			}
		}
	}
}

// TestValidateCommand_MissingRequiredFields verifies all required field validations
func TestValidateCommand_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name          string
		recipeContent string
		expectedError string
	}{
		{
			name: "missing name",
			recipeContent: `
display_name: Test Recipe
model_id: test/model
runtime: vllm
spec:
  resources:
    gpu:
      count: 1
`,
			expectedError: "name",
		},
		{
			name: "missing model_id",
			recipeContent: `
name: test-recipe
display_name: Test Recipe
runtime: vllm
spec:
  resources:
    gpu:
      count: 1
`,
			expectedError: "model_id",
		},
		{
			name: "missing runtime",
			recipeContent: `
name: test-recipe
display_name: Test Recipe
model_id: test/model
spec:
  resources:
    gpu:
      count: 1
`,
			expectedError: "runtime",
		},
		{
			name: "missing spec",
			recipeContent: `
name: test-recipe
display_name: Test Recipe
model_id: test/model
runtime: vllm
`,
			expectedError: "spec",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()
			recipeFile := filepath.Join(tmpDir, "recipe.yaml")

			err := os.WriteFile(recipeFile, []byte(tt.recipeContent), 0644)
			require.NoError(t, err, "failed to create test recipe file")

			cmd := NewValidateCommand()
			output := &bytes.Buffer{}
			cmd.SetOut(output)
			cmd.SetErr(output)

			cmd.SetArgs([]string{recipeFile})
			err = cmd.Execute()

			require.Error(t, err, "command should return error for missing %s", tt.expectedError)
			outputStr := output.String()
			assert.Contains(t, outputStr, tt.expectedError, "output should mention missing field: %s", tt.expectedError)
		})
	}
}

// TestValidateCommand_InvalidRuntimeValue verifies runtime validation
func TestValidateCommand_InvalidRuntimeValue(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	recipeFile := filepath.Join(tmpDir, "recipe.yaml")

	// Create recipe with invalid runtime
	invalidRecipe := `
name: test-recipe
display_name: Test Recipe
model_id: test/model
runtime: invalid-runtime
spec:
  resources:
    gpu:
      count: 1
`
	err := os.WriteFile(recipeFile, []byte(invalidRecipe), 0644)
	require.NoError(t, err, "failed to create test recipe file")

	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{recipeFile})
	err = cmd.Execute()

	require.Error(t, err, "command should return error for invalid runtime")
	outputStr := output.String()
	assert.Contains(t, outputStr, "runtime", "output should mention runtime validation")
}

// TestValidateCommand_EmptyFile verifies handling of empty file
func TestValidateCommand_EmptyFile(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	recipeFile := filepath.Join(tmpDir, "empty.yaml")

	// Create empty file
	err := os.WriteFile(recipeFile, []byte(""), 0644)
	require.NoError(t, err, "failed to create test file")

	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{recipeFile})
	err = cmd.Execute()

	require.Error(t, err, "command should return error for empty file")
}

// TestValidateCommand_YAMLOutput verifies YAML output format
func TestValidateCommand_YAMLOutput(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	recipeFile := filepath.Join(tmpDir, "recipe.yaml")

	// Create a valid recipe file
	validRecipe := `
name: test-recipe
display_name: Test Recipe
description: A test recipe
model_id: test/model
runtime: vllm
spec:
  resources:
    gpu:
      count: 1
`
	err := os.WriteFile(recipeFile, []byte(validRecipe), 0644)
	require.NoError(t, err, "failed to create test recipe file")

	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{recipeFile, "--format", "yaml"})
	err = cmd.Execute()

	require.NoError(t, err, "command should execute successfully with YAML format")

	// Verify output is valid YAML
	var result map[string]interface{}
	err = yaml.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err, "output should be valid YAML")

	// Verify expected fields are present
	assert.Contains(t, result, "valid", "YAML should contain 'valid' field")
	assert.Contains(t, result, "file", "YAML should contain 'file' field")
}

// TestValidateCommand_InvalidFormatFlag verifies error for invalid format flag
func TestValidateCommand_InvalidFormatFlag(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	recipeFile := filepath.Join(tmpDir, "recipe.yaml")

	// Create a valid recipe file
	validRecipe := `
name: test-recipe
display_name: Test Recipe
model_id: test/model
runtime: vllm
spec:
  resources:
    gpu:
      count: 1
`
	err := os.WriteFile(recipeFile, []byte(validRecipe), 0644)
	require.NoError(t, err, "failed to create test recipe file")

	cmd := NewValidateCommand()
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{recipeFile, "--format", "xml"})
	err = cmd.Execute()

	require.Error(t, err, "command should return error for invalid format")
	assert.Contains(t, err.Error(), "format", "error should mention invalid format")
}
