-- Phase 2.15 rollback.
DROP INDEX IF EXISTS idx_work_order_dedup_work_order;
DROP TABLE IF EXISTS work_order_dedup;
