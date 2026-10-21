-- 000001_create_payments_table.up.sql
-- Create payments table with constraints and indexes for high-throughput transactional queries.

CREATE TABLE IF NOT EXISTS payments (
    id              TEXT        PRIMARY KEY,
    idempotency_key TEXT        UNIQUE NOT NULL,
    amount          BIGINT      NOT NULL CHECK (amount > 0),
    currency        CHAR(3)     NOT NULL,
    status          TEXT        NOT NULL
                                CHECK (status IN ('CREATED', 'PENDING', 'SUCCEEDED', 'FAILED', 'UNKNOWN')),
    description     TEXT,
    metadata        JSONB,
    provider_ref    TEXT,
    failure_reason  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Index for status filtering (e.g. list pending payments, metrics)
CREATE INDEX IF NOT EXISTS idx_payments_status ON payments (status);

-- Index for sorting / cursor pagination by creation timestamp
CREATE INDEX IF NOT EXISTS idx_payments_created_at ON payments (created_at DESC);
