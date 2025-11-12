// Package contracts provides contract generation from OpenAPI specifications.
//
// Purpose:
//   This package defines workflows for generating Go types and validation code
//   from OpenAPI specifications using oapi-codegen. It provides validation and
//   code generation capabilities for the API Router Service contracts.
//
// Dependencies:
//   - oapi-codegen: For generating Go types from OpenAPI specs (install via: go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest)
//   - spectral: Optional OpenAPI linting tool (install via: npm install -g @stoplight/spectral-cli)
//
// Key Responsibilities:
//   - Validate OpenAPI contracts
//   - Generate Go types from OpenAPI schemas
//   - Provide Makefile targets for contract generation
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#FR-002 (Request schema validation)
//
package contracts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GenerateOptions configures contract generation.
type GenerateOptions struct {
	OpenAPISpecPath string
	OutputPath      string
	PackageName     string
	GenerateTypes   bool // Generate type definitions
	GenerateServer  bool // Generate server code
	GenerateClient  bool // Generate client code
}

// GenerateGoTypes generates Go types from the OpenAPI specification using oapi-codegen.
func GenerateGoTypes(opts GenerateOptions) error {
	if opts.OpenAPISpecPath == "" {
		opts.OpenAPISpecPath = GetOpenAPISpecPath()
	}
	if opts.OutputPath == "" {
		opts.OutputPath = filepath.Join(filepath.Dir(opts.OpenAPISpecPath), "..", "..", "services", "api-router-service", "pkg", "contracts", "generated.go")
	}
	if opts.PackageName == "" {
		opts.PackageName = "contracts"
	}

	// Validate spec exists
	if _, err := os.Stat(opts.OpenAPISpecPath); os.IsNotExist(err) {
		return fmt.Errorf("OpenAPI spec not found: %s", opts.OpenAPISpecPath)
	}

	// Check if oapi-codegen is available
	oapiCodegenPath, err := exec.LookPath("oapi-codegen")
	if err != nil {
		return fmt.Errorf("oapi-codegen not found. Install with: go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest")
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(opts.OutputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Build oapi-codegen command
	args := []string{
		"-package", opts.PackageName,
		"-o", opts.OutputPath,
	}

	// Add generation flags
	if opts.GenerateTypes {
		args = append(args, "-generate", "types")
	}
	if opts.GenerateServer {
		args = append(args, "-generate", "server")
	}
	if opts.GenerateClient {
		args = append(args, "-generate", "client")
	}

	// If no generation flags specified, default to types
	if !opts.GenerateTypes && !opts.GenerateServer && !opts.GenerateClient {
		args = append(args, "-generate", "types")
	}

	args = append(args, opts.OpenAPISpecPath)

	// Run oapi-codegen
	cmd := exec.Cmd{
		Path:   oapiCodegenPath,
		Args:   append([]string{"oapi-codegen"}, args...),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("oapi-codegen failed: %w", err)
	}

	return nil
}

// ValidateOpenAPI validates the OpenAPI specification.
// Uses spectral if available, otherwise performs basic JSON/YAML validation.
func ValidateOpenAPI(specPath string) error {
	// Check if spec exists
	if _, err := os.Stat(specPath); os.IsNotExist(err) {
		return fmt.Errorf("OpenAPI spec not found: %s", specPath)
	}

	// Try spectral first (more comprehensive validation)
	if spectralPath, err := exec.LookPath("spectral"); err == nil {
		return validateWithSpectral(spectralPath, specPath)
	}

	// Fallback: basic YAML/JSON validation
	return validateBasic(specPath)
}

// validateWithSpectral validates using Spectral CLI.
func validateWithSpectral(spectralPath, specPath string) error {
	cmd := exec.Cmd{
		Path:   spectralPath,
		Args:   []string{"spectral", "lint", specPath},
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("spectral validation failed: %w", err)
	}

	return nil
}

// validateBasic performs basic YAML/JSON validation.
func validateBasic(specPath string) error {
	data, err := os.ReadFile(specPath)
	if err != nil {
		return fmt.Errorf("failed to read spec file: %w", err)
	}

	// Try to parse as JSON first
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		return nil // Valid JSON
	}

	// Try to parse as YAML (basic check - just verify it's not completely malformed)
	// For a more robust YAML check, we'd need a YAML parser, but this is a reasonable fallback
	if strings.Contains(string(data), "openapi:") || strings.Contains(string(data), "swagger:") {
		return nil // Looks like valid OpenAPI YAML
	}

	return fmt.Errorf("spec file does not appear to be valid JSON or OpenAPI YAML")
}

// GetOpenAPISpecPath returns the path to the OpenAPI specification.
func GetOpenAPISpecPath() string {
	// Get the project root (assuming we're in services/api-router-service/pkg/contracts)
	// Try multiple possible locations
	possiblePaths := []string{
		// From pkg/contracts
		filepath.Join("..", "..", "..", "..", "specs", "006-api-router-service", "contracts", "api-router.openapi.yaml"),
		// From service root
		filepath.Join("..", "..", "specs", "006-api-router-service", "contracts", "api-router.openapi.yaml"),
		// Absolute from workspace root
		filepath.Join("specs", "006-api-router-service", "contracts", "api-router.openapi.yaml"),
	}

	for _, path := range possiblePaths {
		if absPath, err := filepath.Abs(path); err == nil {
			if _, err := os.Stat(absPath); err == nil {
				return absPath
			}
		}
	}

	// Default fallback
	return filepath.Join("specs", "006-api-router-service", "contracts", "api-router.openapi.yaml")
}

