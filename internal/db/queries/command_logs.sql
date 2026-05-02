-- name: InsertCommandLog :exec
INSERT INTO command_logs (
    user_id, chat_id, username, command, full_command,
    message_id, message_thread_id,
    execution_time, success, error_message, response_length, created_at
) VALUES (
    $1, $2, $3, $4, $5,
    $6, $7,
    $8, $9, $10, $11, $12
);
