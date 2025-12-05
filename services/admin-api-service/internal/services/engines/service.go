// Package engines provides business logic for inference engine management.
package engines

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	// ErrEngineNotFound indicates the requested engine was not found
	ErrEngineNotFound = errors.New("engine not found")
	// ErrVersionNotFound indicates the requested version was not found
	ErrVersionNotFound = errors.New("version not found")
	// ErrConfigNotFound indicates the requested config was not found
	ErrConfigNotFound = errors.New("config not found")
	// ErrConfigInUse indicates the config is referenced by deployments
	ErrConfigInUse = errors.New("config is in use by deployments")
)

// Service provides inference engine management operations
type Service struct {
	pool *pgxpool.Pool
}

// NewService creates a new inference engine service
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// Engine represents an inference engine type
type Engine struct {
	ID                    uuid.UUID `json:"id"`
	Name                  string    `json:"name"`
	DisplayName           string    `json:"display_name"`
	Description           *string   `json:"description,omitempty"`
	DefaultVersion        string    `json:"default_version"`
	ProtocolVersions      []string  `json:"protocol_versions"`
	SupportedModelFormats []string  `json:"supported_model_formats"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// EngineVersion represents a specific version of an engine
type EngineVersion struct {
	ID             uuid.UUID `json:"id"`
	EngineID       uuid.UUID `json:"engine_id"`
	Version        string    `json:"version"`
	ContainerImage string    `json:"container_image"`
	IsDefault      bool      `json:"is_default"`
	IsDeprecated   bool      `json:"is_deprecated"`
	MinGPUMemoryGB *int      `json:"min_gpu_memory_gb,omitempty"`
	ReleaseNotes   *string   `json:"release_notes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// EngineConfig represents a named resource profile for an engine
type EngineConfig struct {
	ID                    uuid.UUID       `json:"id"`
	Name                  string          `json:"name"`
	EngineID              uuid.UUID       `json:"engine_id"`
	EngineName            string          `json:"engine_name"`
	VersionID             *uuid.UUID      `json:"version_id,omitempty"`
	Version               *string         `json:"version,omitempty"`
	Description           *string         `json:"description,omitempty"`
	GPUCount              int             `json:"gpu_count"`
	GPUType               *string         `json:"gpu_type,omitempty"`
	MemoryGB              int             `json:"memory_gb"`
	CPUCores              int             `json:"cpu_cores"`
	EngineArgs            json.RawMessage `json:"engine_args"`
	MinReplicas           int             `json:"min_replicas"`
	MaxReplicas           int             `json:"max_replicas"`
	StartupTimeoutSeconds int             `json:"startup_timeout_seconds"`
	IsDefault             bool            `json:"is_default"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

// ListEngines returns all available inference engines
func (s *Service) ListEngines(ctx context.Context) ([]Engine, error) {
	query := `
		SELECT id, name, display_name, description, default_version,
		       protocol_versions, supported_model_formats, created_at, updated_at
		FROM inference_engines
		ORDER BY name
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query engines: %w", err)
	}
	defer rows.Close()

	var engines []Engine
	for rows.Next() {
		var e Engine
		err := rows.Scan(
			&e.ID, &e.Name, &e.DisplayName, &e.Description, &e.DefaultVersion,
			&e.ProtocolVersions, &e.SupportedModelFormats, &e.CreatedAt, &e.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan engine: %w", err)
		}
		engines = append(engines, e)
	}

	return engines, rows.Err()
}

// GetEngine returns an engine by name
func (s *Service) GetEngine(ctx context.Context, name string) (*Engine, error) {
	query := `
		SELECT id, name, display_name, description, default_version,
		       protocol_versions, supported_model_formats, created_at, updated_at
		FROM inference_engines
		WHERE name = $1
	`

	var e Engine
	err := s.pool.QueryRow(ctx, query, name).Scan(
		&e.ID, &e.Name, &e.DisplayName, &e.Description, &e.DefaultVersion,
		&e.ProtocolVersions, &e.SupportedModelFormats, &e.CreatedAt, &e.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrEngineNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query engine: %w", err)
	}

	return &e, nil
}

// ListVersions returns all versions for an engine
func (s *Service) ListVersions(ctx context.Context, engineName string) ([]EngineVersion, error) {
	query := `
		SELECT v.id, v.engine_id, v.version, v.container_image, v.is_default,
		       v.is_deprecated, v.min_gpu_memory_gb, v.release_notes, v.created_at
		FROM inference_engine_versions v
		JOIN inference_engines e ON v.engine_id = e.id
		WHERE e.name = $1
		ORDER BY v.created_at DESC
	`

	rows, err := s.pool.Query(ctx, query, engineName)
	if err != nil {
		return nil, fmt.Errorf("query versions: %w", err)
	}
	defer rows.Close()

	var versions []EngineVersion
	for rows.Next() {
		var v EngineVersion
		err := rows.Scan(
			&v.ID, &v.EngineID, &v.Version, &v.ContainerImage, &v.IsDefault,
			&v.IsDeprecated, &v.MinGPUMemoryGB, &v.ReleaseNotes, &v.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}

	return versions, rows.Err()
}

// AddVersionRequest contains data for adding an engine version
type AddVersionRequest struct {
	EngineName     string
	Version        string
	ContainerImage string
	SetDefault     bool
	MinGPUMemoryGB *int
	ReleaseNotes   *string
}

// AddVersion adds a new version for an engine
func (s *Service) AddVersion(ctx context.Context, req AddVersionRequest) (*EngineVersion, error) {
	// Get engine ID
	engine, err := s.GetEngine(ctx, req.EngineName)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// If setting as default, unset other defaults first
	if req.SetDefault {
		_, err = tx.Exec(ctx, `
			UPDATE inference_engine_versions SET is_default = FALSE WHERE engine_id = $1
		`, engine.ID)
		if err != nil {
			return nil, fmt.Errorf("unset defaults: %w", err)
		}
	}

	query := `
		INSERT INTO inference_engine_versions (engine_id, version, container_image, is_default, min_gpu_memory_gb, release_notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at
	`

	var v EngineVersion
	v.EngineID = engine.ID
	v.Version = req.Version
	v.ContainerImage = req.ContainerImage
	v.IsDefault = req.SetDefault
	v.MinGPUMemoryGB = req.MinGPUMemoryGB
	v.ReleaseNotes = req.ReleaseNotes

	err = tx.QueryRow(ctx, query,
		engine.ID, req.Version, req.ContainerImage, req.SetDefault, req.MinGPUMemoryGB, req.ReleaseNotes,
	).Scan(&v.ID, &v.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert version: %w", err)
	}

	// Update engine's default_version if this is the new default
	if req.SetDefault {
		_, err = tx.Exec(ctx, `
			UPDATE inference_engines SET default_version = $1, updated_at = NOW() WHERE id = $2
		`, req.Version, engine.ID)
		if err != nil {
			return nil, fmt.Errorf("update engine default: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &v, nil
}

// SetDefaultVersion sets the default version for an engine
func (s *Service) SetDefaultVersion(ctx context.Context, engineName, version string) error {
	engine, err := s.GetEngine(ctx, engineName)
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Verify version exists
	var versionID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT id FROM inference_engine_versions WHERE engine_id = $1 AND version = $2
	`, engine.ID, version).Scan(&versionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVersionNotFound
	}
	if err != nil {
		return fmt.Errorf("query version: %w", err)
	}

	// Unset all defaults
	_, err = tx.Exec(ctx, `
		UPDATE inference_engine_versions SET is_default = FALSE WHERE engine_id = $1
	`, engine.ID)
	if err != nil {
		return fmt.Errorf("unset defaults: %w", err)
	}

	// Set new default
	_, err = tx.Exec(ctx, `
		UPDATE inference_engine_versions SET is_default = TRUE WHERE id = $1
	`, versionID)
	if err != nil {
		return fmt.Errorf("set default: %w", err)
	}

	// Update engine
	_, err = tx.Exec(ctx, `
		UPDATE inference_engines SET default_version = $1, updated_at = NOW() WHERE id = $2
	`, version, engine.ID)
	if err != nil {
		return fmt.Errorf("update engine: %w", err)
	}

	return tx.Commit(ctx)
}

// DeleteVersion removes a version from an engine
func (s *Service) DeleteVersion(ctx context.Context, engineName, version string) error {
	engine, err := s.GetEngine(ctx, engineName)
	if err != nil {
		return err
	}

	result, err := s.pool.Exec(ctx, `
		DELETE FROM inference_engine_versions WHERE engine_id = $1 AND version = $2
	`, engine.ID, version)
	if err != nil {
		return fmt.Errorf("delete version: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrVersionNotFound
	}

	return nil
}

// ListConfigs returns all engine configs
func (s *Service) ListConfigs(ctx context.Context) ([]EngineConfig, error) {
	query := `
		SELECT c.id, c.name, c.engine_id, e.name as engine_name, c.version_id,
		       v.version, c.description, c.gpu_count, c.gpu_type, c.memory_gb,
		       c.cpu_cores, c.engine_args, c.min_replicas, c.max_replicas,
		       c.startup_timeout_seconds, c.is_default, c.created_at, c.updated_at
		FROM inference_engine_configs c
		JOIN inference_engines e ON c.engine_id = e.id
		LEFT JOIN inference_engine_versions v ON c.version_id = v.id
		ORDER BY c.name
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query configs: %w", err)
	}
	defer rows.Close()

	var configs []EngineConfig
	for rows.Next() {
		var c EngineConfig
		err := rows.Scan(
			&c.ID, &c.Name, &c.EngineID, &c.EngineName, &c.VersionID,
			&c.Version, &c.Description, &c.GPUCount, &c.GPUType, &c.MemoryGB,
			&c.CPUCores, &c.EngineArgs, &c.MinReplicas, &c.MaxReplicas,
			&c.StartupTimeoutSeconds, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		configs = append(configs, c)
	}

	return configs, rows.Err()
}

// GetConfig returns a config by name
func (s *Service) GetConfig(ctx context.Context, name string) (*EngineConfig, error) {
	query := `
		SELECT c.id, c.name, c.engine_id, e.name as engine_name, c.version_id,
		       v.version, c.description, c.gpu_count, c.gpu_type, c.memory_gb,
		       c.cpu_cores, c.engine_args, c.min_replicas, c.max_replicas,
		       c.startup_timeout_seconds, c.is_default, c.created_at, c.updated_at
		FROM inference_engine_configs c
		JOIN inference_engines e ON c.engine_id = e.id
		LEFT JOIN inference_engine_versions v ON c.version_id = v.id
		WHERE c.name = $1
	`

	var c EngineConfig
	err := s.pool.QueryRow(ctx, query, name).Scan(
		&c.ID, &c.Name, &c.EngineID, &c.EngineName, &c.VersionID,
		&c.Version, &c.Description, &c.GPUCount, &c.GPUType, &c.MemoryGB,
		&c.CPUCores, &c.EngineArgs, &c.MinReplicas, &c.MaxReplicas,
		&c.StartupTimeoutSeconds, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query config: %w", err)
	}

	return &c, nil
}

// CreateConfigRequest contains data for creating an engine config
type CreateConfigRequest struct {
	Name                  string
	EngineName            string
	Version               *string // Optional: uses engine default if not specified
	Description           *string
	GPUCount              int
	GPUType               *string
	MemoryGB              int
	CPUCores              int
	EngineArgs            map[string]interface{}
	MinReplicas           int
	MaxReplicas           int
	StartupTimeoutSeconds int
	IsDefault             bool
}

// CreateConfig creates a new engine config
func (s *Service) CreateConfig(ctx context.Context, req CreateConfigRequest) (*EngineConfig, error) {
	engine, err := s.GetEngine(ctx, req.EngineName)
	if err != nil {
		return nil, err
	}

	// Resolve version ID if specified
	var versionID *uuid.UUID
	if req.Version != nil {
		var vid uuid.UUID
		err := s.pool.QueryRow(ctx, `
			SELECT id FROM inference_engine_versions WHERE engine_id = $1 AND version = $2
		`, engine.ID, *req.Version).Scan(&vid)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrVersionNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("query version: %w", err)
		}
		versionID = &vid
	}

	// Marshal engine args
	engineArgs, err := json.Marshal(req.EngineArgs)
	if err != nil {
		return nil, fmt.Errorf("marshal engine args: %w", err)
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// If setting as default, unset other defaults for this engine
	if req.IsDefault {
		_, err = tx.Exec(ctx, `
			UPDATE inference_engine_configs SET is_default = FALSE WHERE engine_id = $1
		`, engine.ID)
		if err != nil {
			return nil, fmt.Errorf("unset defaults: %w", err)
		}
	}

	query := `
		INSERT INTO inference_engine_configs (
			name, engine_id, version_id, description, gpu_count, gpu_type, memory_gb,
			cpu_cores, engine_args, min_replicas, max_replicas, startup_timeout_seconds, is_default
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id, created_at, updated_at
	`

	c := EngineConfig{
		Name:                  req.Name,
		EngineID:              engine.ID,
		EngineName:            engine.Name,
		VersionID:             versionID,
		Description:           req.Description,
		GPUCount:              req.GPUCount,
		GPUType:               req.GPUType,
		MemoryGB:              req.MemoryGB,
		CPUCores:              req.CPUCores,
		EngineArgs:            engineArgs,
		MinReplicas:           req.MinReplicas,
		MaxReplicas:           req.MaxReplicas,
		StartupTimeoutSeconds: req.StartupTimeoutSeconds,
		IsDefault:             req.IsDefault,
	}

	err = tx.QueryRow(ctx, query,
		c.Name, c.EngineID, c.VersionID, c.Description, c.GPUCount, c.GPUType, c.MemoryGB,
		c.CPUCores, c.EngineArgs, c.MinReplicas, c.MaxReplicas, c.StartupTimeoutSeconds, c.IsDefault,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("insert config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &c, nil
}

// UpdateConfigRequest contains data for updating an engine config
type UpdateConfigRequest struct {
	Description           *string
	GPUCount              *int
	GPUType               *string
	MemoryGB              *int
	CPUCores              *int
	EngineArgs            map[string]interface{}
	MinReplicas           *int
	MaxReplicas           *int
	StartupTimeoutSeconds *int
	IsDefault             *bool
}

// UpdateConfig updates an existing engine config
func (s *Service) UpdateConfig(ctx context.Context, name string, req UpdateConfigRequest) (*EngineConfig, error) {
	existing, err := s.GetConfig(ctx, name)
	if err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Build update query dynamically
	updates := []string{}
	args := []interface{}{}
	argNum := 1

	if req.Description != nil {
		updates = append(updates, fmt.Sprintf("description = $%d", argNum))
		args = append(args, req.Description)
		argNum++
	}
	if req.GPUCount != nil {
		updates = append(updates, fmt.Sprintf("gpu_count = $%d", argNum))
		args = append(args, *req.GPUCount)
		argNum++
	}
	if req.GPUType != nil {
		updates = append(updates, fmt.Sprintf("gpu_type = $%d", argNum))
		args = append(args, req.GPUType)
		argNum++
	}
	if req.MemoryGB != nil {
		updates = append(updates, fmt.Sprintf("memory_gb = $%d", argNum))
		args = append(args, *req.MemoryGB)
		argNum++
	}
	if req.CPUCores != nil {
		updates = append(updates, fmt.Sprintf("cpu_cores = $%d", argNum))
		args = append(args, *req.CPUCores)
		argNum++
	}
	if req.EngineArgs != nil {
		engineArgs, err := json.Marshal(req.EngineArgs)
		if err != nil {
			return nil, fmt.Errorf("marshal engine args: %w", err)
		}
		updates = append(updates, fmt.Sprintf("engine_args = $%d", argNum))
		args = append(args, engineArgs)
		argNum++
	}
	if req.MinReplicas != nil {
		updates = append(updates, fmt.Sprintf("min_replicas = $%d", argNum))
		args = append(args, *req.MinReplicas)
		argNum++
	}
	if req.MaxReplicas != nil {
		updates = append(updates, fmt.Sprintf("max_replicas = $%d", argNum))
		args = append(args, *req.MaxReplicas)
		argNum++
	}
	if req.StartupTimeoutSeconds != nil {
		updates = append(updates, fmt.Sprintf("startup_timeout_seconds = $%d", argNum))
		args = append(args, *req.StartupTimeoutSeconds)
		argNum++
	}

	// Handle is_default separately
	if req.IsDefault != nil && *req.IsDefault {
		// Unset other defaults for this engine
		_, err = tx.Exec(ctx, `
			UPDATE inference_engine_configs SET is_default = FALSE WHERE engine_id = $1
		`, existing.EngineID)
		if err != nil {
			return nil, fmt.Errorf("unset defaults: %w", err)
		}
		updates = append(updates, fmt.Sprintf("is_default = $%d", argNum))
		args = append(args, true)
		argNum++
	}

	if len(updates) == 0 {
		return existing, nil
	}

	// Add name to args for WHERE clause
	args = append(args, name)

	query := fmt.Sprintf(`
		UPDATE inference_engine_configs SET %s, updated_at = NOW()
		WHERE name = $%d
		RETURNING id, name, engine_id, version_id, description, gpu_count, gpu_type,
		          memory_gb, cpu_cores, engine_args, min_replicas, max_replicas,
		          startup_timeout_seconds, is_default, created_at, updated_at
	`, joinStrings(updates, ", "), argNum)

	var c EngineConfig
	err = tx.QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.Name, &c.EngineID, &c.VersionID, &c.Description, &c.GPUCount, &c.GPUType,
		&c.MemoryGB, &c.CPUCores, &c.EngineArgs, &c.MinReplicas, &c.MaxReplicas,
		&c.StartupTimeoutSeconds, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	c.EngineName = existing.EngineName
	c.Version = existing.Version

	return &c, nil
}

// DeleteConfig removes an engine config
func (s *Service) DeleteConfig(ctx context.Context, name string) error {
	// Check if config is in use
	var count int
	err := s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM model_deployments d
		JOIN inference_engine_configs c ON d.engine_config_id = c.id
		WHERE c.name = $1
	`, name).Scan(&count)
	if err != nil {
		return fmt.Errorf("check deployments: %w", err)
	}
	if count > 0 {
		return ErrConfigInUse
	}

	result, err := s.pool.Exec(ctx, `DELETE FROM inference_engine_configs WHERE name = $1`, name)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrConfigNotFound
	}

	return nil
}

// GetDefaultConfig returns the default config for an engine
func (s *Service) GetDefaultConfig(ctx context.Context, engineName string) (*EngineConfig, error) {
	query := `
		SELECT c.id, c.name, c.engine_id, e.name as engine_name, c.version_id,
		       v.version, c.description, c.gpu_count, c.gpu_type, c.memory_gb,
		       c.cpu_cores, c.engine_args, c.min_replicas, c.max_replicas,
		       c.startup_timeout_seconds, c.is_default, c.created_at, c.updated_at
		FROM inference_engine_configs c
		JOIN inference_engines e ON c.engine_id = e.id
		LEFT JOIN inference_engine_versions v ON c.version_id = v.id
		WHERE e.name = $1 AND c.is_default = TRUE
	`

	var c EngineConfig
	err := s.pool.QueryRow(ctx, query, engineName).Scan(
		&c.ID, &c.Name, &c.EngineID, &c.EngineName, &c.VersionID,
		&c.Version, &c.Description, &c.GPUCount, &c.GPUType, &c.MemoryGB,
		&c.CPUCores, &c.EngineArgs, &c.MinReplicas, &c.MaxReplicas,
		&c.StartupTimeoutSeconds, &c.IsDefault, &c.CreatedAt, &c.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query default config: %w", err)
	}

	return &c, nil
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}
