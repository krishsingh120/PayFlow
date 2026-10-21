-- 000002_create_refunds_table.up.sql
-- Create refunds table with over-refund protection trigger.

CREATE TABLE IF NOT EXISTS refunds (
    id          TEXT        PRIMARY KEY,
    payment_id  TEXT        NOT NULL REFERENCES payments(id) ON DELETE RESTRICT,
    amount      BIGINT      NOT NULL CHECK (amount > 0),
    status      TEXT        NOT NULL
                            CHECK (status IN ('PENDING', 'SUCCEEDED', 'FAILED')),
    reason      TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refunds_payment_id ON refunds (payment_id);
CREATE INDEX IF NOT EXISTS idx_refunds_status ON refunds (status);

CREATE OR REPLACE FUNCTION check_refund_total()
RETURNS TRIGGER AS $$
DECLARE
    v_payment_amount BIGINT;
    v_total_refunded BIGINT;
BEGIN
    -- Only check against active (non-FAILED) refunds
    IF NEW.status = 'FAILED' THEN
        RETURN NEW;
    END IF;

    SELECT amount INTO v_payment_amount
    FROM payments
    WHERE id = NEW.payment_id;

    IF v_payment_amount IS NULL THEN
        RAISE EXCEPTION 'payment % does not exist', NEW.payment_id;
    END IF;

    SELECT COALESCE(SUM(amount), 0) INTO v_total_refunded
    FROM refunds
    WHERE payment_id = NEW.payment_id
      AND status != 'FAILED'
      AND id != NEW.id;

    IF (v_total_refunded + NEW.amount) > v_payment_amount THEN
        RAISE EXCEPTION 'refund total (% + %) exceeds payment amount (%)',
            v_total_refunded, NEW.amount, v_payment_amount;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_check_refund_total ON refunds;
CREATE TRIGGER trg_check_refund_total
BEFORE INSERT OR UPDATE ON refunds
FOR EACH ROW
EXECUTE FUNCTION check_refund_total();
