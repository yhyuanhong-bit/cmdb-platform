-- D2-P1 from review-2026-04-21-v2: make asset_dependencies.dependency_type
-- layered. The existing column is a free-form verb ('depends_on',
-- 'connects_to', ...) which is fine for display but gives us no way to
-- answer "show me all containment relationships" without a fuzzy LIKE.
-- We add a coarse category on top of the verb so queries and UIs can
-- pivot on four well-known buckets. The legacy verb column stays
-- untouched — no data shape change, no backfill contention with running
-- writers.
--
-- Backfill maps the verbs we've seen in prod/seed data into buckets.
-- Anything unrecognized falls into 'custom' so nothing gets NULL'd out.
-- The default on new inserts is 'dependency' because that matches the
-- pre-migration DEFAULT verb 'depends_on' — callers that don't specify
-- a category behave exactly as before.

BEGIN;

CREATE TYPE dependency_category AS ENUM (
    'containment',
    'dependency',
    'communication',
    'custom'
);

ALTER TABLE asset_dependencies
    ADD COLUMN dependency_category dependency_category;

UPDATE asset_dependencies
SET dependency_category = CASE
    WHEN dependency_type IN ('contains', 'part_of', 'mounted_in', 'hosts')
        THEN 'containment'::dependency_category
    WHEN dependency_type IN ('depends_on', 'requires', 'uses', 'needs')
        THEN 'dependency'::dependency_category
    WHEN dependency_type IN ('connects_to', 'talks_to', 'subscribes_to', 'publishes_to')
        THEN 'communication'::dependency_category
    ELSE 'custom'::dependency_category
END;

ALTER TABLE asset_dependencies
    ALTER COLUMN dependency_category SET NOT NULL,
    ALTER COLUMN dependency_category SET DEFAULT 'dependency';

-- Queries like "how many containment edges in tenant X" and "list all
-- communication links under location Y" will filter on (tenant_id,
-- dependency_category); this composite mirrors the idx_asset_deps_tenant
-- shape so the planner has a direct path.
CREATE INDEX idx_asset_deps_tenant_category
    ON asset_dependencies (tenant_id, dependency_category);

COMMIT;
