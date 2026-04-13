// service.go
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
	err := s.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT sync_version FROM %s WHERE id = $1", entityType),
		entityID).Scan(&version)
	if err != nil {
		version = 0
	}

	env := NewEnvelope(s.nodeID, event.TenantID, entityType, entityID, action, version, event.Payload)

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
				s.reconcile(ctx)
			}
		}
	}()
	zap.L().Info("sync reconciliation started (5m interval)")
}

func (s *Service) reconcile(ctx context.Context) {
	// Find stale sync entries (>1 hour behind)
	rows, err := s.pool.Query(ctx,
		`SELECT node_id, entity_type, last_sync_version, last_sync_at
		 FROM sync_state
		 WHERE status = 'active' AND last_sync_at < now() - interval '1 hour'`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var nodeID, entityType string
		var version int64
		var lastSync time.Time
		if rows.Scan(&nodeID, &entityType, &version, &lastSync) != nil {
			continue
		}

		// Calculate how far behind this node is
		var currentMaxVersion int64
		verr := s.pool.QueryRow(ctx,
			fmt.Sprintf("SELECT COALESCE(MAX(sync_version), 0) FROM %s", entityType),
		).Scan(&currentMaxVersion)
		if verr != nil {
			continue
		}

		gap := currentMaxVersion - version
		if gap <= 0 {
			// Node is up to date, reset last_sync_at
			s.pool.Exec(ctx,
				"UPDATE sync_state SET last_sync_at = now() WHERE node_id = $1 AND entity_type = $2",
				nodeID, entityType)
			continue
		}

		lag := time.Since(lastSync)

		if lag > 24*time.Hour {
			// Critical: mark node as error after 24h
			s.pool.Exec(ctx,
				"UPDATE sync_state SET status = 'error', error_message = $3 WHERE node_id = $1 AND entity_type = $2",
				nodeID, entityType, fmt.Sprintf("sync lag %s, %d versions behind", lag.Round(time.Minute), gap))
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
