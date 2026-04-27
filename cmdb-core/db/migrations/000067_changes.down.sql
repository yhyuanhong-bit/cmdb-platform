DROP TABLE IF EXISTS change_comments;
DROP TABLE IF EXISTS change_problems;
DROP TABLE IF EXISTS change_services;
DROP TABLE IF EXISTS change_assets;
DROP TABLE IF EXISTS change_approvals;

DROP TRIGGER IF EXISTS changes_set_updated_at ON changes;
DROP FUNCTION IF EXISTS trg_changes_set_updated_at();

DROP TABLE IF EXISTS changes;
