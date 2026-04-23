# Database Seeds

Two seed files with distinct purposes:

## `seed.sql` — Production bootstrap + reference data (REQUIRED)

Minimum data a fresh install needs to function:

- 1 primary tenant
- 1 admin user (default password `admin123` — must be changed)
- System roles (`super-admin`, `ops-admin`, `viewer`)
- BIA scoring rules (4 tiers: critical / important / normal / minor)
- Default alert rules (5 standard infra thresholds)
- Field authority definitions (import reconciliation reference data)

**Loaded automatically** by `cmd/server/main.go` on first boot when the
`users` table is empty. Safe to re-run — every INSERT uses
`ON CONFLICT DO NOTHING`.

## `test-fixture.sql` — Development + integration test demo data

Rich realistic dataset on top of `seed.sql`:

- Location hierarchy (Taiwan / cities / campuses)
- Racks + asset placements with U-slot detail
- Sample assets, alerts, work orders, inventory tasks
- Audit events, incidents, predictions, RCA analyses
- Metrics time-series (24-hour hourly readings)
- BIA assessments + dependencies
- Discovered-asset staging examples
- Asset attribute enrichment (CPU / memory / etc. details)

**NEVER load in production.** Contains fake people names, example IP
addresses, and demo business systems that would pollute a real tenant.

## How each scenario loads seeds

| Scenario | seed.sql | test-fixture.sql |
|---|---|---|
| Production deploy (first boot) | ✅ auto | ❌ never |
| Local dev (`docker-compose up`) | ✅ auto | load manually with `make seed-dev` |
| Integration tests (CI) | ✅ loaded in `Apply migrations + seed primary tenant` step | ✅ loaded in `Load test fixture` step |
| Integration tests (local `go test -tags integration`) | prerequisite | prerequisite |

## Running the dev fixture manually

```bash
cd cmdb-core
export DATABASE_URL="postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
make seed-dev   # loads seed.sql + test-fixture.sql
```

## Contributing

When adding new demo data:
- Demo tenant stays: `a0000000-0000-0000-0000-000000000001`
- Stable UUIDs (hardcoded) so tests can reference them
- Use `ON CONFLICT DO NOTHING` so re-runs are safe
- Names / addresses / emails must be obviously fake (`@example.com`,
  RFC 5737 IP ranges, placeholder phone numbers)
