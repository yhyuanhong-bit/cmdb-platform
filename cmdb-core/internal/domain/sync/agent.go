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

func (a *Agent) handleIncomingEnvelope(ctx context.Context, event eventbus.Event) error {
	var env SyncEnvelope
	if err := json.Unmarshal(event.Payload, &env); err != nil {
		zap.L().Warn("sync agent: invalid envelope", zap.Error(err))
		return nil
	}

	// Skip our own envelopes
	if env.Source == a.nodeID {
		return nil
	}

	// Verify checksum
	if !env.VerifyChecksum() {
		zap.L().Warn("sync agent: checksum mismatch", zap.String("id", env.ID))
		return nil
	}

	// Check layer order
	layer := LayerOf(env.EntityType)
	if layer < 0 {
		zap.L().Warn("sync agent: unknown entity type", zap.String("type", env.EntityType))
		return nil
	}

	zap.L().Debug("sync agent: received envelope",
		zap.String("entity_type", env.EntityType),
		zap.String("entity_id", env.EntityID),
		zap.String("action", env.Action),
		zap.Int64("version", env.Version))

	// Update sync state
	_, _ = a.pool.Exec(ctx,
		`INSERT INTO sync_state (node_id, tenant_id, entity_type, last_sync_version, last_sync_at, status)
		 VALUES ($1, $2, $3, $4, now(), 'active')
		 ON CONFLICT (node_id, entity_type) DO UPDATE SET last_sync_version = GREATEST(sync_state.last_sync_version, $4), last_sync_at = now(), status = 'active'`,
		a.nodeID, env.TenantID, env.EntityType, env.Version)

	return nil
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
