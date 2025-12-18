package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
)

// RecipeRepositoryInterface defines the interface for recipe database operations
type RecipeRepositoryInterface interface {
	Create(ctx context.Context, create *domain.RecipeCreate) (*domain.Recipe, error)
	Get(ctx context.Context, name string) (*domain.Recipe, error)
	List(ctx context.Context, params domain.RecipeListParams) (*domain.RecipeListResponse, error)
	Update(ctx context.Context, name string, update *domain.RecipeUpdate) (*domain.Recipe, error)
	Delete(ctx context.Context, name string) error
}

// RecipeRepository handles recipe database operations
type RecipeRepository struct {
	db *DB
}

// NewRecipeRepository creates a new recipe repository
func NewRecipeRepository(db *DB) *RecipeRepository {
	return &RecipeRepository{db: db}
}

// Create creates a new recipe
func (r *RecipeRepository) Create(ctx context.Context, create *domain.RecipeCreate) (*domain.Recipe, error) {
	now := time.Now().UTC()
	recipe := &domain.Recipe{
		ID:          uuid.New(),
		Name:        create.Name,
		DisplayName: create.DisplayName,
		Description: create.Description,
		ModelID:     create.ModelID,
		Runtime:     create.Runtime,
		Spec:        create.Spec,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	query := `
		INSERT INTO recipes (
			id, name, display_name, description, model_id, runtime, spec, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	_, err := r.db.Pool().Exec(ctx, query,
		recipe.ID, recipe.Name, recipe.DisplayName, recipe.Description,
		recipe.ModelID, recipe.Runtime, recipe.Spec, recipe.CreatedAt, recipe.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipe: %w", err)
	}

	return recipe, nil
}

// Get retrieves a recipe by name
func (r *RecipeRepository) Get(ctx context.Context, name string) (*domain.Recipe, error) {
	query := `
		SELECT id, name, display_name, description, model_id, runtime, spec, created_at, updated_at
		FROM recipes
		WHERE name = $1
	`

	var recipe domain.Recipe
	err := r.db.Pool().QueryRow(ctx, query, name).Scan(
		&recipe.ID, &recipe.Name, &recipe.DisplayName, &recipe.Description,
		&recipe.ModelID, &recipe.Runtime, &recipe.Spec, &recipe.CreatedAt, &recipe.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get recipe: %w", err)
	}

	return &recipe, nil
}

// List retrieves recipes with pagination and filtering
func (r *RecipeRepository) List(ctx context.Context, params domain.RecipeListParams) (*domain.RecipeListResponse, error) {
	// Set default and max limits
	limit := params.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	// Build query with optional runtime filter
	var conditions []string
	var args []interface{}
	argIndex := 1

	if params.Runtime != "" {
		conditions = append(conditions, fmt.Sprintf("runtime = $%d", argIndex))
		args = append(args, params.Runtime)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Get total count
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM recipes %s", whereClause)
	var total int
	err := r.db.Pool().QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, fmt.Errorf("failed to count recipes: %w", err)
	}

	// Get paginated results
	dataQuery := fmt.Sprintf(`
		SELECT id, name, display_name, description, model_id, runtime, spec, created_at, updated_at
		FROM recipes
		%s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)

	args = append(args, limit, params.Offset)

	rows, err := r.db.Pool().Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list recipes: %w", err)
	}
	defer rows.Close()

	recipes := make([]domain.Recipe, 0)
	for rows.Next() {
		var recipe domain.Recipe
		err := rows.Scan(
			&recipe.ID, &recipe.Name, &recipe.DisplayName, &recipe.Description,
			&recipe.ModelID, &recipe.Runtime, &recipe.Spec, &recipe.CreatedAt, &recipe.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan recipe: %w", err)
		}
		recipes = append(recipes, recipe)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recipes: %w", err)
	}

	// Build pagination metadata
	pagination := domain.Pagination{
		Total:  total,
		Limit:  limit,
		Offset: params.Offset,
	}

	// Calculate next offset if there are more results
	if params.Offset+limit < total {
		nextOffset := params.Offset + limit
		pagination.NextOffset = &nextOffset
	}

	return &domain.RecipeListResponse{
		Recipes:    recipes,
		Pagination: pagination,
	}, nil
}

// Update updates a recipe
func (r *RecipeRepository) Update(ctx context.Context, name string, update *domain.RecipeUpdate) (*domain.Recipe, error) {
	// Check if recipe exists
	existing, err := r.Get(ctx, name)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, nil
	}

	// Build dynamic update query based on provided fields
	var setClauses []string
	var args []interface{}
	argIndex := 1

	if update.DisplayName != nil {
		setClauses = append(setClauses, fmt.Sprintf("display_name = $%d", argIndex))
		args = append(args, *update.DisplayName)
		argIndex++
	}
	if update.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argIndex))
		args = append(args, *update.Description)
		argIndex++
	}
	if update.Runtime != nil {
		setClauses = append(setClauses, fmt.Sprintf("runtime = $%d", argIndex))
		args = append(args, *update.Runtime)
		argIndex++
	}
	if update.Spec != nil {
		setClauses = append(setClauses, fmt.Sprintf("spec = $%d", argIndex))
		args = append(args, *update.Spec)
		argIndex++
	}

	if len(setClauses) == 0 {
		// No fields to update, return existing
		return existing, nil
	}

	// Add updated_at (automatically set by trigger, but we'll return the value)
	// Add name to args for WHERE clause
	args = append(args, name)

	query := fmt.Sprintf(`
		UPDATE recipes
		SET %s
		WHERE name = $%d
		RETURNING id, name, display_name, description, model_id, runtime, spec, created_at, updated_at
	`, strings.Join(setClauses, ", "), argIndex)

	var recipe domain.Recipe
	err = r.db.Pool().QueryRow(ctx, query, args...).Scan(
		&recipe.ID, &recipe.Name, &recipe.DisplayName, &recipe.Description,
		&recipe.ModelID, &recipe.Runtime, &recipe.Spec, &recipe.CreatedAt, &recipe.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update recipe: %w", err)
	}

	return &recipe, nil
}

// Delete deletes a recipe by name
func (r *RecipeRepository) Delete(ctx context.Context, name string) error {
	query := `DELETE FROM recipes WHERE name = $1`

	result, err := r.db.Pool().Exec(ctx, query, name)
	if err != nil {
		return fmt.Errorf("failed to delete recipe: %w", err)
	}

	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}

	return nil
}
