package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// --- Auto Work Order 1: Warranty Expiry → Renewal Evaluation ---

// StartWarrantyChecker runs daily to check for assets approaching warranty expiry.
func (w *WorkflowSubscriber) StartWarrantyChecker(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	go func() {
		w.runDailyChecks(ctx)
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.runDailyChecks(ctx)
			}
		}
	}()
	zap.L().Info("Daily data governance checker started (24h interval)")
}

// runDailyChecks executes all daily data governance work order checks in sequence.
func (w *WorkflowSubscriber) runDailyChecks(ctx context.Context) {
	w.checkWarrantyExpiry(ctx)
	w.checkEOLReached(ctx)
	w.checkOverLifespan(ctx)
	w.checkFirmwareOutdated(ctx)
}

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
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'warranty_renewal'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("warranty checker: query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var warrantyEnd time.Time
		var warrantyVendor *string
		if rows.Scan(&assetID, &tenantID, &assetTag, &name, &warrantyEnd, &warrantyVendor) != nil {
			continue
		}

		daysLeft := int(time.Until(warrantyEnd).Hours() / 24)
		vendor := "N/A"
		if warrantyVendor != nil {
			vendor = *warrantyVendor
		}

		order, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Warranty Renewal: %s (%s)", name, assetTag),
			Type:        "warranty_renewal",
			Priority:    "medium",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' warranty expires in %d days (vendor: %s, expiry: %s). Evaluate: renew warranty, plan replacement, or accept risk.", name, daysLeft, vendor, warrantyEnd.Format("2006-01-02")),
		})
		if err != nil {
			zap.L().Debug("warranty checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.createNotification(ctx, tenantID, adminID,
				"warranty_expiry",
				fmt.Sprintf("Warranty expiring: %s", assetTag),
				fmt.Sprintf("Asset '%s' warranty expires in %d days. Work order created.", name, daysLeft),
				"work_order", order.ID)
		}

		zap.L().Info("warranty checker: created renewal WO",
			zap.String("asset", assetTag),
			zap.Int("days_left", daysLeft))
	}
}

// --- Auto Work Order 2: CMDB record not seen by scan → Asset Verification ---

// StartAssetVerificationChecker runs weekly to find assets not detected by any network scan.
func (w *WorkflowSubscriber) StartAssetVerificationChecker(ctx context.Context) {
	ticker := time.NewTicker(7 * 24 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.runWeeklyChecks(ctx)
			}
		}
	}()
	zap.L().Info("Weekly data governance checker started (7d interval)")
}

// runWeeklyChecks executes all weekly data governance work order checks in sequence.
func (w *WorkflowSubscriber) runWeeklyChecks(ctx context.Context) {
	w.checkMissingAssets(ctx)
	w.checkShadowIT(ctx)
	w.checkDuplicateSerials(ctx)
	w.checkMissingLocation(ctx)
}

func (w *WorkflowSubscriber) checkMissingAssets(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.ip_address, a.bmc_ip
		 FROM assets a
		 WHERE a.deleted_at IS NULL
		   AND a.status NOT IN ('disposed', 'decommission', 'procurement')
		   AND (a.ip_address IS NOT NULL OR a.bmc_ip IS NOT NULL)
		   AND NOT EXISTS (
		     SELECT 1 FROM discovered_assets da
		     WHERE da.tenant_id = a.tenant_id
		     AND (da.ip_address = a.ip_address OR da.ip_address = a.bmc_ip)
		     AND da.created_at > now() - interval '30 days'
		   )
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'asset_verification'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("asset verification checker: query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var ipAddress, bmcIP *string
		if rows.Scan(&assetID, &tenantID, &assetTag, &name, &ipAddress, &bmcIP) != nil {
			continue
		}

		ip := "N/A"
		if ipAddress != nil {
			ip = *ipAddress
		} else if bmcIP != nil {
			ip = *bmcIP
		}

		order, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Asset Verification: %s (%s)", name, assetTag),
			Type:        "asset_verification",
			Priority:    "low",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' (IP: %s) has not been detected by any network scan in the last 30 days. Please verify: is the asset still physically present? Has it been relocated? Is it powered off?", name, ip),
		})
		if err != nil {
			zap.L().Debug("asset verification checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.createNotification(ctx, tenantID, adminID,
				"asset_verification",
				fmt.Sprintf("Asset not detected: %s", assetTag),
				fmt.Sprintf("Asset '%s' not seen by scans in 30 days. Work order created for verification.", name),
				"work_order", order.ID)
		}

		zap.L().Info("asset verification checker: created WO",
			zap.String("asset", assetTag))
	}
}

// --- Auto Work Order 3: Scan data differs from CMDB → Data Correction ---

// scanDifferencesPayload is the expected event payload for scan differences.
type scanDifferencesPayload struct {
	AssetID   string                 `json:"asset_id"`
	AssetTag  string                 `json:"asset_tag"`
	AssetName string                 `json:"asset_name"`
	Diffs     map[string]interface{} `json:"diffs"`
}

// onScanDifferencesDetected handles SubjectScanDifferencesDetected events published by the
// IPMI collector or discovery pipeline when field values diverge from CMDB records.
func (w *WorkflowSubscriber) onScanDifferencesDetected(ctx context.Context, event eventbus.Event) error {
	var payload scanDifferencesPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		zap.L().Warn("workflow: failed to parse scan differences payload", zap.Error(err))
		return nil
	}

	tenantID, _ := uuid.Parse(event.TenantID)
	assetID, err := uuid.Parse(payload.AssetID)
	if err != nil || tenantID == uuid.Nil {
		return nil
	}

	w.checkScanDifferences(ctx, tenantID, assetID, payload.AssetTag, payload.AssetName, payload.Diffs)
	return nil
}

// checkScanDifferences creates a data correction WO when scan results differ from CMDB.
// It can be called directly from the IPMI collector or via the SubjectScanDifferencesDetected event.
func (w *WorkflowSubscriber) checkScanDifferences(ctx context.Context, tenantID, assetID uuid.UUID, assetTag, assetName string, diffs map[string]interface{}) {
	if len(diffs) == 0 {
		return
	}

	var existingCount int
	w.pool.QueryRow(ctx,
		`SELECT count(*) FROM work_orders
		 WHERE asset_id = $1 AND type = 'data_correction'
		 AND status NOT IN ('completed','verified','rejected')
		 AND deleted_at IS NULL`,
		assetID).Scan(&existingCount)
	if existingCount > 0 {
		return
	}

	diffLines := make([]string, 0, len(diffs))
	for field, val := range diffs {
		if m, ok := val.(map[string]interface{}); ok {
			diffLines = append(diffLines, fmt.Sprintf("- %s: CMDB='%v' → Scanned='%v'", field, m["cmdb"], m["scanned"]))
		}
	}
	if len(diffLines) == 0 {
		return
	}

	description := fmt.Sprintf(
		"Network scan detected data inconsistencies for asset '%s' (%s):\n\n%s\n\nPlease verify and update CMDB records.",
		assetName, assetTag, strings.Join(diffLines, "\n"))

	order, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
		Title:       fmt.Sprintf("Data Correction: %s (%s)", assetName, assetTag),
		Type:        "data_correction",
		Priority:    "low",
		AssetID:     &assetID,
		Description: description,
	})
	if err != nil {
		zap.L().Debug("data correction: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
		return
	}

	admins := w.opsAdminUserIDs(ctx, tenantID)
	for _, adminID := range admins {
		w.createNotification(ctx, tenantID, adminID,
			"data_correction",
			fmt.Sprintf("Data mismatch: %s", assetTag),
			fmt.Sprintf("%d field(s) differ between scan and CMDB for '%s'.", len(diffs), assetName),
			"work_order", order.ID)
	}

	zap.L().Info("data correction WO created",
		zap.String("asset", assetTag),
		zap.Int("diff_count", len(diffs)))
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
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'decommission'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("eol checker: query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var eolDate time.Time
		if rows.Scan(&assetID, &tenantID, &assetTag, &name, &eolDate) != nil {
			continue
		}

		daysPast := int(time.Since(eolDate).Hours() / 24)

		order, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Decommission: %s (%s)", name, assetTag),
			Type:        "decommission",
			Priority:    "high",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' reached end-of-life %d days ago (EOL: %s). Action required: data migration, service failover, physical removal, and CMDB status update to 'disposed'.", name, daysPast, eolDate.Format("2006-01-02")),
		})
		if err != nil {
			zap.L().Debug("eol checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.createNotification(ctx, tenantID, adminID, "eol_reached",
				fmt.Sprintf("EOL reached: %s", assetTag),
				fmt.Sprintf("Asset '%s' has passed its end-of-life date. Decommission work order created.", name),
				"work_order", order.ID)
		}
		zap.L().Info("eol checker: created decommission WO", zap.String("asset", assetTag))
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
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'lifespan_evaluation'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("lifespan checker: query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		var lifespanMonths int
		var createdAt time.Time
		if rows.Scan(&assetID, &tenantID, &assetTag, &name, &lifespanMonths, &createdAt) != nil {
			continue
		}

		actualMonths := int(time.Since(createdAt).Hours() / 24 / 30)

		order, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Lifespan Evaluation: %s (%s)", name, assetTag),
			Type:        "lifespan_evaluation",
			Priority:    "medium",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' has been in service for %d months, exceeding the expected lifespan of %d months. Evaluate: continue operation, plan replacement, or schedule decommission.", name, actualMonths, lifespanMonths),
		})
		if err != nil {
			zap.L().Debug("lifespan checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			continue
		}

		admins := w.opsAdminUserIDs(ctx, tenantID)
		for _, adminID := range admins {
			w.createNotification(ctx, tenantID, adminID, "lifespan_exceeded",
				fmt.Sprintf("Lifespan exceeded: %s", assetTag),
				fmt.Sprintf("Asset '%s' exceeded expected %d-month lifespan.", name, lifespanMonths),
				"work_order", order.ID)
		}
		zap.L().Info("lifespan checker: created evaluation WO", zap.String("asset", assetTag))
	}
}

// --- Auto Work Order 6: Shadow IT — Discovered but not in CMDB ---

func (w *WorkflowSubscriber) checkShadowIT(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT da.id, da.tenant_id, da.hostname, da.ip_address, da.source, da.created_at
		 FROM discovered_assets da
		 WHERE da.status = 'pending'
		   AND da.matched_asset_id IS NULL
		   AND da.created_at < now() - interval '7 days'
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.type = 'shadow_it_registration'
		     AND wo.description LIKE '%' || da.ip_address || '%'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("shadow IT checker: query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var daID, tenantID uuid.UUID
		var hostname, ipAddress, source string
		var createdAt time.Time
		if rows.Scan(&daID, &tenantID, &hostname, &ipAddress, &source, &createdAt) != nil {
			continue
		}

		daysPending := int(time.Since(createdAt).Hours() / 24)

		_, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Shadow IT: Unregistered device %s (%s)", hostname, ipAddress),
			Type:        "shadow_it_registration",
			Priority:    "medium",
			Description: fmt.Sprintf("Network scan (%s) discovered device '%s' (IP: %s) %d days ago but it has no matching CMDB record. This is potential shadow IT. Action: register as new asset, or mark as ignored in discovery.", source, hostname, ipAddress, daysPending),
		})
		if err != nil {
			zap.L().Debug("shadow IT checker: WO creation skipped", zap.String("ip", ipAddress), zap.Error(err))
			continue
		}
		zap.L().Info("shadow IT checker: created registration WO", zap.String("ip", ipAddress), zap.String("hostname", hostname))
	}
}

// --- Auto Work Order 7: Duplicate Serial Number → Dedup Audit ---

func (w *WorkflowSubscriber) checkDuplicateSerials(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT serial_number, tenant_id, array_agg(id) AS asset_ids, array_agg(asset_tag) AS asset_tags
		 FROM assets
		 WHERE serial_number IS NOT NULL
		   AND serial_number != ''
		   AND deleted_at IS NULL
		 GROUP BY serial_number, tenant_id
		 HAVING count(*) > 1`)
	if err != nil {
		zap.L().Warn("dedup checker: query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var serial string
		var tenantID uuid.UUID
		var assetIDs []uuid.UUID
		var assetTags []string
		if rows.Scan(&serial, &tenantID, &assetIDs, &assetTags) != nil {
			continue
		}

		var existingCount int
		w.pool.QueryRow(ctx,
			`SELECT count(*) FROM work_orders
			 WHERE type = 'dedup_audit' AND description LIKE '%' || $1 || '%'
			 AND status NOT IN ('completed','verified','rejected')
			 AND deleted_at IS NULL AND tenant_id = $2`,
			serial, tenantID).Scan(&existingCount)
		if existingCount > 0 {
			continue
		}

		tagList := strings.Join(assetTags, ", ")

		_, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Dedup Audit: Serial %s duplicated", serial),
			Type:        "dedup_audit",
			Priority:    "high",
			Description: fmt.Sprintf("Serial number '%s' appears on %d assets: [%s]. This violates data uniqueness. Action: identify the correct asset, merge or delete duplicates, investigate how the duplication occurred.", serial, len(assetIDs), tagList),
		})
		if err != nil {
			zap.L().Debug("dedup checker: WO creation skipped", zap.String("serial", serial), zap.Error(err))
			continue
		}
		zap.L().Info("dedup checker: created audit WO", zap.String("serial", serial), zap.Int("count", len(assetIDs)))
	}
}

// --- Auto Work Order 8: Missing Location → Location Completion ---

func (w *WorkflowSubscriber) checkMissingLocation(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`SELECT a.id, a.tenant_id, a.asset_tag, a.name
		 FROM assets a
		 WHERE a.location_id IS NULL
		   AND a.rack_id IS NULL
		   AND a.status NOT IN ('disposed', 'decommission', 'procurement')
		   AND a.deleted_at IS NULL
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'location_completion'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("location completion checker: query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	count := 0
	var firstTenantID uuid.UUID
	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name string
		if rows.Scan(&assetID, &tenantID, &assetTag, &name) != nil {
			continue
		}
		if count == 0 {
			firstTenantID = tenantID
		}
		count++

		if count <= 10 {
			_, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
				Title:       fmt.Sprintf("Location Missing: %s (%s)", name, assetTag),
				Type:        "location_completion",
				Priority:    "low",
				AssetID:     &assetID,
				Description: fmt.Sprintf("Asset '%s' has no location or rack assigned. An asset without a known location cannot be physically managed. Please assign location and rack.", name),
			})
			if err != nil {
				zap.L().Debug("location completion checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			}
		}
	}

	if count > 10 {
		_, err := w.maintenanceSvc.Create(ctx, firstTenantID, uuid.Nil, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Bulk Location Completion: %d assets missing location", count),
			Type:        "location_completion",
			Priority:    "medium",
			Description: fmt.Sprintf("%d assets have no location or rack assigned. Run a location audit to assign physical positions.", count),
		})
		if err != nil {
			zap.L().Debug("location completion checker: bulk WO creation skipped", zap.Error(err))
		}
	}

	if count > 0 {
		zap.L().Info("location completion checker: found assets without location", zap.Int("count", count))
	}
}

// --- Auto Work Order 9: Firmware Outdated → Firmware Upgrade ---

func (w *WorkflowSubscriber) checkFirmwareOutdated(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		`WITH latest AS (
		   SELECT bmc_type, MAX(bmc_firmware) AS latest_firmware
		   FROM assets
		   WHERE bmc_type IS NOT NULL AND bmc_firmware IS NOT NULL AND deleted_at IS NULL
		   GROUP BY bmc_type
		 )
		 SELECT a.id, a.tenant_id, a.asset_tag, a.name, a.bmc_type, a.bmc_firmware, l.latest_firmware
		 FROM assets a
		 JOIN latest l ON a.bmc_type = l.bmc_type
		 WHERE a.bmc_firmware IS NOT NULL
		   AND a.bmc_firmware != l.latest_firmware
		   AND a.deleted_at IS NULL
		   AND a.status NOT IN ('disposed', 'decommission')
		   AND NOT EXISTS (
		     SELECT 1 FROM work_orders wo
		     WHERE wo.asset_id = a.id
		     AND wo.type = 'firmware_upgrade'
		     AND wo.status NOT IN ('completed','verified','rejected')
		     AND wo.deleted_at IS NULL
		   )`)
	if err != nil {
		zap.L().Warn("firmware checker: query failed", zap.Error(err))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var assetID, tenantID uuid.UUID
		var assetTag, name, bmcType, currentFW, latestFW string
		if rows.Scan(&assetID, &tenantID, &assetTag, &name, &bmcType, &currentFW, &latestFW) != nil {
			continue
		}

		_, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Firmware Upgrade: %s (%s)", name, assetTag),
			Type:        "firmware_upgrade",
			Priority:    "low",
			AssetID:     &assetID,
			Description: fmt.Sprintf("Asset '%s' BMC firmware (%s: %s) is behind the latest detected version (%s). Schedule firmware upgrade to maintain security compliance.", name, bmcType, currentFW, latestFW),
		})
		if err != nil {
			zap.L().Debug("firmware checker: WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
			continue
		}
		zap.L().Info("firmware checker: created upgrade WO", zap.String("asset", assetTag))
	}
}

// --- Auto Work Order 10: BMC Default Password → Security Hardening (event-driven) ---

// createBMCSecurityWO creates a critical security hardening work order when a BMC default
// password is detected. Called from the onBMCDefaultPassword event handler.
func (w *WorkflowSubscriber) createBMCSecurityWO(ctx context.Context, tenantID, assetID uuid.UUID, assetTag, name, bmcType string) {
	var existingCount int
	w.pool.QueryRow(ctx,
		`SELECT count(*) FROM work_orders
		 WHERE asset_id = $1 AND type = 'security_hardening'
		 AND status NOT IN ('completed','verified','rejected')
		 AND deleted_at IS NULL`,
		assetID).Scan(&existingCount)
	if existingCount > 0 {
		return
	}

	order, err := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
		Title:       fmt.Sprintf("Security: Default BMC Password — %s (%s)", name, assetTag),
		Type:        "security_hardening",
		Priority:    "critical",
		AssetID:     &assetID,
		Description: fmt.Sprintf("CRITICAL: Asset '%s' BMC (%s) is accessible with default credentials. This is a severe security risk. Immediately change the BMC password and verify access controls.", name, bmcType),
	})
	if err != nil {
		zap.L().Debug("security: BMC hardening WO creation skipped", zap.String("asset", assetTag), zap.Error(err))
		return
	}

	admins := w.opsAdminUserIDs(ctx, tenantID)
	for _, adminID := range admins {
		w.createNotification(ctx, tenantID, adminID, "security_hardening",
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
