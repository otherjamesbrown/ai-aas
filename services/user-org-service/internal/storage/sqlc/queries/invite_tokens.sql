-- name: CreateInviteToken :one
INSERT INTO invite_tokens (
    invite_token_id,
    org_id,
    user_id,
    token_hash,
    expires_at,
    created_by_user_id
) VALUES (
    COALESCE(sqlc.narg('invite_token_id'), gen_random_uuid()),
    sqlc.arg('org_id'),
    sqlc.arg('user_id'),
    sqlc.arg('token_hash'),
    sqlc.arg('expires_at'),
    sqlc.arg('created_by_user_id')
) RETURNING *;

-- name: GetInviteTokenByHash :one
SELECT * FROM invite_tokens
WHERE org_id = sqlc.arg('org_id')
  AND token_hash = sqlc.arg('token_hash')
  AND used_at IS NULL
  AND expires_at > now();

-- name: MarkInviteTokenUsed :one
UPDATE invite_tokens
SET used_at = now()
WHERE invite_token_id = sqlc.arg('invite_token_id')
  AND used_at IS NULL
RETURNING *;

-- name: ListInviteTokensForUser :many
SELECT * FROM invite_tokens
WHERE org_id = sqlc.arg('org_id')
  AND user_id = sqlc.arg('user_id')
  AND used_at IS NULL
ORDER BY created_at DESC;

-- name: CleanupExpiredInviteTokens :execrows
DELETE FROM invite_tokens
WHERE expires_at < now() AND used_at IS NULL;

