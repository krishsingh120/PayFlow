-- 000004_create_webhook_deliveries_table.down.sql
-- Revert webhook_deliveries table.

DROP INDEX IF EXISTS idx_webhook_payment_id;
DROP INDEX IF EXISTS idx_webhook_pending;
DROP TABLE IF EXISTS webhook_deliveries;
