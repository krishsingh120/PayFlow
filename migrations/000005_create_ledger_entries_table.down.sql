-- 000005_create_ledger_entries_table.down.sql
-- Revert ledger_entries table.

DROP INDEX IF EXISTS idx_ledger_account;
DROP INDEX IF EXISTS idx_ledger_payment_id;
DROP TABLE IF EXISTS ledger_entries;
