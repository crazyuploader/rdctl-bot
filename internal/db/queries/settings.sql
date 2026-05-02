-- name: GetSetting :one
SELECT * FROM settings WHERE key = $1;

-- name: UpsertSetting :exec
INSERT INTO settings (key, value, updated_at)
VALUES ($1, $2, $3)
ON CONFLICT (key) DO UPDATE SET
    value      = EXCLUDED.value,
    updated_at = EXCLUDED.updated_at;

-- name: GetSettingHistory :many
SELECT * FROM setting_audits WHERE key = $1 ORDER BY changed_at DESC LIMIT $2;

-- name: InsertSettingAudit :exec
INSERT INTO setting_audits (key, old_value, new_value, changed_by, chat_id, changed_at)
VALUES ($1, $2, $3, $4, $5, $6);
