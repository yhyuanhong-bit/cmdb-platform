# Tenant Scope

## What it is

`database.TenantScoped` is a small wrapper around `*pgxpool.Pool` that fails
closed on any SQL statement that does not textually reference `tenant_id`.
The bound tenant UUID is automatically prepended as `$1`, so handlers write
queries as `WHERE tenant_id = $1 AND id = $2 ...` and pass only `id` at the
call site.

Goals:

- Make it impossible to forget the tenant predicate.
- Make cross-tenant queries deliberate and greppable.
- Keep the wrapper dependency-free (pure SQL-text check, no schema parser).

## When to use which

| Situation | Use |
|-----------|-----|
| Handler / domain service with a request-bound tenant | `database.Scope(pool, tenantID)` |
| Background scheduler iterating every tenant | raw `*pgxpool.Pool` + `//tenantlint:allow-direct-pool` |
| Migration / bootstrap code that runs before any tenant exists | raw `*pgxpool.Pool` + `//tenantlint:allow-direct-pool` |
| Anything in between | raw pool — but document why in the commit message |

`Scope` panics on `uuid.Nil` because silently running with a zero tenant has
historically been the root cause of IDOR-style leaks.

## Escape hatch

A file-level comment

```go
//tenantlint:allow-direct-pool
```

silences the `tenantlint` analyzer for an entire file. Use it for:

- cross-tenant reporting / scheduling
- infrastructure code (health checks, schema bootstrap)
- tests that deliberately exercise raw pool behaviour

Every use should be paired with a prose comment explaining why the file is
cross-tenant.

## Linter

`cmdb-core/tools/tenantlint` is a `go/analysis` Analyzer. It flags direct
calls to `*pgxpool.Pool.Exec`, `Query`, and `QueryRow` in handler/domain
packages. Run it as a vet tool:

```sh
go build -o bin/tenantlint ./tools/tenantlint/cmd/tenantlint
go vet -vettool=$(pwd)/bin/tenantlint ./internal/api/... ./internal/domain/...
```

The linter is advisory today — it is not yet wired into CI. A later phase
will migrate existing handlers to `TenantScoped` and then enable the
analyzer as a blocking check.

## Roadmap

- Phase 1.1 (this change): wrapper + analyzer land, no call-site migration.
- Phase 1.2: migrate the five highest-traffic handler packages.
- Phase 1.3: enable tenantlint in CI and tighten the analyzer to cover
  `pgx.Tx` as well as `*pgxpool.Pool`.
