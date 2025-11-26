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

// PolicyRepository handles routing policy database operations
type PolicyRepository struct {
	db *DB
}

// NewPolicyRepository creates a new policy repository
func NewPolicyRepository(db *DB) *PolicyRepository {
	return &PolicyRepository{db: db}
}

// Create creates a new routing policy
func (r *PolicyRepository) Create(ctx context.Context, create *domain.PolicyCreate, createdBy string) (*domain.RoutingPolicy, error) {
	policyID := create.PolicyID
	if policyID == "" {
		policyID = uuid.New().String()
	}

	orgID := create.OrganizationID
	if orgID == "" {
		orgID = "*"
	}

	failoverThreshold := create.FailoverThreshold
	if failoverThreshold == 0 {
		failoverThreshold = 3
	}

	enabled := true
	if create.Enabled != nil {
		enabled = *create.Enabled
	}

	backendsJSON, _ := json.Marshal(create.Backends)
	fallbackJSON, _ := json.Marshal(create.FallbackBackends)
	metadataJSON, _ := json.Marshal(create.Metadata)

	now := time.Now().UTC()

	query := `
		INSERT INTO routing_policies (
			policy_id, organization_id, model, backends, fallback_backends,
			failover_threshold, enabled, version, metadata,
			created_at, updated_at, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, $9, $9, $10, $10)
		RETURNING policy_id, organization_id, model, backends, fallback_backends,
			failover_threshold, enabled, version, metadata, created_at, updated_at,
			created_by, updated_by
	`

	var policy domain.RoutingPolicy
	var backends, fallback, metadata []byte

	err := r.db.Pool().QueryRow(ctx, query,
		policyID, orgID, create.Model, backendsJSON, fallbackJSON,
		failoverThreshold, enabled, metadataJSON, now, createdBy,
	).Scan(
		&policy.PolicyID, &policy.OrganizationID, &policy.Model,
		&backends, &fallback, &policy.FailoverThreshold, &policy.Enabled,
		&policy.Version, &metadata, &policy.CreatedAt, &policy.UpdatedAt,
		&policy.CreatedBy, &policy.UpdatedBy,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create policy: %w", err)
	}

	json.Unmarshal(backends, &policy.Backends)
	json.Unmarshal(fallback, &policy.FallbackBackends)
	json.Unmarshal(metadata, &policy.Metadata)

	return &policy, nil
}

// GetByID retrieves a policy by ID
func (r *PolicyRepository) GetByID(ctx context.Context, id string) (*domain.RoutingPolicy, error) {
	query := `
		SELECT policy_id, organization_id, model, backends, fallback_backends,
			failover_threshold, enabled, version, metadata, created_at, updated_at,
			created_by, updated_by
		FROM routing_policies
		WHERE policy_id = $1 AND deleted_at IS NULL
	`

	var policy domain.RoutingPolicy
	var backends, fallback, metadata []byte

	err := r.db.Pool().QueryRow(ctx, query, id).Scan(
		&policy.PolicyID, &policy.OrganizationID, &policy.Model,
		&backends, &fallback, &policy.FailoverThreshold, &policy.Enabled,
		&policy.Version, &metadata, &policy.CreatedAt, &policy.UpdatedAt,
		&policy.CreatedBy, &policy.UpdatedBy,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get policy: %w", err)
	}

	json.Unmarshal(backends, &policy.Backends)
	json.Unmarshal(fallback, &policy.FallbackBackends)
	json.Unmarshal(metadata, &policy.Metadata)

	return &policy, nil
}

// List retrieves policies with filtering and pagination
func (r *PolicyRepository) List(ctx context.Context, params domain.PolicyListParams) (*domain.PolicyListResponse, error) {
	baseQuery := `FROM routing_policies WHERE deleted_at IS NULL`
	args := []interface{}{}
	argNum := 1

	if params.Model != "" {
		baseQuery += fmt.Sprintf(" AND model = $%d", argNum)
		args = append(args, params.Model)
		argNum++
	}

	if params.OrganizationID != "" {
		baseQuery += fmt.Sprintf(" AND organization_id = $%d", argNum)
		args = append(args, params.OrganizationID)
		argNum++
	}

	if params.Enabled != nil {
		baseQuery += fmt.Sprintf(" AND enabled = $%d", argNum)
		args = append(args, *params.Enabled)
		argNum++
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := r.db.Pool().QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count policies: %w", err)
	}

	// Fetch page
	limit := params.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	selectQuery := `
		SELECT policy_id, organization_id, model, backends, fallback_backends,
			failover_threshold, enabled, version, metadata, created_at, updated_at,
			created_by, updated_by
	` + baseQuery + fmt.Sprintf(" ORDER BY updated_at DESC LIMIT $%d OFFSET $%d", argNum, argNum+1)

	args = append(args, limit, params.Offset)

	rows, err := r.db.Pool().Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list policies: %w", err)
	}
	defer rows.Close()

	policies := []domain.RoutingPolicy{}
	for rows.Next() {
		var policy domain.RoutingPolicy
		var backends, fallback, metadata []byte

		if err := rows.Scan(
			&policy.PolicyID, &policy.OrganizationID, &policy.Model,
			&backends, &fallback, &policy.FailoverThreshold, &policy.Enabled,
			&policy.Version, &metadata, &policy.CreatedAt, &policy.UpdatedAt,
			&policy.CreatedBy, &policy.UpdatedBy,
		); err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}

		json.Unmarshal(backends, &policy.Backends)
		json.Unmarshal(fallback, &policy.FallbackBackends)
		json.Unmarshal(metadata, &policy.Metadata)

		policies = append(policies, policy)
	}

	response := &domain.PolicyListResponse{
		Policies: policies,
		Pagination: domain.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: params.Offset,
		},
	}

	if params.Offset+len(policies) < total {
		nextOffset := params.Offset + limit
		response.Pagination.NextOffset = &nextOffset
	}

	return response, nil
}

// Update updates a policy and increments version
func (r *PolicyRepository) Update(ctx context.Context, id string, update *domain.PolicyUpdate, updatedBy string) (*domain.RoutingPolicy, error) {
	setClauses := []string{"updated_at = NOW()", "version = version + 1", fmt.Sprintf("updated_by = $1")}
	args := []interface{}{updatedBy}
	argNum := 2

	if update.Backends != nil {
		backendsJSON, _ := json.Marshal(update.Backends)
		setClauses = append(setClauses, fmt.Sprintf("backends = $%d", argNum))
		args = append(args, backendsJSON)
		argNum++
	}

	if update.FallbackBackends != nil {
		fallbackJSON, _ := json.Marshal(update.FallbackBackends)
		setClauses = append(setClauses, fmt.Sprintf("fallback_backends = $%d", argNum))
		args = append(args, fallbackJSON)
		argNum++
	}

	if update.FailoverThreshold != nil {
		setClauses = append(setClauses, fmt.Sprintf("failover_threshold = $%d", argNum))
		args = append(args, *update.FailoverThreshold)
		argNum++
	}

	if update.Enabled != nil {
		setClauses = append(setClauses, fmt.Sprintf("enabled = $%d", argNum))
		args = append(args, *update.Enabled)
		argNum++
	}

	if update.Metadata != nil {
		metadataJSON, _ := json.Marshal(update.Metadata)
		setClauses = append(setClauses, fmt.Sprintf("metadata = $%d", argNum))
		args = append(args, metadataJSON)
		argNum++
	}

	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE routing_policies SET %s
		WHERE policy_id = $%d AND deleted_at IS NULL
		RETURNING policy_id, organization_id, model, backends, fallback_backends,
			failover_threshold, enabled, version, metadata, created_at, updated_at,
			created_by, updated_by
	`, joinStrings(setClauses, ", "), argNum)

	var policy domain.RoutingPolicy
	var backends, fallback, metadata []byte

	err := r.db.Pool().QueryRow(ctx, query, args...).Scan(
		&policy.PolicyID, &policy.OrganizationID, &policy.Model,
		&backends, &fallback, &policy.FailoverThreshold, &policy.Enabled,
		&policy.Version, &metadata, &policy.CreatedAt, &policy.UpdatedAt,
		&policy.CreatedBy, &policy.UpdatedBy,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update policy: %w", err)
	}

	json.Unmarshal(backends, &policy.Backends)
	json.Unmarshal(fallback, &policy.FallbackBackends)
	json.Unmarshal(metadata, &policy.Metadata)

	return &policy, nil
}

// Delete soft-deletes a policy
func (r *PolicyRepository) Delete(ctx context.Context, id string) error {
	query := `UPDATE routing_policies SET deleted_at = NOW() WHERE policy_id = $1 AND deleted_at IS NULL`
	_, err := r.db.Pool().Exec(ctx, query, id)
	return err
}

// SetEnabled enables or disables a policy
func (r *PolicyRepository) SetEnabled(ctx context.Context, id string, enabled bool, updatedBy string) (*domain.RoutingPolicy, error) {
	query := `
		UPDATE routing_policies SET enabled = $1, updated_at = NOW(), updated_by = $2, version = version + 1
		WHERE policy_id = $3 AND deleted_at IS NULL
		RETURNING policy_id, organization_id, model, backends, fallback_backends,
			failover_threshold, enabled, version, metadata, created_at, updated_at,
			created_by, updated_by
	`

	var policy domain.RoutingPolicy
	var backends, fallback, metadata []byte

	err := r.db.Pool().QueryRow(ctx, query, enabled, updatedBy, id).Scan(
		&policy.PolicyID, &policy.OrganizationID, &policy.Model,
		&backends, &fallback, &policy.FailoverThreshold, &policy.Enabled,
		&policy.Version, &metadata, &policy.CreatedAt, &policy.UpdatedAt,
		&policy.CreatedBy, &policy.UpdatedBy,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to set policy enabled: %w", err)
	}

	json.Unmarshal(backends, &policy.Backends)
	json.Unmarshal(fallback, &policy.FallbackBackends)
	json.Unmarshal(metadata, &policy.Metadata)

	return &policy, nil
}

// GetForSync retrieves policies changed since a version for syncing
func (r *PolicyRepository) GetForSync(ctx context.Context, sinceVersion int, environment string) (*domain.PolicySyncResponse, error) {
	query := `
		SELECT policy_id, organization_id, model, backends, fallback_backends,
			failover_threshold, enabled, version, metadata, created_at, updated_at,
			created_by, updated_by, deleted_at
		FROM routing_policies
		WHERE version > $1
		ORDER BY version ASC
	`

	rows, err := r.db.Pool().Query(ctx, query, sinceVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to get policies for sync: %w", err)
	}
	defer rows.Close()

	items := []domain.PolicySyncItem{}
	maxVersion := sinceVersion

	for rows.Next() {
		var policy domain.RoutingPolicy
		var backends, fallback, metadata []byte
		var deletedAt *time.Time

		if err := rows.Scan(
			&policy.PolicyID, &policy.OrganizationID, &policy.Model,
			&backends, &fallback, &policy.FailoverThreshold, &policy.Enabled,
			&policy.Version, &metadata, &policy.CreatedAt, &policy.UpdatedAt,
			&policy.CreatedBy, &policy.UpdatedBy, &deletedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan policy: %w", err)
		}

		json.Unmarshal(backends, &policy.Backends)
		json.Unmarshal(fallback, &policy.FallbackBackends)
		json.Unmarshal(metadata, &policy.Metadata)

		item := domain.PolicySyncItem{
			RoutingPolicy: policy,
			Deleted:       deletedAt != nil,
		}
		items = append(items, item)

		if policy.Version > maxVersion {
			maxVersion = policy.Version
		}
	}

	now := time.Now().UTC()
	return &domain.PolicySyncResponse{
		Policies: items,
		SyncMetadata: domain.SyncMetadata{
			ServerTime:            now,
			MaxVersion:            maxVersion,
			NextSyncRecommendedAt: now.Add(30 * time.Second),
		},
	}, nil
}

