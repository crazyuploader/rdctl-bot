-- 000002_upgrade_from_gorm.down.sql
-- This migration is intentionally irreversible. The old schema stored Telegram
-- IDs in several foreign-key columns; after upgrading and backfilling to
-- internal surrogate IDs, a safe automatic downgrade is not possible.

SET search_path = public;

SELECT 1;
