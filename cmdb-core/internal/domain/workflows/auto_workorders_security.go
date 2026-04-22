package workflows

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

//tenantlint:allow-direct-pool — cross-tenant security audit scheduler

// --- Auto Work Order 9: Firmware Outdated → Firmware Upgrade ---

// firmwareAssetRow is the per-asset projection the firmware scan loads
// from the database so the "latest firmware per bmc_type" aggregation
// can be computed in Go using semver ordering instead of SQL MAX(),
// which is lexicographic and would mis-order e.g. "1.10.0" < "1.9.0".
type firmwareAssetRow struct {
	assetID   uuid.UUID
	tenantID  uuid.UUID
	assetTag  string
	name      string
	bmcType   string
	firmware  string
	hasOpenWO bool
}

func (w *WorkflowSubscriber) checkFirmwareOutdated(ctx context.Context) {
	// NOTE: we deliberately avoid the old `WITH latest AS (SELECT MAX(bmc_firmware) …)`
	// pattern because SQL MAX() on text is lexicographic: "1.9.0" beats
	// "1.10.0". Instead, we pull the raw rows and compute the max per
	// bmc_type in Go via maxFirmwareVersion (semver with lex fallback).
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.bmc_type, a.bmc_firmware,
		        EXISTS (
		          SELECT 1 FROM work_orders wo
		          WHERE wo.asset_id = a.id
		            AND wo.type = 'firmware_upgrade'
		            AND wo.status NOT IN ('completed','verified','rejected')
		            AND wo.deleted_at IS NULL
		        )
		 FROM assets a
		 WHERE a.bmc_type IS NOT NULL
		   AND a.bmc_firmware IS NOT NULL
		   AND a.deleted_at IS NULL
		   AND a.status NOT IN ('disposed', 'decommission')`)
	if err != nil {
		zap.L().Warn("firmware checker: query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceFirmwareCheck, telemetry.ReasonDBQueryFailed).Inc()
		return
	}
	defer rows.Close()

	var assets []firmwareAssetRow
	versionsByType := make(map[string][]string)
	for rows.Next() {
		var r firmwareAssetRow
		if scanErr := rows.Scan(&r.assetID, &r.tenantID, &r.assetTag, &r.name, &r.bmcType, &r.firmware, &r.hasOpenWO); scanErr != nil {
			zap.L().Warn("firmware checker: row scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceFirmwareCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}
		assets = append(assets, r)
		versionsByType[r.bmcType] = append(versionsByType[r.bmcType], r.firmware)
	}
	if err := rows.Err(); err != nil {
		zap.L().Warn("firmware checker: row iteration failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceFirmwareCheck, telemetry.ReasonRowsIterFailed).Inc()
		return
	}

	// Compute the "latest" firmware per bmc_type using semver ordering.
	latestByType := make(map[string]string, len(versionsByType))
	for t, vs := range versionsByType {
		latestByType[t] = maxFirmwareVersion(vs)
	}

	for _, a := range assets {
		if a.hasOpenWO {
			continue
		}
		latestFW := latestByType[a.bmcType]
		if latestFW == "" {
			continue
		}
		// Only create a WO when the asset's current firmware is STRICTLY
		// behind the latest known firmware for its BMC type under semver
		// comparison. Equal or ahead is a no-op.
		if compareFirmwareVersion(a.firmware, latestFW) >= 0 {
			continue
		}

		sysUID, ok := w.resolveSystemUser(ctx, a.tenantID, sourceFirmwareCheck)
		if !ok {
			continue
		}
		_, err := w.maintenanceSvc.Create(ctx, a.tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Firmware Upgrade: %s (%s)", a.name, a.assetTag),
			Type:        "firmware_upgrade",
			Priority:    "low",
			AssetID:     &a.assetID,
			Description: fmt.Sprintf("Asset '%s' BMC firmware (%s: %s) is behind the latest detected version (%s). Schedule firmware upgrade to maintain security compliance.", a.name, a.bmcType, a.firmware, latestFW),
		})
		if err != nil {
			zap.L().Warn("firmware checker: WO creation skipped", zap.String("asset", a.assetTag), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceFirmwareCheck, reasonWOCreateFailed).Inc()
			continue
		}
		zap.L().Info("firmware checker: created upgrade WO", zap.String("asset", a.assetTag))
	}
}

// --- Auto Work Order 10: BMC Default Password → Security Hardening (event-driven) ---

// createBMCSecurityWO creates a critical security hardening work order when a BMC default
// password is detected. Called from the onBMCDefaultPassword event handler.
func (w *WorkflowSubscriber) createBMCSecurityWO(ctx context.Context, tenantID, assetID uuid.UUID, assetTag, name, bmcType string) {
	var existingCount int
	if scanErr := w.pool.QueryRow(ctx,
		`SELECT count(*) FROM work_orders
		 WHERE asset_id = $1 AND type = 'security_hardening'
		 AND status NOT IN ('completed','verified','rejected')
		 AND deleted_at IS NULL`,
		assetID).Scan(&existingCount); scanErr != nil {
		// Prefer skipping over double-WOs — a broken dedup probe
		// must not flood the critical-security queue with replays.
		zap.L().Warn("security: BMC dedup probe failed", zap.String("asset", assetTag), zap.Error(scanErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceBMCSecurityCheck, telemetry.ReasonRowScanFailed).Inc()
		return
	}
	if existingCount > 0 {
		return
	}

	sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceBMCSecurityCheck)
	if !ok {
		return
	}
	order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
		Title:       fmt.Sprintf("Security: Default BMC Password — %s (%s)", name, assetTag),
		Type:        "security_hardening",
		Priority:    "critical",
		AssetID:     &assetID,
		Description: fmt.Sprintf("CRITICAL: Asset '%s' BMC (%s) is accessible with default credentials. This is a severe security risk. Immediately change the BMC password and verify access controls.", name, bmcType),
	})
	if err != nil {
		zap.L().Warn("security: BMC hardening WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceBMCSecurityCheck, reasonWOCreateFailed).Inc()
		return
	}

	admins := w.opsAdminUserIDs(ctx, tenantID)
	for _, adminID := range admins {
		w.warnNotify(ctx, sourceBMCSecurityCheck, tenantID, adminID, "security_hardening",
			fmt.Sprintf("CRITICAL: Default BMC password on %s", assetTag),
			fmt.Sprintf("Asset '%s' BMC uses default credentials. Immediate action required.", name),
			"work_order", order.ID)
	}
	zap.L().Warn("security: created BMC hardening WO", zap.String("asset", assetTag))
}

func (w *WorkflowSubscriber) onBMCDefaultPassword(ctx context.Context, event eventbus.Event) error {
	var payload struct {
		AssetID  string `json:"asset_id"`
		AssetTag string `json:"asset_tag"`
		Name     string `json:"name"`
		BMCType  string `json:"bmc_type"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		zap.L().Warn("workflow: failed to parse bmc_default_password payload", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceBMCDefaultEventParse, telemetry.ReasonJSONUnmarshal).Inc()
		return nil
	}

	tenantID, _ := uuid.Parse(event.TenantID)
	assetID, err := uuid.Parse(payload.AssetID)
	if err != nil || tenantID == uuid.Nil {
		return nil
	}

	w.createBMCSecurityWO(ctx, tenantID, assetID, payload.AssetTag, payload.Name, payload.BMCType)
	return nil
}
