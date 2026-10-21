-- 000001_create_payments_table.down.sql
-- Revert payments table creation.

DROP TABLE IF EXISTS payments CASCADE;
