package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// tokenPolicyColumns is the canonical column list for token_rate_limit_policies queries.
const tokenPolicyColumns = `id, org_id, name, description, limit_1h, limit_24h, limit_7d, is_builtin, created_at, updated_at`

// CreateTokenPolicy creates a new token rate-limit policy for an organization.
func (s *Store) CreateTokenPolicy(ctx context.Context, params CreateTokenPolicyParams) (TokenRateLimitPolicy, error) {
	// Validate: at least one limit must be specified
	if params.Limit1h == nil && params.Limit24h == nil && params.Limit7d == nil {
		return TokenRateLimitPolicy{}, fmt.Errorf("at least one limit (1h, 24h, or 7d) must be specified")
	}

	// Reserved name check
	if strings.EqualFold(params.Name, "No Token Rate-Limit") {
		return TokenRateLimitPolicy{}, ErrConflict
	}

	var out TokenRateLimitPolicy
	err := s.withTenantTx(ctx, params.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO token_rate_limit_policies (org_id, name, description, limit_1h, limit_24h, limit_7d)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING `+tokenPolicyColumns,
			params.OrgID,
			params.Name,
			params.Description,
			params.Limit1h,
			params.Limit24h,
			params.Limit7d,
		)
		policy, err := scanTokenPolicy(row)
		if err != nil {
			if isDuplicateKeyError(err) {
				return ErrConflict
			}
			return err
		}
		out = policy
		return nil
	})
	return out, err
}

// GetTokenPolicy retrieves a token policy by ID.
func (s *Store) GetTokenPolicy(ctx context.Context, policyID uuid.UUID) (TokenRateLimitPolicy, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+tokenPolicyColumns+`
		FROM token_rate_limit_policies
		WHERE id = $1
	`, policyID)

	policy, err := scanTokenPolicy(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return TokenRateLimitPolicy{}, ErrNotFound
		}
		return TokenRateLimitPolicy{}, err
	}
	return policy, nil
}

// GetTokenPolicyByName retrieves a policy by name within an org (or built-in).
func (s *Store) GetTokenPolicyByName(ctx context.Context, orgID uuid.UUID, name string) (TokenRateLimitPolicy, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT `+tokenPolicyColumns+`
		FROM token_rate_limit_policies
		WHERE (org_id = $1 OR org_id IS NULL) AND name = $2
		ORDER BY org_id NULLS LAST
		LIMIT 1
	`, orgID, name)

	policy, err := scanTokenPolicy(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return TokenRateLimitPolicy{}, ErrNotFound
		}
		return TokenRateLimitPolicy{}, err
	}
	return policy, nil
}

// ListTokenPolicies lists all token policies for an org, including built-in policies.
func (s *Store) ListTokenPolicies(ctx context.Context, orgID uuid.UUID) ([]TokenRateLimitPolicy, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+tokenPolicyColumns+`
		FROM token_rate_limit_policies
		WHERE org_id = $1 OR org_id IS NULL
		ORDER BY is_builtin DESC, name ASC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var policies []TokenRateLimitPolicy
	for rows.Next() {
		policy, err := scanTokenPolicy(rows)
		if err != nil {
			return nil, err
		}
		policies = append(policies, policy)
	}
	if policies == nil {
		policies = []TokenRateLimitPolicy{}
	}
	return policies, rows.Err()
}

// UpdateTokenPolicy updates a token policy.
func (s *Store) UpdateTokenPolicy(ctx context.Context, params UpdateTokenPolicyParams) (TokenRateLimitPolicy, error) {
	// First check if it's a builtin policy
	existing, err := s.GetTokenPolicy(ctx, params.ID)
	if err != nil {
		return TokenRateLimitPolicy{}, err
	}
	if existing.IsBuiltin {
		return TokenRateLimitPolicy{}, ErrForbidden
	}

	// Build dynamic update
	var setClauses []string
	var args []any
	argNum := 1

	if params.Name != nil {
		if strings.EqualFold(*params.Name, "No Token Rate-Limit") {
			return TokenRateLimitPolicy{}, ErrConflict
		}
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argNum))
		args = append(args, *params.Name)
		argNum++
	}

	if params.Description != nil {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", argNum))
		args = append(args, *params.Description)
		argNum++
	}

	if params.Limit1h != nil || params.ClearLimit1h {
		setClauses = append(setClauses, fmt.Sprintf("limit_1h = $%d", argNum))
		args = append(args, params.Limit1h)
		argNum++
	}

	if params.Limit24h != nil || params.ClearLimit24h {
		setClauses = append(setClauses, fmt.Sprintf("limit_24h = $%d", argNum))
		args = append(args, params.Limit24h)
		argNum++
	}

	if params.Limit7d != nil || params.ClearLimit7d {
		setClauses = append(setClauses, fmt.Sprintf("limit_7d = $%d", argNum))
		args = append(args, params.Limit7d)
		argNum++
	}

	if len(setClauses) == 0 {
		// Nothing to update
		return existing, nil
	}

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, params.ID)

	query := fmt.Sprintf(`
		UPDATE token_rate_limit_policies
		SET %s
		WHERE id = $%d AND is_builtin = FALSE
		RETURNING `+tokenPolicyColumns,
		strings.Join(setClauses, ", "),
		argNum,
	)

	row := s.pool.QueryRow(ctx, query, args...)
	policy, err := scanTokenPolicy(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return TokenRateLimitPolicy{}, ErrNotFound
		}
		if isDuplicateKeyError(err) {
			return TokenRateLimitPolicy{}, ErrConflict
		}
		return TokenRateLimitPolicy{}, err
	}
	return policy, nil
}

// DeleteTokenPolicy deletes a token policy if it's not in use.
func (s *Store) DeleteTokenPolicy(ctx context.Context, policyID uuid.UUID) error {
	// Check if it's a builtin policy
	existing, err := s.GetTokenPolicy(ctx, policyID)
	if err != nil {
		return err
	}
	if existing.IsBuiltin {
		return ErrForbidden
	}

	// Check if policy is in use as org default
	var orgCount int
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM orgs WHERE default_token_policy_id = $1
	`, policyID).Scan(&orgCount)
	if err != nil {
		return err
	}
	if orgCount > 0 {
		return ErrConflict
	}

	// Check if policy is in use as user override
	var userCount int
	err = s.pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE token_policy_override_id = $1
	`, policyID).Scan(&userCount)
	if err != nil {
		return err
	}
	if userCount > 0 {
		return ErrConflict
	}

	// Delete the policy
	result, err := s.pool.Exec(ctx, `
		DELETE FROM token_rate_limit_policies
		WHERE id = $1 AND is_builtin = FALSE
	`, policyID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetBuiltinNoLimitPolicy returns the system "No Token Rate-Limit" policy.
func (s *Store) GetBuiltinNoLimitPolicy(ctx context.Context) (TokenRateLimitPolicy, error) {
	return s.GetTokenPolicy(ctx, NoTokenRateLimitPolicyID)
}

// SetOrgDefaultTokenPolicy sets the default token policy for an organization.
func (s *Store) SetOrgDefaultTokenPolicy(ctx context.Context, orgID, policyID uuid.UUID) error {
	// Validate policy exists and is accessible to this org
	policy, err := s.GetTokenPolicy(ctx, policyID)
	if err != nil {
		return err
	}
	// Policy must be builtin (org_id is nil) or belong to this org
	if policy.OrgID != nil && *policy.OrgID != orgID {
		return ErrNotFound
	}

	result, err := s.pool.Exec(ctx, `
		UPDATE orgs
		SET default_token_policy_id = $1, updated_at = now(), version = version + 1
		WHERE org_id = $2 AND deleted_at IS NULL
	`, policyID, orgID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetOrgDefaultTokenPolicy gets the default token policy for an organization.
func (s *Store) GetOrgDefaultTokenPolicy(ctx context.Context, orgID uuid.UUID) (TokenRateLimitPolicy, error) {
	var policyID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT COALESCE(default_token_policy_id, $1)
		FROM orgs
		WHERE org_id = $2 AND deleted_at IS NULL
	`, NoTokenRateLimitPolicyID, orgID).Scan(&policyID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return TokenRateLimitPolicy{}, ErrNotFound
		}
		return TokenRateLimitPolicy{}, err
	}
	return s.GetTokenPolicy(ctx, policyID)
}

// SetUserTokenPolicyOverride sets a user's token policy override.
func (s *Store) SetUserTokenPolicyOverride(ctx context.Context, orgID, userID, policyID uuid.UUID) error {
	// Validate policy exists and is accessible to this org
	policy, err := s.GetTokenPolicy(ctx, policyID)
	if err != nil {
		return err
	}
	// Policy must be builtin (org_id is nil) or belong to this org
	if policy.OrgID != nil && *policy.OrgID != orgID {
		return ErrNotFound
	}

	return s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE users
			SET token_policy_override_id = $1, updated_at = now(), version = version + 1
			WHERE org_id = $2 AND user_id = $3 AND deleted_at IS NULL
		`, policyID, orgID, userID)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// ClearUserTokenPolicyOverride removes a user's token policy override (inherit from org).
func (s *Store) ClearUserTokenPolicyOverride(ctx context.Context, orgID, userID uuid.UUID) error {
	return s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		result, err := tx.Exec(ctx, `
			UPDATE users
			SET token_policy_override_id = NULL, updated_at = now(), version = version + 1
			WHERE org_id = $1 AND user_id = $2 AND deleted_at IS NULL
		`, orgID, userID)
		if err != nil {
			return err
		}
		if result.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// GetEffectiveTokenPolicy returns a user's effective token policy (override > org default).
func (s *Store) GetEffectiveTokenPolicy(ctx context.Context, orgID, userID uuid.UUID) (EffectiveTokenPolicy, error) {
	// Get user's override and org's default in one query
	row := s.pool.QueryRow(ctx, `
		SELECT
			u.token_policy_override_id,
			COALESCE(o.default_token_policy_id, $1)
		FROM users u
		JOIN orgs o ON o.org_id = u.org_id
		WHERE u.org_id = $2 AND u.user_id = $3 AND u.deleted_at IS NULL
	`, NoTokenRateLimitPolicyID, orgID, userID)

	var overrideID pgtype.UUID
	var defaultID uuid.UUID
	if err := row.Scan(&overrideID, &defaultID); err != nil {
		if err == pgx.ErrNoRows {
			return EffectiveTokenPolicy{}, ErrNotFound
		}
		return EffectiveTokenPolicy{}, err
	}

	// Use override if set, otherwise default
	var policyID uuid.UUID
	var source string
	if overrideID.Valid {
		policyID = overrideID.Bytes
		source = "override"
	} else {
		policyID = defaultID
		source = "inherited"
	}

	policy, err := s.GetTokenPolicy(ctx, policyID)
	if err != nil {
		return EffectiveTokenPolicy{}, err
	}

	return EffectiveTokenPolicy{
		Policy: policy,
		Source: source,
	}, nil
}

// GetEffectiveTokenPolicyByUserID returns a user's effective token policy given only userID.
// This is used by internal M2M endpoints where orgID is not known.
func (s *Store) GetEffectiveTokenPolicyByUserID(ctx context.Context, userID uuid.UUID) (EffectiveTokenPolicy, error) {
	// Get user's org, override, and org's default in one query
	row := s.pool.QueryRow(ctx, `
		SELECT
			u.org_id,
			u.token_policy_override_id,
			COALESCE(o.default_token_policy_id, $1)
		FROM users u
		JOIN orgs o ON o.org_id = u.org_id
		WHERE u.user_id = $2 AND u.deleted_at IS NULL
	`, NoTokenRateLimitPolicyID, userID)

	var orgID uuid.UUID
	var overrideID pgtype.UUID
	var defaultID uuid.UUID
	if err := row.Scan(&orgID, &overrideID, &defaultID); err != nil {
		if err == pgx.ErrNoRows {
			return EffectiveTokenPolicy{}, ErrNotFound
		}
		return EffectiveTokenPolicy{}, err
	}

	// Use override if set, otherwise default
	var policyID uuid.UUID
	var source string
	if overrideID.Valid {
		policyID = overrideID.Bytes
		source = "override"
	} else {
		policyID = defaultID
		source = "inherited"
	}

	policy, err := s.GetTokenPolicy(ctx, policyID)
	if err != nil {
		return EffectiveTokenPolicy{}, err
	}

	return EffectiveTokenPolicy{
		Policy: policy,
		Source: source,
	}, nil
}

// scanTokenPolicy scans a row into a TokenRateLimitPolicy.
func scanTokenPolicy(row pgx.Row) (TokenRateLimitPolicy, error) {
	var p TokenRateLimitPolicy
	var orgID pgtype.UUID
	var description pgtype.Text
	var limit1h, limit24h, limit7d pgtype.Int8

	err := row.Scan(
		&p.ID,
		&orgID,
		&p.Name,
		&description,
		&limit1h,
		&limit24h,
		&limit7d,
		&p.IsBuiltin,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return TokenRateLimitPolicy{}, err
	}

	p.OrgID = uuidPtr(orgID)
	p.Description = textPtr(description)
	if limit1h.Valid {
		p.Limit1h = &limit1h.Int64
	}
	if limit24h.Valid {
		p.Limit24h = &limit24h.Int64
	}
	if limit7d.Valid {
		p.Limit7d = &limit7d.Int64
	}

	return p, nil
}

// isDuplicateKeyError checks if the error is a unique constraint violation.
func isDuplicateKeyError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "duplicate key")
}
