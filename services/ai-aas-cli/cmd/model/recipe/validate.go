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
// Supports both simple format and Kubernetes CRD format
type RecipeFile struct {
	// Simple format fields (top-level)
	Name        string                 `yaml:"name,omitempty" json:"name,omitempty"`
	DisplayName string                 `yaml:"display_name,omitempty" json:"display_name,omitempty"`
	Description string                 `yaml:"description,omitempty" json:"description,omitempty"`
	ModelID     string                 `yaml:"model_id,omitempty" json:"model_id,omitempty"`
	Runtime     string                 `yaml:"runtime,omitempty" json:"runtime,omitempty"`
	Spec        map[string]interface{} `yaml:"spec,omitempty" json:"spec,omitempty"`

	// Kubernetes CRD format fields
	APIVersion string                 `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
	Kind       string                 `yaml:"kind,omitempty" json:"kind,omitempty"`
	Metadata   map[string]interface{} `yaml:"metadata,omitempty" json:"metadata,omitempty"`
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

	// Determine if this is a Kubernetes CRD format or simple format
	isKubernetesCRD := recipe.APIVersion != "" && recipe.Kind != ""

	var name, modelID, runtime string
	var spec map[string]interface{}

	if isKubernetesCRD {
		// Extract fields from Kubernetes CRD format
		if recipe.Metadata != nil {
			if n, ok := recipe.Metadata["name"].(string); ok {
				name = n
			}
		}

		if recipe.Spec != nil {
			// Extract modelID from spec
			if mid, ok := recipe.Spec["modelID"].(string); ok {
				modelID = mid
			}
			// Extract runtime from spec
			if rt, ok := recipe.Spec["runtime"].(string); ok {
				runtime = rt
			}
			spec = recipe.Spec
		}
	} else {
		// Use top-level fields from simple format
		name = recipe.Name
		modelID = recipe.ModelID
		runtime = recipe.Runtime
		spec = recipe.Spec
	}

	// Check required fields
	if name == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "name: required field is missing")
	}

	if modelID == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "model_id: required field is missing")
	}

	if runtime == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "runtime: required field is missing")
	} else {
		// Validate runtime value
		validRuntimes := []string{"vllm", "triton", "tgi"}
		if !containsString(validRuntimes, runtime) {
			result.Valid = false
			result.Errors = append(result.Errors, "runtime: must be one of: vllm, triton, tgi")
		}
	}

	if spec == nil || len(spec) == 0 {
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
			// Try to extract name from the file
			if len(result.Errors) == 0 {
				// Read the file to get the recipe name
				data, err := os.ReadFile(result.File)
				if err == nil {
					var recipe RecipeFile
					if err := yaml.Unmarshal(data, &recipe); err == nil {
						// Try simple format first
						if recipe.Name != "" {
							recipeName = recipe.Name
						} else if recipe.Metadata != nil {
							// Try Kubernetes CRD format
							if n, ok := recipe.Metadata["name"].(string); ok && n != "" {
								recipeName = n
							}
						}
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
