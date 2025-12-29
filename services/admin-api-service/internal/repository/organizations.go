package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
)

// OrganizationRepository handles organization database operations
type OrganizationRepository struct {
	db *DB
}

// NewOrganizationRepository creates a new organization repository
func NewOrganizationRepository(db *DB) *OrganizationRepository {
	return &OrganizationRepository{db: db}
}

// Create creates a new organization
func (r *OrganizationRepository) Create(ctx context.Context, create *domain.OrganizationCreate) (*domain.Organization, error) {
	planTier := create.PlanTier
	if planTier == "" {
		planTier = "free"
	}

	org := &domain.Organization{
		OrganizationID:    uuid.New(),
		Slug:              create.Slug,
		DisplayName:       create.DisplayName,
		PlanTier:          planTier,
		Status:            "active",
		BudgetLimitTokens: create.BudgetLimitTokens,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}

	query := `
		INSERT INTO organizations (
			organization_id, slug, display_name, plan_tier, status,
			budget_limit_tokens, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool().Exec(ctx, query,
		org.OrganizationID, org.Slug, org.DisplayName, org.PlanTier,
		org.Status, org.BudgetLimitTokens, org.CreatedAt, org.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}

	return org, nil
}

// GetByID retrieves an organization by ID
func (r *OrganizationRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	query := `
		SELECT organization_id, slug, display_name, plan_tier, status,
			budget_limit_tokens, created_at, updated_at
		FROM organizations
		WHERE organization_id = $1
	`

	var org domain.Organization
	err := r.db.Pool().QueryRow(ctx, query, id).Scan(
		&org.OrganizationID, &org.Slug, &org.DisplayName, &org.PlanTier,
		&org.Status, &org.BudgetLimitTokens, &org.CreatedAt, &org.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return &org, nil
}

// GetBySlug retrieves an organization by slug
func (r *OrganizationRepository) GetBySlug(ctx context.Context, slug string) (*domain.Organization, error) {
	query := `
		SELECT organization_id, slug, display_name, plan_tier, status,
			budget_limit_tokens, created_at, updated_at
		FROM organizations
		WHERE slug = $1
	`

	var org domain.Organization
	err := r.db.Pool().QueryRow(ctx, query, slug).Scan(
		&org.OrganizationID, &org.Slug, &org.DisplayName, &org.PlanTier,
		&org.Status, &org.BudgetLimitTokens, &org.CreatedAt, &org.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}

	return &org, nil
}

// List retrieves organizations with pagination
func (r *OrganizationRepository) List(ctx context.Context, params domain.OrganizationListParams) (*domain.OrganizationListResponse, error) {
	// Count total
	var total int
	if err := r.db.Pool().QueryRow(ctx, "SELECT COUNT(*) FROM organizations").Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count organizations: %w", err)
	}

	// Fetch page
	limit := params.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	query := `
		SELECT organization_id, slug, display_name, plan_tier, status,
			budget_limit_tokens, created_at, updated_at
		FROM organizations
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.db.Pool().Query(ctx, query, limit, params.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list organizations: %w", err)
	}
	defer rows.Close()

	orgs := []domain.Organization{}
	for rows.Next() {
		var org domain.Organization
		if err := rows.Scan(
			&org.OrganizationID, &org.Slug, &org.DisplayName, &org.PlanTier,
			&org.Status, &org.BudgetLimitTokens, &org.CreatedAt, &org.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan organization: %w", err)
		}
		orgs = append(orgs, org)
	}

	response := &domain.OrganizationListResponse{
		Organizations: orgs,
		Pagination: domain.Pagination{
			Total:  total,
			Limit:  limit,
			Offset: params.Offset,
		},
	}

	if params.Offset+len(orgs) < total {
		nextOffset := params.Offset + limit
		response.Pagination.NextOffset = &nextOffset
	}

	return response, nil
}

// Update updates specific fields of an organization
func (r *OrganizationRepository) Update(ctx context.Context, id uuid.UUID, update *domain.OrganizationUpdate) (*domain.Organization, error) {
	setClauses := []string{"updated_at = NOW()"}
	args := []interface{}{}
	argNum := 1

	if update.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argNum))
		args = append(args, *update.DisplayName)
		argNum++
	}
	if update.PlanTier != nil {
		setClauses = append(setClauses, fmt.Sprintf("plan_tier = $%d", argNum))
		args = append(args, *update.PlanTier)
		argNum++
	}
	if update.Status != nil {
		setClauses = append(setClauses, fmt.Sprintf("status = $%d", argNum))
		args = append(args, *update.Status)
		argNum++
	}
	if update.BudgetLimitTokens != nil {
		setClauses = append(setClauses, fmt.Sprintf("budget_limit_tokens = $%d", argNum))
		args = append(args, *update.BudgetLimitTokens)
		argNum++
	}

	args = append(args, id)

	query := fmt.Sprintf(`
		UPDATE organizations SET %s
		WHERE organization_id = $%d
		RETURNING organization_id, slug, display_name, plan_tier, status,
			budget_limit_tokens, created_at, updated_at
	`, joinStrings(setClauses, ", "), argNum)

	var org domain.Organization
	err := r.db.Pool().QueryRow(ctx, query, args...).Scan(
		&org.OrganizationID, &org.Slug, &org.DisplayName, &org.PlanTier,
		&org.Status, &org.BudgetLimitTokens, &org.CreatedAt, &org.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update organization: %w", err)
	}

	return &org, nil
}
