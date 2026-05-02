-- 000001_initial.up.sql
-- Initial schema migration (v1)

CREATE TABLE IF NOT EXISTS chats (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    chat_id    bigint      NOT NULL,
    title      text,
    username   text,
    type       text,
    is_forum   bool        NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT chats_chat_id_key UNIQUE (chat_id)
);

CREATE TABLE IF NOT EXISTS users (
    id             bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id        bigint      NOT NULL,
    username       text,
    first_name     text,
    last_name      text,
    language_code  text,
    is_bot         bool        NOT NULL DEFAULT false,
    is_premium     bool        NOT NULL DEFAULT false,
    is_super_admin bool        NOT NULL DEFAULT false,
    is_allowed     bool        NOT NULL DEFAULT false,
    ban_reason     text,
    banned_at      timestamptz,
    first_seen_at  timestamptz NOT NULL DEFAULT now(),
    last_seen_at   timestamptz NOT NULL DEFAULT now(),
    total_commands       bigint      NOT NULL DEFAULT 0,
    total_torrents_added bigint      NOT NULL DEFAULT 0,
    total_downloads      bigint      NOT NULL DEFAULT 0,
    created_at           timestamptz NOT NULL DEFAULT now(),
    updated_at           timestamptz NOT NULL DEFAULT now(),
    deleted_at           timestamptz,
    CONSTRAINT users_user_id_key UNIQUE (user_id)
);

CREATE TABLE IF NOT EXISTS activity_logs (
    id                bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id        text,
    user_id           bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id           bigint      NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    username          text,
    activity_type     text        NOT NULL,
    command           text,
    message_id        bigint,
    message_thread_id bigint,
    success           bool        NOT NULL DEFAULT true,
    error_message     text,
    metadata          jsonb       NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object'),
    created_at        timestamptz NOT NULL DEFAULT now(),
    created_date      date        GENERATED ALWAYS AS (created_at::date) STORED
);

CREATE TABLE IF NOT EXISTS torrent_activities (
    id             bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id     text,
    user_id        bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id        bigint      NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    torrent_id     text        NOT NULL,
    torrent_hash   text,
    torrent_name   text,
    magnet_link    text,
    action         text        NOT NULL,
    status         text,
    file_size      bigint,
    progress       numeric(5,2)             CHECK (progress >= 0 AND progress <= 100),
    success        bool        NOT NULL DEFAULT true,
    error_message  text,
    metadata       jsonb       NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object'),
    created_at     timestamptz NOT NULL DEFAULT now(),
    created_date   date        GENERATED ALWAYS AS (created_at::date) STORED,
    selected_files jsonb       NOT NULL DEFAULT '[]' CHECK (jsonb_typeof(selected_files) = 'array')
);

CREATE TABLE IF NOT EXISTS download_activities (
    id                  bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    request_id          text,
    user_id             bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id             bigint      NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    download_id         text,
    original_link       text,
    file_name           text,
    file_size           bigint,
    host                text,
    action              text        NOT NULL,
    success             bool        NOT NULL DEFAULT true,
    error_message       text,
    metadata            jsonb       NOT NULL DEFAULT '{}' CHECK (jsonb_typeof(metadata) = 'object'),
    created_at          timestamptz NOT NULL DEFAULT now(),
    created_date        date        GENERATED ALWAYS AS (created_at::date) STORED,
    torrent_activity_id bigint      REFERENCES torrent_activities(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS command_logs (
    id                bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id           bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id           bigint      NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    username          text,
    command           text        NOT NULL,
    full_command      text,
    message_id        bigint,
    message_thread_id bigint,
    execution_time    bigint,
    success           bool        NOT NULL DEFAULT true,
    error_message     text,
    response_length   bigint,
    created_at        timestamptz NOT NULL DEFAULT now(),
    created_date      date        GENERATED ALWAYS AS (created_at::date) STORED
);

CREATE TABLE IF NOT EXISTS settings (
    key        text        PRIMARY KEY,
    value      text        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS kept_torrents (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    torrent_id text        NOT NULL,
    filename   text,
    kept_by_id bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    kept_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_kept_torrents_torrent_user UNIQUE (torrent_id, kept_by_id)
);

CREATE TABLE IF NOT EXISTS kept_torrent_actions (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    torrent_id text        NOT NULL,
    action     text        NOT NULL CHECK (action IN ('keep', 'unkeep')),
    user_id    bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    username   text,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- setting_audits.changed_by references users(id) (internal PK), consistent with
-- all other tables. The repository resolves the Telegram user_id to users.id
-- before inserting.
CREATE TABLE IF NOT EXISTS setting_audits (
    id         bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    key        text        NOT NULL,
    old_value  text,
    new_value  text,
    changed_by bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id    bigint      REFERENCES chats(id) ON DELETE SET NULL,
    changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS messages (
    id           bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    message_id   bigint      NOT NULL,
    chat_id      bigint      NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    user_id      bigint      REFERENCES users(id) ON DELETE SET NULL,
    thread_id    bigint,
    text         text,
    message_type text        NOT NULL DEFAULT 'text',
    reply_to_id  bigint,
    is_forward   bool        NOT NULL DEFAULT false,
    sent_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_message_chat UNIQUE (message_id, chat_id)
);

CREATE TABLE IF NOT EXISTS user_chat_memberships (
    id            bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id       bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    chat_id       bigint      NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    first_seen_at timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz NOT NULL DEFAULT now(),
    command_count bigint      NOT NULL DEFAULT 0,
    CONSTRAINT uq_user_chat_membership UNIQUE (user_id, chat_id)
);

-- ── users ──────────────────────────────────────────────────────────────────
-- Partial index: active (non-deleted) user lookups dominate queries
CREATE INDEX IF NOT EXISTS idx_users_active     ON users (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_username   ON users (username) WHERE deleted_at IS NULL;
-- Full deleted_at for soft-delete aware scans
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

-- ── activity_logs ──────────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_activity_logs_request_id    ON activity_logs (request_id);
-- Covering index: user feed ordered by time (most common query pattern)
CREATE INDEX IF NOT EXISTS idx_user_created                ON activity_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_created                ON activity_logs (chat_id, created_at DESC);
-- Individual column indexes for filtered queries
CREATE INDEX IF NOT EXISTS idx_activity_logs_activity_type ON activity_logs (activity_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_logs_command       ON activity_logs (command) WHERE command IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_activity_logs_username      ON activity_logs (username) WHERE username IS NOT NULL;

-- ── torrent_activities ─────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_torrent_activities_request_id   ON torrent_activities (request_id);
-- Covering index: feed ordered by time per user
CREATE INDEX IF NOT EXISTS idx_torrent_user_time               ON torrent_activities (user_id, created_at DESC);
-- Composite for count queries (e.g. COUNT WHERE user_id=? AND action='add')
CREATE INDEX IF NOT EXISTS idx_torrent_user_action             ON torrent_activities (user_id, action);
CREATE INDEX IF NOT EXISTS idx_torrent_activities_torrent_id   ON torrent_activities (torrent_id);
CREATE INDEX IF NOT EXISTS idx_torrent_activities_torrent_hash ON torrent_activities (torrent_hash) WHERE torrent_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_torrent_activities_chat_id      ON torrent_activities (chat_id);

-- ── download_activities ────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_download_activities_request_id          ON download_activities (request_id);
CREATE INDEX IF NOT EXISTS idx_download_user_time                      ON download_activities (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_download_user_action                    ON download_activities (user_id, action);
CREATE INDEX IF NOT EXISTS idx_download_activities_download_id         ON download_activities (download_id) WHERE download_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_download_activities_host                ON download_activities (host) WHERE host IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_download_activities_torrent_activity_id ON download_activities (torrent_activity_id) WHERE torrent_activity_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_download_activities_chat_id             ON download_activities (chat_id);

-- ── command_logs ───────────────────────────────────────────────────────────
-- Covering index: user command history ordered by time
CREATE INDEX IF NOT EXISTS idx_command_logs_user_time  ON command_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_command_logs_chat_id    ON command_logs (chat_id);
CREATE INDEX IF NOT EXISTS idx_command_logs_command    ON command_logs (command);

-- ── kept_torrents ──────────────────────────────────────────────────────────
-- Covering index: count kept torrents per user (used in limit check)
CREATE INDEX IF NOT EXISTS idx_kept_by_torrent ON kept_torrents (kept_by_id, torrent_id);

-- ── kept_torrent_actions ───────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_kept_torrent_actions_torrent_id ON kept_torrent_actions (torrent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_kept_torrent_actions_user_id    ON kept_torrent_actions (user_id);

-- ── setting_audits ─────────────────────────────────────────────────────────
-- Covering index: history lookup ordered by time per key
CREATE INDEX IF NOT EXISTS idx_setting_audits_key_time   ON setting_audits (key, changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_setting_audits_changed_by ON setting_audits (changed_by);
-- FK index: chat_id is nullable so partial index skips NULLs
CREATE INDEX IF NOT EXISTS idx_setting_audits_chat_id    ON setting_audits (chat_id) WHERE chat_id IS NOT NULL;

-- ── messages ───────────────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_messages_chat_sent    ON messages (chat_id, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_user_sent    ON messages (user_id, sent_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_thread       ON messages (chat_id, thread_id) WHERE thread_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_reply_to     ON messages (reply_to_id) WHERE reply_to_id IS NOT NULL;

-- ── user_chat_memberships ──────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_memberships_chat_id  ON user_chat_memberships (chat_id);
CREATE INDEX IF NOT EXISTS idx_memberships_user_id  ON user_chat_memberships (user_id);

-- ── BRIN indexes for append-only high-volume tables ────────────────────────
-- BRIN is minimal-storage; effective because rows are written in time order
CREATE INDEX IF NOT EXISTS idx_activity_logs_created_at_brin    ON activity_logs    USING BRIN (created_at);
CREATE INDEX IF NOT EXISTS idx_torrent_activities_created_at_brin ON torrent_activities USING BRIN (created_at);
CREATE INDEX IF NOT EXISTS idx_download_activities_created_at_brin ON download_activities USING BRIN (created_at);
CREATE INDEX IF NOT EXISTS idx_command_logs_created_at_brin     ON command_logs     USING BRIN (created_at);

-- ── created_date indexes for date-bucket GROUP BY queries ──────────────────
CREATE INDEX IF NOT EXISTS idx_activity_logs_created_date    ON activity_logs    (created_date);
CREATE INDEX IF NOT EXISTS idx_torrent_activities_created_date ON torrent_activities (created_date);
CREATE INDEX IF NOT EXISTS idx_download_activities_created_date ON download_activities (created_date);
CREATE INDEX IF NOT EXISTS idx_command_logs_created_date     ON command_logs     (created_date);

-- ── command popularity ─────────────────────────────────────────────────────
CREATE INDEX IF NOT EXISTS idx_command_logs_command_date ON command_logs (command, created_date);

-- ──────────────────────────────────────────────────────────────────────────
-- Stats / aggregation tables
-- ──────────────────────────────────────────────────────────────────────────

-- Pre-aggregated global daily stats (one row per calendar day).
-- Incremented atomically on each relevant event.
CREATE TABLE IF NOT EXISTS daily_stats (
    stat_date        date        PRIMARY KEY,
    commands_total   bigint      NOT NULL DEFAULT 0,
    commands_success bigint      NOT NULL DEFAULT 0,
    commands_fail    bigint      NOT NULL DEFAULT 0,
    torrents_added   bigint      NOT NULL DEFAULT 0,
    downloads_total  bigint      NOT NULL DEFAULT 0,
    active_users     bigint      NOT NULL DEFAULT 0,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

-- Per-user per-day aggregates for top-user and personal-stats queries.
CREATE TABLE IF NOT EXISTS user_daily_stats (
    id              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stat_date       date        NOT NULL,
    user_id         bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    commands        bigint      NOT NULL DEFAULT 0,
    torrents_added  bigint      NOT NULL DEFAULT 0,
    downloads       bigint      NOT NULL DEFAULT 0,
    CONSTRAINT uq_user_daily_stats UNIQUE (stat_date, user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_daily_stats_user_date ON user_daily_stats (user_id, stat_date DESC);
CREATE INDEX IF NOT EXISTS idx_user_daily_stats_date      ON user_daily_stats (stat_date);

-- Seed system user (user_id=0) and system chat (chat_id=0)
INSERT INTO chats (chat_id, title, type, created_at, updated_at)
VALUES (0, 'System Chat', 'system', NOW(), NOW())
ON CONFLICT (chat_id) DO NOTHING;

INSERT INTO users (user_id, username, first_name, last_name, is_super_admin, is_allowed, first_seen_at, last_seen_at, total_commands, created_at, updated_at)
VALUES (0, 'system', 'System', 'Bot', false, false, NOW(), NOW(), 0, NOW(), NOW())
ON CONFLICT (user_id) DO NOTHING;
