-- Cleanup order matters: work_orders.requestor_id and the other FKs have
-- ON DELETE NO ACTION, so we null them first before deleting the system
-- user rows.

BEGIN;

DROP TRIGGER IF EXISTS trg_tenant_seed_system_user ON tenants;
DROP FUNCTION IF EXISTS seed_system_user_for_new_tenant();

UPDATE work_orders
   SET requestor_id = NULL
 WHERE requestor_id IN (SELECT id FROM users WHERE source = 'system');
UPDATE work_orders
   SET assignee_id = NULL
 WHERE assignee_id IN (SELECT id FROM users WHERE source = 'system');
UPDATE work_order_logs
   SET operator_id = NULL
 WHERE operator_id IN (SELECT id FROM users WHERE source = 'system');
UPDATE inventory_tasks
   SET assigned_to = NULL
 WHERE assigned_to IN (SELECT id FROM users WHERE source = 'system');
UPDATE inventory_items
   SET scanned_by = NULL
 WHERE scanned_by IN (SELECT id FROM users WHERE source = 'system');

DELETE FROM users WHERE source = 'system';

COMMIT;
