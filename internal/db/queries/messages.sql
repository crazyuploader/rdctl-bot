-- name: UpsertMessage :one
INSERT INTO messages (message_id, chat_id, user_id, thread_id, text, message_type, reply_to_id, is_forward, sent_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (message_id, chat_id) DO UPDATE SET
    text         = EXCLUDED.text,
    message_type = EXCLUDED.message_type
RETURNING *;

-- name: GetMessagesByUser :many
SELECT * FROM messages WHERE user_id = $1 ORDER BY sent_at DESC LIMIT $2;

-- name: GetMessagesByChat :many
SELECT * FROM messages WHERE chat_id = $1 ORDER BY sent_at DESC LIMIT $2;

-- name: CountMessagesByUser :one
SELECT COUNT(*) FROM messages WHERE user_id = $1;
