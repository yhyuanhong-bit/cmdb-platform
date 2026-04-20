# ingestion-engine migrations

This directory is managed jointly with `cmdb-core/db/migrations/` under a single
global migration number registry. See `/cmdb-platform/docs/MIGRATIONS.md` for
the authoritative list of which number belongs to which service.

## Numbering

- Numbers are **globally unique across both services**.
- The CI guard at `.github/workflows/migration-check.yml` will fail any PR
  that introduces the same `NNNNNN_` prefix in both `cmdb-core/db/migrations/`
  and `ingestion-engine/db/migrations/`.
- Before creating a new migration here, update `docs/MIGRATIONS.md` first and
  pick the next free number listed under **Next available**.

## Why 000011 is missing

Historically `000011_bmc_field_authorities.*.sql` lived here and collided with
`cmdb-core`'s `000011_prediction_tables.*.sql`. In Phase 4.5 the ingestion
file was renamed to `000052_bmc_field_authorities.*.sql` to resolve the
collision. The file contents are byte-identical before and after rename.

If you are deploying this service to an environment where the old 000011 was
already applied (`SELECT version FROM schema_migrations` returns `11`), you
must first run the one-time metadata fix at:

    /cmdb-platform/ops/cutover/2026-04-20-ingestion-migration-rename.sql

That script only updates `schema_migrations.version` from 11 to 52. It does
not touch table data. In fresh environments where 000011 was never applied,
skip the cutover — the renamed file will run normally on first `migrate up`.
