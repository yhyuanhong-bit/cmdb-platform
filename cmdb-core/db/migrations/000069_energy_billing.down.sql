DROP TABLE IF EXISTS energy_daily_kwh;

DROP TRIGGER IF EXISTS energy_tariffs_set_updated_at ON energy_tariffs;
DROP FUNCTION IF EXISTS trg_energy_tariffs_set_updated_at();

DROP TABLE IF EXISTS energy_tariffs;
