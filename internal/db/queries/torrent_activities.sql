-- name: InsertTorrentActivity :exec
INSERT INTO torrent_activities (
    request_id, user_id, chat_id, torrent_id, torrent_hash, torrent_name,
    magnet_link, action, status, file_size, progress, success, error_message,
    metadata, created_at, selected_files
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10, $11, $12, $13,
    $14, $15, $16
);

-- name: GetTorrentActivities :many
SELECT * FROM torrent_activities
WHERE ($1::bigint IS NULL OR user_id = $1)
ORDER BY created_at DESC
LIMIT $2;

-- name: CountTorrentAddsByUser :one
SELECT COUNT(*) FROM torrent_activities WHERE user_id = $1 AND action = 'add';
