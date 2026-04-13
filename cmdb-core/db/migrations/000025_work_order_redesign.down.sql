DROP TABLE IF EXISTS notifications;
UPDATE work_orders SET status = 'draft' WHERE status = 'submitted';
UPDATE work_orders SET status = 'closed' WHERE status = 'verified';
ALTER TABLE work_orders DROP COLUMN IF EXISTS sla_breached;
ALTER TABLE work_orders DROP COLUMN IF EXISTS sla_warning_sent;
ALTER TABLE work_orders DROP COLUMN IF EXISTS sla_deadline;
ALTER TABLE work_orders DROP COLUMN IF EXISTS approved_by;
ALTER TABLE work_orders DROP COLUMN IF EXISTS approved_at;
