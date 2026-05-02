-- name: UpsertChat :one
INSERT INTO chats (chat_id, title, type, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (chat_id) DO UPDATE SET
    title      = EXCLUDED.title,
    type       = EXCLUDED.type,
    updated_at = EXCLUDED.updated_at
RETURNING *;
