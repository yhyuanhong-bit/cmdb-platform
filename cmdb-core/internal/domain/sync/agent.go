// agent.go
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Agent runs on Edge nodes to handle initial sync and incremental apply.
type Agent struct {
	pool   *pgxpool.Pool
	bus    eventbus.Bus
	cfg    *config.Config
	nodeID string
}

// NewAgent creates a SyncAgent for Edge nodes.
func NewAgent(pool *pgxpool.Pool, bus eventbus.Bus, cfg *config.Config) *Agent {
	return &Agent{pool: pool, bus: bus, cfg: cfg, nodeID: cfg.EdgeNodeID}
}

// Start runs the sync agent lifecycle.
func (a *Agent) Start(ctx context.Context) {
	// Check if initial sync is needed
	var count int
	err := a.pool.QueryRow(ctx, "SELECT count(*) FROM sync_state WHERE node_id = $1", a.nodeID).Scan(&count)
	if err != nil || count == 0 {
		zap.L().Info("sync agent: no sync state found, initial sync may be needed",
			zap.String("node_id", a.nodeID))
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
				a.updateSyncState(ctx)
			}
		}
	}()
}

// isFromCentral returns true if the envelope originated from the Central node.
func isFromCentral(env SyncEnvelope) bool {
	return env.Source == "central"
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

	if !env.VerifyChecksum() {
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
	default:
		err = a.applyGeneric(ctx, env)
	}

	if err != nil {
		zap.L().Error("sync agent: apply failed",
			zap.String("entity_type", env.EntityType),
			zap.String("entity_id", env.EntityID),
			zap.Error(err))
		return nil
	}

	// Update sync_state after successful apply
	_, _ = a.pool.Exec(ctx,
		`INSERT INTO sync_state (node_id, tenant_id, entity_type, last_sync_version, last_sync_at, status)
		 VALUES ($1, $2, $3, $4, now(), 'active')
		 ON CONFLICT (node_id, entity_type) DO UPDATE SET last_sync_version = GREATEST(sync_state.last_sync_version, $4), last_sync_at = now(), status = 'active'`,
		a.nodeID, env.TenantID, env.EntityType, env.Version)

	return nil
}

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
		   sync_version = EXCLUDED.sync_version`,
		payload["id"], payload["tenant_id"], payload["name"], payload["metric_name"],
		conditionJSON, payload["severity"], payload["enabled"], env.Version)
	return err
}

func (a *Agent) applyGeneric(ctx context.Context, env SyncEnvelope) error {
	_, err := a.pool.Exec(ctx,
		fmt.Sprintf("UPDATE %s SET sync_version = $1, updated_at = now() WHERE id = $2 AND sync_version < $1", env.EntityType),
		env.Version, env.EntityID)
	return err
}

func (a *Agent) updateSyncState(ctx context.Context) {
	// For each syncable table, record current max sync_version
	tables := []string{"assets", "locations", "racks", "work_orders", "alert_events", "inventory_tasks"}
	for _, table := range tables {
		var maxVersion int64
		err := a.pool.QueryRow(ctx,
			fmt.Sprintf("SELECT COALESCE(MAX(sync_version), 0) FROM %s", table)).Scan(&maxVersion)
		if err != nil {
			continue
		}
		_, _ = a.pool.Exec(ctx,
			`INSERT INTO sync_state (node_id, tenant_id, entity_type, last_sync_version, last_sync_at, status)
			 VALUES ($1, $2, $3, $4, now(), 'active')
			 ON CONFLICT (node_id, entity_type) DO UPDATE SET last_sync_version = $4, last_sync_at = now()`,
			a.nodeID, a.cfg.TenantID, table, maxVersion)
	}
}
