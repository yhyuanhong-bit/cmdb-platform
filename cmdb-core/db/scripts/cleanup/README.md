# db/scripts/cleanup

One-shot maintenance scripts. **Nothing in this directory is auto-applied by
`main.go`** — the startup migrator only scans `db/migrations/`. Operators run
these manually when the prerequisites documented in each script are met.

## clear_integration_plaintext.sql

Clears the plaintext `integration_adapters.config` and
`webhook_subscriptions.secret` columns *after* dual-write has fully populated
their encrypted counterparts (`config_encrypted` / `secret_encrypted`).

**Prerequisites** (all must be true — the script also guards them in a
pre-flight block, but checking first avoids a noisy abort):

1. Migration `000038_encrypt_integration_secrets` is applied.
2. `CMDB_SECRET_KEY` has been stable long enough that every active adapter and
   webhook has written a ciphertext row (one full release cycle of the
   dual-write code is the conservative minimum).
3. Any historical rows that pre-date dual-write have been backfilled —
   see `docs/integration-encryption-deployment.md` §7.
4. You have taken a fresh `pg_dump` backup.

**Behavior.** Runs as a single transaction. If *any* non-empty plaintext row
lacks a ciphertext, the script `RAISE EXCEPTION` and rolls back untouched.
Otherwise it resets adapter `config` to `'{}'::jsonb` and webhook `secret` to
`NULL`, then writes one `integration_plaintext_cleared` audit event **per
affected tenant** (aggregated from the update `RETURNING` clauses).
Per-tenant rather than a single cross-tenant row because
`audit_events.tenant_id` is NOT NULL with an FK to `tenants(id)`.

Idempotent: a second run on already-cleared data is a no-op.

**Run it.**

```bash
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
  -f cmdb-core/db/scripts/cleanup/clear_integration_plaintext.sql
```

The `\echo` block at the end prints `adapters_cleared` and `webhooks_cleared`
counts to stdout.

---

If you're adding another cleanup script here, document it in the same shape:
purpose, prerequisites, behavior, invocation.
