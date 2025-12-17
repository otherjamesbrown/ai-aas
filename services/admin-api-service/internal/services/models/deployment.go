// Package models provides the business logic for model management operations.
package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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
	ModelID      string     // Full model path (e.g., "unsloth/gpt-oss-20b")
	ExternalName string     // Name exposed in OpenAI-compatible APIs
	CacheID      *uuid.UUID
	Environment  string
	Namespace    string
	GPUCount     int
	MemoryGB     int
	Replicas     int
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
			}
			model, err = s.AddModel(ctx, autoRegReq)
			if err != nil {
				return nil, fmt.Errorf("auto-register model: %w", err)
			}
		} else {
			return nil, err
		}
	} else if req.ModelID != "" || req.ExternalName != "" {
		// Model exists - update hf_model_id and external_name if provided
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

	return &d, nil
}

// UpdateDeploymentStatusRequest contains data for updating deployment status
type UpdateDeploymentStatusRequest struct {
	Status                 string
	InferenceServiceName   *string
	Endpoint               *string
	ReplicasReady          int
	LastHealthCheckAt      *time.Time
	LastHealthStatus       *string
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
