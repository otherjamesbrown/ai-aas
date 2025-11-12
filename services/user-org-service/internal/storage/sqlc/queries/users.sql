-- name: CreateUser :one
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
) VALUES (
    COALESCE(sqlc.narg('user_id'), gen_random_uuid()),
    sqlc.arg('org_id'),
    LOWER(sqlc.arg('email')),
    sqlc.arg('display_name'),
    sqlc.arg('password_hash'),
    sqlc.arg('status'),
    COALESCE(sqlc.arg('mfa_enrolled'), false),
    COALESCE(sqlc.arg('mfa_methods'), '[]'::jsonb),
    sqlc.arg('mfa_secret'),
    sqlc.arg('last_login_at'),
    sqlc.arg('lockout_until'),
    COALESCE(sqlc.arg('recovery_tokens'), '[]'::jsonb),
    sqlc.arg('external_idp_id'),
    COALESCE(sqlc.arg('metadata'), '{}'::jsonb)
) RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users
WHERE user_id = sqlc.arg('user_id') AND deleted_at IS NULL;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE org_id = sqlc.arg('org_id') AND email = LOWER(sqlc.arg('email')) AND deleted_at IS NULL;

-- name: UpdateUserStatus :one
UPDATE users
SET status = sqlc.arg('status'),
    lockout_until = sqlc.arg('lockout_until'),
    version = version + 1
WHERE user_id = sqlc.arg('user_id') AND version = sqlc.arg('version') AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserProfile :one
UPDATE users
SET display_name = sqlc.arg('display_name'),
    mfa_enrolled = sqlc.arg('mfa_enrolled'),
    mfa_methods = COALESCE(sqlc.arg('mfa_methods'), mfa_methods),
    mfa_secret = COALESCE(sqlc.arg('mfa_secret'), mfa_secret),
    metadata = COALESCE(sqlc.arg('metadata'), metadata),
    version = version + 1
WHERE user_id = sqlc.arg('user_id') AND version = sqlc.arg('version') AND deleted_at IS NULL
RETURNING *;

-- name: UpdateUserPasswordHash :one
UPDATE users
SET password_hash = sqlc.arg('password_hash'),
    version = version + 1
WHERE user_id = sqlc.arg('user_id') AND version = sqlc.arg('version') AND deleted_at IS NULL
RETURNING *;

-- name: TouchUserLogin :exec
UPDATE users
SET last_login_at = now(), version = version + 1
WHERE user_id = sqlc.arg('user_id') AND deleted_at IS NULL;

-- name: SoftDeleteUser :execrows
UPDATE users SET deleted_at = now(), version = version + 1
WHERE user_id = sqlc.arg('user_id') AND deleted_at IS NULL;

