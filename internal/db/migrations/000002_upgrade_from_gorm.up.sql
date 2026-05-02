-- 000002_upgrade_from_gorm.up.sql
-- Upgrade an existing main-branch GORM schema to the sqlc/pgx schema.
-- Safe to run after 000001 on a fresh install (all statements are idempotent).

SET search_path = public;

-- ── chats ──────────────────────────────────────────────────────────────────
ALTER TABLE chats
    ADD COLUMN IF NOT EXISTS username text,
    ADD COLUMN IF NOT EXISTS is_forum boolean NOT NULL DEFAULT false;

-- ── users ──────────────────────────────────────────────────────────────────
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS language_code text,
    ADD COLUMN IF NOT EXISTS is_bot boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS is_premium boolean NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS ban_reason text,
    ADD COLUMN IF NOT EXISTS banned_at timestamptz,
    ADD COLUMN IF NOT EXISTS total_torrents_added bigint NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS total_downloads bigint NOT NULL DEFAULT 0;

ALTER TABLE users
    ALTER COLUMN total_commands TYPE bigint;

-- ── activity_logs ──────────────────────────────────────────────────────────
ALTER TABLE activity_logs
    ADD COLUMN IF NOT EXISTS message_id bigint;

-- Convert metadata to jsonb unless it is already jsonb.
DO $$
BEGIN
    IF (SELECT data_type FROM information_schema.columns
        WHERE table_name = 'activity_logs' AND column_name = 'metadata') <> 'jsonb' THEN
        EXECUTE $q$
            ALTER TABLE activity_logs
                ALTER COLUMN metadata TYPE jsonb
                USING CASE
                    WHEN metadata IS NULL THEN '{}'::jsonb
                    ELSE metadata::text::jsonb
                END
        $q$;
    END IF;
END $$;

UPDATE activity_logs
SET metadata = '{}'::jsonb
WHERE metadata IS NULL OR jsonb_typeof(metadata) IS DISTINCT FROM 'object';

ALTER TABLE activity_logs
    ALTER COLUMN metadata SET DEFAULT '{}'::jsonb,
    ALTER COLUMN metadata SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'activity_logs'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%jsonb_typeof(metadata)%'
    ) THEN
        ALTER TABLE activity_logs
            ADD CONSTRAINT activity_logs_metadata_check
            CHECK (jsonb_typeof(metadata) = 'object');
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'activity_logs' AND column_name = 'created_date'
    ) THEN
        EXECUTE 'ALTER TABLE activity_logs ADD COLUMN created_date date GENERATED ALWAYS AS ((created_at AT TIME ZONE ''UTC'')::date) STORED';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_activity_logs_created_date ON activity_logs (created_date);

-- ── torrent_activities ─────────────────────────────────────────────────────
ALTER TABLE torrent_activities
    ALTER COLUMN progress TYPE numeric(5,2)
    USING CASE
        WHEN progress IS NULL THEN NULL
        ELSE round(progress::numeric, 2)
    END;

DO $$
BEGIN
    IF (SELECT data_type FROM information_schema.columns
        WHERE table_name = 'torrent_activities' AND column_name = 'metadata') <> 'jsonb' THEN
        EXECUTE $q$
            ALTER TABLE torrent_activities
                ALTER COLUMN metadata TYPE jsonb
                USING CASE
                    WHEN metadata IS NULL THEN '{}'::jsonb
                    ELSE metadata::text::jsonb
                END
        $q$;
    END IF;
END $$;

DO $$
BEGIN
    IF (SELECT data_type FROM information_schema.columns
        WHERE table_name = 'torrent_activities' AND column_name = 'selected_files') <> 'jsonb' THEN
        EXECUTE $q$
            ALTER TABLE torrent_activities
                ALTER COLUMN selected_files TYPE jsonb
                USING CASE
                    WHEN selected_files IS NULL THEN '[]'::jsonb
                    ELSE selected_files::text::jsonb
                END
        $q$;
    END IF;
END $$;

UPDATE torrent_activities
SET metadata = '{}'::jsonb
WHERE metadata IS NULL OR jsonb_typeof(metadata) IS DISTINCT FROM 'object';

UPDATE torrent_activities
SET selected_files = '[]'::jsonb
WHERE selected_files IS NULL OR jsonb_typeof(selected_files) IS DISTINCT FROM 'array';

ALTER TABLE torrent_activities
    ALTER COLUMN metadata SET DEFAULT '{}'::jsonb,
    ALTER COLUMN metadata SET NOT NULL,
    ALTER COLUMN selected_files SET DEFAULT '[]'::jsonb,
    ALTER COLUMN selected_files SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'torrent_activities'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%jsonb_typeof(metadata)%'
    ) THEN
        ALTER TABLE torrent_activities
            ADD CONSTRAINT torrent_activities_metadata_check
            CHECK (jsonb_typeof(metadata) = 'object');
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'torrent_activities'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%jsonb_typeof(selected_files)%'
    ) THEN
        ALTER TABLE torrent_activities
            ADD CONSTRAINT torrent_activities_selected_files_check
            CHECK (jsonb_typeof(selected_files) = 'array');
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'torrent_activities' AND column_name = 'created_date'
    ) THEN
        EXECUTE 'ALTER TABLE torrent_activities ADD COLUMN created_date date GENERATED ALWAYS AS ((created_at AT TIME ZONE ''UTC'')::date) STORED';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_torrent_activities_created_date ON torrent_activities (created_date);

-- ── download_activities ────────────────────────────────────────────────────
DO $$
BEGIN
    IF (SELECT data_type FROM information_schema.columns
        WHERE table_name = 'download_activities' AND column_name = 'metadata') <> 'jsonb' THEN
        EXECUTE $q$
            ALTER TABLE download_activities
                ALTER COLUMN metadata TYPE jsonb
                USING CASE
                    WHEN metadata IS NULL THEN '{}'::jsonb
                    ELSE metadata::text::jsonb
                END
        $q$;
    END IF;
END $$;

UPDATE download_activities
SET metadata = '{}'::jsonb
WHERE metadata IS NULL OR jsonb_typeof(metadata) IS DISTINCT FROM 'object';

ALTER TABLE download_activities
    ALTER COLUMN metadata SET DEFAULT '{}'::jsonb,
    ALTER COLUMN metadata SET NOT NULL;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'download_activities'::regclass AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%jsonb_typeof(metadata)%'
    ) THEN
        ALTER TABLE download_activities
            ADD CONSTRAINT download_activities_metadata_check
            CHECK (jsonb_typeof(metadata) = 'object');
    END IF;
END $$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'download_activities' AND column_name = 'created_date'
    ) THEN
        EXECUTE 'ALTER TABLE download_activities ADD COLUMN created_date date GENERATED ALWAYS AS ((created_at AT TIME ZONE ''UTC'')::date) STORED';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_download_activities_created_date ON download_activities (created_date);

-- ── command_logs ───────────────────────────────────────────────────────────
ALTER TABLE command_logs
    ADD COLUMN IF NOT EXISTS message_id bigint;

ALTER TABLE command_logs
    ALTER COLUMN response_length TYPE bigint;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'command_logs' AND column_name = 'created_date'
    ) THEN
        EXECUTE 'ALTER TABLE command_logs ADD COLUMN created_date date GENERATED ALWAYS AS ((created_at AT TIME ZONE ''UTC'')::date) STORED';
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_command_logs_created_date ON command_logs (created_date);
CREATE INDEX IF NOT EXISTS idx_command_logs_command_date ON command_logs (command, created_date);

-- ── FK backfill: Telegram IDs → internal surrogate IDs ────────────────────
-- Old GORM schema stored Telegram chat_id in the FK columns; the new schema
-- references chats.id (internal PK). Drop the old FK definitions first so the
-- value rewrite is not blocked by constraints against chats(chat_id) or
-- users(user_id).

ALTER TABLE activity_logs DROP CONSTRAINT IF EXISTS fk_activity_logs_chat_id;
ALTER TABLE activity_logs DROP CONSTRAINT IF EXISTS activity_logs_chat_id_fkey;
ALTER TABLE torrent_activities DROP CONSTRAINT IF EXISTS fk_torrent_activities_chat_id;
ALTER TABLE torrent_activities DROP CONSTRAINT IF EXISTS torrent_activities_chat_id_fkey;
ALTER TABLE download_activities DROP CONSTRAINT IF EXISTS fk_download_activities_chat_id;
ALTER TABLE download_activities DROP CONSTRAINT IF EXISTS download_activities_chat_id_fkey;
ALTER TABLE command_logs DROP CONSTRAINT IF EXISTS fk_command_logs_chat_id;
ALTER TABLE command_logs DROP CONSTRAINT IF EXISTS command_logs_chat_id_fkey;
ALTER TABLE setting_audits DROP CONSTRAINT IF EXISTS fk_setting_audits_chat_id;
ALTER TABLE setting_audits DROP CONSTRAINT IF EXISTS setting_audits_chat_id_fkey;
ALTER TABLE setting_audits DROP CONSTRAINT IF EXISTS fk_setting_audits_changed_by;
ALTER TABLE setting_audits DROP CONSTRAINT IF EXISTS setting_audits_changed_by_fkey;

-- Seed any missing legacy chats referenced by log/audit rows before the
-- rewrite so old orphaned data survives the upgrade.
INSERT INTO chats (chat_id, title, type, created_at, updated_at)
SELECT DISTINCT legacy_chat_id,
       'Legacy Chat ' || legacy_chat_id,
       'unknown',
       NOW(),
       NOW()
FROM (
    SELECT chat_id AS legacy_chat_id FROM activity_logs
    UNION
    SELECT chat_id AS legacy_chat_id FROM torrent_activities
    UNION
    SELECT chat_id AS legacy_chat_id FROM download_activities
    UNION
    SELECT chat_id AS legacy_chat_id FROM command_logs
    UNION
    SELECT chat_id AS legacy_chat_id FROM setting_audits WHERE chat_id IS NOT NULL
) refs
WHERE NOT EXISTS (
    SELECT 1 FROM chats c WHERE c.chat_id = refs.legacy_chat_id
);

-- Seed any missing users referenced by setting_audits.changed_by while it
-- still stores Telegram user_id values.
INSERT INTO users (
    user_id, username, first_name, last_name,
    is_super_admin, is_allowed, first_seen_at, last_seen_at,
    total_commands, created_at, updated_at
)
SELECT DISTINCT legacy_user_id,
       'legacy_user_' || legacy_user_id,
       'Legacy',
       'User',
       false,
       false,
       NOW(),
       NOW(),
       0,
       NOW(),
       NOW()
FROM (
    SELECT changed_by AS legacy_user_id FROM setting_audits
) refs
WHERE NOT EXISTS (
    SELECT 1 FROM users u WHERE u.user_id = refs.legacy_user_id
);

UPDATE activity_logs al
SET chat_id = c.id
FROM chats c
WHERE al.chat_id = c.chat_id;

DELETE FROM activity_logs
WHERE chat_id NOT IN (SELECT id FROM chats);

UPDATE torrent_activities ta
SET chat_id = c.id
FROM chats c
WHERE ta.chat_id = c.chat_id;

DELETE FROM torrent_activities
WHERE chat_id NOT IN (SELECT id FROM chats);

UPDATE download_activities da
SET chat_id = c.id
FROM chats c
WHERE da.chat_id = c.chat_id;

DELETE FROM download_activities
WHERE chat_id NOT IN (SELECT id FROM chats);

UPDATE command_logs cl
SET chat_id = c.id
FROM chats c
WHERE cl.chat_id = c.chat_id;

DELETE FROM command_logs
WHERE chat_id NOT IN (SELECT id FROM chats);

UPDATE setting_audits sa
SET chat_id = c.id
FROM chats c
WHERE sa.chat_id = c.chat_id
  AND sa.chat_id IS NOT NULL;

-- Convert setting_audits.changed_by from users.user_id to users.id.
UPDATE setting_audits sa
SET changed_by = u.id
FROM users u
WHERE sa.changed_by = u.user_id;

DELETE FROM setting_audits
WHERE changed_by NOT IN (SELECT id FROM users);

-- ── Recreate foreign keys against internal PKs ─────────────────────────────
ALTER TABLE activity_logs
    ADD CONSTRAINT fk_activity_logs_chat_id
        FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE;

ALTER TABLE torrent_activities
    ADD CONSTRAINT fk_torrent_activities_chat_id
        FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE;

ALTER TABLE download_activities
    ADD CONSTRAINT fk_download_activities_chat_id
        FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE;

ALTER TABLE command_logs
    ADD CONSTRAINT fk_command_logs_chat_id
        FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE CASCADE;

ALTER TABLE setting_audits
    ADD CONSTRAINT fk_setting_audits_changed_by
        FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE setting_audits
    ADD CONSTRAINT fk_setting_audits_chat_id
        FOREIGN KEY (chat_id) REFERENCES chats(id) ON DELETE SET NULL;

-- ── New tables (not present in old GORM schema) ───────────────────────────

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

CREATE TABLE IF NOT EXISTS user_daily_stats (
    id              bigint      GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    stat_date       date        NOT NULL,
    user_id         bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    commands        bigint      NOT NULL DEFAULT 0,
    torrents_added  bigint      NOT NULL DEFAULT 0,
    downloads       bigint      NOT NULL DEFAULT 0,
    CONSTRAINT uq_user_daily_stats UNIQUE (stat_date, user_id)
);

-- ── New indexes ────────────────────────────────────────────────────────────

-- users
CREATE INDEX IF NOT EXISTS idx_users_active     ON users (user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_username   ON users (username) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);

-- BRIN indexes for append-only high-volume tables
CREATE INDEX IF NOT EXISTS idx_activity_logs_created_at_brin     ON activity_logs     USING BRIN (created_at);
CREATE INDEX IF NOT EXISTS idx_torrent_activities_created_at_brin ON torrent_activities USING BRIN (created_at);
CREATE INDEX IF NOT EXISTS idx_download_activities_created_at_brin ON download_activities USING BRIN (created_at);
CREATE INDEX IF NOT EXISTS idx_command_logs_created_at_brin      ON command_logs      USING BRIN (created_at);

-- created_date indexes for GROUP BY queries
CREATE INDEX IF NOT EXISTS idx_activity_logs_created_date     ON activity_logs     (created_date);
CREATE INDEX IF NOT EXISTS idx_torrent_activities_created_date ON torrent_activities (created_date);
CREATE INDEX IF NOT EXISTS idx_download_activities_created_date ON download_activities (created_date);
CREATE INDEX IF NOT EXISTS idx_command_logs_created_date      ON command_logs      (created_date);

-- command popularity
CREATE INDEX IF NOT EXISTS idx_command_logs_command_date ON command_logs (command, created_date);

-- activity_logs
CREATE INDEX IF NOT EXISTS idx_activity_logs_request_id    ON activity_logs (request_id);
CREATE INDEX IF NOT EXISTS idx_user_created                ON activity_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_chat_created                ON activity_logs (chat_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_logs_activity_type ON activity_logs (activity_type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activity_logs_command       ON activity_logs (command) WHERE command IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_activity_logs_username      ON activity_logs (username) WHERE username IS NOT NULL;

-- torrent_activities
CREATE INDEX IF NOT EXISTS idx_torrent_activities_request_id   ON torrent_activities (request_id);
CREATE INDEX IF NOT EXISTS idx_torrent_user_time               ON torrent_activities (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_torrent_user_action             ON torrent_activities (user_id, action);
CREATE INDEX IF NOT EXISTS idx_torrent_activities_torrent_id   ON torrent_activities (torrent_id);
CREATE INDEX IF NOT EXISTS idx_torrent_activities_torrent_hash ON torrent_activities (torrent_hash) WHERE torrent_hash IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_torrent_activities_chat_id      ON torrent_activities (chat_id);

-- download_activities
CREATE INDEX IF NOT EXISTS idx_download_activities_request_id          ON download_activities (request_id);
CREATE INDEX IF NOT EXISTS idx_download_user_time                      ON download_activities (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_download_user_action                    ON download_activities (user_id, action);
CREATE INDEX IF NOT EXISTS idx_download_activities_download_id         ON download_activities (download_id) WHERE download_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_download_activities_host                ON download_activities (host) WHERE host IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_download_activities_torrent_activity_id ON download_activities (torrent_activity_id) WHERE torrent_activity_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_download_activities_chat_id             ON download_activities (chat_id);

-- command_logs
CREATE INDEX IF NOT EXISTS idx_command_logs_user_time ON command_logs (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_command_logs_chat_id   ON command_logs (chat_id);
CREATE INDEX IF NOT EXISTS idx_command_logs_command   ON command_logs (command);

-- kept_torrents / kept_torrent_actions
CREATE INDEX IF NOT EXISTS idx_kept_by_torrent                ON kept_torrents         (kept_by_id, torrent_id);
CREATE INDEX IF NOT EXISTS idx_kept_torrent_actions_torrent_id ON kept_torrent_actions  (torrent_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_kept_torrent_actions_user_id   ON kept_torrent_actions  (user_id);

-- setting_audits
CREATE INDEX IF NOT EXISTS idx_setting_audits_key_time   ON setting_audits (key, changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_setting_audits_changed_by ON setting_audits (changed_by);
CREATE INDEX IF NOT EXISTS idx_setting_audits_chat_id    ON setting_audits (chat_id) WHERE chat_id IS NOT NULL;

-- messages
CREATE INDEX IF NOT EXISTS idx_messages_chat_sent ON messages (chat_id, sent_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_user_sent ON messages (user_id, sent_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_thread    ON messages (chat_id, thread_id) WHERE thread_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_messages_reply_to  ON messages (reply_to_id) WHERE reply_to_id IS NOT NULL;

-- user_chat_memberships
CREATE INDEX IF NOT EXISTS idx_memberships_chat_id ON user_chat_memberships (chat_id);
CREATE INDEX IF NOT EXISTS idx_memberships_user_id ON user_chat_memberships (user_id);

-- user_daily_stats
CREATE INDEX IF NOT EXISTS idx_user_daily_stats_user_date ON user_daily_stats (user_id, stat_date DESC);
CREATE INDEX IF NOT EXISTS idx_user_daily_stats_date      ON user_daily_stats (stat_date);
