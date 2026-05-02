-- ── daily_stats ────────────────────────────────────────────────────────────

-- name: IncrementDailyCommand :exec
INSERT INTO daily_stats (stat_date, commands_total, commands_success, commands_fail, updated_at)
VALUES ($1, 1, CASE WHEN $2::bool THEN 1 ELSE 0 END, CASE WHEN $2::bool THEN 0 ELSE 1 END, now())
ON CONFLICT (stat_date) DO UPDATE SET
    commands_total   = daily_stats.commands_total   + 1,
    commands_success = daily_stats.commands_success + CASE WHEN $2::bool THEN 1 ELSE 0 END,
    commands_fail    = daily_stats.commands_fail    + CASE WHEN $2::bool THEN 0 ELSE 1 END,
    updated_at       = now();

-- name: IncrementDailyTorrent :exec
INSERT INTO daily_stats (stat_date, torrents_added, updated_at)
VALUES ($1, 1, now())
ON CONFLICT (stat_date) DO UPDATE SET
    torrents_added = daily_stats.torrents_added + 1,
    updated_at     = now();

-- name: IncrementDailyDownload :exec
INSERT INTO daily_stats (stat_date, downloads_total, updated_at)
VALUES ($1, 1, now())
ON CONFLICT (stat_date) DO UPDATE SET
    downloads_total = daily_stats.downloads_total + 1,
    updated_at      = now();

-- name: GetDailyStats :many
SELECT * FROM daily_stats
WHERE stat_date BETWEEN $1 AND $2
ORDER BY stat_date ASC;

-- name: GetDailyStatsSingle :one
SELECT * FROM daily_stats WHERE stat_date = $1;

-- ── user_daily_stats ───────────────────────────────────────────────────────

-- name: IncrementUserDailyCommand :exec
INSERT INTO user_daily_stats (stat_date, user_id, commands)
VALUES ($1, $2, 1)
ON CONFLICT (stat_date, user_id) DO UPDATE SET
    commands = user_daily_stats.commands + 1;

-- name: IncrementUserDailyTorrent :exec
INSERT INTO user_daily_stats (stat_date, user_id, torrents_added)
VALUES ($1, $2, 1)
ON CONFLICT (stat_date, user_id) DO UPDATE SET
    torrents_added = user_daily_stats.torrents_added + 1;

-- name: IncrementUserDailyDownload :exec
INSERT INTO user_daily_stats (stat_date, user_id, downloads)
VALUES ($1, $2, 1)
ON CONFLICT (stat_date, user_id) DO UPDATE SET
    downloads = user_daily_stats.downloads + 1;

-- name: GetUserDailyStats :many
SELECT * FROM user_daily_stats
WHERE user_id = $1 AND stat_date BETWEEN $2 AND $3
ORDER BY stat_date ASC;

-- ── top users ──────────────────────────────────────────────────────────────

-- name: GetTopUsersByCommands :many
SELECT u.user_id, u.username, u.first_name, u.last_name, u.total_commands
FROM users u
WHERE u.deleted_at IS NULL
ORDER BY u.total_commands DESC
LIMIT $1;

-- name: GetTopUsersByTorrents :many
SELECT u.user_id, u.username, u.first_name, u.last_name, u.total_torrents_added
FROM users u
WHERE u.deleted_at IS NULL
ORDER BY u.total_torrents_added DESC
LIMIT $1;

-- name: GetTopUsersByDownloads :many
SELECT u.user_id, u.username, u.first_name, u.last_name, u.total_downloads
FROM users u
WHERE u.deleted_at IS NULL
ORDER BY u.total_downloads DESC
LIMIT $1;

-- name: GetTopUsersByCommandsInRange :many
SELECT uds.user_id, u.username, u.first_name, u.last_name,
       SUM(uds.commands) AS commands
FROM user_daily_stats uds
JOIN users u ON u.id = uds.user_id
WHERE uds.stat_date BETWEEN $1 AND $2
  AND u.deleted_at IS NULL
GROUP BY uds.user_id, u.username, u.first_name, u.last_name
ORDER BY commands DESC
LIMIT $3;

-- ── command popularity ─────────────────────────────────────────────────────

-- name: GetCommandPopularity :many
SELECT command, COUNT(*) AS total, SUM(CASE WHEN success THEN 1 ELSE 0 END) AS success_count
FROM command_logs
WHERE created_date BETWEEN $1 AND $2
GROUP BY command
ORDER BY total DESC
LIMIT $3;

-- name: GetCommandPopularityAllTime :many
SELECT command, COUNT(*) AS total, SUM(CASE WHEN success THEN 1 ELSE 0 END) AS success_count
FROM command_logs
GROUP BY command
ORDER BY total DESC
LIMIT $1;

-- ── activity heatmap (hour-of-day × day-of-week) ──────────────────────────

-- name: GetActivityHeatmap :many
SELECT
    EXTRACT(DOW  FROM created_at)  AS day_of_week,
    EXTRACT(HOUR FROM created_at)  AS hour_of_day,
    COUNT(*)                       AS count
FROM command_logs
WHERE created_date BETWEEN $1 AND $2
GROUP BY day_of_week, hour_of_day
ORDER BY day_of_week, hour_of_day;

-- ── summary stats ──────────────────────────────────────────────────────────

-- name: GetGlobalSummaryStats :one
SELECT
    (SELECT COUNT(*) FROM users  WHERE deleted_at IS NULL)                          AS total_users,
    (SELECT COUNT(*) FROM users  WHERE deleted_at IS NULL AND is_allowed = true)    AS allowed_users,
    (SELECT COUNT(*) FROM users  WHERE deleted_at IS NULL AND ban_reason IS NOT NULL) AS banned_users,
    (SELECT COUNT(*) FROM chats)                                                    AS total_chats,
    (SELECT COALESCE(SUM(total_commands),       0) FROM users WHERE deleted_at IS NULL) AS total_commands,
    (SELECT COALESCE(SUM(total_torrents_added), 0) FROM users WHERE deleted_at IS NULL) AS total_torrents,
    (SELECT COALESCE(SUM(total_downloads),      0) FROM users WHERE deleted_at IS NULL) AS total_downloads;
