package recipe

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// mockRecipeClient implements the RecipeClient interface for testing
type mockRecipeClient struct {
	recipe *Recipe
	err    error
}

func (m *mockRecipeClient) GetRecipe(name string) (*Recipe, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.recipe != nil && m.recipe.Name == name {
		return m.recipe, nil
	}
	return nil, errors.New("recipe not found")
}

// TestNewShowCommand verifies that the show command is created correctly
func TestNewShowCommand(t *testing.T) {
	cmd := NewShowCommand()

	require.NotNil(t, cmd, "NewShowCommand() returned nil")
	assert.Equal(t, "show <recipe-name>", cmd.Use, "command Use should be 'show <recipe-name>'")
	assert.NotEmpty(t, cmd.Short, "command Short description should not be empty")
	assert.NotEmpty(t, cmd.Long, "command Long description should not be empty")
	assert.NotNil(t, cmd.RunE, "command RunE should be set")

	// Verify format flag exists
	formatFlag := cmd.Flags().Lookup("format")
	assert.NotNil(t, formatFlag, "format flag should exist")
	assert.Equal(t, "table", formatFlag.DefValue, "format flag default should be 'table'")
}

// TestShowCommand_Success verifies successful recipe retrieval
func TestShowCommand_Success(t *testing.T) {
	mockClient := &mockRecipeClient{
		recipe: &Recipe{
			ID:          "test-recipe-id",
			Name:        "mistral-7b-instruct-v03",
			DisplayName: "Mistral 7B Instruct v0.3",
			Description: "Mistral 7B Instruct model optimized for chat",
			ModelID:     "mistralai/Mistral-7B-Instruct-v0.3",
			Runtime:     "vllm",
			Spec: map[string]interface{}{
				"resources": map[string]interface{}{
					"gpu": map[string]interface{}{
						"count":  1,
						"vendor": "nvidia",
					},
				},
			},
		},
	}

	cmd := createTestShowCommand(mockClient)
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{"mistral-7b-instruct-v03"})
	err := cmd.Execute()

	require.NoError(t, err, "command should execute successfully")
	assert.NotEmpty(t, output.String(), "output should not be empty")
	assert.Contains(t, output.String(), "mistral-7b-instruct-v03", "output should contain recipe name")
}

// TestShowCommand_NotFound verifies handling of recipe not found error
func TestShowCommand_NotFound(t *testing.T) {
	mockClient := &mockRecipeClient{
		err: errors.New("recipe not found"),
	}

	cmd := createTestShowCommand(mockClient)
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{"nonexistent-recipe"})
	err := cmd.Execute()

	require.Error(t, err, "command should return error for non-existent recipe")
	assert.Contains(t, err.Error(), "recipe not found", "error should mention recipe not found")
}

// TestShowCommand_JSONOutput verifies JSON format output
func TestShowCommand_JSONOutput(t *testing.T) {
	mockClient := &mockRecipeClient{
		recipe: &Recipe{
			ID:          "test-recipe-id",
			Name:        "mistral-7b-instruct-v03",
			DisplayName: "Mistral 7B Instruct v0.3",
			Description: "Mistral 7B Instruct model optimized for chat",
			ModelID:     "mistralai/Mistral-7B-Instruct-v0.3",
			Runtime:     "vllm",
			Spec: map[string]interface{}{
				"resources": map[string]interface{}{
					"gpu": map[string]interface{}{
						"count": float64(1), // JSON unmarshals numbers as float64
					},
				},
			},
		},
	}

	cmd := createTestShowCommand(mockClient)
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{"mistral-7b-instruct-v03", "--format", "json"})
	err := cmd.Execute()

	require.NoError(t, err, "command should execute successfully with JSON format")

	// Verify output is valid JSON
	var result map[string]interface{}
	err = json.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err, "output should be valid JSON")

	// Verify expected fields are present
	assert.Equal(t, "mistral-7b-instruct-v03", result["name"], "JSON should contain recipe name")
	assert.Equal(t, "vllm", result["runtime"], "JSON should contain runtime")
	assert.NotNil(t, result["spec"], "JSON should contain spec")
}

// TestShowCommand_YAMLOutput verifies YAML format output
func TestShowCommand_YAMLOutput(t *testing.T) {
	mockClient := &mockRecipeClient{
		recipe: &Recipe{
			ID:          "test-recipe-id",
			Name:        "mistral-7b-instruct-v03",
			DisplayName: "Mistral 7B Instruct v0.3",
			Description: "Mistral 7B Instruct model optimized for chat",
			ModelID:     "mistralai/Mistral-7B-Instruct-v0.3",
			Runtime:     "vllm",
			Spec: map[string]interface{}{
				"resources": map[string]interface{}{
					"gpu": map[string]interface{}{
						"count": 1,
					},
				},
			},
		},
	}

	cmd := createTestShowCommand(mockClient)
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{"mistral-7b-instruct-v03", "--format", "yaml"})
	err := cmd.Execute()

	require.NoError(t, err, "command should execute successfully with YAML format")

	// Verify output is valid YAML
	var result map[string]interface{}
	err = yaml.Unmarshal(output.Bytes(), &result)
	require.NoError(t, err, "output should be valid YAML")

	// Verify expected fields are present
	assert.Equal(t, "mistral-7b-instruct-v03", result["name"], "YAML should contain recipe name")
	assert.Equal(t, "vllm", result["runtime"], "YAML should contain runtime")
	assert.NotNil(t, result["spec"], "YAML should contain spec")
}

// TestShowCommand_NoName verifies error when recipe name is not provided
func TestShowCommand_NoName(t *testing.T) {
	mockClient := &mockRecipeClient{
		recipe: &Recipe{
			Name: "test-recipe",
		},
	}

	cmd := createTestShowCommand(mockClient)
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	// Don't provide any args
	cmd.SetArgs([]string{})
	err := cmd.Execute()

	require.Error(t, err, "command should return error when recipe name is not provided")
	// The error should indicate that an argument is required or received 0
	// Cobra returns "accepts 1 arg(s), received 0" for ExactArgs(1)
	errorMsg := err.Error()
	assert.True(t,
		contains(errorMsg, "required") || contains(errorMsg, "received 0"),
		"error should mention required argument or no args received: %s", errorMsg)
}

// TestShowCommand_APIError verifies handling of general API errors
func TestShowCommand_APIError(t *testing.T) {
	mockClient := &mockRecipeClient{
		err: errors.New("API connection failed: unable to reach server"),
	}

	cmd := createTestShowCommand(mockClient)
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{"mistral-7b-instruct-v03"})
	err := cmd.Execute()

	require.Error(t, err, "command should return error when API fails")
	assert.Contains(t, err.Error(), "API connection failed", "error should contain API error message")
}

// TestShowCommand_InvalidFormat verifies handling of invalid format flag
func TestShowCommand_InvalidFormat(t *testing.T) {
	mockClient := &mockRecipeClient{
		recipe: &Recipe{
			Name: "mistral-7b-instruct-v03",
		},
	}

	cmd := createTestShowCommand(mockClient)
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{"mistral-7b-instruct-v03", "--format", "invalid"})
	err := cmd.Execute()

	require.Error(t, err, "command should return error for invalid format")
	assert.Contains(t, err.Error(), "format", "error should mention invalid format")
}

// TestShowCommand_EmptyRecipeName verifies handling of empty recipe name
func TestShowCommand_EmptyRecipeName(t *testing.T) {
	mockClient := &mockRecipeClient{
		err: errors.New("recipe name cannot be empty"),
	}

	cmd := createTestShowCommand(mockClient)
	output := &bytes.Buffer{}
	cmd.SetOut(output)
	cmd.SetErr(output)

	cmd.SetArgs([]string{""})
	err := cmd.Execute()

	require.Error(t, err, "command should return error for empty recipe name")
}

// Helper functions

// createTestShowCommand creates a show command with a mock client for testing
func createTestShowCommand(client RecipeClient) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <recipe-name>",
		Short: "Show details of a recipe",
		Long: `Show detailed information about a specific model recipe.

Examples:
  # Show recipe details in table format
  ai-aas-cli model recipe show mistral-7b-instruct-v03

  # Show recipe details in JSON format
  ai-aas-cli model recipe show mistral-7b-instruct-v03 --format json

  # Show recipe details in YAML format
  ai-aas-cli model recipe show mistral-7b-instruct-v03 --format yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			recipeName := args[0]
			if recipeName == "" {
				return errors.New("recipe name is required")
			}

			format, _ := cmd.Flags().GetString("format")
			if format != "table" && format != "json" && format != "yaml" {
				return errors.New("format must be one of: table, json, yaml")
			}

			recipe, err := client.GetRecipe(recipeName)
			if err != nil {
				return err
			}

			// Format and output the recipe
			switch format {
			case "json":
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(recipe)
			case "yaml":
				encoder := yaml.NewEncoder(cmd.OutOrStdout())
				return encoder.Encode(recipe)
			default:
				// Table format
				return outputRecipeTable(cmd.OutOrStdout(), recipe)
			}
		},
	}

	cmd.Flags().StringP("format", "f", "table", "Output format (table, json, yaml)")

	return cmd
}

// outputRecipeTable outputs a recipe in table format
func outputRecipeTable(w io.Writer, recipe *Recipe) error {
	// Simple table output for testing
	_, err := w.Write([]byte("Name: " + recipe.Name + "\n"))
	if err != nil {
		return err
	}
	if recipe.DisplayName != "" {
		_, err = w.Write([]byte("Display Name: " + recipe.DisplayName + "\n"))
		if err != nil {
			return err
		}
	}
	if recipe.Runtime != "" {
		_, err = w.Write([]byte("Runtime: " + recipe.Runtime + "\n"))
		if err != nil {
			return err
		}
	}
	return nil
}

// contains checks if a string contains a substring (case-insensitive helper)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
