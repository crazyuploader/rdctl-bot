-- name: InsertActivityLog :exec
INSERT INTO activity_logs (
    request_id, user_id, chat_id, username, activity_type, command,
    message_thread_id, success, error_message, metadata, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11
);

-- name: CountActivitiesByUser :one
SELECT COUNT(*) FROM activity_logs WHERE user_id = $1;
