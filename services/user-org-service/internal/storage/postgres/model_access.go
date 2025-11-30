package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// GetUserModelAccess retrieves the user's access mode and all granted models.
func (s *Store) GetUserModelAccess(ctx context.Context, orgID, userID uuid.UUID) (UserModelAccessInfo, error) {
	var info UserModelAccessInfo
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		// Get access mode (default to "restricted" if no record exists)
		var accessMode string
		err := tx.QueryRow(ctx, `
			SELECT access_mode FROM user_model_access
			WHERE org_id = $1 AND user_id = $2
		`, orgID, userID).Scan(&accessMode)
		if err != nil {
			if err == pgx.ErrNoRows {
				// Default to restricted mode if no record exists
				accessMode = "restricted"
			} else {
				return err
			}
		}
		info.AccessMode = accessMode

		// Get all active grants (not expired)
		rows, err := tx.Query(ctx, `
			SELECT id, org_id, user_id, model_name, granted_at, granted_by, expires_at
			FROM user_model_grants
			WHERE org_id = $1 AND user_id = $2
			AND (expires_at IS NULL OR expires_at > NOW())
			ORDER BY granted_at DESC
		`, orgID, userID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			grant, err := scanUserModelGrant(rows)
			if err != nil {
				return err
			}
			info.GrantedModels = append(info.GrantedModels, grant)
		}
		if info.GrantedModels == nil {
			info.GrantedModels = []UserModelGrant{}
		}
		return rows.Err()
	})
	return info, err
}

// SetUserAccessMode sets or creates the access mode for a user.
func (s *Store) SetUserAccessMode(ctx context.Context, orgID, userID uuid.UUID, mode string) (UserModelAccess, error) {
	var out UserModelAccess
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO user_model_access (org_id, user_id, access_mode)
			VALUES ($1, $2, $3)
			ON CONFLICT (org_id, user_id) DO UPDATE
			SET access_mode = EXCLUDED.access_mode,
			    version = user_model_access.version + 1
			RETURNING id, org_id, user_id, access_mode, version, created_at, updated_at
		`, orgID, userID, mode)

		access, err := scanUserModelAccess(row)
		if err != nil {
			return err
		}
		out = access
		return nil
	})
	return out, err
}

// CreateModelGrant creates a new model grant for a user.
// Returns ErrConflict if grant already exists.
func (s *Store) CreateModelGrant(ctx context.Context, params CreateModelGrantParams) (UserModelGrant, error) {
	var out UserModelGrant
	err := s.withTenantTx(ctx, params.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		// Check for existing grant
		var existingID uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT id FROM user_model_grants
			WHERE org_id = $1 AND user_id = $2 AND model_name = $3
		`, params.OrgID, params.UserID, params.ModelName).Scan(&existingID)
		if err == nil {
			return ErrConflict
		}
		if err != pgx.ErrNoRows {
			return err
		}

		// Create the grant
		row := tx.QueryRow(ctx, `
			INSERT INTO user_model_grants (org_id, user_id, model_name, granted_by, expires_at)
			VALUES ($1, $2, $3, $4, $5)
			RETURNING id, org_id, user_id, model_name, granted_at, granted_by, expires_at
		`, params.OrgID, params.UserID, params.ModelName, params.GrantedBy, params.ExpiresAt)

		grant, err := scanUserModelGrant(row)
		if err != nil {
			return err
		}
		out = grant
		return nil
	})
	return out, err
}

// DeleteModelGrant removes a model grant for a user.
// Returns ErrNotFound if grant doesn't exist.
func (s *Store) DeleteModelGrant(ctx context.Context, orgID, userID uuid.UUID, modelName string) error {
	return s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		cmd, err := tx.Exec(ctx, `
			DELETE FROM user_model_grants
			WHERE org_id = $1 AND user_id = $2 AND model_name = $3
		`, orgID, userID, modelName)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ListModelGrants lists all active grants for a user.
func (s *Store) ListModelGrants(ctx context.Context, orgID, userID uuid.UUID) ([]UserModelGrant, error) {
	var out []UserModelGrant
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, org_id, user_id, model_name, granted_at, granted_by, expires_at
			FROM user_model_grants
			WHERE org_id = $1 AND user_id = $2
			AND (expires_at IS NULL OR expires_at > NOW())
			ORDER BY granted_at DESC
		`, orgID, userID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			grant, err := scanUserModelGrant(rows)
			if err != nil {
				return err
			}
			out = append(out, grant)
		}
		return rows.Err()
	})
	if out == nil {
		out = []UserModelGrant{}
	}
	return out, err
}

// CheckModelAccess checks if a user has access to a specific model.
// This is the hot path query - optimized for performance.
// Returns: (hasAccess bool, accessMode string, err error)
func (s *Store) CheckModelAccess(ctx context.Context, orgID, userID uuid.UUID, modelName string) (bool, string, error) {
	var hasAccess bool
	var accessMode string

	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		// Single query to check access mode and grant status
		err := tx.QueryRow(ctx, `
			WITH user_access AS (
				SELECT access_mode
				FROM user_model_access
				WHERE org_id = $1 AND user_id = $2
			)
			SELECT
				COALESCE((SELECT access_mode FROM user_access), 'restricted') as access_mode,
				CASE
					WHEN (SELECT access_mode FROM user_access) = 'auto_grant' THEN true
					ELSE EXISTS (
						SELECT 1 FROM user_model_grants
						WHERE org_id = $1 AND user_id = $2 AND model_name = $3
						AND (expires_at IS NULL OR expires_at > NOW())
					)
				END as has_access
		`, orgID, userID, modelName).Scan(&accessMode, &hasAccess)
		return err
	})
	return hasAccess, accessMode, err
}

// GrantAllCurrentModels grants all available models to a user (snapshot).
// Uses the provided model list (caller should fetch from routing_policies or model registry).
// Returns the number of NEW grants created (excludes already-existing grants).
func (s *Store) GrantAllCurrentModels(ctx context.Context, orgID, userID uuid.UUID, grantedBy *uuid.UUID, modelNames []string) (int, error) {
	var count int
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		for _, modelName := range modelNames {
			cmd, err := tx.Exec(ctx, `
				INSERT INTO user_model_grants (org_id, user_id, model_name, granted_by)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (org_id, user_id, model_name) DO NOTHING
			`, orgID, userID, modelName, grantedBy)
			if err != nil {
				return err
			}
			// Only count rows that were actually inserted (not conflicts)
			count += int(cmd.RowsAffected())
		}
		return nil
	})
	return count, err
}

// GetGrantedModelNames returns just the model names for a user (used for auth response).
func (s *Store) GetGrantedModelNames(ctx context.Context, orgID, userID uuid.UUID) ([]string, error) {
	var out []string
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT model_name
			FROM user_model_grants
			WHERE org_id = $1 AND user_id = $2
			AND (expires_at IS NULL OR expires_at > NOW())
		`, orgID, userID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var name string
			if err := rows.Scan(&name); err != nil {
				return err
			}
			out = append(out, name)
		}
		return rows.Err()
	})
	if out == nil {
		out = []string{}
	}
	return out, err
}

// GetUserAccessMode returns the access mode for a user (or "restricted" if not set).
func (s *Store) GetUserAccessMode(ctx context.Context, orgID, userID uuid.UUID) (string, error) {
	var accessMode string
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			SELECT access_mode FROM user_model_access
			WHERE org_id = $1 AND user_id = $2
		`, orgID, userID).Scan(&accessMode)
		if err != nil {
			if err == pgx.ErrNoRows {
				accessMode = "restricted"
				return nil
			}
			return err
		}
		return nil
	})
	return accessMode, err
}

// scan helpers

func scanUserModelAccess(row pgx.Row) (UserModelAccess, error) {
	var a UserModelAccess
	err := row.Scan(
		&a.ID,
		&a.OrgID,
		&a.UserID,
		&a.AccessMode,
		&a.Version,
		&a.CreatedAt,
		&a.UpdatedAt,
	)
	return a, err
}

func scanUserModelGrant(row pgx.Row) (UserModelGrant, error) {
	var (
		g         UserModelGrant
		grantedBy pgtype.UUID
		expiresAt pgtype.Timestamptz
	)
	err := row.Scan(
		&g.ID,
		&g.OrgID,
		&g.UserID,
		&g.ModelName,
		&g.GrantedAt,
		&grantedBy,
		&expiresAt,
	)
	if err != nil {
		return UserModelGrant{}, err
	}
	g.GrantedBy = uuidPtr(grantedBy)
	g.ExpiresAt = timePtr(expiresAt)
	return g, nil
}
