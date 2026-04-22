-- Reverse of 000059. Drop the constraint before the column so a
-- concurrent session checking the constraint can't race a column drop.
BEGIN;

ALTER TABLE quality_scores
    DROP CONSTRAINT IF EXISTS quality_scores_access_weight_range;

ALTER TABLE quality_scores
    DROP COLUMN IF EXISTS access_weight;

COMMIT;
