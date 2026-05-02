-- name: UpsertKeptTorrent :exec
INSERT INTO kept_torrents (torrent_id, filename, kept_by_id, kept_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (torrent_id, kept_by_id) DO UPDATE SET
    filename = EXCLUDED.filename,
    kept_at  = EXCLUDED.kept_at;

-- name: DeleteKeptTorrent :exec
DELETE FROM kept_torrents WHERE torrent_id = $1 AND kept_by_id = $2;

-- name: DeleteKeptTorrentAdmin :exec
DELETE FROM kept_torrents WHERE torrent_id = $1;

-- name: GetKeptTorrent :one
SELECT * FROM kept_torrents WHERE torrent_id = $1 LIMIT 1;

-- name: ListKeptTorrents :many
SELECT
    kt.id,
    kt.torrent_id,
    kt.filename,
    kt.kept_by_id,
    kt.kept_at,
    u.id          AS user_id_pk,
    u.user_id     AS user_user_id,
    u.username    AS user_username,
    u.first_name  AS user_first_name,
    u.last_name   AS user_last_name
FROM kept_torrents kt
JOIN users u ON u.id = kt.kept_by_id
ORDER BY kt.kept_at DESC;

-- name: CountKeptByUser :one
SELECT COUNT(*) FROM kept_torrents WHERE kept_by_id = $1;

-- name: GetAllKeptTorrentIDs :many
SELECT torrent_id FROM kept_torrents;

-- name: CountKeptExcluding :one
SELECT COUNT(*) FROM kept_torrents WHERE kept_by_id = $1 AND torrent_id != $2;

-- name: InsertKeptTorrentAction :exec
INSERT INTO kept_torrent_actions (torrent_id, action, user_id, username, created_at)
VALUES ($1, $2, $3, $4, $5);
