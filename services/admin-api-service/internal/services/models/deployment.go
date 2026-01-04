// Package models provides the business logic for model management operations.
package models

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// ErrDeploymentNotFound indicates the requested deployment was not found
var ErrDeploymentNotFound = errors.New("deployment not found")

// Deployment represents a model deployment in an environment
type Deployment struct {
	ID                   uuid.UUID
	ModelID              uuid.UUID
	CacheID              *uuid.UUID
	Environment          string
	Namespace            string
	InferenceServiceName *string
	Endpoint             *string
	Enabled              bool
	Status               string
	StatusChangedAt      *time.Time
	ReplicasDesired      int
	ReplicasReady        int
	GPUCount             int
	MemoryGB             *int
	LastHealthCheckAt    *time.Time
	LastHealthStatus     *string
	LastEnabledAt        *time.Time
	LastEnabledBy        *string
	LastDisabledAt       *time.Time
	LastDisabledBy       *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// DeploymentStatus constants
const (
	DeploymentStatusPending    = "pending"
	DeploymentStatusDeploying  = "deploying"
	DeploymentStatusReady      = "ready"
	DeploymentStatusFailed     = "failed"
	DeploymentStatusDisabled   = "disabled"
	DeploymentStatusTerminated = "terminated"
)

// ListDeploymentsOptions configures deployment listing
type ListDeploymentsOptions struct {
	Environment string
	ModelName   string
	Status      string
	Enabled     *bool
}

// ListDeployments returns deployments matching the given options
func (s *Service) ListDeployments(ctx context.Context, opts ListDeploymentsOptions) ([]Deployment, error) {
	query := `
		SELECT d.id, d.model_id, d.cache_id, d.environment, d.namespace,
		       d.inferenceservice_name, d.endpoint, d.enabled, d.status,
		       d.status_changed_at,
		       d.replicas_desired, d.replicas_ready, d.gpu_count, d.memory_gb,
		       d.last_health_check_at, d.last_health_status,
		       d.last_enabled_at, d.last_enabled_by,
		       d.last_disabled_at, d.last_disabled_by,
		       d.created_at, d.updated_at
		FROM model_deployments d
		JOIN model_registry r ON d.model_id = r.id
		WHERE 1=1
	`
	args := []interface{}{}
	argNum := 1

	if opts.Environment != "" {
		query += fmt.Sprintf(" AND d.environment = $%d", argNum)
		args = append(args, opts.Environment)
		argNum++
	}

	if opts.ModelName != "" {
		query += fmt.Sprintf(" AND r.name = $%d", argNum)
		args = append(args, opts.ModelName)
		argNum++
	}

	if opts.Status != "" {
		query += fmt.Sprintf(" AND d.status = $%d", argNum)
		args = append(args, opts.Status)
		argNum++
	}

	if opts.Enabled != nil {
		query += fmt.Sprintf(" AND d.enabled = $%d", argNum)
		args = append(args, *opts.Enabled)
		argNum++
	}

	query += " ORDER BY r.name, d.environment"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query deployments: %w", err)
	}
	defer rows.Close()

	var deployments []Deployment
	for rows.Next() {
		var d Deployment
		err := rows.Scan(
			&d.ID, &d.ModelID, &d.CacheID, &d.Environment, &d.Namespace,
			&d.InferenceServiceName, &d.Endpoint, &d.Enabled, &d.Status,
			&d.StatusChangedAt,
			&d.ReplicasDesired, &d.ReplicasReady, &d.GPUCount, &d.MemoryGB,
			&d.LastHealthCheckAt, &d.LastHealthStatus,
			&d.LastEnabledAt, &d.LastEnabledBy,
			&d.LastDisabledAt, &d.LastDisabledBy,
			&d.CreatedAt, &d.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan deployment: %w", err)
		}
		deployments = append(deployments, d)
	}

	return deployments, rows.Err()
}

// GetDeployment returns a specific deployment by model name and environment
func (s *Service) GetDeployment(ctx context.Context, modelName, environment string) (*Deployment, error) {
	query := `
		SELECT d.id, d.model_id, d.cache_id, d.environment, d.namespace,
		       d.inferenceservice_name, d.endpoint, d.enabled, d.status,
		       d.status_changed_at,
		       d.replicas_desired, d.replicas_ready, d.gpu_count, d.memory_gb,
		       d.last_health_check_at, d.last_health_status,
		       d.last_enabled_at, d.last_enabled_by,
		       d.last_disabled_at, d.last_disabled_by,
		       d.created_at, d.updated_at
		FROM model_deployments d
		JOIN model_registry r ON d.model_id = r.id
		WHERE r.name = $1 AND d.environment = $2
	`

	var d Deployment
	err := s.pool.QueryRow(ctx, query, modelName, environment).Scan(
		&d.ID, &d.ModelID, &d.CacheID, &d.Environment, &d.Namespace,
		&d.InferenceServiceName, &d.Endpoint, &d.Enabled, &d.Status,
		&d.StatusChangedAt,
		&d.ReplicasDesired, &d.ReplicasReady, &d.GPUCount, &d.MemoryGB,
		&d.LastHealthCheckAt, &d.LastHealthStatus,
		&d.LastEnabledAt, &d.LastEnabledBy,
		&d.LastDisabledAt, &d.LastDisabledBy,
		&d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrDeploymentNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query deployment: %w", err)
	}

	return &d, nil
}

// CreateDeploymentRequest contains data for creating a deployment
type CreateDeploymentRequest struct {
	ModelName    string
	ModelID      string // Full model path (e.g., "unsloth/gpt-oss-20b")
	ExternalName string // Name exposed in OpenAI-compatible APIs
	CacheID      *uuid.UUID
	Environment  string
	Namespace    string
	GPUCount     int
	MemoryGB     int
	Replicas     int
	ModelType    string // Model type (text, vision-language, embedding, audio)
}

// CreateDeployment creates a new deployment record
func (s *Service) CreateDeployment(ctx context.Context, req CreateDeploymentRequest) (*Deployment, error) {
	// Get the model ID, or auto-register if it doesn't exist
	model, err := s.GetModel(ctx, req.ModelName)
	if err != nil {
		if errors.Is(err, ErrModelNotFound) {
			// Auto-register the model with minimal metadata
			// This allows AIModel CRs to be deployed via GitOps without manual registry steps
			hfModelID := req.ModelID
			if hfModelID == "" {
				hfModelID = req.ModelName // Fallback for backwards compatibility
			}
			autoRegReq := AddModelRequest{
				Name:         req.ModelName,
				HFModelID:    hfModelID,
				ExternalName: req.ExternalName,
				RequiresAuth: false,
				IsGated:      false,
				GPUMemoryGB:  0, // Will be determined from deployment resources
				CPUMemoryGB:  0,
				ModelType:    req.ModelType,
			}
			model, err = s.AddModel(ctx, autoRegReq)
			if err != nil {
				return nil, fmt.Errorf("auto-register model: %w", err)
			}
		} else {
			return nil, err
		}
	} else if req.ModelID != "" || req.ExternalName != "" || req.ModelType != "" {
		// Model exists - update hf_model_id, external_name, and model_type if provided
		if req.ModelID != "" && model.HFModelID != req.ModelID {
			err := s.updateModelHFModelID(ctx, model.ID, req.ModelID)
			if err != nil {
				return nil, fmt.Errorf("update model hf_model_id: %w", err)
			}
		}
		if req.ExternalName != "" && (model.ExternalName == nil || *model.ExternalName != req.ExternalName) {
			err := s.updateModelExternalName(ctx, model.ID, req.ExternalName)
			if err != nil {
				return nil, fmt.Errorf("update model external_name: %w", err)
			}
		}
		if req.ModelType != "" && (model.ModelType == nil || *model.ModelType != req.ModelType) {
			err := s.updateModelType(ctx, model.ID, req.ModelType)
			if err != nil {
				return nil, fmt.Errorf("update model model_type: %w", err)
			}
		}
	}

	replicas := req.Replicas
	if replicas == 0 {
		replicas = 1
	}

	gpuCount := req.GPUCount
	if gpuCount == 0 {
		gpuCount = 1
	}

	var memoryGB *int
	if req.MemoryGB > 0 {
		memoryGB = &req.MemoryGB
	}

	query := `
		INSERT INTO model_deployments (
			model_id, cache_id, environment, namespace,
			replicas_desired, gpu_count, memory_gb, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at, enabled
	`

	d := Deployment{
		ModelID:         model.ID,
		CacheID:         req.CacheID,
		Environment:     req.Environment,
		Namespace:       req.Namespace,
		ReplicasDesired: replicas,
		GPUCount:        gpuCount,
		MemoryGB:        memoryGB,
		Status:          DeploymentStatusPending,
	}

	err = s.pool.QueryRow(ctx, query,
		d.ModelID, d.CacheID, d.Environment, d.Namespace,
		d.ReplicasDesired, d.GPUCount, d.MemoryGB, d.Status,
	).Scan(&d.ID, &d.CreatedAt, &d.UpdatedAt, &d.Enabled)

	if err != nil {
		return nil, fmt.Errorf("insert deployment: %w", err)
	}

	// Auto-create routing policy if this is the first deployment for this model
	// This ensures the model is immediately accessible via the API router
	if !s.policyExists(ctx, req.ModelName) {
		if err := s.createDefaultRoutingPolicy(ctx, req.ModelName); err != nil {
			// Log warning but don't fail the deployment
			// Policy can be created manually if needed
			s.logger.Warn("failed to auto-create routing policy",
				zap.String("model", req.ModelName),
				zap.Error(err),
			)
		} else {
			s.logger.Info("auto-created routing policy",
				zap.String("model", req.ModelName),
			)
		}
	}

	return &d, nil
}

// UpdateDeploymentStatusRequest contains data for updating deployment status
type UpdateDeploymentStatusRequest struct {
	Status               string
	ModelID              string // Full model path for updating model_registry.hf_model_id
	ExternalName         string // For updating model_registry.external_name
	InferenceServiceName *string
	Endpoint             *string
	ReplicasReady        int
	LastHealthCheckAt    *time.Time
	LastHealthStatus     *string
}

// UpdateDeploymentStatus updates a deployment's status and health info
func (s *Service) UpdateDeploymentStatus(ctx context.Context, id uuid.UUID, req UpdateDeploymentStatusRequest) error {
	query := `
		UPDATE model_deployments
		SET status = $2,
		    inferenceservice_name = COALESCE($3, inferenceservice_name),
		    endpoint = COALESCE($4, endpoint),
		    replicas_ready = $5,
		    last_health_check_at = COALESCE($6, last_health_check_at),
		    last_health_status = COALESCE($7, last_health_status)
		WHERE id = $1
	`

	result, err := s.pool.Exec(ctx, query,
		id, req.Status, req.InferenceServiceName, req.Endpoint,
		req.ReplicasReady, req.LastHealthCheckAt, req.LastHealthStatus,
	)
	if err != nil {
		return fmt.Errorf("update deployment status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrDeploymentNotFound
	}

	return nil
}

// UpdateDeploymentStatusWithModel updates a deployment's status and also updates the model_registry if needed
func (s *Service) UpdateDeploymentStatusWithModel(ctx context.Context, modelName, environment string, req UpdateDeploymentStatusRequest) error {
	// Get the deployment to find the model_id
	deployment, err := s.GetDeployment(ctx, modelName, environment)
	if err != nil {
		return err
	}

	// Update deployment status
	err = s.UpdateDeploymentStatus(ctx, deployment.ID, req)
	if err != nil {
		return err
	}

	// If ModelID or ExternalName provided, update the model_registry
	if req.ModelID != "" || req.ExternalName != "" {
		model, err := s.GetModel(ctx, modelName)
		if err != nil {
			return fmt.Errorf("get model for update: %w", err)
		}

		if req.ModelID != "" && model.HFModelID != req.ModelID {
			if err := s.updateModelHFModelID(ctx, model.ID, req.ModelID); err != nil {
				return fmt.Errorf("update model hf_model_id: %w", err)
			}
		}
		if req.ExternalName != "" && (model.ExternalName == nil || *model.ExternalName != req.ExternalName) {
			if err := s.updateModelExternalName(ctx, model.ID, req.ExternalName); err != nil {
				return fmt.Errorf("update model external_name: %w", err)
			}
		}
	}

	return nil
}

// EnableDeployment enables a deployment
func (s *Service) EnableDeployment(ctx context.Context, modelName, environment, enabledBy string) error {
	deployment, err := s.GetDeployment(ctx, modelName, environment)
	if err != nil {
		return err
	}

	query := `
		UPDATE model_deployments
		SET enabled = true, status = 'pending',
		    last_enabled_at = NOW(), last_enabled_by = $2
		WHERE id = $1
	`

	_, err = s.pool.Exec(ctx, query, deployment.ID, enabledBy)
	if err != nil {
		return fmt.Errorf("enable deployment: %w", err)
	}

	return nil
}

// DisableDeployment disables a deployment
func (s *Service) DisableDeployment(ctx context.Context, modelName, environment, disabledBy string) error {
	deployment, err := s.GetDeployment(ctx, modelName, environment)
	if err != nil {
		return err
	}

	query := `
		UPDATE model_deployments
		SET enabled = false, status = 'disabled',
		    last_disabled_at = NOW(), last_disabled_by = $2
		WHERE id = $1
	`

	_, err = s.pool.Exec(ctx, query, deployment.ID, disabledBy)
	if err != nil {
		return fmt.Errorf("disable deployment: %w", err)
	}

	return nil
}

// ScaleDeploymentRequest contains data for scaling a deployment
type ScaleDeploymentRequest struct {
	ReplicasDesired int
	GPUCount        *int
	MemoryGB        *int
}

// ScaleDeployment updates a deployment's scaling parameters
func (s *Service) ScaleDeployment(ctx context.Context, modelName, environment string, req ScaleDeploymentRequest) error {
	deployment, err := s.GetDeployment(ctx, modelName, environment)
	if err != nil {
		return err
	}

	query := `
		UPDATE model_deployments
		SET replicas_desired = $2,
		    gpu_count = COALESCE($3, gpu_count),
		    memory_gb = COALESCE($4, memory_gb)
		WHERE id = $1
	`

	_, err = s.pool.Exec(ctx, query,
		deployment.ID, req.ReplicasDesired, req.GPUCount, req.MemoryGB,
	)
	if err != nil {
		return fmt.Errorf("scale deployment: %w", err)
	}

	return nil
}

// DeleteDeployment marks a deployment as terminated
func (s *Service) DeleteDeployment(ctx context.Context, modelName, environment string) error {
	deployment, err := s.GetDeployment(ctx, modelName, environment)
	if err != nil {
		return err
	}

	query := `
		UPDATE model_deployments
		SET status = 'terminated', enabled = false
		WHERE id = $1
	`

	_, err = s.pool.Exec(ctx, query, deployment.ID)
	if err != nil {
		return fmt.Errorf("delete deployment: %w", err)
	}

	return nil
}

// GetModelDeploymentSummary returns deployment info joined with model data
type ModelDeploymentSummary struct {
	ModelName    string
	ModelID      uuid.UUID
	Environment  string
	Status       string
	Enabled      bool
	Endpoint     *string
	ReplicasInfo string // e.g., "2/3 ready"
}

// GetAllDeploymentSummaries returns a summary of all deployments
func (s *Service) GetAllDeploymentSummaries(ctx context.Context, environment string) ([]ModelDeploymentSummary, error) {
	query := `
		SELECT r.name, r.id, d.environment, d.status, d.enabled, d.endpoint,
		       d.replicas_ready || '/' || d.replicas_desired || ' ready' as replicas_info
		FROM model_deployments d
		JOIN model_registry r ON d.model_id = r.id
		WHERE ($1 = '' OR d.environment = $1)
		ORDER BY r.name, d.environment
	`

	rows, err := s.pool.Query(ctx, query, environment)
	if err != nil {
		return nil, fmt.Errorf("query deployment summaries: %w", err)
	}
	defer rows.Close()

	var summaries []ModelDeploymentSummary
	for rows.Next() {
		var s ModelDeploymentSummary
		err := rows.Scan(
			&s.ModelName, &s.ModelID, &s.Environment,
			&s.Status, &s.Enabled, &s.Endpoint, &s.ReplicasInfo,
		)
		if err != nil {
			return nil, fmt.Errorf("scan deployment summary: %w", err)
		}
		summaries = append(summaries, s)
	}

	return summaries, rows.Err()
}

// policyExists checks if a routing policy exists for a given model
func (s *Service) policyExists(ctx context.Context, modelName string) bool {
	query := `
		SELECT EXISTS(
			SELECT 1 FROM routing_policies
			WHERE model = $1 AND organization_id = '*' AND deleted_at IS NULL
		)
	`

	var exists bool
	err := s.pool.QueryRow(ctx, query, modelName).Scan(&exists)
	if err != nil {
		// If query fails, return true to avoid creating duplicate policies
		return true
	}
	return exists
}

// createDefaultRoutingPolicy creates a global routing policy for the model
func (s *Service) createDefaultRoutingPolicy(ctx context.Context, modelName string) error {
	// Build the backend ID from model name (matches deployment naming convention)
	backendID := modelName

	// Create backend JSON
	backend := map[string]interface{}{
		"backend_id": backendID,
		"weight":     100,
	}
	backends := []map[string]interface{}{backend}
	backendsJSON, err := json.Marshal(backends)
	if err != nil {
		return fmt.Errorf("marshal backends: %w", err)
	}

	// Create metadata JSON
	metadata := map[string]interface{}{
		"auto_created":   true,
		"created_reason": "auto-created on deployment",
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	now := time.Now().UTC()
	query := `
		INSERT INTO routing_policies (
			policy_id, organization_id, model, backends,
			failover_threshold, enabled, version, metadata, backend_type,
			created_at, updated_at, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, 1, $7, $8, $9, $9, $10, $10)
	`

	policyID := uuid.New().String()
	_, err = s.pool.Exec(ctx, query,
		policyID, "*", modelName, backendsJSON,
		3, true, metadataJSON, "openai",
		now, "system",
	)

	if err != nil {
		// Check if error is due to unique constraint (policy already exists)
		if isPolicyAlreadyExistsError(err) {
			// This is expected and not an error
			return nil
		}
		return fmt.Errorf("insert routing policy: %w", err)
	}

	return nil
}

// isPolicyAlreadyExistsError checks if the error is due to unique constraint violation
func isPolicyAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	// Check for PostgreSQL unique constraint violation
	return strings.Contains(errStr, "unique") ||
		strings.Contains(errStr, "duplicate") ||
		strings.Contains(errStr, "idx_routing_policies_unique_org_model")
}
