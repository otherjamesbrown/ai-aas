// Package models provides the business logic for model management operations.
package models

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrModelNotFound indicates the requested model was not found
var ErrModelNotFound = errors.New("model not found")

// ErrCredentialNotFound indicates the requested credential was not found
var ErrCredentialNotFound = errors.New("credential not found")

// Service provides model management operations
type Service struct {
	pool            *pgxpool.Pool
	encryptor       Encryptor
	s3ClientFactory S3ClientFactory
}

// Encryptor provides encryption/decryption for credentials
type Encryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// NewService creates a new model management service
func NewService(pool *pgxpool.Pool, encryptor Encryptor) *Service {
	return &Service{
		pool:      pool,
		encryptor: encryptor,
	}
}

// SetS3ClientFactory sets the S3 client factory for the service
// This allows for dependency injection and easier testing
func (s *Service) SetS3ClientFactory(factory S3ClientFactory) {
	s.s3ClientFactory = factory
}

// Model represents a registered model
type Model struct {
	ID                     uuid.UUID
	Name                   string
	ExternalName           *string
	HFModelID              string
	HFRevision             string
	RequiresAuth           bool
	IsGated                bool
	LicenseType            *string
	LicenseURL             *string
	LicenseAcceptedAt      *time.Time
	LicenseAcceptedBy      *string
	RecommendedGPUMemoryGB *int32
	RecommendedCPUMemoryGB *int32
	PinnedVersion          *string
	Metadata               []byte
	CreatedAt              time.Time
	UpdatedAt              time.Time

	// Computed fields
	CacheStatus      *string
	DeploymentStatus *string
}

// ListModelsOptions configures model listing
type ListModelsOptions struct {
	Cached      bool
	Deployed    bool
	Orphaned    bool
	Environment string
}

// ListModels returns models matching the given options
func (s *Service) ListModels(ctx context.Context, opts ListModelsOptions) ([]Model, error) {
	// Query with LEFT JOINs to get cache and deployment status
	// Get the latest cache entry status and the deployment status for any environment
	query := `
		SELECT
			r.id, r.name, r.external_name, r.hf_model_id, r.hf_revision, r.requires_auth, r.is_gated,
			r.license_type, r.license_url, r.license_accepted_at, r.license_accepted_by,
			r.recommended_gpu_memory_gb, r.recommended_cpu_memory_gb,
			r.pinned_version, r.metadata, r.created_at, r.updated_at,
			-- Cache status: get status of the most recent cache entry
			(SELECT status FROM model_cache WHERE model_id = r.id ORDER BY cached_at DESC LIMIT 1) as cache_status,
			-- Deployment status: get status if any deployment exists (priority: ready > deploying > others)
			(SELECT status FROM model_deployments
			 WHERE model_id = r.id
			 ORDER BY
			   CASE
			     WHEN status = 'ready' THEN 1
			     WHEN status = 'deploying' THEN 2
			     WHEN status = 'pending' THEN 3
			     ELSE 4
			   END,
			   updated_at DESC
			 LIMIT 1) as deployment_status
		FROM model_registry r
		ORDER BY r.name
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query models: %w", err)
	}
	defer rows.Close()

	var models []Model
	for rows.Next() {
		var m Model
		err := rows.Scan(
			&m.ID, &m.Name, &m.ExternalName, &m.HFModelID, &m.HFRevision, &m.RequiresAuth, &m.IsGated,
			&m.LicenseType, &m.LicenseURL, &m.LicenseAcceptedAt, &m.LicenseAcceptedBy,
			&m.RecommendedGPUMemoryGB, &m.RecommendedCPUMemoryGB,
			&m.PinnedVersion, &m.Metadata, &m.CreatedAt, &m.UpdatedAt,
			&m.CacheStatus, &m.DeploymentStatus,
		)
		if err != nil {
			return nil, fmt.Errorf("scan model: %w", err)
		}
		models = append(models, m)
	}

	return models, rows.Err()
}

// GetModel returns a model by name
func (s *Service) GetModel(ctx context.Context, name string) (*Model, error) {
	query := `
		SELECT
			r.id, r.name, r.external_name, r.hf_model_id, r.hf_revision, r.requires_auth, r.is_gated,
			r.license_type, r.license_url, r.license_accepted_at, r.license_accepted_by,
			r.recommended_gpu_memory_gb, r.recommended_cpu_memory_gb,
			r.pinned_version, r.metadata, r.created_at, r.updated_at,
			-- Cache status: get status of the most recent cache entry
			(SELECT status FROM model_cache WHERE model_id = r.id ORDER BY cached_at DESC LIMIT 1) as cache_status,
			-- Deployment status: get status if any deployment exists
			(SELECT status FROM model_deployments
			 WHERE model_id = r.id
			 ORDER BY
			   CASE
			     WHEN status = 'ready' THEN 1
			     WHEN status = 'deploying' THEN 2
			     WHEN status = 'pending' THEN 3
			     ELSE 4
			   END,
			   updated_at DESC
			 LIMIT 1) as deployment_status
		FROM model_registry r
		WHERE r.name = $1
	`

	var m Model
	err := s.pool.QueryRow(ctx, query, name).Scan(
		&m.ID, &m.Name, &m.ExternalName, &m.HFModelID, &m.HFRevision, &m.RequiresAuth, &m.IsGated,
		&m.LicenseType, &m.LicenseURL, &m.LicenseAcceptedAt, &m.LicenseAcceptedBy,
		&m.RecommendedGPUMemoryGB, &m.RecommendedCPUMemoryGB,
		&m.PinnedVersion, &m.Metadata, &m.CreatedAt, &m.UpdatedAt,
		&m.CacheStatus, &m.DeploymentStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrModelNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query model: %w", err)
	}

	return &m, nil
}

// AddModelRequest contains the data for adding a model
type AddModelRequest struct {
	Name           string
	ExternalName   string // Optional, derived from HFModelID if not set
	HFModelID      string
	RequiresAuth   bool
	IsGated        bool
	LicenseType    string
	AcceptLicense  bool
	AcceptedBy     string
	GPUMemoryGB    int
	CPUMemoryGB    int
}

// AddModel registers a new model
func (s *Service) AddModel(ctx context.Context, req AddModelRequest) (*Model, error) {
	var licenseAcceptedAt *time.Time
	var licenseAcceptedBy *string

	if req.AcceptLicense && req.IsGated {
		now := time.Now()
		licenseAcceptedAt = &now
		licenseAcceptedBy = &req.AcceptedBy
	}

	var licenseType *string
	if req.LicenseType != "" {
		licenseType = &req.LicenseType
	}

	var gpuMemory, cpuMemory *int32
	if req.GPUMemoryGB > 0 {
		g := int32(req.GPUMemoryGB)
		gpuMemory = &g
	}
	if req.CPUMemoryGB > 0 {
		c := int32(req.CPUMemoryGB)
		cpuMemory = &c
	}

	// Derive external_name from HFModelID if not explicitly set
	externalName := req.ExternalName
	if externalName == "" {
		externalName = deriveExternalName(req.HFModelID)
	}
	var externalNamePtr *string
	if externalName != "" {
		externalNamePtr = &externalName
	}

	query := `
		INSERT INTO model_registry (
			name, external_name, hf_model_id, hf_revision, requires_auth, is_gated,
			license_type, license_accepted_at, license_accepted_by,
			recommended_gpu_memory_gb, recommended_cpu_memory_gb
		) VALUES ($1, $2, $3, 'main', $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at, updated_at
	`

	m := Model{
		Name:                   req.Name,
		ExternalName:           externalNamePtr,
		HFModelID:              req.HFModelID,
		HFRevision:             "main",
		RequiresAuth:           req.RequiresAuth,
		IsGated:                req.IsGated,
		LicenseType:            licenseType,
		LicenseAcceptedAt:      licenseAcceptedAt,
		LicenseAcceptedBy:      licenseAcceptedBy,
		RecommendedGPUMemoryGB: gpuMemory,
		RecommendedCPUMemoryGB: cpuMemory,
	}

	err := s.pool.QueryRow(ctx, query,
		m.Name, m.ExternalName, m.HFModelID, m.RequiresAuth, m.IsGated,
		m.LicenseType, m.LicenseAcceptedAt, m.LicenseAcceptedBy,
		m.RecommendedGPUMemoryGB, m.RecommendedCPUMemoryGB,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert model: %w", err)
	}

	return &m, nil
}

// deriveExternalName derives an external name from a model ID.
// If the model ID contains a "/", returns the part after the last "/".
// Otherwise, returns the model ID as-is.
func deriveExternalName(modelID string) string {
	for i := len(modelID) - 1; i >= 0; i-- {
		if modelID[i] == '/' {
			return modelID[i+1:]
		}
	}
	return modelID
}

// RemoveModel deletes a model from the registry
func (s *Service) RemoveModel(ctx context.Context, name string, force bool) error {
	if !force {
		// Check for active deployments
		var count int
		err := s.pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM model_deployments d
			JOIN model_registry r ON d.model_id = r.id
			WHERE r.name = $1 AND d.status NOT IN ('terminated', 'disabled')
		`, name).Scan(&count)
		if err != nil {
			return fmt.Errorf("check deployments: %w", err)
		}
		if count > 0 {
			return errors.New("model has active deployments, use --force to remove")
		}
	}

	result, err := s.pool.Exec(ctx, `DELETE FROM model_registry WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("delete model: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrModelNotFound
	}

	return nil
}

// Credential operations

// ListCredentials returns all stored credential types (values masked)
func (s *Service) ListCredentials(ctx context.Context) ([]CredentialInfo, error) {
	query := `SELECT credential_type, metadata, created_at, updated_at FROM platform_credentials`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query credentials: %w", err)
	}
	defer rows.Close()

	var creds []CredentialInfo
	for rows.Next() {
		var c CredentialInfo
		var metadata []byte
		err := rows.Scan(&c.Type, &metadata, &c.CreatedAt, &c.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan credential: %w", err)
		}
		c.Masked = "****configured****"
		creds = append(creds, c)
	}

	return creds, rows.Err()
}

// CredentialInfo contains credential metadata (never the value)
type CredentialInfo struct {
	Type      string
	Masked    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// SetCredential stores an encrypted credential
func (s *Service) SetCredential(ctx context.Context, credType, value string, metadata map[string]interface{}) error {
	encrypted, err := s.encryptor.Encrypt(value)
	if err != nil {
		return fmt.Errorf("encrypt credential: %w", err)
	}

	query := `
		INSERT INTO platform_credentials (credential_type, encrypted_value, metadata)
		VALUES ($1, $2, $3)
		ON CONFLICT (credential_type) DO UPDATE
		SET encrypted_value = $2, metadata = $3, updated_at = NOW()
	`

	_, err = s.pool.Exec(ctx, query, credType, encrypted, metadata)
	if err != nil {
		return fmt.Errorf("store credential: %w", err)
	}

	return nil
}

// GetCredential retrieves and decrypts a credential
func (s *Service) GetCredential(ctx context.Context, credType string) (string, error) {
	var encrypted string
	err := s.pool.QueryRow(ctx, `
		SELECT encrypted_value FROM platform_credentials WHERE credential_type = $1
	`, credType).Scan(&encrypted)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrCredentialNotFound
	}
	if err != nil {
		return "", fmt.Errorf("query credential: %w", err)
	}

	value, err := s.encryptor.Decrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt credential: %w", err)
	}

	return value, nil
}

// DeleteCredential removes a credential
func (s *Service) DeleteCredential(ctx context.Context, credType string) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM platform_credentials WHERE credential_type = $1`, credType)
	if err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrCredentialNotFound
	}

	return nil
}

// updateModelHFModelID updates a model's hf_model_id
func (s *Service) updateModelHFModelID(ctx context.Context, modelID uuid.UUID, hfModelID string) error {
	query := `UPDATE model_registry SET hf_model_id = $2, updated_at = NOW() WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, modelID, hfModelID)
	return err
}

// updateModelExternalName updates a model's external_name
func (s *Service) updateModelExternalName(ctx context.Context, modelID uuid.UUID, externalName string) error {
	query := `UPDATE model_registry SET external_name = $2, updated_at = NOW() WHERE id = $1`
	_, err := s.pool.Exec(ctx, query, modelID, externalName)
	return err
}

