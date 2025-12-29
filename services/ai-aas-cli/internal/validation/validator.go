// Package validation provides model validation functionality.
package validation

import (
	"context"
	"fmt"
	"time"
)

// ValidationResult represents the result of a validation check
type ValidationResult struct {
	Name       string        `json:"name"`
	Passed     bool          `json:"passed"`
	Message    string        `json:"message,omitempty"`
	Details    string        `json:"details,omitempty"`
	Duration   time.Duration `json:"duration"`
	Severity   Severity      `json:"severity"`
	Skipped    bool          `json:"skipped,omitempty"`
	SkipReason string        `json:"skip_reason,omitempty"`
}

// Severity indicates the severity of a validation failure
type Severity string

const (
	SeverityCritical Severity = "critical"
	SeverityWarning  Severity = "warning"
	SeverityInfo     Severity = "info"
)

// Check is a validation check function
type Check func(ctx context.Context, model *ModelInfo) *ValidationResult

// ModelInfo contains information about the model being validated
type ModelInfo struct {
	ModelID    string
	HFRevision string
	StorageURI string
	LocalPath  string
	GPUCount   int
	MemoryGB   int
}

// Validator runs validation checks on a model
type Validator struct {
	checks  []Check
	timeout time.Duration
}

// NewValidator creates a new validator with default checks
func NewValidator() *Validator {
	v := &Validator{
		timeout: 90 * time.Second, // Increased from 30s for GPU model validation
	}

	// Register default checks
	v.checks = []Check{
		checkModelID,
		checkRequiredFiles,
		checkConfigFormat,
		checkTokenizerConfig,
		checkModelSize,
	}

	return v
}

// AddCheck adds a custom validation check
func (v *Validator) AddCheck(check Check) {
	v.checks = append(v.checks, check)
}

// SetTimeout sets the validation timeout
func (v *Validator) SetTimeout(timeout time.Duration) {
	v.timeout = timeout
}

// ValidationReport contains the full validation report
type ValidationReport struct {
	ModelID   string             `json:"model_id"`
	Timestamp time.Time          `json:"timestamp"`
	Duration  time.Duration      `json:"duration"`
	Passed    bool               `json:"passed"`
	Results   []ValidationResult `json:"results"`
	Summary   ValidationSummary  `json:"summary"`
}

// ValidationSummary summarizes validation results
type ValidationSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Failed   int `json:"failed"`
	Warnings int `json:"warnings"`
	Skipped  int `json:"skipped"`
	Critical int `json:"critical"`
}

// Validate runs all validation checks
func (v *Validator) Validate(ctx context.Context, model *ModelInfo) (*ValidationReport, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(ctx, v.timeout)
	defer cancel()

	report := &ValidationReport{
		ModelID:   model.ModelID,
		Timestamp: time.Now().UTC(),
		Passed:    true,
	}

	for _, check := range v.checks {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("validation timeout exceeded")
		default:
		}

		result := check(ctx, model)
		report.Results = append(report.Results, *result)

		// Update summary
		report.Summary.Total++
		if result.Skipped {
			report.Summary.Skipped++
		} else if result.Passed {
			report.Summary.Passed++
		} else {
			report.Summary.Failed++
			if result.Severity == SeverityCritical {
				report.Summary.Critical++
				report.Passed = false
			} else if result.Severity == SeverityWarning {
				report.Summary.Warnings++
			}
		}
	}

	report.Duration = time.Since(start)
	return report, nil
}

// Default validation checks

func checkModelID(ctx context.Context, model *ModelInfo) *ValidationResult {
	start := time.Now()
	result := &ValidationResult{
		Name:     "model_id_format",
		Severity: SeverityCritical,
	}

	if model.ModelID == "" {
		result.Message = "Model ID is empty"
		result.Passed = false
	} else if len(model.ModelID) > 256 {
		result.Message = "Model ID exceeds maximum length"
		result.Passed = false
	} else {
		result.Passed = true
		result.Message = "Model ID format is valid"
	}

	result.Duration = time.Since(start)
	return result
}

func checkRequiredFiles(ctx context.Context, model *ModelInfo) *ValidationResult {
	start := time.Now()
	result := &ValidationResult{
		Name:     "required_files",
		Severity: SeverityCritical,
	}

	if model.LocalPath == "" {
		result.Skipped = true
		result.SkipReason = "No local path provided"
		result.Duration = time.Since(start)
		return result
	}

	// Check for required model files
	requiredFiles := []string{
		"config.json",
	}

	var missing []string
	for _, file := range requiredFiles {
		if !fileExists(model.LocalPath, file) {
			missing = append(missing, file)
		}
	}

	if len(missing) > 0 {
		result.Passed = false
		result.Message = fmt.Sprintf("Missing required files: %v", missing)
	} else {
		result.Passed = true
		result.Message = "All required files present"
	}

	result.Duration = time.Since(start)
	return result
}

func checkConfigFormat(ctx context.Context, model *ModelInfo) *ValidationResult {
	start := time.Now()
	result := &ValidationResult{
		Name:     "config_format",
		Severity: SeverityCritical,
	}

	if model.LocalPath == "" {
		result.Skipped = true
		result.SkipReason = "No local path provided"
		result.Duration = time.Since(start)
		return result
	}

	// Verify config.json is valid JSON
	valid, err := validateJSONFile(model.LocalPath, "config.json")
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Config validation error: %v", err)
	} else if !valid {
		result.Passed = false
		result.Message = "config.json is not valid JSON"
	} else {
		result.Passed = true
		result.Message = "config.json is valid"
	}

	result.Duration = time.Since(start)
	return result
}

func checkTokenizerConfig(ctx context.Context, model *ModelInfo) *ValidationResult {
	start := time.Now()
	result := &ValidationResult{
		Name:     "tokenizer_config",
		Severity: SeverityWarning,
	}

	if model.LocalPath == "" {
		result.Skipped = true
		result.SkipReason = "No local path provided"
		result.Duration = time.Since(start)
		return result
	}

	// Check for tokenizer files
	tokenizerFiles := []string{"tokenizer_config.json", "tokenizer.json", "tokenizer.model"}
	found := false
	for _, file := range tokenizerFiles {
		if fileExists(model.LocalPath, file) {
			found = true
			break
		}
	}

	if !found {
		result.Passed = false
		result.Message = "No tokenizer configuration found"
	} else {
		result.Passed = true
		result.Message = "Tokenizer configuration present"
	}

	result.Duration = time.Since(start)
	return result
}

func checkModelSize(ctx context.Context, model *ModelInfo) *ValidationResult {
	start := time.Now()
	result := &ValidationResult{
		Name:     "model_size",
		Severity: SeverityInfo,
	}

	if model.LocalPath == "" {
		result.Skipped = true
		result.SkipReason = "No local path provided"
		result.Duration = time.Since(start)
		return result
	}

	// Get total size of model files
	size, err := getDirSize(model.LocalPath)
	if err != nil {
		result.Passed = false
		result.Message = fmt.Sprintf("Could not calculate model size: %v", err)
	} else {
		result.Passed = true
		result.Message = fmt.Sprintf("Model size: %.2f GB", float64(size)/(1024*1024*1024))
		result.Details = fmt.Sprintf("%d bytes", size)
	}

	result.Duration = time.Since(start)
	return result
}
