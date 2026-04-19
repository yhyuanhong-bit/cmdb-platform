# Sync Conflict Policy

> Scope: how the CMDB handles divergent writes across Central and Edge nodes
> during automatic sync, and what the `sync_conflicts` table actually exists
> for.

## TL;DR

- Automatic sync uses **last-write-wins (LWW)** gated on `sync_version`.
- The `sync_conflicts` table and the `POST /api/v1/sync/conflicts/:id/resolve`
  endpoint form a **manual-intervention channel only**. The sync agent does
  not insert into this table. Rows are filed by operators.
- If you are looking for automatic conflict detection — it does not exist
  today, on purpose. See [Why not auto-detect](#why-not-auto-detect).

## Policy

### Automatic sync: last-write-wins

`internal/domain/sync/agent.go` subscribes to `sync.>` envelopes from the
event bus and dispatches to per-entity apply functions (`applyWorkOrder`,
`applyAlertEvent`, `applyInventoryTask`, etc.). Every apply function does one
of two things:

1. **Version-gated overwrite.** `UPDATE ... WHERE sync_version < $N` —
   strictly newer envelopes replace the local row; equal or older envelopes
   are silently dropped.
2. **Append-only insert.** For `audit_events`: `INSERT ... ON CONFLICT DO
   NOTHING` — duplicates are dropped.

There is no branch that inspects the local row, detects a divergent concurrent
edit, and files a conflict. Downstream replicas will overwrite local state
unless a human operator explicitly intervenes first.

This is a deliberate tradeoff. It keeps apply fast, keeps the sync pipeline
stateless, and matches the product reality that the vast majority of CMDB
edits are not concurrent.

### Manual conflicts: `sync_conflicts` table

The `sync_conflicts` table exists for the small residual set of cases where
LWW is the wrong answer and a human needs to arbitrate. Schema (migration
`000027_sync_system.up.sql`):

| column           | purpose                                                   |
| ---------------- | --------------------------------------------------------- |
| `id`             | conflict PK                                               |
| `tenant_id`      | owning tenant (enforced on all reads/writes)              |
| `entity_type`    | target table (`assets`, `work_orders`, …)                 |
| `entity_id`      | target row id                                             |
| `local_version`  | local `sync_version` at time of dispute                   |
| `remote_version` | remote `sync_version` at time of dispute                  |
| `local_diff`     | JSONB: the local-wins candidate payload                   |
| `remote_diff`    | JSONB: the remote-wins candidate payload                  |
| `resolution`     | `pending` / `local_wins` / `remote_wins` / `auto_expired` |
| `resolved_by`    | user that resolved (nullable until resolved)              |
| `resolved_at`    | resolution timestamp (nullable until resolved)            |
| `created_at`     | filed-at                                                  |

## Lifecycle

```
         (filed manually by operator / support tool)
                        │
                        ▼
              sync_conflicts row
              resolution = 'pending'
                        │
        ┌───────────────┼───────────────┬──────────────────┐
        ▼               ▼               ▼                  ▼
   local_wins      remote_wins     auto_expired      (still pending)
   (operator       (operator       (>7d, cleanup    (admin ignores)
    confirms        confirms        worker in
    local is        remote is       workflows/
    correct)        correct)        cleanup.go)
```

- **INSERT** is not done by the sync agent. Operators file rows through
  admin tooling, support scripts, or ad-hoc SQL when a dispute is reported.
- **Resolution** happens via `POST /api/v1/sync/conflicts/:id/resolve` with
  body `{"resolution": "local_wins" | "remote_wins"}`. The handler:
  1. Validates the caller's tenant owns the conflict.
  2. If `remote_wins`, validates every key in `remote_diff` against the
     per-entity column whitelist (`allowedResolveColumns` in
     `internal/api/sync_endpoints.go`).
  3. Marks the row `resolution = <chosen>`, `resolved_by`, `resolved_at`.
  4. If `remote_wins`, applies `remote_diff` to the target entity with column
     identifiers sanitized via `pgx.Identifier{}.Sanitize()` and values bound
     through positional placeholders, then bumps `sync_version`.
  5. Emits a `sync.conflict_resolved` audit event.
  6. Returns `204 No Content`.
- **Auto-expiry** — rows left pending for more than 7 days are flipped to
  `auto_expired` by `WorkflowSubscriber.autoResolveStaleConflicts` in
  `internal/domain/workflows/cleanup.go`. At 3 days a notification is sent to
  ops admins warning about the approaching SLA. This does not change entity
  data — it only releases the conflict row so the UI stops showing it.

## Why not auto-detect

Automatic detection is technically reasonable but currently unfunded. A
correct implementation would need at least one of:

- **Row-version columns with optimistic concurrency.** Every write path — not
  just the sync agent, but every HTTP handler, every background worker, every
  cascade trigger — would have to stamp a monotonic `sync_version` AND pass
  the last-observed version on update, so the sync agent could compare
  `incoming_base_version vs current_version` and detect divergence. Retrofitting
  this across the existing handler surface is non-trivial and easy to get
  subtly wrong (miss one handler, get silent data loss).
- **Vector clocks or dotted-version vectors.** Correct but invasive:
  every replica tracks a per-node version vector and merges at apply time.
  Extra column, extra complexity in every apply path, and the merge logic
  still has to fall back to a human when the vectors are incomparable.
- **CRDTs.** Eliminate conflicts at the type level. Only works for data
  that has a natural lattice (sets, counters, last-writer-wins registers,
  sequence CRDTs). A CMDB's entity shapes (arbitrary JSONB attributes,
  foreign-key relationships, status state machines) are not CRDT-friendly
  without per-field design work.

Until one of these is designed, budgeted, and rolled out, the codebase uses
LWW for the common case and treats `sync_conflicts` as the operator escape
hatch for the uncommon one. Anyone considering auto-detection should start
from this doc, not re-invent it.

## Operator playbook

### Filing a conflict

There is currently no admin-UI button for "file a conflict." The expected
flow is:

1. Support receives a ticket: "the rack named `R-12` shows different
   `total_u` on Central vs Edge-BEIJING and nobody knows which is right."
2. Support pulls both rows (via the read APIs or directly from the DB) and
   assembles the `local_diff` / `remote_diff` JSON blobs with only the
   disputed columns.
3. Support inserts a row:
   ```sql
   INSERT INTO sync_conflicts
     (tenant_id, entity_type, entity_id,
      local_version, remote_version,
      local_diff, remote_diff)
   VALUES
     ($1, 'racks', $2,
      $3, $4,
      $5::jsonb, $6::jsonb);
   ```
4. The ops admin for that tenant sees the pending conflict in the admin UI
   (via `GET /api/v1/sync/conflicts`) and is notified at the 3-day mark if
   it's still open.

### Resolving

```
POST /api/v1/sync/conflicts/<conflict-id>/resolve
Authorization: Bearer <token>
Content-Type: application/json

{"resolution": "remote_wins"}
```

- `local_wins` — keep the current row on this node; the conflict row is
  closed. No entity data is touched. Downstream replicas will eventually
  re-converge via normal LWW if the local row later emits a newer
  `sync_version`.
- `remote_wins` — apply `remote_diff` to the entity on this node, bump
  `sync_version`, then close the conflict. The updated row will propagate
  outward through the normal sync pipeline.
- Only columns in the per-entity `allowedResolveColumns` whitelist can be
  written via `remote_diff`. System columns (`id`, `tenant_id`, `created_at`,
  `updated_at`, `sync_version`) are always rejected.

### Auditing

Every resolution emits a `sync.conflict_resolved` audit event with the
chosen resolution, entity type, and entity id. Use
`GET /api/v1/audit/events?action=sync.conflict_resolved` to review.

## Future-proofing

If and when automatic conflict detection is designed:

- Keep this table and endpoint — manual arbitration is still needed for
  non-mergeable disputes even with CRDTs or vector clocks.
- The current schema already carries `local_version` + `remote_version` +
  both diffs, which is enough to represent a detected conflict; automatic
  detection would just add an insertion path in the sync agent when
  `incoming_base_version != current_version`.
- The `local_wins` / `remote_wins` vocabulary is strategy-agnostic and can
  be extended with `merged` when mergeable semantics are introduced per
  entity type.

## Related code

- `internal/domain/sync/agent.go` — apply functions, LWW policy.
- `internal/api/sync_endpoints.go` — `SyncGetConflicts`, `SyncResolveConflict`,
  the `allowedResolveColumns` whitelist.
- `internal/domain/workflows/cleanup.go` — 7-day auto-expiry +
  3-day SLA warning notifications.
- `db/migrations/000027_sync_system.up.sql` — `sync_conflicts` schema.
