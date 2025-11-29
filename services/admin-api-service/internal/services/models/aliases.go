// Package models provides the business logic for model management operations.
package models

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrAliasNotFound indicates the requested alias was not found
var ErrAliasNotFound = errors.New("alias not found")

// ErrAliasExists indicates an alias with the same name already exists
var ErrAliasExists = errors.New("alias already exists")

// Alias represents a model alias
type Alias struct {
	ID          uuid.UUID
	AliasName   string
	ModelID     uuid.UUID
	ModelName   string // populated from join
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ListAliases returns all aliases
func (s *Service) ListAliases(ctx context.Context) ([]Alias, error) {
	query := `
		SELECT a.id, a.alias_name, a.model_id, r.name, a.description, a.created_at, a.updated_at
		FROM model_aliases a
		JOIN model_registry r ON a.model_id = r.id
		ORDER BY a.alias_name
	`

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query aliases: %w", err)
	}
	defer rows.Close()

	var aliases []Alias
	for rows.Next() {
		var a Alias
		var description *string
		err := rows.Scan(
			&a.ID, &a.AliasName, &a.ModelID, &a.ModelName, &description,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan alias: %w", err)
		}
		if description != nil {
			a.Description = *description
		}
		aliases = append(aliases, a)
	}

	return aliases, rows.Err()
}

// GetAlias returns an alias by name
func (s *Service) GetAlias(ctx context.Context, aliasName string) (*Alias, error) {
	query := `
		SELECT a.id, a.alias_name, a.model_id, r.name, a.description, a.created_at, a.updated_at
		FROM model_aliases a
		JOIN model_registry r ON a.model_id = r.id
		WHERE a.alias_name = $1
	`

	var a Alias
	var description *string
	err := s.pool.QueryRow(ctx, query, aliasName).Scan(
		&a.ID, &a.AliasName, &a.ModelID, &a.ModelName, &description,
		&a.CreatedAt, &a.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrAliasNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query alias: %w", err)
	}
	if description != nil {
		a.Description = *description
	}

	return &a, nil
}

// ResolveAlias resolves an alias to its target model name
// Returns the original name if not an alias
func (s *Service) ResolveAlias(ctx context.Context, name string) (string, error) {
	alias, err := s.GetAlias(ctx, name)
	if err == ErrAliasNotFound {
		// Not an alias, return as-is
		return name, nil
	}
	if err != nil {
		return "", err
	}
	return alias.ModelName, nil
}

// CreateAliasRequest contains data for creating an alias
type CreateAliasRequest struct {
	AliasName   string
	ModelName   string
	Description string
}

// CreateAlias creates a new alias
func (s *Service) CreateAlias(ctx context.Context, req CreateAliasRequest) (*Alias, error) {
	// Get the model ID
	model, err := s.GetModel(ctx, req.ModelName)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO model_aliases (alias_name, model_id, description)
		VALUES ($1, $2, $3)
		RETURNING id, created_at, updated_at
	`

	var description *string
	if req.Description != "" {
		description = &req.Description
	}

	a := Alias{
		AliasName:   req.AliasName,
		ModelID:     model.ID,
		ModelName:   model.Name,
		Description: req.Description,
	}

	err = s.pool.QueryRow(ctx, query,
		a.AliasName, a.ModelID, description,
	).Scan(&a.ID, &a.CreatedAt, &a.UpdatedAt)

	if err != nil {
		// Check for unique constraint violation using PostgreSQL error code
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // 23505 is unique_violation
			return nil, ErrAliasExists
		}
		return nil, fmt.Errorf("insert alias: %w", err)
	}

	return &a, nil
}

// UpdateAliasRequest contains data for updating an alias
type UpdateAliasRequest struct {
	ModelName   *string
	Description *string
}

// UpdateAlias updates an alias to point to a different model
func (s *Service) UpdateAlias(ctx context.Context, aliasName string, req UpdateAliasRequest) (*Alias, error) {
	// Get the current alias
	alias, err := s.GetAlias(ctx, aliasName)
	if err != nil {
		return nil, err
	}

	// Build update query
	updates := []string{}
	args := []interface{}{alias.ID}
	argNum := 2

	if req.ModelName != nil {
		// Get the new model ID
		model, err := s.GetModel(ctx, *req.ModelName)
		if err != nil {
			return nil, err
		}
		updates = append(updates, fmt.Sprintf("model_id = $%d", argNum))
		args = append(args, model.ID)
		argNum++
	}

	if req.Description != nil {
		updates = append(updates, fmt.Sprintf("description = $%d", argNum))
		args = append(args, *req.Description)
		argNum++
	}

	if len(updates) == 0 {
		return alias, nil
	}

	query := fmt.Sprintf(`
		UPDATE model_aliases
		SET %s
		WHERE id = $1
	`, strings.Join(updates, ", "))

	_, err = s.pool.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("update alias: %w", err)
	}

	// Return updated alias
	return s.GetAlias(ctx, aliasName)
}

// DeleteAlias removes an alias
func (s *Service) DeleteAlias(ctx context.Context, aliasName string) error {
	result, err := s.pool.Exec(ctx, `DELETE FROM model_aliases WHERE alias_name = $1`, aliasName)
	if err != nil {
		return fmt.Errorf("delete alias: %w", err)
	}

	if result.RowsAffected() == 0 {
		return ErrAliasNotFound
	}

	return nil
}

// GetAliasesForModel returns all aliases pointing to a model
func (s *Service) GetAliasesForModel(ctx context.Context, modelName string) ([]Alias, error) {
	query := `
		SELECT a.id, a.alias_name, a.model_id, r.name, a.description, a.created_at, a.updated_at
		FROM model_aliases a
		JOIN model_registry r ON a.model_id = r.id
		WHERE r.name = $1
		ORDER BY a.alias_name
	`

	rows, err := s.pool.Query(ctx, query, modelName)
	if err != nil {
		return nil, fmt.Errorf("query aliases for model: %w", err)
	}
	defer rows.Close()

	var aliases []Alias
	for rows.Next() {
		var a Alias
		var description *string
		err := rows.Scan(
			&a.ID, &a.AliasName, &a.ModelID, &a.ModelName, &description,
			&a.CreatedAt, &a.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan alias: %w", err)
		}
		if description != nil {
			a.Description = *description
		}
		aliases = append(aliases, a)
	}

	return aliases, rows.Err()
}

