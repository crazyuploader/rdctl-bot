-- name: UpsertUserChatMembership :exec
INSERT INTO user_chat_memberships (user_id, chat_id, first_seen_at, last_seen_at, command_count)
VALUES ($1, $2, $3, $3, 0)
ON CONFLICT (user_id, chat_id) DO UPDATE SET
    last_seen_at = EXCLUDED.last_seen_at;

-- name: IncrementMembershipCommandCount :exec
UPDATE user_chat_memberships
SET command_count = command_count + 1, last_seen_at = $3
WHERE user_id = $1 AND chat_id = $2;

-- name: GetMembershipsByUser :many
SELECT * FROM user_chat_memberships WHERE user_id = $1 ORDER BY last_seen_at DESC;

-- name: GetMembershipsByChat :many
SELECT * FROM user_chat_memberships WHERE chat_id = $1 ORDER BY last_seen_at DESC;

-- name: GetMembership :one
SELECT * FROM user_chat_memberships WHERE user_id = $1 AND chat_id = $2;
