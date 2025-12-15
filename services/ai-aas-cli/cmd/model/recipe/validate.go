package recipe

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// RecipeFile represents a recipe YAML file for validation
// This is different from the Recipe type which represents API responses
type RecipeFile struct {
	Name        string                 `yaml:"name" json:"name"`
	DisplayName string                 `yaml:"display_name" json:"display_name,omitempty"`
	Description string                 `yaml:"description" json:"description,omitempty"`
	ModelID     string                 `yaml:"model_id" json:"model_id"`
	Runtime     string                 `yaml:"runtime" json:"runtime"`
	Spec        map[string]interface{} `yaml:"spec" json:"spec"`
}

// ValidationResult represents the result of a validation
type ValidationResult struct {
	Valid  bool     `json:"valid" yaml:"valid"`
	File   string   `json:"file" yaml:"file"`
	Errors []string `json:"errors,omitempty" yaml:"errors,omitempty"`
}

// NewValidateCommand creates the recipe validate command
func NewValidateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate <file>",
		Short: "Validate a recipe YAML file",
		Long: `Validate a model recipe YAML file for correctness.

The validation checks:
- YAML syntax is correct
- Required fields are present (name, model_id, runtime, spec)
- Runtime value is valid (vllm, triton, tgi)
- Spec structure is valid

Examples:
  # Validate a recipe file
  ai-aas-cli model recipe validate recipe.yaml

  # Validate and output results as JSON
  ai-aas-cli model recipe validate recipe.yaml --format json

  # Validate and output results as YAML
  ai-aas-cli model recipe validate recipe.yaml --format yaml`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filePath := args[0]
			format, _ := cmd.Flags().GetString("format")

			// Validate format flag
			if format != "text" && format != "json" && format != "yaml" {
				return fmt.Errorf("invalid format: must be one of: text, json, yaml")
			}

			// Read file
			data, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}

			// Parse YAML
			var recipe RecipeFile
			if err := yaml.Unmarshal(data, &recipe); err != nil {
				return fmt.Errorf("failed to parse YAML: %w", err)
			}

			// Validate recipe
			result := validateRecipe(&recipe, filePath)

			// Output result
			if err := outputResult(cmd, result, format); err != nil {
				return err
			}

			// Return error if validation failed
			if !result.Valid {
				return fmt.Errorf("recipe validation failed")
			}

			return nil
		},
	}

	cmd.Flags().StringP("format", "f", "text", "Output format (text, json, yaml)")

	return cmd
}

// validateRecipe performs validation on a recipe
func validateRecipe(recipe *RecipeFile, filePath string) ValidationResult {
	result := ValidationResult{
		Valid:  true,
		File:   filePath,
		Errors: []string{},
	}

	// Check required fields
	if recipe.Name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "name: required field is missing")
	}

	if recipe.ModelID == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "model_id: required field is missing")
	}

	if recipe.Runtime == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "runtime: required field is missing")
	} else {
		// Validate runtime value
		validRuntimes := []string{"vllm", "triton", "tgi"}
		if !containsString(validRuntimes, recipe.Runtime) {
			result.Valid = false
			result.Errors = append(result.Errors, "runtime: must be one of: vllm, triton, tgi")
		}
	}

	if recipe.Spec == nil || len(recipe.Spec) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "spec: required field is missing")
	}

	return result
}

// containsString checks if a slice contains a string
func containsString(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}

// outputResult outputs the validation result in the requested format
func outputResult(cmd *cobra.Command, result ValidationResult, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		cmd.Println(string(data))

	case "yaml":
		data, err := yaml.Marshal(result)
		if err != nil {
			return fmt.Errorf("failed to marshal YAML: %w", err)
		}
		cmd.Print(string(data))

	case "text":
		if result.Valid {
			// Extract recipe name for output
			recipeName := result.File
			// Try to extract name from errors if available
			if len(result.Errors) == 0 {
				// Read the file to get the recipe name
				data, err := os.ReadFile(result.File)
				if err == nil {
					var recipe RecipeFile
					if err := yaml.Unmarshal(data, &recipe); err == nil && recipe.Name != "" {
						recipeName = recipe.Name
					}
				}
			}
			cmd.Printf("✓ Recipe '%s' is valid\n", recipeName)
		} else {
			cmd.Println("✗ Recipe validation failed:")
			for _, err := range result.Errors {
				cmd.Printf("  - %s\n", err)
			}
		}

	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	return nil
}
