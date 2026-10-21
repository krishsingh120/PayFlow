-- 000002_create_refunds_table.down.sql
-- Revert refunds table and trigger.

DROP TRIGGER IF EXISTS trg_check_refund_total ON refunds;
DROP FUNCTION IF EXISTS check_refund_total();
DROP TABLE IF EXISTS refunds CASCADE;
