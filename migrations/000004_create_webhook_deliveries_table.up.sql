-- 000004_create_webhook_deliveries_table.up.sql
-- Webhook deliveries tracking table with retry backoff state.

CREATE TABLE IF NOT EXISTS webhook_deliveries (
    id            TEXT        PRIMARY KEY,
    payment_id    TEXT        REFERENCES payments(id) ON DELETE SET NULL,
    event_type    TEXT        NOT NULL,
    url           TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    status        TEXT        NOT NULL CHECK (status IN ('PENDING', 'DELIVERED', 'FAILED')),
    attempts      INT         NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_webhook_pending ON webhook_deliveries(next_retry_at)
    WHERE status = 'PENDING';
CREATE INDEX IF NOT EXISTS idx_webhook_payment_id ON webhook_deliveries(payment_id);
