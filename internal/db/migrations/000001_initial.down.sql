-- 000001_initial.down.sql
-- Drop all tables in reverse FK dependency order

DROP TABLE IF EXISTS setting_audits;
DROP TABLE IF EXISTS kept_torrent_actions;
DROP TABLE IF EXISTS kept_torrents;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS command_logs;
DROP TABLE IF EXISTS download_activities;
DROP TABLE IF EXISTS torrent_activities;
DROP TABLE IF EXISTS activity_logs;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS chats;
