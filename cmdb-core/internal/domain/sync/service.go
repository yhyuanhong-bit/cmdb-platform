// service.go
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Source label for telemetry.ErrorsSuppressedTotal on sync
// reconciliation watermark updates. A broken UPDATE on sync_state
// here means the health view ("last_sync_at" freshness, node
// status transitions) goes stale — route it through the shared
// counter so the edge fleet health dashboard lights up.
const sourceSyncReconcile = "sync.service.reconcile"

// Service handles sync envelope creation and distribution.
type Service struct {
	pool   *pgxpool.Pool
	bus    eventbus.Bus
	cfg    *config.Config
	nodeID string
}

// NewService creates a SyncService.
func NewService(pool *pgxpool.Pool, bus eventbus.Bus, cfg *config.Config) *Service {
	nodeID := cfg.EdgeNodeID
	if nodeID == "" {
		nodeID = "central"
	}
	return &Service{pool: pool, bus: bus, cfg: cfg, nodeID: nodeID}
}

// RegisterSubscribers subscribes to all domain events and wraps them as SyncEnvelopes.
func (s *Service) RegisterSubscribers() {
	if s.bus == nil {
		return
	}

	subjects := []struct {
		subject    string
		entityType string
		action     string
	}{
		{eventbus.SubjectAssetCreated, "assets", "create"},
		{eventbus.SubjectAssetUpdated, "assets", "update"},
		{eventbus.SubjectAssetDeleted, "assets", "delete"},
		{eventbus.SubjectLocationCreated, "locations", "create"},
		{eventbus.SubjectLocationUpdated, "locations", "update"},
		{eventbus.SubjectLocationDeleted, "locations", "delete"},
		{eventbus.SubjectRackCreated, "racks", "create"},
		{eventbus.SubjectRackUpdated, "racks", "update"},
		{eventbus.SubjectRackDeleted, "racks", "delete"},
		{eventbus.SubjectOrderCreated, "work_orders", "create"},
		{eventbus.SubjectOrderTransitioned, "work_orders", "update"},
		{eventbus.SubjectAlertFired, "alert_events", "create"},
		{eventbus.SubjectAlertResolved, "alert_events", "update"},
		{eventbus.SubjectInventoryTaskCreated, "inventory_tasks", "create"},
		{eventbus.SubjectInventoryTaskCompleted, "inventory_tasks", "update"},
		{eventbus.SubjectAlertRuleCreated, "alert_rules", "create"},
		{eventbus.SubjectAlertRuleUpdated, "alert_rules", "update"},
		{eventbus.SubjectAlertRuleDeleted, "alert_rules", "delete"},
		{eventbus.SubjectInventoryItemCreated, "inventory_items", "create"},
		{eventbus.SubjectInventoryItemUpdated, "inventory_items", "update"},
		{eventbus.SubjectAuditRecorded, "audit_events", "create"},
	}

	for _, sub := range subjects {
		sub := sub
		s.bus.Subscribe(sub.subject, func(ctx context.Context, event eventbus.Event) error {
			return s.onDomainEvent(ctx, event, sub.entityType, sub.action)
		})
	}

	zap.L().Info("sync subscribers registered", zap.Int("count", len(subjects)))
}

func (s *Service) onDomainEvent(ctx context.Context, event eventbus.Event, entityType, action string) error {
	var payload map[string]interface{}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil
	}

	// Extract entity ID from payload (convention: first field ending in _id matching entity type)
	entityID := extractEntityID(payload, entityType)
	if entityID == "" {
		return nil
	}

	// Query current sync_version
	var version int64
	tenantUUID, parseErr := uuid.Parse(event.TenantID)
	if parseErr != nil {
		return nil
	}
	sc := database.Scope(s.pool, tenantUUID)
	tableIdent := pgx.Identifier{entityType}.Sanitize()
	err := sc.QueryRow(ctx,
		fmt.Sprintf("SELECT sync_version FROM %s WHERE id = $2 AND tenant_id = $1", tableIdent),
		entityID).Scan(&version)
	if err != nil {
		version = 0
	}

	env := NewEnvelope(s.nodeID, event.TenantID, entityType, entityID, action, version, event.Payload)

	// HMAC sign every outbound envelope when a keyring is configured.
	// No-op when unset (rollout grace window — see signing.go policy).
	ActiveKeyRing().Sign(&env)

	// Publish to sync stream
	syncSubject := fmt.Sprintf("sync.%s.%s.%s", event.TenantID, entityType, action)
	data, _ := json.Marshal(env)
	return s.bus.Publish(ctx, eventbus.Event{
		Subject:  syncSubject,
		TenantID: event.TenantID,
		Payload:  data,
	})
}

// StartReconciliation runs a background task to check sync state every 5 minutes.
func (s *Service) StartReconciliation(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				tickCtx, end := telemetry.StartTickSpan(ctx, "sync.tick.reconcile")
				s.reconcile(tickCtx)
				end()
			}
		}
	}()
	zap.L().Info("sync reconciliation started (5m interval)")
}

func (s *Service) reconcile(ctx context.Context) {
	telemetry.SyncReconciliationRuns.Inc()
	q := dbgen.New(s.pool)
	// Find stale sync entries (>1 hour behind). Cross-tenant on purpose —
	// the reconciler operates across every tenant's edge fleet.
	staleRows, err := q.ListStaleSyncStates(ctx)
	if err != nil {
		return
	}

	for _, row := range staleRows {
		nodeID := row.NodeID
		entityType := row.EntityType
		var version int64
		if row.LastSyncVersion.Valid {
			version = row.LastSyncVersion.Int64
		}
		var lastSync time.Time
		if row.LastSyncAt.Valid {
			lastSync = row.LastSyncAt.Time
		}

		// Calculate how far behind this node is. The entity_type name is
		// not user-controlled — it comes from sync_state rows that were
		// inserted by the agent's table allowlist. Using Sprintf here is
		// consistent with the pre-sqlc implementation.
		var currentMaxVersion int64
		sc := database.Scope(s.pool, row.TenantID)
		tableIdent := pgx.Identifier{entityType}.Sanitize()
		verr := sc.QueryRow(ctx,
			fmt.Sprintf("SELECT COALESCE(MAX(sync_version), 0) FROM %s WHERE tenant_id = $1", tableIdent),
		).Scan(&currentMaxVersion)
		if verr != nil {
			continue
		}

		gap := currentMaxVersion - version
		if gap <= 0 {
			// Node is up to date, reset last_sync_at. A failed
			// UPDATE here means the freshness heartbeat doesn't
			// advance — the edge still looks stale on the dashboard
			// even though it's caught up.
			if err := q.UpdateSyncStateHeartbeat(ctx, dbgen.UpdateSyncStateHeartbeatParams{
				NodeID:     nodeID,
				EntityType: entityType,
			}); err != nil {
				zap.L().Warn("sync reconciliation: heartbeat update failed",
					zap.String("node_id", nodeID),
					zap.String("entity_type", entityType),
					zap.Error(err))
				telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSyncReconcile, telemetry.ReasonDBExecFailed).Inc()
			}
			continue
		}

		lag := time.Since(lastSync)

		if lag > 24*time.Hour {
			// Critical: mark node as error after 24h
			if err := q.MarkSyncStateError(ctx, dbgen.MarkSyncStateErrorParams{
				NodeID:       nodeID,
				EntityType:   entityType,
				ErrorMessage: pgtype.Text{String: fmt.Sprintf("sync lag %s, %d versions behind", lag.Round(time.Minute), gap), Valid: true},
			}); err != nil {
				zap.L().Warn("sync reconciliation: error-status update failed",
					zap.String("node_id", nodeID),
					zap.String("entity_type", entityType),
					zap.Error(err))
				telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceSyncReconcile, telemetry.ReasonDBExecFailed).Inc()
			}
			zap.L().Error("sync reconciliation: node marked as error",
				zap.String("node_id", nodeID),
				zap.String("entity_type", entityType),
				zap.Int64("gap", gap),
				zap.Duration("lag", lag))
		} else {
			// Warning: publish re-sync hint via NATS
			if s.bus != nil {
				payload, _ := json.Marshal(map[string]interface{}{
					"node_id":       nodeID,
					"entity_type":   entityType,
					"since_version": version,
					"current_max":   currentMaxVersion,
				})
				s.bus.Publish(ctx, eventbus.Event{
					Subject: "sync.resync_hint",
					Payload: payload,
				})
			}
			zap.L().Warn("sync reconciliation: resync hint published",
				zap.String("node_id", nodeID),
				zap.String("entity_type", entityType),
				zap.Int64("gap", gap))
		}
	}
}

func extractEntityID(payload map[string]interface{}, entityType string) string {
	// Try common ID field names
	keys := []string{"asset_id", "location_id", "rack_id", "order_id", "alert_id", "task_id", "id"}
	for _, k := range keys {
		if v, ok := payload[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
		}
	}
	return ""
}
