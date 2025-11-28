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
	pool      *pgxpool.Pool
	encryptor Encryptor
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

// Model represents a registered model
type Model struct {
	ID                     uuid.UUID
	Name                   string
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
	query := `
		SELECT id, name, hf_model_id, hf_revision, requires_auth, is_gated,
		       license_type, license_url, license_accepted_at, license_accepted_by,
		       recommended_gpu_memory_gb, recommended_cpu_memory_gb,
		       pinned_version, metadata, created_at, updated_at
		FROM model_registry
		ORDER BY name
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
			&m.ID, &m.Name, &m.HFModelID, &m.HFRevision, &m.RequiresAuth, &m.IsGated,
			&m.LicenseType, &m.LicenseURL, &m.LicenseAcceptedAt, &m.LicenseAcceptedBy,
			&m.RecommendedGPUMemoryGB, &m.RecommendedCPUMemoryGB,
			&m.PinnedVersion, &m.Metadata, &m.CreatedAt, &m.UpdatedAt,
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
		SELECT id, name, hf_model_id, hf_revision, requires_auth, is_gated,
		       license_type, license_url, license_accepted_at, license_accepted_by,
		       recommended_gpu_memory_gb, recommended_cpu_memory_gb,
		       pinned_version, metadata, created_at, updated_at
		FROM model_registry
		WHERE name = $1
	`

	var m Model
	err := s.pool.QueryRow(ctx, query, name).Scan(
		&m.ID, &m.Name, &m.HFModelID, &m.HFRevision, &m.RequiresAuth, &m.IsGated,
		&m.LicenseType, &m.LicenseURL, &m.LicenseAcceptedAt, &m.LicenseAcceptedBy,
		&m.RecommendedGPUMemoryGB, &m.RecommendedCPUMemoryGB,
		&m.PinnedVersion, &m.Metadata, &m.CreatedAt, &m.UpdatedAt,
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

	query := `
		INSERT INTO model_registry (
			name, hf_model_id, hf_revision, requires_auth, is_gated,
			license_type, license_accepted_at, license_accepted_by,
			recommended_gpu_memory_gb, recommended_cpu_memory_gb
		) VALUES ($1, $2, 'main', $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at
	`

	m := Model{
		Name:                   req.Name,
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
		m.Name, m.HFModelID, m.RequiresAuth, m.IsGated,
		m.LicenseType, m.LicenseAcceptedAt, m.LicenseAcceptedBy,
		m.RecommendedGPUMemoryGB, m.RecommendedCPUMemoryGB,
	).Scan(&m.ID, &m.CreatedAt, &m.UpdatedAt)

	if err != nil {
		return nil, fmt.Errorf("insert model: %w", err)
	}

	return &m, nil
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

