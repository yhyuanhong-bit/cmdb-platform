# Migration Number Registry

> **Source of truth**: this file. Every new migration must be registered
> here, and CI enforces that disk files and this table stay in sync.

## Conventions

- Use 6-digit zero-padded prefixes: `NNNNNN_<snake_case_subject>.up.sql`
  and the matching `.down.sql`.
- Numbers are **globally unique across both `cmdb-core` and
  `ingestion-engine`**. See "Next available" below before picking one.
- Every `.up.sql` must have a paired `.down.sql` (even if it is just
  `DROP TABLE IF EXISTS …`).
- A PR that creates a migration must update this file in the same PR.
  The strict mode of `scripts/check-migration-collision.sh` will fail
  otherwise.

## Existing registrations

| #      | Service          | Subject                               | Commit | Notes |
|--------|------------------|---------------------------------------|--------|-------|
| 000001 | cmdb-core        | init_extensions                       | -      | uuid-ossp, pgcrypto |
| 000002 | cmdb-core        | tenants_and_identity                  | -      | |
| 000003 | cmdb-core        | locations                             | -      | |
| 000004 | cmdb-core        | assets_and_racks                      | -      | |
| 000005 | cmdb-core        | maintenance                           | -      | |
| 000006 | cmdb-core        | monitoring                            | -      | |
| 000007 | cmdb-core        | inventory                             | -      | |
| 000008 | cmdb-core        | audit                                 | -      | audit_events base table |
| 000009 | cmdb-core        | timescaledb_metrics                   | -      | |
| 000010 | ingestion-engine | ingestion_tables                      | -      | asset_field_authorities etc. |
| 000011 | cmdb-core        | prediction_tables                     | -      | Historical collision; ingestion-engine yielded to 000052 |
| 000012 | cmdb-core        | integration_tables                    | -      | |
| 000013 | cmdb-core        | bia_tables                            | -      | |
| 000014 | cmdb-core        | webhook_bia_filter                    | -      | |
| 000015 | cmdb-core        | quality_tables                        | -      | |
| 000016 | cmdb-core        | discovered_assets                     | -      | |
| 000017 | cmdb-core        | discovery_collectors                  | -      | |
| 000018 | cmdb-core        | phase3_tables                         | -      | |
| 000019 | cmdb-core        | phase4_group1                         | -      | Historical naming; unrelated to Phase 4 in this doc tree |
| 000020 | cmdb-core        | phase4_group2                         | -      | Historical naming |
| 000021 | cmdb-core        | soft_delete_location_filter           | -      | |
| 000022 | cmdb-core        | inventory_items_indexes               | -      | |
| 000023 | cmdb-core        | database_hardening                    | -      | FK indexes, pg_trgm |
| 000024 | cmdb-core        | placeholder                           | -      | No-op; historical slot preserved |
| 000025 | cmdb-core        | work_order_redesign                   | -      | |
| 000026 | cmdb-core        | timescaledb_compression               | -      | |
| 000027 | cmdb-core        | sync_system                           | -      | |
| 000028 | cmdb-core        | audit_and_constraints                 | -      | Append-only audit trigger |
| 000029 | cmdb-core        | unified_soft_delete                   | -      | |
| 000030 | cmdb-core        | sync_phase_a                          | -      | |
| 000031 | cmdb-core        | inventory_items_sync                  | -      | |
| 000032 | cmdb-core        | location_detect                       | -      | |
| 000033 | cmdb-core        | location_coordinates                  | -      | |
| 000034 | cmdb-core        | uniqueness_constraints                | -      | |
| 000035 | cmdb-core        | asset_bmc_fields                      | -      | |
| 000036 | cmdb-core        | asset_warranty_lifecycle              | -      | |
| 000037 | cmdb-core        | rack_management_fixes                 | -      | |
| 000038 | cmdb-core        | encrypt_integration_secrets           | -      | |
| 000039 | cmdb-core        | adapter_failure_state                 | -      | |
| 000040 | —                | RESERVED gap (historical)             | -      | Roadmap § 1.2 preallocation, never used; **do not reclaim** |
| 000041 | —                | RESERVED gap (historical)             | -      | Roadmap § 1.3 preallocation, never used; **do not reclaim** |
| 000042 | —                | RESERVED gap (historical)             | -      | Roadmap § 2.5 preallocation, never used; **do not reclaim** |
| 000043 | —                | RESERVED gap (historical)             | -      | Roadmap § 2.15 preallocation, never used; **do not reclaim** |
| 000044 | cmdb-core        | password_changed_at                   | -      | |
| 000045 | cmdb-core        | user_roles_tenant_id                  | -      | |
| 000046 | cmdb-core        | discovered_assets_approved_asset_id   | -      | |
| 000047 | cmdb-core        | alert_events_dedup_key                | -      | |
| 000048 | cmdb-core        | webhook_circuit_breaker               | -      | |
| 000049 | cmdb-core        | work_order_dedup                      | -      | |
| 000050 | cmdb-core        | users_username_tenant_scoped          | -      | |
| 000051 | cmdb-core        | audit_partitioning _(pending)_        | -      | Reserved for Phase 4.2; not yet on disk |
| 000052 | ingestion-engine | bmc_field_authorities                 | -      | Renamed from 000011 to resolve cross-service collision |

## Next available

- Next available number: **000053**
- Update this file in the same PR that creates the `.up.sql` / `.down.sql`.

## Reserved / blocked numbers

- `000040`, `000041`, `000042`, `000043` — historical gaps, must not be
  reclaimed. If you need a new migration, go to the "Next available" value.
- `000051` — reserved for Phase 4.2 `audit_partitioning`. If you need a
  number before that lands, take 000053 or later, not 000051.

## Rename protocol

If two PRs race to claim the same number, the **later-merged** PR owner:

1. Picks the next available number from this file.
2. Renames the `.up.sql` / `.down.sql` pair in their service.
3. Updates this registry.
4. If the old number was already applied in any environment, contacts
   the DBA to run `UPDATE schema_migrations SET version = NEW WHERE
   version = OLD` on the affected database. Sample SQL patterned after
   `ops/cutover/2026-04-20-ingestion-migration-rename.sql`.

## Collision history

| Date       | Resolution                                                                   |
|------------|------------------------------------------------------------------------------|
| 2026-04-20 | ingestion-engine `000011_bmc_field_authorities` → `000052_bmc_field_authorities`. File bytes unchanged. Cutover SQL: `ops/cutover/2026-04-20-ingestion-migration-rename.sql`. |

## Related

- CI guard: `.github/workflows/migration-check.yml`
- Scanner: `scripts/check-migration-collision.sh`
- Design rationale: `docs/reports/phase4/4.5-migration-number-registry.md`
