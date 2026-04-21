-- Phase 4.8: audit_events gains an operator_type discriminator so system /
-- integration / sync writes can record NULL operator_id without violating
-- the FK. CHECK enforces: user <=> operator_id required; non-user <=>
-- operator_id must be NULL. Append-only trigger (000028) is temporarily
-- disabled to allow the backfill UPDATEs; it is re-enabled inside the same
-- transaction so no window exists where audit_events is mutable.

BEGIN;

CREATE TYPE audit_operator_type AS ENUM (
    'user',
    'system',
    'integration',
    'sync',
    'anonymous'
);

ALTER TABLE audit_events DISABLE TRIGGER audit_events_no_update;

ALTER TABLE audit_events
    ADD COLUMN operator_type audit_operator_type NOT NULL DEFAULT 'user';

UPDATE audit_events SET operator_type = 'integration'
    WHERE operator_id IS NULL AND module = 'integration';
UPDATE audit_events SET operator_type = 'system'
    WHERE operator_id IS NULL AND operator_type = 'user';

UPDATE audit_events SET operator_id = NULL, operator_type = 'system'
    WHERE operator_id = '00000000-0000-0000-0000-000000000000'::uuid;

ALTER TABLE audit_events ADD CONSTRAINT chk_audit_operator_type_id_match
    CHECK (
        (operator_type = 'user' AND operator_id IS NOT NULL)
        OR (operator_type <> 'user' AND operator_id IS NULL)
    );

CREATE INDEX idx_audit_events_type_created
    ON audit_events(operator_type, created_at DESC);

ALTER TABLE audit_events ENABLE TRIGGER audit_events_no_update;

COMMIT;
