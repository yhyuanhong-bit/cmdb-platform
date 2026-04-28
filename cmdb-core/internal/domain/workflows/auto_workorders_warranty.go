package workflows

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

//tenantlint:allow-direct-pool — cross-tenant warranty expiry scheduler

// --- Auto Work Order 1: Warranty Expiry → Renewal Evaluation ---

func (w *WorkflowSubscriber) checkWarrantyExpiry(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.warranty_end, a.warranty_vendor
		 FROM assets a
		 WHERE a.warranty_end IS NOT NULL
		   AND a.warranty_end > now()
		   AND a.warranty_end <= now() + interval '30 days'
		   AND a.deleted_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.tenant_id = a.tenant_id
		     AND wo.asset_id = a.id
		     AND wo.type = 'warranty_renewal'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("warranty checker: query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceWarrantyCheck, telemetry.ReasonDBQueryFailed).Inc()
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var warrantyEnd time.Time
		var warrantyVendor *string
		if scanErr := rows.Scan(&assetID, &tenantID, &assetTag, &name, &warrantyEnd, &warrantyVendor); scanErr != nil {
			zap.L().Warn("warranty checker: row scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceWarrantyCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}

		daysLeft := int(time.Until(warrantyEnd).Hours() / 24)
		vendor := "N/A"
		if warrantyVendor != nil {
			vendor = *warrantyVendor
		}

		sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceWarrantyCheck)
		if !ok {
			continue
		}
		order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Warranty Renewal: %s (%s)", name, assetTag),
			Type:        "warranty_renewal",
			Priority:    "medium",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' warranty expires in %d days (vendor: %s, expiry: %s). Evaluate: renew warranty, plan replacement, or accept risk.", name, daysLeft, vendor, warrantyEnd.Format("2006-01-02")),
		})
		if err != nil {
			zap.L().Warn("warranty checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceWarrantyCheck, reasonWOCreateFailed).Inc()
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.warnNotify(ctx, sourceWarrantyCheck, tenantID, adminID,
				"warranty_expiry",
				fmt.Sprintf("Warranty expiring: %s", assetTag),
				fmt.Sprintf("Asset '%s' warranty expires in %d days. Work order created.", name, daysLeft),
				"work_order", order.ID)
		}

		zap.L().Info("warranty checker: created renewal WO",
			zap.String("asset", assetTag),
			zap.Int("days_left", daysLeft))
	}
	if iterErr := rows.Err(); iterErr != nil {
		zap.L().Warn("warranty checker: rows iter failed", zap.Error(iterErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceWarrantyCheck, telemetry.ReasonRowsIterFailed).Inc()
	}
}

// --- Auto Work Order 4: EOL Reached → Decommission ---

func (w *WorkflowSubscriber) checkEOLReached(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.eol_date
		 FROM assets a
		 WHERE a.eol_date IS NOT NULL
		   AND a.eol_date <= now()
		   AND a.status NOT IN ('disposed', 'decommission')
		   AND a.deleted_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.tenant_id = a.tenant_id
		     AND wo.asset_id = a.id
		     AND wo.type = 'decommission'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("eol checker: query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceEOLCheck, telemetry.ReasonDBQueryFailed).Inc()
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var eolDate time.Time
		if scanErr := rows.Scan(&assetID, &tenantID, &assetTag, &name, &eolDate); scanErr != nil {
			zap.L().Warn("eol checker: row scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceEOLCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}

		daysPast := int(time.Since(eolDate).Hours() / 24)

		sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceEOLCheck)
		if !ok {
			continue
		}
		order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Decommission: %s (%s)", name, assetTag),
			Type:        "decommission",
			Priority:    "high",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' reached end-of-life %d days ago (EOL: %s). Action required: data migration, service failover, physical removal, and CMDB status update to 'disposed'.", name, daysPast, eolDate.Format("2006-01-02")),
		})
		if err != nil {
			zap.L().Warn("eol checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceEOLCheck, reasonWOCreateFailed).Inc()
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.warnNotify(ctx, sourceEOLCheck, tenantID, adminID, "eol_reached",
				fmt.Sprintf("EOL reached: %s", assetTag),
				fmt.Sprintf("Asset '%s' has passed its end-of-life date. Decommission work order created.", name),
				"work_order", order.ID)
		}
		zap.L().Info("eol checker: created decommission WO", zap.String("asset", assetTag))
	}
	if iterErr := rows.Err(); iterErr != nil {
		zap.L().Warn("eol checker: rows iter failed", zap.Error(iterErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceEOLCheck, telemetry.ReasonRowsIterFailed).Inc()
	}
}

// --- Auto Work Order 5: Over Expected Lifespan → Lifespan Evaluation ---

func (w *WorkflowSubscriber) checkOverLifespan(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.expected_lifespan_months, a.created_at
		 FROM assets a
		 WHERE a.expected_lifespan_months IS NOT NULL
		   AND a.created_at + (a.expected_lifespan_months || ' months')::interval < now()
		   AND a.status NOT IN ('disposed', 'decommission')
		   AND a.deleted_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.tenant_id = a.tenant_id
		     AND wo.asset_id = a.id
		     AND wo.type = 'lifespan_evaluation'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("lifespan checker: query failed", zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceLifespanCheck, telemetry.ReasonDBQueryFailed).Inc()
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var lifespanMonths int
		var createdAt time.Time
		if scanErr := rows.Scan(&assetID, &tenantID, &assetTag, &name, &lifespanMonths, &createdAt); scanErr != nil {
			zap.L().Warn("lifespan checker: row scan failed", zap.Error(scanErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceLifespanCheck, telemetry.ReasonRowScanFailed).Inc()
			continue
		}

		actualMonths := int(time.Since(createdAt).Hours() / 24 / 30)

		sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceLifespanCheck)
		if !ok {
			continue
		}
		order, err := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Lifespan Evaluation: %s (%s)", name, assetTag),
			Type:        "lifespan_evaluation",
			Priority:    "medium",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' has been in service for %d months, exceeding the expected lifespan of %d months. Evaluate: continue operation, plan replacement, or schedule decommission.", name, actualMonths, lifespanMonths),
		})
		if err != nil {
			zap.L().Warn("lifespan checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceLifespanCheck, reasonWOCreateFailed).Inc()
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.warnNotify(ctx, sourceLifespanCheck, tenantID, adminID, "lifespan_exceeded",
				fmt.Sprintf("Lifespan exceeded: %s", assetTag),
				fmt.Sprintf("Asset '%s' exceeded expected %d-month lifespan.", name, lifespanMonths),
				"work_order", order.ID)
		}
		zap.L().Info("lifespan checker: created evaluation WO", zap.String("asset", assetTag))
	}
	if iterErr := rows.Err(); iterErr != nil {
		zap.L().Warn("lifespan checker: rows iter failed", zap.Error(iterErr))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceLifespanCheck, telemetry.ReasonRowsIterFailed).Inc()
	}
}
