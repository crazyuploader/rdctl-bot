-- name: UpsertUser :one
INSERT INTO users (
    user_id, username, first_name, last_name, is_super_admin, is_allowed,
    first_seen_at, last_seen_at, total_commands, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, false,
    $6, $7, 0, $8, $9
)
ON CONFLICT (user_id) DO UPDATE SET
    username       = EXCLUDED.username,
    first_name     = EXCLUDED.first_name,
    last_name      = EXCLUDED.last_name,
    is_super_admin = EXCLUDED.is_super_admin,
    last_seen_at   = EXCLUDED.last_seen_at,
    updated_at     = EXCLUDED.updated_at
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByUserID :one
SELECT * FROM users WHERE user_id = $1 AND deleted_at IS NULL;

-- name: IncrementUserCommands :exec
UPDATE users SET total_commands = total_commands + 1 WHERE id = $1;

-- name: LockUserForUpdate :one
SELECT * FROM users WHERE user_id = $1 AND deleted_at IS NULL FOR UPDATE;
