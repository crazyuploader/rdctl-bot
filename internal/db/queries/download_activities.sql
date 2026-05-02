-- name: InsertDownloadActivity :exec
INSERT INTO download_activities (
    request_id, user_id, chat_id, download_id, original_link, file_name,
    file_size, host, action, success, error_message, metadata, created_at,
    torrent_activity_id
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12, $13,
    $14
);

-- name: CountDownloadsByUser :one
SELECT COUNT(*) FROM download_activities WHERE user_id = $1 AND action = 'unrestrict';
