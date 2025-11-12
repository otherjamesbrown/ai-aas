-- name: CreateServiceAccount :one
INSERT INTO service_accounts (
    service_account_id,
    org_id,
    name,
    description,
    status,
    metadata,
    last_rotation_at
) VALUES (
    COALESCE(sqlc.narg('service_account_id'), gen_random_uuid()),
    sqlc.arg('org_id'),
    sqlc.arg('name'),
    sqlc.arg('description'),
    sqlc.arg('status'),
    COALESCE(sqlc.arg('metadata'), '{}'::jsonb),
    sqlc.arg('last_rotation_at')
) RETURNING *;

-- name: GetServiceAccountByID :one
SELECT * FROM service_accounts
WHERE service_account_id = sqlc.arg('service_account_id') AND deleted_at IS NULL;

-- name: UpdateServiceAccount :one
UPDATE service_accounts
SET description = sqlc.arg('description'),
    status = sqlc.arg('status'),
    metadata = COALESCE(sqlc.arg('metadata'), metadata),
    last_rotation_at = sqlc.arg('last_rotation_at'),
    version = version + 1
WHERE service_account_id = sqlc.arg('service_account_id') AND version = sqlc.arg('version') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteServiceAccount :execrows
UPDATE service_accounts SET deleted_at = now(), version = version + 1
WHERE service_account_id = sqlc.arg('service_account_id') AND deleted_at IS NULL;

