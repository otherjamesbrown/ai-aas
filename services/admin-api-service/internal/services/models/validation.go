// Package models provides the business logic for model management operations.
package models

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/google/uuid"
)

// KServe naming validation errors
var (
	ErrNameEmpty         = errors.New("name cannot be empty")
	ErrNameTooLong       = errors.New("name too long (max 253 characters)")
	ErrInvalidKServeName = errors.New("invalid kserve name format: must match [a-z]([-a-z0-9]*[a-z0-9])?")
)

// KServe naming regex: lowercase alphanumeric with hyphens, starting with letter
var kserveNameRegex = regexp.MustCompile(`^[a-z]([-a-z0-9]*[a-z0-9])?$`)

// ValidateKServeName checks if a name is valid for KServe resources
func ValidateKServeName(name string) error {
	if name == "" {
		return ErrNameEmpty
	}
	if len(name) > 253 {
		return ErrNameTooLong
	}
	if !kserveNameRegex.MatchString(name) {
		return ErrInvalidKServeName
	}
	return nil
}

// ValidationResult represents a single validation check result
type ValidationResult struct {
	ID             uuid.UUID
	ModelID        uuid.UUID
	Environment    string
	ValidationType string
	CheckName      string
	Status         string
	Message        string
	Remediation    string
	ValidatedAt    time.Time
}

// Validation status constants
const (
	ValidationStatusPass = "pass"
	ValidationStatusWarn = "warn"
	ValidationStatusFail = "fail"
	ValidationStatusSkip = "skip"
)

// Validation type constants
const (
	ValidationTypeRegistry   = "registry"
	ValidationTypeCache      = "cache"
	ValidationTypeDeployment = "deployment"
	ValidationTypeEndpoint   = "endpoint"
	ValidationTypeRouter     = "router"
)

// ValidateModelRequest contains options for model validation
type ValidateModelRequest struct {
	ModelName   string
	Environment string
	Layers      []string // Optional: specific layers to validate
}

// ValidateModel runs all validation checks for a model
func (s *Service) ValidateModel(ctx context.Context, req ValidateModelRequest) ([]ValidationResult, error) {
	// Get the model first
	model, err := s.GetModel(ctx, req.ModelName)
	if err != nil {
		return nil, err
	}

	var results []ValidationResult
	now := time.Now()

	// Determine which layers to validate
	layers := req.Layers
	if len(layers) == 0 {
		layers = []string{
			ValidationTypeRegistry,
			ValidationTypeCache,
			ValidationTypeDeployment,
			ValidationTypeEndpoint,
			ValidationTypeRouter,
		}
	}

	for _, layer := range layers {
		switch layer {
		case ValidationTypeRegistry:
			results = append(results, s.validateRegistry(model, now)...)
		case ValidationTypeCache:
			cacheResults, err := s.validateCache(ctx, model, now)
			if err != nil {
				results = append(results, ValidationResult{
					ModelID:        model.ID,
					ValidationType: ValidationTypeCache,
					CheckName:      "cache_query",
					Status:         ValidationStatusFail,
					Message:        fmt.Sprintf("Failed to query cache: %v", err),
					ValidatedAt:    now,
				})
			} else {
				results = append(results, cacheResults...)
			}
		case ValidationTypeDeployment:
			if req.Environment != "" {
				deployResults, err := s.validateDeployment(ctx, model, req.Environment, now)
				if err != nil {
					results = append(results, ValidationResult{
						ModelID:        model.ID,
						Environment:    req.Environment,
						ValidationType: ValidationTypeDeployment,
						CheckName:      "deployment_query",
						Status:         ValidationStatusFail,
						Message:        fmt.Sprintf("Failed to query deployment: %v", err),
						ValidatedAt:    now,
					})
				} else {
					results = append(results, deployResults...)
				}
			} else {
				results = append(results, ValidationResult{
					ModelID:        model.ID,
					ValidationType: ValidationTypeDeployment,
					CheckName:      "deployment_check",
					Status:         ValidationStatusSkip,
					Message:        "No environment specified for deployment validation",
					ValidatedAt:    now,
				})
			}
		case ValidationTypeEndpoint:
			results = append(results, s.validateEndpoint(model, req.Environment, now)...)
		case ValidationTypeRouter:
			results = append(results, s.validateRouter(model, req.Environment, now)...)
		}
	}

	// Store validation results
	for i := range results {
		results[i].ID = uuid.New()
		err := s.storeValidationResult(ctx, results[i])
		if err != nil {
			// Log but don't fail the validation
			continue
		}
	}

	return results, nil
}

// validateRegistry checks registry-level validations
func (s *Service) validateRegistry(model *Model, now time.Time) []ValidationResult {
	var results []ValidationResult

	// Check HF model ID format
	if model.HFModelID == "" {
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			ValidationType: ValidationTypeRegistry,
			CheckName:      "hf_model_id",
			Status:         ValidationStatusFail,
			Message:        "HuggingFace model ID is not set",
			Remediation:    "Set the hf_model_id field with a valid HuggingFace model path",
			ValidatedAt:    now,
		})
	} else {
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			ValidationType: ValidationTypeRegistry,
			CheckName:      "hf_model_id",
			Status:         ValidationStatusPass,
			Message:        fmt.Sprintf("HuggingFace model ID is set: %s", model.HFModelID),
			ValidatedAt:    now,
		})
	}

	// Check gated model license acceptance
	if model.IsGated {
		if model.LicenseAcceptedAt == nil {
			results = append(results, ValidationResult{
				ModelID:        model.ID,
				ValidationType: ValidationTypeRegistry,
				CheckName:      "license_acceptance",
				Status:         ValidationStatusFail,
				Message:        "Model is gated but license has not been accepted",
				Remediation:    "Accept the model license using: ai-aas-cli model license accept <model>",
				ValidatedAt:    now,
			})
		} else {
			results = append(results, ValidationResult{
				ModelID:        model.ID,
				ValidationType: ValidationTypeRegistry,
				CheckName:      "license_acceptance",
				Status:         ValidationStatusPass,
				Message:        fmt.Sprintf("License accepted at %s", model.LicenseAcceptedAt.Format(time.RFC3339)),
				ValidatedAt:    now,
			})
		}
	}

	// Check resource recommendations
	if model.RecommendedGPUMemoryGB == nil {
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			ValidationType: ValidationTypeRegistry,
			CheckName:      "gpu_memory_recommendation",
			Status:         ValidationStatusWarn,
			Message:        "GPU memory recommendation is not set",
			Remediation:    "Set recommended_gpu_memory_gb for better deployment planning",
			ValidatedAt:    now,
		})
	}

	return results
}

// validateCache checks cache-level validations
func (s *Service) validateCache(ctx context.Context, model *Model, now time.Time) ([]ValidationResult, error) {
	var results []ValidationResult

	cacheEntries, err := s.GetModelCache(ctx, model.Name)
	if err != nil {
		return nil, err
	}

	if len(cacheEntries) == 0 {
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			ValidationType: ValidationTypeCache,
			CheckName:      "cache_exists",
			Status:         ValidationStatusWarn,
			Message:        "No cache entries found for this model",
			Remediation:    "Pull the model using: ai-aas-cli model pull <model>",
			ValidatedAt:    now,
		})
	} else {
		// Check for ready cache entries
		readyCount := 0
		for _, entry := range cacheEntries {
			if entry.Status == CacheStatusReady {
				readyCount++
			}
		}

		if readyCount == 0 {
			results = append(results, ValidationResult{
				ModelID:        model.ID,
				ValidationType: ValidationTypeCache,
				CheckName:      "cache_ready",
				Status:         ValidationStatusFail,
				Message:        fmt.Sprintf("Found %d cache entries but none are ready", len(cacheEntries)),
				Remediation:    "Check pull job status or retry the pull operation",
				ValidatedAt:    now,
			})
		} else {
			results = append(results, ValidationResult{
				ModelID:        model.ID,
				ValidationType: ValidationTypeCache,
				CheckName:      "cache_ready",
				Status:         ValidationStatusPass,
				Message:        fmt.Sprintf("Found %d ready cache entries", readyCount),
				ValidatedAt:    now,
			})
		}
	}

	return results, nil
}

// validateDeployment checks deployment-level validations
func (s *Service) validateDeployment(ctx context.Context, model *Model, environment string, now time.Time) ([]ValidationResult, error) {
	var results []ValidationResult

	deployment, err := s.GetDeployment(ctx, model.Name, environment)
	if err == ErrDeploymentNotFound {
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			Environment:    environment,
			ValidationType: ValidationTypeDeployment,
			CheckName:      "deployment_exists",
			Status:         ValidationStatusWarn,
			Message:        fmt.Sprintf("No deployment found for environment: %s", environment),
			Remediation:    fmt.Sprintf("Deploy the model using: ai-aas-cli model deploy <model> --env %s", environment),
			ValidatedAt:    now,
		})
		return results, nil
	}
	if err != nil {
		return nil, err
	}

	// Check deployment exists
	results = append(results, ValidationResult{
		ModelID:        model.ID,
		Environment:    environment,
		ValidationType: ValidationTypeDeployment,
		CheckName:      "deployment_exists",
		Status:         ValidationStatusPass,
		Message:        "Deployment exists",
		ValidatedAt:    now,
	})

	// Check deployment status
	switch deployment.Status {
	case DeploymentStatusReady:
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			Environment:    environment,
			ValidationType: ValidationTypeDeployment,
			CheckName:      "deployment_status",
			Status:         ValidationStatusPass,
			Message:        "Deployment is ready",
			ValidatedAt:    now,
		})
	case DeploymentStatusFailed:
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			Environment:    environment,
			ValidationType: ValidationTypeDeployment,
			CheckName:      "deployment_status",
			Status:         ValidationStatusFail,
			Message:        "Deployment has failed",
			Remediation:    "Check deployment logs and retry",
			ValidatedAt:    now,
		})
	case DeploymentStatusDisabled:
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			Environment:    environment,
			ValidationType: ValidationTypeDeployment,
			CheckName:      "deployment_status",
			Status:         ValidationStatusWarn,
			Message:        "Deployment is disabled",
			Remediation:    fmt.Sprintf("Enable using: ai-aas-cli model enable <model> --env %s", environment),
			ValidatedAt:    now,
		})
	default:
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			Environment:    environment,
			ValidationType: ValidationTypeDeployment,
			CheckName:      "deployment_status",
			Status:         ValidationStatusWarn,
			Message:        fmt.Sprintf("Deployment status: %s", deployment.Status),
			ValidatedAt:    now,
		})
	}

	// Check replica readiness
	if deployment.ReplicasReady < deployment.ReplicasDesired {
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			Environment:    environment,
			ValidationType: ValidationTypeDeployment,
			CheckName:      "replicas_ready",
			Status:         ValidationStatusWarn,
			Message:        fmt.Sprintf("Only %d/%d replicas ready", deployment.ReplicasReady, deployment.ReplicasDesired),
			Remediation:    "Check pod status and resource availability",
			ValidatedAt:    now,
		})
	} else if deployment.ReplicasReady > 0 {
		results = append(results, ValidationResult{
			ModelID:        model.ID,
			Environment:    environment,
			ValidationType: ValidationTypeDeployment,
			CheckName:      "replicas_ready",
			Status:         ValidationStatusPass,
			Message:        fmt.Sprintf("All %d replicas ready", deployment.ReplicasReady),
			ValidatedAt:    now,
		})
	}

	return results, nil
}

// validateEndpoint checks endpoint-level validations (placeholder)
func (s *Service) validateEndpoint(model *Model, environment string, now time.Time) []ValidationResult {
	// This would require actual HTTP checks to the inference endpoint
	// For now, return a skip status
	return []ValidationResult{{
		ModelID:        model.ID,
		Environment:    environment,
		ValidationType: ValidationTypeEndpoint,
		CheckName:      "endpoint_health",
		Status:         ValidationStatusSkip,
		Message:        "Endpoint health check requires external connectivity",
		ValidatedAt:    now,
	}}
}

// validateRouter checks router-level validations (placeholder)
func (s *Service) validateRouter(model *Model, environment string, now time.Time) []ValidationResult {
	// This would require checking the API router configuration
	// For now, return a skip status
	return []ValidationResult{{
		ModelID:        model.ID,
		Environment:    environment,
		ValidationType: ValidationTypeRouter,
		CheckName:      "router_registration",
		Status:         ValidationStatusSkip,
		Message:        "Router registration check requires API router access",
		ValidatedAt:    now,
	}}
}

// storeValidationResult saves a validation result to the database
func (s *Service) storeValidationResult(ctx context.Context, result ValidationResult) error {
	query := `
		INSERT INTO model_validations (
			id, model_id, environment, validation_type, check_name,
			status, message, remediation, validated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := s.pool.Exec(ctx, query,
		result.ID, result.ModelID, result.Environment, result.ValidationType,
		result.CheckName, result.Status, result.Message, result.Remediation,
		result.ValidatedAt,
	)
	if err != nil {
		return fmt.Errorf("store validation result: %w", err)
	}

	return nil
}

// GetValidationHistory returns recent validation results for a model
func (s *Service) GetValidationHistory(ctx context.Context, modelName string, limit int) ([]ValidationResult, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT v.id, v.model_id, v.environment, v.validation_type, v.check_name,
		       v.status, v.message, v.remediation, v.validated_at
		FROM model_validations v
		JOIN model_registry r ON v.model_id = r.id
		WHERE r.name = $1
		ORDER BY v.validated_at DESC
		LIMIT $2
	`

	rows, err := s.pool.Query(ctx, query, modelName, limit)
	if err != nil {
		return nil, fmt.Errorf("query validation history: %w", err)
	}
	defer rows.Close()

	var results []ValidationResult
	for rows.Next() {
		var r ValidationResult
		var env, remediation *string
		err := rows.Scan(
			&r.ID, &r.ModelID, &env, &r.ValidationType, &r.CheckName,
			&r.Status, &r.Message, &remediation, &r.ValidatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan validation result: %w", err)
		}
		if env != nil {
			r.Environment = *env
		}
		if remediation != nil {
			r.Remediation = *remediation
		}
		results = append(results, r)
	}

	return results, rows.Err()
}
