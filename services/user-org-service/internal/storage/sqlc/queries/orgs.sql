-- name: CreateOrg :one
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
) VALUES (
    COALESCE(sqlc.narg('org_id'), gen_random_uuid()),
    sqlc.arg('slug'),
    sqlc.arg('name'),
    sqlc.arg('status'),
    sqlc.arg('billing_owner_user_id'),
    sqlc.arg('budget_policy_id'),
    sqlc.arg('declarative_mode'),
    sqlc.arg('declarative_repo_url'),
    sqlc.arg('declarative_branch'),
    sqlc.arg('declarative_last_commit'),
    COALESCE(sqlc.arg('mfa_required_roles'), '[]'::jsonb),
    COALESCE(sqlc.arg('metadata'), '{}'::jsonb)
) RETURNING *;

-- name: GetOrgByID :one
SELECT * FROM orgs WHERE org_id = sqlc.arg('org_id') AND deleted_at IS NULL;

-- name: GetOrgBySlug :one
SELECT * FROM orgs WHERE slug = sqlc.arg('slug') AND deleted_at IS NULL;

-- name: UpdateOrg :one
UPDATE orgs
SET
    name = sqlc.arg('name'),
    status = sqlc.arg('status'),
    billing_owner_user_id = sqlc.arg('billing_owner_user_id'),
    budget_policy_id = sqlc.arg('budget_policy_id'),
    declarative_mode = sqlc.arg('declarative_mode'),
    declarative_repo_url = sqlc.arg('declarative_repo_url'),
    declarative_branch = sqlc.arg('declarative_branch'),
    declarative_last_commit = sqlc.arg('declarative_last_commit'),
    mfa_required_roles = COALESCE(sqlc.arg('mfa_required_roles'), mfa_required_roles),
    metadata = COALESCE(sqlc.arg('metadata'), metadata),
    version = version + 1
WHERE org_id = sqlc.arg('org_id') AND version = sqlc.arg('version') AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteOrg :execrows
UPDATE orgs SET deleted_at = now(), version = version + 1
WHERE org_id = sqlc.arg('org_id') AND deleted_at IS NULL;

