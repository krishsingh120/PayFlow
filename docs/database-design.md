# Database Design

## Overview

All tables use `TEXT` primary keys (ULIDs) for sortable, URL-safe identifiers.
All monetary amounts are stored as `BIGINT` representing paise (1 INR = 100 paise).
Timestamps are `TIMESTAMPTZ` (timezone-aware) and always stored in UTC.

---

## Tables

### `payments`

Core payment record. One row per unique payment attempt.

```sql
CREATE TABLE payments (
    id              TEXT        PRIMARY KEY,
    idempotency_key TEXT        UNIQUE NOT NULL,
    amount          BIGINT      NOT NULL CHECK (amount > 0),
    currency        CHAR(3)     NOT NULL,
    status          TEXT        NOT NULL
                                CHECK (status IN ('CREATED','PENDING','SUCCEEDED','FAILED','UNKNOWN')),
    description     TEXT,
    metadata        JSONB,
    provider_ref    TEXT,               -- Provider's transaction reference
    failure_reason  TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_payments_status     ON payments(status);
CREATE INDEX idx_payments_created_at ON payments(created_at DESC);
```

**Key constraints:**
- `UNIQUE(idempotency_key)` — database-level idempotency enforcement
- `CHECK (amount > 0)` — no zero or negative payments
- `CHECK (status IN (...))` — state machine enforced at DB level

---

### `refunds`

Refund records. Multiple refunds per payment are allowed (up to total payment amount).

```sql
CREATE TABLE refunds (
    id          TEXT        PRIMARY KEY,
    payment_id  TEXT        NOT NULL REFERENCES payments(id),
    amount      BIGINT      NOT NULL CHECK (amount > 0),
    status      TEXT        NOT NULL
                            CHECK (status IN ('PENDING','SUCCEEDED','FAILED')),
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_refunds_payment_id ON refunds(payment_id);
```

**Over-refund protection:**
A trigger prevents the sum of refund amounts from exceeding the parent payment amount:

```sql
CREATE OR REPLACE FUNCTION check_refund_total()
RETURNS TRIGGER AS $$
BEGIN
    IF (
        SELECT COALESCE(SUM(amount), 0)
        FROM refunds
        WHERE payment_id = NEW.payment_id
          AND status != 'FAILED'
    ) > (SELECT amount FROM payments WHERE id = NEW.payment_id)
    THEN
        RAISE EXCEPTION 'refund total exceeds payment amount';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

The application also uses `SELECT ... FOR UPDATE` on the payment row to serialise concurrent refund requests.

---

### `outbox_events` (Phase 5)

Transactional Outbox Pattern. Written atomically with payment state changes,
then relayed to RabbitMQ by a background relay process.

```sql
CREATE TABLE outbox_events (
    id           BIGSERIAL   PRIMARY KEY,
    aggregate_id TEXT        NOT NULL,   -- payment ID
    event_type   TEXT        NOT NULL,   -- e.g. 'payment.succeeded'
    payload      JSONB       NOT NULL,
    published    BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_outbox_unpublished ON outbox_events(created_at)
    WHERE published = FALSE;
```

---

### `webhook_deliveries` (Phase 5)

Tracks webhook delivery attempts and retry state.

```sql
CREATE TABLE webhook_deliveries (
    id            TEXT        PRIMARY KEY,
    payment_id    TEXT        REFERENCES payments(id),
    event_type    TEXT        NOT NULL,
    url           TEXT        NOT NULL,
    payload       JSONB       NOT NULL,
    status        TEXT        NOT NULL
                              CHECK (status IN ('PENDING','DELIVERED','FAILED')),
    attempts      INT         NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMPTZ,
    last_error    TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_webhook_pending ON webhook_deliveries(next_retry_at)
    WHERE status = 'PENDING';
```

---

### `ledger_entries` (Phase 6)

Double-entry bookkeeping. Every financial event produces two matching entries
(debit + credit) summing to zero.

```sql
CREATE TABLE ledger_entries (
    id         BIGSERIAL   PRIMARY KEY,
    payment_id TEXT        REFERENCES payments(id),
    account    TEXT        NOT NULL,   -- 'receivable', 'revenue', 'refund_liability'
    direction  TEXT        NOT NULL CHECK (direction IN ('DEBIT','CREDIT')),
    amount     BIGINT      NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ledger_payment_id ON ledger_entries(payment_id);
```

**Invariant:** For every payment, `SUM(DEBIT) = SUM(CREDIT)` across all ledger entries.

---

## Design Notes

- **No soft deletes** — payments and refunds are append-only financial records
- **JSONB metadata** — flexible per-payment key-value store without schema changes
- **Partial indexes** — `WHERE published = FALSE` and `WHERE status = 'PENDING'` keep query plans efficient as the table grows
- **Trigger-enforced money safety** — critical invariants are not left to application code alone
