-- name: UpsertChat :one
INSERT INTO chats (chat_id, title, username, type, is_forum, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (chat_id) DO UPDATE SET
    title      = EXCLUDED.title,
    username   = EXCLUDED.username,
    type       = EXCLUDED.type,
    is_forum   = EXCLUDED.is_forum,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: GetChatByID :one
SELECT * FROM chats WHERE id = $1;

-- name: GetChatByChatID :one
SELECT * FROM chats WHERE chat_id = $1;
