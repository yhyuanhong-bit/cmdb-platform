package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/integration"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

func (w *WorkflowSubscriber) StartMetricsPuller(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.pullMetricsFromAdapters(ctx)
			}
		}
	}()
	zap.L().Info("Metrics puller started (5m interval)")
}

func (w *WorkflowSubscriber) pullMetricsFromAdapters(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT id, tenant_id, name, type, endpoint, config, config_encrypted FROM integration_adapters
		 WHERE direction = 'inbound' AND enabled = true`)
	if err != nil {
		zap.L().Warn("metrics puller: failed to query adapters", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, tenantID uuid.UUID
		var name, adapterType string
		var endpoint *string
		var configRaw, configEncrypted []byte
		if rows.Scan(&id, &tenantID, &name, &adapterType, &endpoint, &configRaw, &configEncrypted) != nil {
			continue
		}
		if endpoint == nil || *endpoint == "" {
			continue
		}

		plainConfig, err := integration.DecryptConfigWithFallback(w.cipher, configEncrypted, configRaw)
		if err != nil {
			zap.L().Warn("metrics puller: decrypt config failed",
				zap.String("adapter", name), zap.Error(err))
			continue
		}

		pullErr := w.pullFromAdapter(ctx, id, tenantID, name, adapterType, *endpoint, plainConfig)
		if pullErr != nil {
			w.adapterFailures[id]++
			zap.L().Warn("metrics puller: pull failed",
				zap.String("adapter", name),
				zap.String("type", adapterType),
				zap.Int("consecutive_failures", w.adapterFailures[id]),
				zap.Error(pullErr))
			if w.adapterFailures[id] >= 3 {
				w.disableAdapter(ctx, id, tenantID, name)
			}
		} else {
			w.adapterFailures[id] = 0
		}
	}
}

func (w *WorkflowSubscriber) pullFromAdapter(ctx context.Context, adapterID, tenantID uuid.UUID, name, adapterType, endpoint string, configRaw []byte) error {
	adapter := GetAdapter(adapterType)
	if adapter == nil {
		// Fallback: try as prometheus for backward compat with type="rest"
		adapter = GetAdapter("prometheus")
	}
	if adapter == nil {
		return fmt.Errorf("unsupported adapter type: %s", adapterType)
	}

	points, err := adapter.Fetch(ctx, endpoint, configRaw)
	if err != nil {
		return err
	}

	for _, pt := range points {
		var assetID pgtype.UUID
		if pt.IP != "" {
			asset, err := w.queries.FindAssetByIP(ctx, dbgen.FindAssetByIPParams{
				TenantID:  tenantID,
				IpAddress: pgtype.Text{String: pt.IP, Valid: true},
			})
			if err == nil {
				assetID = pgtype.UUID{Bytes: asset.ID, Valid: true}
			}
		}
		labelsJSON, _ := json.Marshal(pt.Labels)
		w.pool.Exec(ctx,
			"INSERT INTO metrics (time, asset_id, tenant_id, name, value, labels) VALUES ($1, $2, $3, $4, $5, $6)",
			pt.Timestamp, assetID, tenantID, pt.Name, pt.Value, labelsJSON)
	}

	zap.L().Debug("metrics puller: stored metrics",
		zap.String("adapter", name),
		zap.String("type", adapterType),
		zap.Int("count", len(points)))

	return nil
}

func (w *WorkflowSubscriber) disableAdapter(ctx context.Context, adapterID, tenantID uuid.UUID, name string) {
	_, err := w.pool.Exec(ctx,
		"UPDATE integration_adapters SET enabled = false WHERE id = $1", adapterID)
	if err != nil {
		zap.L().Error("metrics puller: failed to disable adapter", zap.String("adapter", name), zap.Error(err))
		return
	}
	zap.L().Warn("metrics puller: adapter disabled after 3 consecutive failures", zap.String("adapter", name))

	admins := w.opsAdminUserIDs(ctx, tenantID)
	for _, adminID := range admins {
		w.createNotification(ctx, tenantID, adminID,
			"adapter_error",
			fmt.Sprintf("Adapter '%s' disabled", name),
			fmt.Sprintf("The inbound adapter '%s' has been automatically disabled after 3 consecutive pull failures.", name),
			"integration_adapter", adapterID)
	}
	delete(w.adapterFailures, adapterID)
}
