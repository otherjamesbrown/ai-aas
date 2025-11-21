package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides Postgres-backed persistence for the user-org service.
type Store struct {
	pool     *pgxpool.Pool
	ownsPool bool
}

// NewStore creates a store using the provided connection string and takes ownership of the pool.
func NewStore(ctx context.Context, connString string) (*Store, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}
	return &Store{pool: pool, ownsPool: true}, nil
}

// NewStoreFromPool wraps an existing pgx pool.
func NewStoreFromPool(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Close closes the underlying pool if the store owns it.
func (s *Store) Close() {
	if s.ownsPool && s.pool != nil {
		s.pool.Close()
	}
}

// Pool exposes the underlying pgx pool for internal collaborators (e.g., OAuth store).
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *Store) withTx(ctx context.Context, fn func(context.Context, pgx.Tx) error) error {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if err = fn(ctx, tx); err != nil {
		return err
	}

	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return nil
}

func (s *Store) withTenantTx(ctx context.Context, orgID uuid.UUID, fn func(context.Context, pgx.Tx) error) error {
	return s.withTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// SET LOCAL doesn't support parameters, use string interpolation with proper escaping
		escapedOrgID := strings.ReplaceAll(orgID.String(), "'", "''")
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.org_id = '%s'", escapedOrgID)); err != nil {
			return err
		}
		return fn(ctx, tx)
	})
}

// CreateOrg inserts a new organization row.
func (s *Store) CreateOrg(ctx context.Context, params CreateOrgParams) (Org, error) {
	if params.Metadata == nil {
		params.Metadata = map[string]any{}
	}
	if params.MFARequiredRoles == nil {
		params.MFARequiredRoles = []string{}
	}
	if params.DeclarativeMode == "" {
		params.DeclarativeMode = "disabled"
	}

	orgID := params.ID
	if orgID == uuid.Nil {
		orgID = uuid.New()
	}

	var out Org
	err := s.withTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		// SET LOCAL doesn't support parameters, use pgx.Identifier for safe escaping
		// Use Exec with a format string, but ensure UUID is properly escaped
		escapedOrgID := strings.ReplaceAll(orgID.String(), "'", "''")
		if _, err := tx.Exec(ctx, fmt.Sprintf("SET LOCAL app.org_id = '%s'", escapedOrgID)); err != nil {
			return err
		}

		mfaJSON, err := mustJSONB(params.MFARequiredRoles)
		if err != nil {
			return err
		}
		metadataJSON, err := mustJSONB(params.Metadata)
		if err != nil {
			return err
		}

		row := tx.QueryRow(ctx, `
			INSERT INTO orgs (
				org_id,
				slug,
				name,
				status,
				billing_owner_user_id,
				budget_policy_id,
				declarative_mode,
				declarative_repo_url,
				declarative_branch,
				declarative_last_commit,
				mfa_required_roles,
				metadata
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
			RETURNING *
		`,
			orgID,
			params.Slug,
			params.Name,
			params.Status,
			params.BillingOwnerUserID,
			params.BudgetPolicyID,
			params.DeclarativeMode,
			params.DeclarativeRepoURL,
			params.DeclarativeBranch,
			params.DeclarativeLastCommit,
			string(mfaJSON),
			string(metadataJSON),
		)

		org, err := scanOrg(row)
		if err != nil {
			return err
		}
		out = org
		return nil
	})
	return out, err
}

// GetOrg retrieves an organization by ID.
func (s *Store) GetOrg(ctx context.Context, id uuid.UUID) (Org, error) {
	var out Org
	err := s.withTenantTx(ctx, id, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `SELECT * FROM orgs WHERE org_id = $1 AND deleted_at IS NULL`, id)
		org, err := scanOrg(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		out = org
		return nil
	})
	return out, err
}

// GetOrgBySlug retrieves an organization by slug.
func (s *Store) GetOrgBySlug(ctx context.Context, slug string) (Org, error) {
	var out Org
	err := s.withTx(ctx, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `SELECT * FROM orgs WHERE slug = $1 AND deleted_at IS NULL`, slug)
		org, err := scanOrg(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		out = org
		return nil
	})
	return out, err
}

// GetUserByEmail retrieves a user by email within an organization.
func (s *Store) GetUserByEmail(ctx context.Context, orgID uuid.UUID, email string) (User, error) {
	var out User
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT * FROM users
			WHERE org_id = $1 AND email = LOWER($2) AND deleted_at IS NULL
		`, orgID, email)
		user, err := scanUser(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		out = user
		return nil
	})
	return out, err
}

// GetUserByID retrieves a user by ID within an organization.
func (s *Store) GetUserByID(ctx context.Context, orgID, userID uuid.UUID) (User, error) {
	var out User
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT * FROM users
			WHERE org_id = $1 AND user_id = $2 AND deleted_at IS NULL
		`, orgID, userID)
		user, err := scanUser(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		out = user
		return nil
	})
	return out, err
}

// GetUserByExternalIDP retrieves a user by external IdP identifier within an organization.
func (s *Store) GetUserByExternalIDP(ctx context.Context, orgID uuid.UUID, externalIDP string) (User, error) {
	var out User
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT * FROM users
			WHERE org_id = $1 AND external_idp_id = $2 AND deleted_at IS NULL
		`, orgID, externalIDP)
		user, err := scanUser(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		out = user
		return nil
	})
	return out, err
}

// GetUserOrgIDByUserID retrieves the organization ID for a user by their user ID.
// This method does not use tenant transactions since we're looking up the user's org.
func (s *Store) GetUserOrgIDByUserID(ctx context.Context, userID uuid.UUID) (uuid.UUID, error) {
	var orgID uuid.UUID
	err := s.pool.QueryRow(ctx, `
		SELECT org_id FROM users
		WHERE user_id = $1 AND deleted_at IS NULL
		LIMIT 1
	`, userID).Scan(&orgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, ErrNotFound
		}
		return uuid.Nil, err
	}
	return orgID, nil
}

// ValidateUserOrgMembership checks if a user belongs to a specific organization.
// Returns nil if the user belongs to the org, ErrNotFound otherwise.
func (s *Store) ValidateUserOrgMembership(ctx context.Context, userID, orgID uuid.UUID) error {
	var count int
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM users
			WHERE user_id = $1 AND org_id = $2 AND deleted_at IS NULL
		`, userID, orgID).Scan(&count)
	})
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateOrg updates mutable fields using optimistic locking.
func (s *Store) UpdateOrg(ctx context.Context, params UpdateOrgParams) (Org, error) {
	if params.Metadata == nil {
		params.Metadata = map[string]any{}
	}
	if params.MFARequiredRoles == nil {
		params.MFARequiredRoles = []string{}
	}

	var out Org
	err := s.withTenantTx(ctx, params.ID, func(ctx context.Context, tx pgx.Tx) error {
		mfaJSON, err := mustJSONB(params.MFARequiredRoles)
		if err != nil {
			return err
		}
		metadataJSON, err := mustJSONB(params.Metadata)
		if err != nil {
			return err
		}

		row := tx.QueryRow(ctx, `
			UPDATE orgs
			SET name = $1,
				status = $2,
				billing_owner_user_id = $3,
				budget_policy_id = $4,
				declarative_mode = $5,
				declarative_repo_url = $6,
				declarative_branch = $7,
				declarative_last_commit = $8,
				mfa_required_roles = $9,
				metadata = $10,
				version = version + 1
			WHERE org_id = $11 AND version = $12 AND deleted_at IS NULL
			RETURNING *
		`,
			params.Name,
			params.Status,
			params.BillingOwnerUserID,
			params.BudgetPolicyID,
			params.DeclarativeMode,
			params.DeclarativeRepoURL,
			params.DeclarativeBranch,
			params.DeclarativeLastCommit,
			string(mfaJSON),
			string(metadataJSON),
			params.ID,
			params.Version,
		)

		org, err := scanOrg(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrOptimisticLock
			}
			return err
		}
		out = org
		return nil
	})
	return out, err
}

// CreateUser creates a new user within an organization.
func (s *Store) CreateUser(ctx context.Context, params CreateUserParams) (User, error) {
	if params.Metadata == nil {
		params.Metadata = map[string]any{}
	}
	if params.MFAMethods == nil {
		params.MFAMethods = []string{}
	}
	if params.RecoveryTokens == nil {
		params.RecoveryTokens = []string{}
	}
	if params.PasswordHash == "" {
		return User{}, fmt.Errorf("password hash must be provided")
	}
	userID := params.ID
	if userID == uuid.Nil {
		userID = uuid.New()
	}

	var out User
	err := s.withTenantTx(ctx, params.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		mfaJSON, err := mustJSONB(params.MFAMethods)
		if err != nil {
			return err
		}
		recoveryJSON, err := mustJSONB(params.RecoveryTokens)
		if err != nil {
			return err
		}
		metadataJSON, err := mustJSONB(params.Metadata)
		if err != nil {
			return err
		}

		row := tx.QueryRow(ctx, `
			INSERT INTO users (
				user_id,
				org_id,
				email,
				display_name,
				password_hash,
				status,
				mfa_enrolled,
				mfa_methods,
				mfa_secret,
				last_login_at,
				lockout_until,
				recovery_tokens,
				external_idp_id,
				metadata
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
			RETURNING *
		`,
			userID,
			params.OrgID,
			params.Email,
			params.DisplayName,
			params.PasswordHash,
			params.Status,
			params.MFAEnrolled,
			string(mfaJSON),
			params.MFASecret,
			params.LastLoginAt,
			params.LockoutUntil,
			string(recoveryJSON),
			params.ExternalIDP,
			string(metadataJSON),
		)

		user, err := scanUser(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrOptimisticLock
			}
			return err
		}
		out = user
		return nil
	})
	return out, err
}

func (s *Store) UpdateUserStatus(ctx context.Context, params UpdateUserStatusParams) (User, error) {
	var out User
	err := s.withTenantTx(ctx, params.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE users
			SET status = $1,
				lockout_until = $2,
				version = version + 1
			WHERE user_id = $3 AND version = $4 AND deleted_at IS NULL
			RETURNING *
		`,
			params.Status,
			params.LockoutUntil,
			params.ID,
			params.Version,
		)
		user, err := scanUser(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrOptimisticLock
			}
			return err
		}
		out = user
		return nil
	})
	return out, err
}

// UpdateUserProfile updates user profile fields using optimistic locking.
func (s *Store) UpdateUserProfile(ctx context.Context, params UpdateUserProfileParams) (User, error) {
	if params.Metadata == nil {
		params.Metadata = map[string]any{}
	}
	if params.MFAMethods == nil {
		params.MFAMethods = []string{}
	}

	var out User
	err := s.withTenantTx(ctx, params.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		mfaJSON, err := mustJSONB(params.MFAMethods)
		if err != nil {
			return err
		}
		metadataJSON, err := mustJSONB(params.Metadata)
		if err != nil {
			return err
		}

		row := tx.QueryRow(ctx, `
			UPDATE users
			SET display_name = $1,
				mfa_enrolled = $2,
				mfa_methods = COALESCE($3, mfa_methods),
				mfa_secret = COALESCE($4, mfa_secret),
				metadata = COALESCE($5, metadata),
				version = version + 1
			WHERE user_id = $6 AND version = $7 AND deleted_at IS NULL
			RETURNING *
		`,
			params.DisplayName,
			params.MFAEnrolled,
			string(mfaJSON),
			params.MFASecret,
			string(metadataJSON),
			params.ID,
			params.Version,
		)
		user, err := scanUser(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrOptimisticLock
			}
			return err
		}
		out = user
		return nil
	})
	return out, err
}

// UpdateUserPasswordHash updates the password hash using optimistic locking.
func (s *Store) UpdateUserPasswordHash(ctx context.Context, params UpdateUserPasswordHashParams) (User, error) {
	if params.PasswordHash == "" {
		return User{}, fmt.Errorf("password hash must be provided")
	}
	var out User
	err := s.withTenantTx(ctx, params.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE users
			SET password_hash = $1,
				version = version + 1
			WHERE user_id = $2 AND version = $3 AND deleted_at IS NULL
			RETURNING *
		`,
			params.PasswordHash,
			params.ID,
			params.Version,
		)
		user, err := scanUser(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrOptimisticLock
			}
			return err
		}
		out = user
		return nil
	})
	return out, err
}

// UpdateUserExternalIDP updates a user's external IdP identifier using optimistic locking.
func (s *Store) UpdateUserExternalIDP(ctx context.Context, orgID, userID uuid.UUID, version int64, externalIDP string) (User, error) {
	var out User
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE users
			SET external_idp_id = $4, updated_at = NOW(), version = version + 1
			WHERE org_id = $1 AND user_id = $2 AND version = $3 AND deleted_at IS NULL
			RETURNING *
		`, orgID, userID, version, externalIDP)
		user, err := scanUser(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrOptimisticLock
			}
			return err
		}
		out = user
		return nil
	})
	return out, err
}

// UpdateUserRecoveryTokens updates the recovery_tokens array using optimistic locking.
func (s *Store) UpdateUserRecoveryTokens(ctx context.Context, orgID, userID uuid.UUID, version int64, recoveryTokens []string) (User, error) {
	if recoveryTokens == nil {
		recoveryTokens = []string{}
	}
	var out User
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		recoveryJSON, err := mustJSONB(recoveryTokens)
		if err != nil {
			return err
		}
		row := tx.QueryRow(ctx, `
			UPDATE users
			SET recovery_tokens = $1,
				version = version + 1
			WHERE user_id = $2 AND version = $3 AND deleted_at IS NULL
			RETURNING *
		`,
			string(recoveryJSON),
			userID,
			version,
		)
		user, err := scanUser(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrOptimisticLock
			}
			return err
		}
		out = user
		return nil
	})
	return out, err
}

// CreateSession inserts a new session row.
func (s *Store) CreateSession(ctx context.Context, params CreateSessionParams) (Session, error) {
	sessionID := params.ID
	if sessionID == uuid.Nil {
		sessionID = uuid.New()
	}
	var out Session
	err := s.withTenantTx(ctx, params.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			INSERT INTO sessions (
				session_id,
				org_id,
				user_id,
				refresh_token_hash,
				ip_address,
				user_agent,
				mfa_verified_at,
				expires_at
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			RETURNING *
		`,
			sessionID,
			params.OrgID,
			params.UserID,
			params.RefreshTokenHash,
			params.IPAddress,
			params.UserAgent,
			params.MFAVerifiedAt,
			params.ExpiresAt,
		)
		sess, err := scanSession(row)
		if err != nil {
			return err
		}
		out = sess
		return nil
	})
	return out, err
}

// RevokeSession marks a session as revoked using optimistic locking.
func (s *Store) RevokeSession(ctx context.Context, params RevokeSessionParams, orgID uuid.UUID) error {
	return s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		cmd, err := tx.Exec(ctx, `
			UPDATE sessions
			SET revoked_at = $1,
				version = version + 1
			WHERE session_id = $2 AND version = $3 AND revoked_at IS NULL
		`,
			params.Time,
			params.ID,
			params.Version,
		)
		if err != nil {
			return err
		}
		if cmd.RowsAffected() == 0 {
			return ErrOptimisticLock
		}
		return nil
	})
}

// CreateAPIKey issues a new API key record.
func (s *Store) CreateAPIKey(ctx context.Context, params CreateAPIKeyParams) (APIKey, error) {
	if params.Scopes == nil {
		params.Scopes = []string{}
	}
	if params.Annotations == nil {
		params.Annotations = map[string]any{}
	}
	apiKeyID := params.ID
	if apiKeyID == uuid.Nil {
		apiKeyID = uuid.New()
	}
	var out APIKey
	err := s.withTenantTx(ctx, params.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		scopesJSON, err := mustJSONB(params.Scopes)
		if err != nil {
			return err
		}
		annotationsJSON, err := mustJSONB(params.Annotations)
		if err != nil {
			return err
		}
		row := tx.QueryRow(ctx, `
			INSERT INTO api_keys (
				api_key_id,
				org_id,
				principal_type,
				principal_id,
				fingerprint,
				status,
				scopes,
				expires_at,
				annotations
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			RETURNING *
		`,
			apiKeyID,
			params.OrgID,
			string(params.PrincipalType),
			params.PrincipalID,
			params.Fingerprint,
			params.Status,
			string(scopesJSON),
			params.ExpiresAt,
			string(annotationsJSON),
		)
		key, err := scanAPIKey(row)
		if err != nil {
			return err
		}
		out = key
		return nil
	})
	return out, err
}

// GetAPIKeyByID retrieves an API key by its ID.
func (s *Store) GetAPIKeyByID(ctx context.Context, apiKeyID uuid.UUID) (APIKey, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT *
		FROM api_keys
		WHERE api_key_id = $1 AND deleted_at IS NULL
	`, apiKeyID)
	key, err := scanAPIKey(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return APIKey{}, ErrNotFound
		}
		return APIKey{}, err
	}
	return key, nil
}

// GetAPIKeyByFingerprint retrieves an API key by its fingerprint within an organization.
func (s *Store) GetAPIKeyByFingerprint(ctx context.Context, orgID uuid.UUID, fingerprint string) (APIKey, error) {
	var out APIKey
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			SELECT *
			FROM api_keys
			WHERE org_id = $1 AND fingerprint = $2 AND deleted_at IS NULL
		`, orgID, fingerprint)
		key, err := scanAPIKey(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrNotFound
			}
			return err
		}
		out = key
		return nil
	})
	return out, err
}

// GetAPIKeyByFingerprintAnyOrg retrieves an API key by its fingerprint across all organizations.
// This is less efficient than GetAPIKeyByFingerprint but supports org-agnostic validation.
// Use this only when org_id is not available (e.g., API Router initial lookup).
func (s *Store) GetAPIKeyByFingerprintAnyOrg(ctx context.Context, fingerprint string) (APIKey, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT *
		FROM api_keys
		WHERE fingerprint = $1 AND deleted_at IS NULL
		LIMIT 1
	`, fingerprint)

	key, err := scanAPIKey(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return APIKey{}, ErrNotFound
		}
		return APIKey{}, err
	}
	return key, nil
}

// ListAPIKeysForPrincipal lists all API keys for a given principal (user or service account) within an organization.
func (s *Store) ListAPIKeysForPrincipal(ctx context.Context, orgID uuid.UUID, principalType PrincipalType, principalID uuid.UUID) ([]APIKey, error) {
	var out []APIKey
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT *
			FROM api_keys
			WHERE org_id = $1
			  AND principal_type = $2
			  AND principal_id = $3
			  AND deleted_at IS NULL
			ORDER BY created_at DESC
		`, orgID, string(principalType), principalID)
		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			key, err := scanAPIKey(rows)
			if err != nil {
				return err
			}
			out = append(out, key)
		}
		return rows.Err()
	})
	return out, err
}

// CreateServiceAccount creates a new service account within an organization.
func (s *Store) CreateServiceAccount(ctx context.Context, params CreateServiceAccountParams) (ServiceAccount, error) {
	if params.Metadata == nil {
		params.Metadata = map[string]any{}
	}
	if params.Status == "" {
		params.Status = "active"
	}
	serviceAccountID := params.ID
	if serviceAccountID == uuid.Nil {
		serviceAccountID = uuid.New()
	}

	var out ServiceAccount
	err := s.withTenantTx(ctx, params.OrgID, func(ctx context.Context, tx pgx.Tx) error {
		metadataJSON, err := mustJSONB(params.Metadata)
		if err != nil {
			return err
		}

		row := tx.QueryRow(ctx, `
			INSERT INTO service_accounts (
				service_account_id,
				org_id,
				name,
				description,
				status,
				metadata,
				last_rotation_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING *
		`,
			serviceAccountID,
			params.OrgID,
			params.Name,
			params.Description,
			params.Status,
			string(metadataJSON),
			params.LastRotationAt,
		)

		sa, err := scanServiceAccount(row)
		if err != nil {
			return err
		}
		out = sa
		return nil
	})
	return out, err
}

// GetServiceAccountByID retrieves a service account by its ID.
func (s *Store) GetServiceAccountByID(ctx context.Context, serviceAccountID uuid.UUID) (ServiceAccount, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT *
		FROM service_accounts
		WHERE service_account_id = $1 AND deleted_at IS NULL
	`, serviceAccountID)
	sa, err := scanServiceAccount(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return ServiceAccount{}, ErrNotFound
		}
		return ServiceAccount{}, err
	}
	return sa, nil
}

// UpdateAPIKeyLastUsed updates the last_used_at timestamp for an API key.
func (s *Store) UpdateAPIKeyLastUsed(ctx context.Context, apiKeyID uuid.UUID, lastUsedAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE api_keys
		SET last_used_at = $1,
			version = version + 1
		WHERE api_key_id = $2 AND deleted_at IS NULL
	`, lastUsedAt, apiKeyID)
	return err
}

func (s *Store) RevokeAPIKey(ctx context.Context, params RevokeAPIKeyParams, orgID uuid.UUID) (APIKey, error) {
	var out APIKey
	err := s.withTenantTx(ctx, orgID, func(ctx context.Context, tx pgx.Tx) error {
		row := tx.QueryRow(ctx, `
			UPDATE api_keys
			SET status = $1,
				revoked_at = $2,
				version = version + 1
			WHERE api_key_id = $3 AND version = $4 AND revoked_at IS NULL
			RETURNING *
		`,
			params.Status,
			params.RevokedAt,
			params.ID,
			params.Version,
		)
		key, err := scanAPIKey(row)
		if err != nil {
			if err == pgx.ErrNoRows {
				return ErrOptimisticLock
			}
			return err
		}
		out = key
		return nil
	})
	return out, err
}

// scan helpers ---------------------------------------------------------------

func scanOrg(row pgx.Row) (Org, error) {
	var (
		o            Org
		billingOwner pgtype.UUID
		budgetPolicy pgtype.UUID
		repoURL      pgtype.Text
		branch       pgtype.Text
		lastCommit   pgtype.Text
		mfaJSON      []byte
		metadataJSON []byte
		deleted      pgtype.Timestamptz
	)

	err := row.Scan(
		&o.ID,
		&o.Slug,
		&o.Name,
		&o.Status,
		&billingOwner,
		&budgetPolicy,
		&o.DeclarativeMode,
		&repoURL,
		&branch,
		&lastCommit,
		&mfaJSON,
		&metadataJSON,
		&o.Version,
		&o.CreatedAt,
		&o.UpdatedAt,
		&deleted,
	)
	if err != nil {
		return Org{}, err
	}

	o.BillingOwnerUserID = uuidPtr(billingOwner)
	o.BudgetPolicyID = uuidPtr(budgetPolicy)
	o.DeclarativeRepoURL = textPtr(repoURL)
	o.DeclarativeBranch = textPtr(branch)
	o.DeclarativeLastCommit = textPtr(lastCommit)

	roles, err := jsonSliceStringDefault(mfaJSON)
	if err != nil {
		return Org{}, err
	}
	o.MFARequiredRoles = roles

	metadata, err := jsonStringMap(metadataJSON)
	if err != nil {
		return Org{}, err
	}
	o.Metadata = metadata

	o.DeletedAt = timePtr(deleted)
	return o, nil
}

func scanUser(row pgx.Row) (User, error) {
	var (
		u            User
		orgID        uuid.UUID
		passwordHash string
		mfaJSON      []byte
		mfaSecret    pgtype.Text
		recoveryJSON []byte
		metadataJSON []byte
		lastLogin    pgtype.Timestamptz
		lockout      pgtype.Timestamptz
		externalIDP  pgtype.Text
		deleted      pgtype.Timestamptz
	)
	err := row.Scan(
		&u.ID,
		&orgID,
		&u.Email,
		&u.DisplayName,
		&passwordHash,
		&u.Status,
		&u.MFAEnrolled,
		&mfaJSON,
		&mfaSecret,
		&lastLogin,
		&lockout,
		&recoveryJSON,
		&externalIDP,
		&metadataJSON,
		&u.Version,
		&u.CreatedAt,
		&u.UpdatedAt,
		&deleted,
	)
	if err != nil {
		return User{}, err
	}
	u.OrgID = orgID
	u.PasswordHash = passwordHash

	mfa, err := jsonSliceStringDefault(mfaJSON)
	if err != nil {
		return User{}, err
	}
	u.MFAMethods = mfa
	u.MFASecret = textPtr(mfaSecret)

	recovery, err := jsonSliceStringDefault(recoveryJSON)
	if err != nil {
		return User{}, err
	}
	u.RecoveryTokens = recovery

	meta, err := jsonStringMap(metadataJSON)
	if err != nil {
		return User{}, err
	}
	u.Metadata = meta

	u.LastLoginAt = timePtr(lastLogin)
	u.LockoutUntil = timePtr(lockout)
	u.ExternalIDP = textPtr(externalIDP)
	u.DeletedAt = timePtr(deleted)
	return u, nil
}

func scanAPIKey(row pgx.Row) (APIKey, error) {
	var (
		key             APIKey
		principalType   string
		scopesJSON      []byte
		annotationsJSON []byte
		revoked         pgtype.Timestamptz
		expires         pgtype.Timestamptz
		lastUsed        pgtype.Timestamptz
		deleted         pgtype.Timestamptz
	)
	err := row.Scan(
		&key.ID,
		&key.OrgID,
		&principalType,
		&key.PrincipalID,
		&key.Fingerprint,
		&key.Status,
		&scopesJSON,
		&key.IssuedAt,
		&revoked,
		&expires,
		&lastUsed,
		&annotationsJSON,
		&key.Version,
		&key.CreatedAt,
		&key.UpdatedAt,
		&deleted,
	)
	if err != nil {
		return APIKey{}, err
	}
	key.PrincipalType = PrincipalType(principalType)

	scopes, err := jsonSliceStringDefault(scopesJSON)
	if err != nil {
		return APIKey{}, err
	}
	key.Scopes = scopes

	annotations, err := jsonStringMap(annotationsJSON)
	if err != nil {
		return APIKey{}, err
	}
	key.Annotations = annotations

	key.RevokedAt = timePtr(revoked)
	key.ExpiresAt = timePtr(expires)
	key.LastUsedAt = timePtr(lastUsed)
	key.DeletedAt = timePtr(deleted)
	return key, nil
}

func scanServiceAccount(row pgx.Row) (ServiceAccount, error) {
	var (
		sa           ServiceAccount
		orgID        uuid.UUID
		description  pgtype.Text
		metadataJSON []byte
		lastRotation pgtype.Timestamptz
		deleted      pgtype.Timestamptz
	)
	err := row.Scan(
		&sa.ID,
		&orgID,
		&sa.Name,
		&description,
		&sa.Status,
		&metadataJSON,
		&lastRotation,
		&sa.Version,
		&sa.CreatedAt,
		&sa.UpdatedAt,
		&deleted,
	)
	if err != nil {
		return ServiceAccount{}, err
	}
	sa.OrgID = orgID
	sa.Description = textPtr(description)
	sa.LastRotationAt = timePtr(lastRotation)
	sa.DeletedAt = timePtr(deleted)

	metadata, err := jsonStringMap(metadataJSON)
	if err != nil {
		return ServiceAccount{}, err
	}
	sa.Metadata = metadata

	return sa, nil
}

func scanSession(row pgx.Row) (Session, error) {
	var (
		s           Session
		ip          pgtype.Text
		ua          pgtype.Text
		mfaVerified pgtype.Timestamptz
		revoked     pgtype.Timestamptz
		deleted     pgtype.Timestamptz
	)
	err := row.Scan(
		&s.ID,
		&s.OrgID,
		&s.UserID,
		&s.RefreshTokenHash,
		&ip,
		&ua,
		&mfaVerified,
		&s.ExpiresAt,
		&revoked,
		&s.Version,
		&s.CreatedAt,
		&s.UpdatedAt,
		&deleted,
	)
	if err != nil {
		return Session{}, err
	}
	s.IPAddress = textPtr(ip)
	s.UserAgent = textPtr(ua)
	s.MFAVerifiedAt = timePtr(mfaVerified)
	s.RevokedAt = timePtr(revoked)
	s.DeletedAt = timePtr(deleted)
	return s, nil
}
