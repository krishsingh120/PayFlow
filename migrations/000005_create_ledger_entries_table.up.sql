-- 000005_create_ledger_entries_table.up.sql
-- Create ledger_entries table for double-entry bookkeeping.

CREATE TABLE IF NOT EXISTS ledger_entries (
    id         BIGSERIAL   PRIMARY KEY,
    payment_id TEXT        REFERENCES payments(id) ON DELETE RESTRICT,
    account    TEXT        NOT NULL,   -- e.g. 'receivable', 'revenue', 'refund_liability'
    direction  TEXT        NOT NULL CHECK (direction IN ('DEBIT', 'CREDIT')),
    amount     BIGINT      NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_payment_id ON ledger_entries(payment_id);
CREATE INDEX IF NOT EXISTS idx_ledger_account ON ledger_entries(account);
