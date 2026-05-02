-- name: UpsertUser :one
INSERT INTO users (
    user_id, username, first_name, last_name,
    language_code, is_bot, is_premium,
    is_super_admin, is_allowed,
    first_seen_at, last_seen_at, total_commands, created_at, updated_at
) VALUES (
    $1, $2, $3, $4,
    $5, $6, $7,
    $8, false,
    $9, $10, 0, $11, $12
)
ON CONFLICT (user_id) DO UPDATE SET
    username       = EXCLUDED.username,
    first_name     = EXCLUDED.first_name,
    last_name      = EXCLUDED.last_name,
    language_code  = EXCLUDED.language_code,
    is_bot         = EXCLUDED.is_bot,
    is_premium     = EXCLUDED.is_premium,
    is_super_admin = EXCLUDED.is_super_admin,
    last_seen_at   = EXCLUDED.last_seen_at,
    updated_at     = EXCLUDED.updated_at
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL;

-- name: GetUserByUserID :one
SELECT * FROM users WHERE user_id = $1 AND deleted_at IS NULL;

-- name: IncrementUserCommands :exec
UPDATE users SET total_commands = total_commands + 1 WHERE user_id = $1 AND deleted_at IS NULL;

-- name: IncrementUserTorrents :exec
UPDATE users SET total_torrents_added = total_torrents_added + 1 WHERE user_id = $1 AND deleted_at IS NULL;

-- name: IncrementUserDownloads :exec
UPDATE users SET total_downloads = total_downloads + 1 WHERE user_id = $1 AND deleted_at IS NULL;

-- name: LockUserForUpdate :one
SELECT * FROM users WHERE user_id = $1 AND deleted_at IS NULL FOR UPDATE;

-- name: BanUser :exec
UPDATE users SET is_allowed = false, ban_reason = $2, banned_at = $3, updated_at = $4
WHERE user_id = $1 AND deleted_at IS NULL;

-- name: UnbanUser :exec
UPDATE users SET is_allowed = true, ban_reason = NULL, banned_at = NULL, updated_at = $2
WHERE user_id = $1 AND deleted_at IS NULL;
