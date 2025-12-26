package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
)

// ModelRepository handles model database operations
type ModelRepository struct {
	db *DB
}

// NewModelRepository creates a new model repository
func NewModelRepository(db *DB) *ModelRepository {
	return &ModelRepository{db: db}
}

// Upsert creates or updates a model
func (r *ModelRepository) Upsert(ctx context.Context, reg *domain.ModelRegistration) (*domain.Model, bool, error) {
	metadata, err := json.Marshal(reg.Metadata)
	if err != nil {
		return nil, false, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	status := reg.DeploymentStatus
	if status == "" {
		status = "pending"
	}

	target := reg.DeploymentTarget
	if target == "" {
		target = "managed"
	}

	revision := reg.Revision
	if revision == 0 {
		revision = 1
	}

	var model domain.Model
	var created bool

	query := `
		INSERT INTO model_registry_entries (
			model_id, model_name, deployment_endpoint, deployment_environment,
			deployment_namespace, deployment_status, deployment_target,
			revision, cost_per_1k_tokens, metadata, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11
		)
		ON CONFLICT (model_name, deployment_environment) DO UPDATE SET
			deployment_endpoint = EXCLUDED.deployment_endpoint,
			deployment_status = EXCLUDED.deployment_status,
			deployment_target = EXCLUDED.deployment_target,
			revision = EXCLUDED.revision,
			cost_per_1k_tokens = EXCLUDED.cost_per_1k_tokens,
			metadata = EXCLUDED.metadata,
			updated_at = EXCLUDED.updated_at
		RETURNING model_id, model_name, deployment_endpoint, deployment_environment,
			deployment_namespace, deployment_status, deployment_target, revision,
			cost_per_1k_tokens, metadata, created_at, updated_at,
			(xmax = 0) as created
	`

	now := time.Now().UTC()
	modelID := uuid.New()

	var metadataJSON []byte
	err = r.db.Pool().QueryRow(ctx, query,
		modelID, reg.ModelName, reg.DeploymentEndpoint, reg.DeploymentEnvironment,
		reg.DeploymentNamespace, status, target, revision, reg.CostPer1kTokens,
		metadata, now,
	).Scan(
		&model.ModelID, &model.ModelName, &model.DeploymentEndpoint, &model.DeploymentEnvironment,
		&model.DeploymentNamespace, &model.DeploymentStatus, &model.DeploymentTarget, &model.Revision,
		&model.CostPer1kTokens, &metadataJSON, &model.CreatedAt, &model.UpdatedAt, &created,
	)

	if err != nil {
		return nil, false, fmt.Errorf("failed to upsert model: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &model.Metadata); err != nil {
		model.Metadata = make(map[string]interface{})
	}

	return &model, created, nil
}

// GetByNameAndEnv retrieves a model by name and environment
func (r *ModelRepository) GetByNameAndEnv(ctx context.Context, name, environment string) (*domain.Model, error) {
	query := `
		SELECT model_id, model_name, deployment_endpoint, deployment_environment,
			deployment_namespace, deployment_status, deployment_target, revision,
			cost_per_1k_tokens, metadata, created_at, updated_at
		FROM model_registry_entries
		WHERE model_name = $1 AND deployment_environment = $2
	`

	var model domain.Model
	var metadataJSON []byte

	err := r.db.Pool().QueryRow(ctx, query, name, environment).Scan(
		&model.ModelID, &model.ModelName, &model.DeploymentEndpoint, &model.DeploymentEnvironment,
		&model.DeploymentNamespace, &model.DeploymentStatus, &model.DeploymentTarget, &model.Revision,
		&model.CostPer1kTokens, &metadataJSON, &model.CreatedAt, &model.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get model: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &model.Metadata); err != nil {
		model.Metadata = make(map[string]interface{})
	}

	return &model, nil
}

// List retrieves models with filtering and pagination
func (r *ModelRepository) List(ctx context.Context, params domain.ModelListParams) (*domain.ModelListResponse, error) {
	// Build query
	baseQuery := `FROM model_registry_entries WHERE 1=1`
	args := []interface{}{}
	argNum := 1

	if params.Environment != "" {
		baseQuery += fmt.Sprintf(" AND deployment_environment = $%d", argNum)
		args = append(args, params.Environment)
		argNum++
	}

	if params.Status != "" {
		baseQuery += fmt.Sprintf(" AND deployment_status = $%d", argNum)
		args = append(args, params.Status)
		argNum++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := r.db.Pool().QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count models: %w", err)
	}

	// Fetch page
	limit := params.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	selectQuery := `
		SELECT model_id, model_name, deployment_endpoint, deployment_environment,
			deployment_namespace, deployment_status, deployment_target, revision,
			cost_per_1k_tokens, metadata, created_at, updated_at
	` + baseQuery + fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argNum, argNum+1)

	args = append(args, limit, params.Offset)

	rows, err := r.db.Pool().Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list models: %w", err)
	}
	defer rows.Close()

	models := []domain.Model{}
	for rows.Next() {
		var model domain.Model
		var metadataJSON []byte

		if err := rows.Scan(
			&model.ModelID, &model.ModelName, &model.DeploymentEndpoint, &model.DeploymentEnvironment,
			&model.DeploymentNamespace, &model.DeploymentStatus, &model.DeploymentTarget, &model.Revision,
			&model.CostPer1kTokens, &metadataJSON, &model.CreatedAt, &model.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan model: %w", err)
		}

		if err := json.Unmarshal(metadataJSON, &model.Metadata); err != nil {
			model.Metadata = make(map[string]interface{})
		}

		models = append(models, model)
	}

	response := &domain.ModelListResponse{
		Models: models,
		Pagination: domain.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: params.Offset,
		},
	}

	if params.Offset+len(models) < total {
		nextOffset := params.Offset + limit
		response.Pagination.NextOffset = &nextOffset
	}

	return response, nil
}

// Update updates specific fields of a model
func (r *ModelRepository) Update(ctx context.Context, name, environment string, update *domain.ModelUpdate) (*domain.Model, error) {
	// Build dynamic update query
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argNum := 1

	if update.DeploymentEndpoint != nil {
		setClauses = append(setClauses, fmt.Sprintf("deployment_endpoint = $%d", argNum))
		args = append(args, *update.DeploymentEndpoint)
		argNum++
	}
	if update.DeploymentStatus != nil {
		setClauses = append(setClauses, fmt.Sprintf("deployment_status = $%d", argNum))
		args = append(args, *update.DeploymentStatus)
		argNum++
	}
	if update.DeploymentTarget != nil {
		setClauses = append(setClauses, fmt.Sprintf("deployment_target = $%d", argNum))
		args = append(args, *update.DeploymentTarget)
		argNum++
	}
	if update.Revision != nil {
		setClauses = append(setClauses, fmt.Sprintf("revision = $%d", argNum))
		args = append(args, *update.Revision)
		argNum++
	}
	if update.CostPer1kTokens != nil {
		setClauses = append(setClauses, fmt.Sprintf("cost_per_1k_tokens = $%d", argNum))
		args = append(args, *update.CostPer1kTokens)
		argNum++
	}
	if update.Metadata != nil {
		metadata, _ := json.Marshal(update.Metadata)
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", argNum))
		args = append(args, metadata)
		argNum++
	}

	args = append(args, name, environment)

	query := fmt.Sprintf(`
		UPDATE model_registry_entries SET %s
		WHERE model_name = $%d AND deployment_environment = $%d
		RETURNING model_id, model_name, deployment_endpoint, deployment_environment,
			deployment_namespace, deployment_status, deployment_target, revision,
			cost_per_1k_tokens, metadata, created_at, updated_at
	`, joinStrings(setClauses, ", "), argNum, argNum+1)

	var model domain.Model
	var metadataJSON []byte

	err := r.db.Pool().QueryRow(ctx, query, args...).Scan(
		&model.ModelID, &model.ModelName, &model.DeploymentEndpoint, &model.DeploymentEnvironment,
		&model.DeploymentNamespace, &model.DeploymentStatus, &model.DeploymentTarget, &model.Revision,
		&model.CostPer1kTokens, &metadataJSON, &model.CreatedAt, &model.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update model: %w", err)
	}

	if err := json.Unmarshal(metadataJSON, &model.Metadata); err != nil {
		model.Metadata = make(map[string]interface{})
	}

	return &model, nil
}

// Delete removes a model
func (r *ModelRepository) Delete(ctx context.Context, name, environment string) error {
	query := `DELETE FROM model_registry_entries WHERE model_name = $1 AND deployment_environment = $2`
	_, err := r.db.Pool().Exec(ctx, query, name, environment)
	return err
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

