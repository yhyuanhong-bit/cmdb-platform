ALTER TABLE locations ADD COLUMN IF NOT EXISTS latitude DOUBLE PRECISION;
ALTER TABLE locations ADD COLUMN IF NOT EXISTS longitude DOUBLE PRECISION;

-- Backfill known locations from seed data
UPDATE locations SET latitude = 23.5, longitude = 121.0 WHERE slug = 'tw';
UPDATE locations SET latitude = 35.0, longitude = 105.0 WHERE slug = 'china';
UPDATE locations SET latitude = 36.0, longitude = 138.0 WHERE slug = 'japan';
UPDATE locations SET latitude = 1.35, longitude = 103.8 WHERE slug = 'singapore';

-- Sub-locations
UPDATE locations SET latitude = 25.03, longitude = 121.56 WHERE slug = 'north' AND parent_id IS NOT NULL;
UPDATE locations SET latitude = 22.63, longitude = 120.30 WHERE slug = 'south' AND parent_id IS NOT NULL;
UPDATE locations SET latitude = 25.04, longitude = 121.51 WHERE slug = 'taipei';
UPDATE locations SET latitude = 22.62, longitude = 120.31 WHERE slug = 'kaohsiung';
