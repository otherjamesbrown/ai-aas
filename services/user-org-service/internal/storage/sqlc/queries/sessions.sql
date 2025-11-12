-- name: CreateSession :one
INSERT INTO sessions (
    session_id,
    org_id,
    user_id,
    refresh_token_hash,
    ip_address,
    user_agent,
    mfa_verified_at,
    expires_at
) VALUES (
    COALESCE(sqlc.narg('session_id'), gen_random_uuid()),
    sqlc.arg('org_id'),
    sqlc.arg('user_id'),
    sqlc.arg('refresh_token_hash'),
    sqlc.arg('ip_address'),
    sqlc.arg('user_agent'),
    sqlc.arg('mfa_verified_at'),
    sqlc.arg('expires_at')
) RETURNING *;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE session_id = sqlc.arg('session_id') AND revoked_at IS NULL AND deleted_at IS NULL;

-- name: RevokeSession :execrows
UPDATE sessions
SET revoked_at = sqlc.arg('revoked_at'),
    version = version + 1
WHERE session_id = sqlc.arg('session_id') AND version = sqlc.arg('version') AND revoked_at IS NULL AND deleted_at IS NULL;

-- name: RevokeSessionsForUser :execrows
UPDATE sessions
SET revoked_at = sqlc.arg('revoked_at'),
    version = version + 1
WHERE user_id = sqlc.arg('user_id') AND revoked_at IS NULL AND deleted_at IS NULL;


