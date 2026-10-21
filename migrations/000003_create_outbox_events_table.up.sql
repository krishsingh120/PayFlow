-- 000003_create_outbox_events_table.up.sql
-- Transactional outbox table for reliable at-least-once message publishing.

CREATE TABLE IF NOT EXISTS outbox_events (
    id           BIGSERIAL   PRIMARY KEY,
    aggregate_id TEXT        NOT NULL,
    event_type   TEXT        NOT NULL,
    payload      JSONB       NOT NULL,
    published    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_outbox_unpublished ON outbox_events(created_at)
    WHERE published = FALSE;
