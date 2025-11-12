-- name: CreateAPIKey :one
INSERT INTO api_keys (
    api_key_id,
    org_id,
    principal_type,
    principal_id,
    fingerprint,
    status,
    scopes,
    issued_at,
    expires_at,
    annotations
) VALUES (
    COALESCE(sqlc.narg('api_key_id'), gen_random_uuid()),
    sqlc.arg('org_id'),
    sqlc.arg('principal_type'),
    sqlc.arg('principal_id'),
    sqlc.arg('fingerprint'),
    sqlc.arg('status'),
    COALESCE(sqlc.arg('scopes'), '[]'::jsonb),
    COALESCE(sqlc.arg('issued_at'), now()),
    sqlc.arg('expires_at'),
    COALESCE(sqlc.arg('annotations'), '{}'::jsonb)
) RETURNING *;

-- name: GetAPIKeyByID :one
SELECT * FROM api_keys
WHERE api_key_id = sqlc.arg('api_key_id') AND deleted_at IS NULL;

-- name: ListAPIKeysForPrincipal :many
SELECT * FROM api_keys
WHERE org_id = sqlc.arg('org_id')
  AND principal_type = sqlc.arg('principal_type')
  AND principal_id = sqlc.arg('principal_id')
  AND deleted_at IS NULL
ORDER BY created_at DESC;

-- name: RevokeAPIKey :one
UPDATE api_keys
SET status = sqlc.arg('status'),
    revoked_at = sqlc.arg('revoked_at'),
    version = version + 1
WHERE api_key_id = sqlc.arg('api_key_id') AND version = sqlc.arg('version') AND deleted_at IS NULL
RETURNING *;

-- name: TouchAPIKeyUsage :execrows
UPDATE api_keys
SET last_used_at = sqlc.arg('last_used_at'),
    version = version + 1
WHERE api_key_id = sqlc.arg('api_key_id') AND deleted_at IS NULL;

