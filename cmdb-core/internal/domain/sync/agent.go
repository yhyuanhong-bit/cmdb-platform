// agent.go
//
// Package sync implements the Edge-node sync agent and conflict-resolution
// surface for the CMDB. It consumes SyncEnvelope messages from the event bus
// and applies them to local tables.
//
// Conflict policy (IMPORTANT):
//
// The default conflict strategy for automatic sync is last-write-wins (LWW).
// Apply functions compare the envelope's version against the local row's
// sync_version and unconditionally overwrite when the envelope is newer —
// they do NOT detect divergent concurrent edits, and they do NOT automatically
// insert rows into the sync_conflicts table. Any row newer than what we have
// wins; older envelopes are silently skipped by the `sync_version < $N`
// guards in each UPDATE.
//
// The sync_conflicts table exists as a manual-intervention channel only:
// operators file rows into it via admin tooling / support workflows when a
// human dispute needs arbitration, and the SyncResolveConflict HTTP handler
// applies the chosen resolution. Nothing in this package inserts into
// sync_conflicts; see docs/SYNC_CONFLICT.md for the full policy and operator
// playbook.
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Source label for telemetry.ErrorsSuppressedTotal on sync_state
// writes. A broken sync_state table would have masked an edge node
// silently falling behind; route each UPSERT failure through the
// shared counter so the edge-health dashboard lights up.
const sourceSyncStateUpsert = "sync.agent.sync_state_upsert"

// Agent runs on Edge nodes to handle initial sync and incremental apply.
type Agent struct {
	pool            *pgxpool.Pool
	bus             eventbus.Bus
	cfg             *config.Config
	nodeID          string
	InitialSyncDone *atomic.Bool
}

// NewAgent creates a SyncAgent for Edge nodes.
func NewAgent(pool *pgxpool.Pool, bus eventbus.Bus, cfg *config.Config) *Agent {
	return &Agent{pool: pool, bus: bus, cfg: cfg, nodeID: cfg.EdgeNodeID}
}

// Start runs the sync agent lifecycle.
func (a *Agent) Start(ctx context.Context) {
	// Check if initial sync is needed. The node_id namespace is operator-
	// scoped rather than tenant-scoped — cross-tenant by design.
	count, err := dbgen.New(a.pool).CountSyncStateByNode(ctx, a.nodeID)
	if err != nil || count == 0 {
		zap.L().Info("sync agent: no sync state found, initial sync needed",
			zap.String("node_id", a.nodeID))
	}

	// Mark initial sync as done (sync_state exists = not first boot)
	if a.InitialSyncDone != nil {
		a.InitialSyncDone.Store(true)
		zap.L().Info("sync agent: initial sync complete, API unblocked")
	}

	// Subscribe to incoming sync envelopes from Central
	if a.bus != nil {
		a.bus.Subscribe("sync.>", func(ctx context.Context, event eventbus.Event) error {
			return a.handleIncomingEnvelope(ctx, event)
		})
		zap.L().Info("sync agent: listening for sync envelopes", zap.String("node_id", a.nodeID))
	}

	// Periodic state update
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				tickCtx, end := telemetry.StartTickSpan(ctx, "sync.tick.agent_state")
				a.updateSyncState(tickCtx)
				end()
			}
		}
	}()
}

// isFromCentral returns true if the envelope originated from the Central node.
func isFromCentral(env SyncEnvelope) bool {
	return env.Source == "central"
}

// subjectTenantSegment returns the tenant segment of a sync subject of the
// form `sync.<tenant>.<entity_type>.<action>`, or "" if the subject does
// not carry a tenant scope (e.g. `sync.resync_hint`, operational broadcasts).
//
// The NATS subject is the publisher-chosen routing key: if it asserts a
// tenant, that tenant is authoritative. An in-body env.TenantID that
// disagrees is either a cross-tenant replay attempt or a publisher bug —
// either way it must be dropped before any apply runs.
func subjectTenantSegment(subject string) string {
	const prefix = "sync."
	if !strings.HasPrefix(subject, prefix) {
		return ""
	}
	rest := subject[len(prefix):]
	parts := strings.Split(rest, ".")
	// A fully-qualified sync subject has at least three segments after
	// the `sync.` prefix: <tenant>.<entity_type>.<action>. Anything with
	// fewer (e.g. operational broadcasts like `sync.resync_hint`) is not
	// tenant-scoped and the guard below does not engage.
	if len(parts) < 3 {
		return ""
	}
	return parts[0]
}

// deriveStatusSQL mirrors the DeriveStatus Go function for use in SQL-context apply.
func deriveStatusSQL(exec, gov string) string {
	switch gov {
	case "in_progress", "completed":
		gov = "approved"
	}
	if gov == "verified" {
		return "verified"
	}
	if gov == "rejected" {
		return "rejected"
	}
	switch exec {
	case "done":
		return "completed"
	case "working":
		return "in_progress"
	default:
		if gov == "approved" {
			return "approved"
		}
		return "submitted"
	}
}

func (a *Agent) handleIncomingEnvelope(ctx context.Context, event eventbus.Event) error {
	var env SyncEnvelope
	if err := json.Unmarshal(event.Payload, &env); err != nil {
		zap.L().Warn("sync agent: invalid envelope", zap.Error(err))
		return nil
	}

	if env.Source == a.nodeID {
		return nil
	}

	// Bug #2 guard: cross-check env.TenantID against the NATS subject.
	// Publishers set the subject as `sync.<tenant>.<entity>.<action>`; a
	// mismatch between that tenant segment and the in-body env.TenantID
	// is either a cross-tenant replay or a publisher bug. Drop + metric
	// + log, never dispatch. Subjects without a tenant segment (e.g.
	// `sync.resync_hint`) bypass the guard — there is no tenant to check.
	if routedTenant := subjectTenantSegment(event.Subject); routedTenant != "" && routedTenant != env.TenantID {
		telemetry.SyncEnvelopeRejected.WithLabelValues(env.EntityType, "tenant_mismatch").Inc()
		zap.L().Warn("sync agent: tenant mismatch between subject and envelope, dropping",
			zap.String("id", env.ID),
			zap.String("subject", event.Subject),
			zap.String("subject_tenant", routedTenant),
			zap.String("envelope_tenant", env.TenantID),
			zap.String("source", env.Source),
			zap.String("entity_type", env.EntityType))
		return nil
	}

	if !env.VerifyChecksum() {
		telemetry.SyncEnvelopeRejected.WithLabelValues(env.EntityType, "bad_checksum").Inc()
		zap.L().Warn("sync agent: checksum mismatch", zap.String("id", env.ID))
		return nil
	}

	layer := LayerOf(env.EntityType)
	if layer < 0 {
		zap.L().Warn("sync agent: unknown entity type", zap.String("type", env.EntityType))
		return nil
	}

	zap.L().Debug("sync agent: applying envelope",
		zap.String("entity_type", env.EntityType),
		zap.String("entity_id", env.EntityID),
		zap.String("action", env.Action),
		zap.Int64("version", env.Version))

	var err error
	switch env.EntityType {
	case "work_orders":
		err = a.applyWorkOrder(ctx, env)
	case "alert_events":
		err = a.applyAlertEvent(ctx, env)
	case "alert_rules":
		err = a.applyAlertRule(ctx, env)
	case "inventory_tasks":
		err = a.applyInventoryTask(ctx, env)
	case "inventory_items":
		err = a.applyInventoryItem(ctx, env)
	case "audit_events":
		err = a.applyAuditEvent(ctx, env)
	default:
		err = a.applyGeneric(ctx, env)
	}

	if err != nil {
		telemetry.SyncEnvelopeFailed.WithLabelValues(env.EntityType).Inc()
		zap.L().Error("sync agent: apply failed",
			zap.String("entity_type", env.EntityType),
			zap.String("entity_id", env.EntityID),
			zap.Error(err))
		return nil
	}

	telemetry.SyncEnvelopeApplied.WithLabelValues(env.EntityType).Inc()

	// Update sync_state after successful apply. A failed UPSERT here
	// means the watermark didn't advance and the next pull will
	// re-fetch envelopes we already applied — idempotent but wasteful,
	// and a persistent pattern means sync is stuck.
	tenantUUID, tenantParseErr := uuid.Parse(env.TenantID)
	if tenantParseErr != nil {
		zap.L().Warn("sync agent: invalid tenant_id in envelope",
			zap.String("tenant_id", env.TenantID),
			zap.Error(tenantParseErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSyncStateUpsert, telemetry.ReasonDBExecFailed).Inc()
		return nil
	}
	if err := dbgen.New(a.pool).UpsertSyncState(ctx, dbgen.UpsertSyncStateParams{
		NodeID:          a.nodeID,
		TenantID:        tenantUUID,
		EntityType:      env.EntityType,
		LastSyncVersion: pgtype.Int8{Int64: env.Version, Valid: true},
	}); err != nil {
		zap.L().Warn("sync agent: sync_state upsert failed",
			zap.String("entity_type", env.EntityType),
			zap.Int64("version", env.Version),
			zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSyncStateUpsert, telemetry.ReasonDBExecFailed).Inc()
	}

	return nil
}

// applyWorkOrder applies a work-order envelope using last-write-wins.
// The `sync_version < $N` guard ensures older envelopes are dropped, but
// concurrent divergent edits are NOT detected — rows are NOT inserted into
// sync_conflicts. See the package doc comment and docs/SYNC_CONFLICT.md.
func (a *Agent) applyWorkOrder(ctx context.Context, env SyncEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Diff, &payload); err != nil {
		return fmt.Errorf("unmarshal work order payload: %w", err)
	}

	if isFromCentral(env) {
		gov, _ := payload["governance_status"].(string)
		if gov == "" {
			return nil
		}
		var currentExec string
		err := a.pool.QueryRow(ctx,
			"SELECT execution_status FROM work_orders WHERE id = $1", env.EntityID).Scan(&currentExec)
		if err != nil {
			return fmt.Errorf("read current execution_status: %w", err)
		}
		derived := deriveStatusSQL(currentExec, gov)
		_, err = a.pool.Exec(ctx,
			`UPDATE work_orders SET governance_status = $1, status = $2, sync_version = $3, updated_at = now()
			 WHERE id = $4 AND sync_version < $3`,
			gov, derived, env.Version, env.EntityID)
		return err
	}

	exec, _ := payload["execution_status"].(string)
	if exec == "" {
		return nil
	}
	var currentGov string
	err := a.pool.QueryRow(ctx,
		"SELECT governance_status FROM work_orders WHERE id = $1", env.EntityID).Scan(&currentGov)
	if err != nil {
		return fmt.Errorf("read current governance_status: %w", err)
	}
	derived := deriveStatusSQL(exec, currentGov)
	_, err = a.pool.Exec(ctx,
		`UPDATE work_orders SET execution_status = $1, status = $2, sync_version = $3, updated_at = now()
		 WHERE id = $4 AND sync_version < $3`,
		exec, derived, env.Version, env.EntityID)
	return err
}

// applyAlertEvent applies an alert-event envelope using last-write-wins
// (ordered by fired_at, then sync_version). No automatic conflict detection;
// see package doc and docs/SYNC_CONFLICT.md.
func (a *Agent) applyAlertEvent(ctx context.Context, env SyncEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Diff, &payload); err != nil {
		return fmt.Errorf("unmarshal alert event payload: %w", err)
	}

	_, err := a.pool.Exec(ctx,
		`INSERT INTO alert_events (id, tenant_id, rule_id, asset_id, status, severity, message, trigger_value, fired_at, acked_at, resolved_at, sync_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT (id) DO UPDATE SET
		   status = EXCLUDED.status,
		   severity = EXCLUDED.severity,
		   message = EXCLUDED.message,
		   trigger_value = EXCLUDED.trigger_value,
		   acked_at = EXCLUDED.acked_at,
		   resolved_at = EXCLUDED.resolved_at,
		   sync_version = EXCLUDED.sync_version
		 WHERE alert_events.fired_at < EXCLUDED.fired_at
		    OR (alert_events.fired_at = EXCLUDED.fired_at AND alert_events.sync_version < EXCLUDED.sync_version)`,
		payload["id"], payload["tenant_id"], payload["rule_id"], payload["asset_id"],
		payload["status"], payload["severity"], payload["message"], payload["trigger_value"],
		payload["fired_at"], payload["acked_at"], payload["resolved_at"], env.Version)
	return err
}

// applyAlertRule applies an alert-rule envelope using last-write-wins,
// gated on sync_version. The ON CONFLICT clause includes a strict
// `alert_rules.sync_version < EXCLUDED.sync_version` guard so a stale
// envelope (e.g. redelivered from JetStream after a newer one has already
// been applied) becomes a no-op UPDATE rather than silently resurrecting
// an old rule definition — which previously allowed an attacker or a
// replayed message to downgrade/disable a live alert rule.
//
// No automatic conflict detection; see package doc and docs/SYNC_CONFLICT.md.
func (a *Agent) applyAlertRule(ctx context.Context, env SyncEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Diff, &payload); err != nil {
		return fmt.Errorf("unmarshal alert rule payload: %w", err)
	}

	conditionJSON, _ := json.Marshal(payload["condition"])

	_, err := a.pool.Exec(ctx,
		`INSERT INTO alert_rules (id, tenant_id, name, metric_name, condition, severity, enabled, sync_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id) DO UPDATE SET
		   name = EXCLUDED.name,
		   metric_name = EXCLUDED.metric_name,
		   condition = EXCLUDED.condition,
		   severity = EXCLUDED.severity,
		   enabled = EXCLUDED.enabled,
		   sync_version = EXCLUDED.sync_version
		 WHERE alert_rules.sync_version < EXCLUDED.sync_version`,
		payload["id"], payload["tenant_id"], payload["name"], payload["metric_name"],
		conditionJSON, payload["severity"], payload["enabled"], env.Version)
	return err
}

// applyInventoryTask applies an inventory-task envelope using last-write-wins
// gated on sync_version. No automatic conflict detection; see package doc
// and docs/SYNC_CONFLICT.md.
func (a *Agent) applyInventoryTask(ctx context.Context, env SyncEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Diff, &payload); err != nil {
		return fmt.Errorf("unmarshal inventory task payload: %w", err)
	}

	_, err := a.pool.Exec(ctx,
		`INSERT INTO inventory_tasks (id, tenant_id, code, name, scope_location_id, status, method, planned_date, completed_date, assigned_to, created_at, sync_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		 ON CONFLICT (id) DO UPDATE SET
		   name = EXCLUDED.name,
		   status = EXCLUDED.status,
		   method = EXCLUDED.method,
		   planned_date = EXCLUDED.planned_date,
		   completed_date = EXCLUDED.completed_date,
		   assigned_to = EXCLUDED.assigned_to,
		   sync_version = EXCLUDED.sync_version
		 WHERE inventory_tasks.sync_version < EXCLUDED.sync_version`,
		payload["id"], payload["tenant_id"], payload["code"], payload["name"],
		payload["scope_location_id"], payload["status"], payload["method"],
		payload["planned_date"], payload["completed_date"], payload["assigned_to"],
		payload["created_at"], env.Version)
	return err
}

// applyInventoryItem applies an inventory-item envelope using last-write-wins
// gated on sync_version. No automatic conflict detection; see package doc
// and docs/SYNC_CONFLICT.md.
func (a *Agent) applyInventoryItem(ctx context.Context, env SyncEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Diff, &payload); err != nil {
		return fmt.Errorf("unmarshal inventory item payload: %w", err)
	}

	expectedJSON, _ := json.Marshal(payload["expected"])
	actualJSON, _ := json.Marshal(payload["actual"])

	_, err := a.pool.Exec(ctx,
		`INSERT INTO inventory_items (id, task_id, asset_id, rack_id, expected, actual, status, scanned_at, scanned_by, sync_version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (id) DO UPDATE SET
		   actual = EXCLUDED.actual,
		   status = EXCLUDED.status,
		   scanned_at = EXCLUDED.scanned_at,
		   scanned_by = EXCLUDED.scanned_by,
		   sync_version = EXCLUDED.sync_version
		 WHERE inventory_items.sync_version < EXCLUDED.sync_version`,
		payload["id"], payload["task_id"], payload["asset_id"], payload["rack_id"],
		expectedJSON, actualJSON, payload["status"],
		payload["scanned_at"], payload["scanned_by"], env.Version)
	return err
}

// applyAuditEvent applies an audit-event envelope. Audit events are
// append-only, keyed by id with ON CONFLICT DO NOTHING — so duplicates are
// dropped rather than overwritten. There is no conflict surface here: audit
// events cannot diverge. See package doc and docs/SYNC_CONFLICT.md.
func (a *Agent) applyAuditEvent(ctx context.Context, env SyncEnvelope) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(env.Diff, &payload); err != nil {
		return fmt.Errorf("unmarshal audit event payload: %w", err)
	}

	diffJSON, _ := json.Marshal(payload["diff"])

	_, err := a.pool.Exec(ctx,
		`INSERT INTO audit_events (id, tenant_id, action, module, target_type, target_id, operator_id, diff, source, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (id) DO NOTHING`,
		payload["id"], payload["tenant_id"], payload["action"], payload["module"],
		payload["target_type"], payload["target_id"], payload["operator_id"],
		diffJSON, payload["source"], payload["created_at"])
	return err
}

// applyGeneric is the fallback apply path for entity types without a
// dedicated handler. It bumps sync_version + updated_at only, using
// last-write-wins on sync_version. No conflict detection, no sync_conflicts
// insertion. See package doc and docs/SYNC_CONFLICT.md.
func (a *Agent) applyGeneric(ctx context.Context, env SyncEnvelope) error {
	_, err := a.pool.Exec(ctx,
		fmt.Sprintf("UPDATE %s SET sync_version = $1, updated_at = now() WHERE id = $2 AND sync_version < $1", env.EntityType),
		env.Version, env.EntityID)
	return err
}

func (a *Agent) updateSyncState(ctx context.Context) {
	q := dbgen.New(a.pool)
	tenantUUID, err := uuid.Parse(a.cfg.TenantID)
	if err != nil {
		zap.L().Warn("sync agent: invalid configured tenant_id",
			zap.String("tenant_id", a.cfg.TenantID),
			zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSyncStateUpsert, telemetry.ReasonDBExecFailed).Inc()
		return
	}
	// For each syncable table, record current max sync_version.
	// The table name is compile-time controlled (see allowlist above).
	tables := []string{"assets", "locations", "racks", "work_orders", "alert_events", "inventory_tasks", "inventory_items"}
	for _, table := range tables {
		var maxVersion int64
		probeErr := a.pool.QueryRow(ctx,
			fmt.Sprintf("SELECT COALESCE(MAX(sync_version), 0) FROM %s", table)).Scan(&maxVersion)
		if probeErr != nil {
			zap.L().Warn("sync agent: max version probe failed",
				zap.String("table", table), zap.Error(probeErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSyncStateUpsert, telemetry.ReasonRowScanFailed).Inc()
			continue
		}
		if execErr := q.UpsertSyncStateAbsolute(ctx, dbgen.UpsertSyncStateAbsoluteParams{
			NodeID:          a.nodeID,
			TenantID:        tenantUUID,
			EntityType:      table,
			LastSyncVersion: pgtype.Int8{Int64: maxVersion, Valid: true},
		}); execErr != nil {
			zap.L().Warn("sync agent: sync_state upsert failed",
				zap.String("table", table), zap.Error(execErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSyncStateUpsert, telemetry.ReasonDBExecFailed).Inc()
		}
	}

	// audit_events: no sync_version, use created_at epoch
	var auditMax int64
	probeErr := a.pool.QueryRow(ctx,
		"SELECT COALESCE(MAX(EXTRACT(EPOCH FROM created_at))::bigint, 0) FROM audit_events WHERE tenant_id = $1",
		a.cfg.TenantID).Scan(&auditMax)
	if probeErr != nil {
		zap.L().Warn("sync agent: audit max probe failed", zap.Error(probeErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSyncStateUpsert, telemetry.ReasonRowScanFailed).Inc()
		return
	}
	if execErr := q.UpsertSyncStateAbsolute(ctx, dbgen.UpsertSyncStateAbsoluteParams{
		NodeID:          a.nodeID,
		TenantID:        tenantUUID,
		EntityType:      "audit_events",
		LastSyncVersion: pgtype.Int8{Int64: auditMax, Valid: true},
	}); execErr != nil {
		zap.L().Warn("sync agent: audit sync_state upsert failed", zap.Error(execErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSyncStateUpsert, telemetry.ReasonDBExecFailed).Inc()
	}
}
