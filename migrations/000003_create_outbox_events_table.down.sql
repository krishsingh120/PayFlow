-- 000003_create_outbox_events_table.down.sql
-- Revert outbox_events table.

DROP INDEX IF EXISTS idx_outbox_unpublished;
DROP TABLE IF EXISTS outbox_events;
