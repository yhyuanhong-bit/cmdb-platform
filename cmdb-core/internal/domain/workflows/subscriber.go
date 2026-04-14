package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// WorkflowSubscriber handles cross-module reactions to domain events.
type WorkflowSubscriber struct {
	pool            *pgxpool.Pool
	queries         *dbgen.Queries
	bus             eventbus.Bus
	maintenanceSvc  *maintenance.Service
	adapterFailures map[uuid.UUID]int
}

// New creates a WorkflowSubscriber.
func New(pool *pgxpool.Pool, queries *dbgen.Queries, bus eventbus.Bus, maintenanceSvc *maintenance.Service) *WorkflowSubscriber {
	return &WorkflowSubscriber{
		pool:            pool,
		queries:         queries,
		bus:             bus,
		maintenanceSvc:  maintenanceSvc,
		adapterFailures: make(map[uuid.UUID]int),
	}
}

// Register subscribes to all relevant event subjects.
func (w *WorkflowSubscriber) Register() {
	if w.bus == nil {
		return
	}

	w.bus.Subscribe(eventbus.SubjectOrderTransitioned, w.onOrderTransitioned)
	w.bus.Subscribe("alert.fired", w.onAlertFired)
	w.bus.Subscribe(eventbus.SubjectAssetCreated, w.onAssetCreatedNotify)
	w.bus.Subscribe(eventbus.SubjectInventoryTaskCompleted, w.onInventoryCompletedNotify)
	w.bus.Subscribe(eventbus.SubjectImportCompleted, w.onImportCompletedNotify)
	w.bus.Subscribe(eventbus.SubjectScanDifferencesDetected, w.onScanDifferencesDetected)
	w.bus.Subscribe(eventbus.SubjectBMCDefaultPassword, w.onBMCDefaultPassword)

	zap.L().Info("workflow subscribers registered")
}

// orderTransitionPayload is the expected event payload for work order transitions.
type orderTransitionPayload struct {
	OrderID  string `json:"order_id"`
	Status   string `json:"status"`
	TenantID string `json:"tenant_id"`
}

func (w *WorkflowSubscriber) onOrderTransitioned(ctx context.Context, event eventbus.Event) error {
	var payload orderTransitionPayload
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		zap.L().Warn("workflow: failed to parse order transition", zap.Error(err))
		return nil
	}

	if payload.Status != "completed" {
		return nil
	}

	// Work order completed - trigger cross-module actions
	orderID, err := uuid.Parse(payload.OrderID)
	if err != nil {
		return nil
	}
	tenantID, _ := uuid.Parse(event.TenantID)

	order, err := w.queries.GetWorkOrder(ctx, dbgen.GetWorkOrderParams{ID: orderID, TenantID: tenantID})
	if err != nil {
		zap.L().Warn("workflow: order not found", zap.String("order_id", payload.OrderID))
		return nil
	}

	// 1. If order is linked to an asset with active alerts, auto-resolve them
	if order.AssetID.Valid {
		_, resolveErr := w.pool.Exec(ctx,
			"UPDATE alert_events SET status = 'resolved', resolved_at = now() WHERE asset_id = $1 AND status = 'firing' AND tenant_id = $2",
			order.AssetID.Bytes, tenantID)
		if resolveErr == nil {
			assetUUID := uuid.UUID(order.AssetID.Bytes)
			zap.L().Info("workflow: auto-resolved alerts for completed work order",
				zap.String("order_id", payload.OrderID),
				zap.String("asset_id", assetUUID.String()))
		}
	}

	// 2. If order is linked to an asset, update the asset's updated_at
	if order.AssetID.Valid {
		w.pool.Exec(ctx,
			"UPDATE assets SET updated_at = now() WHERE id = $1 AND tenant_id = $2",
			order.AssetID.Bytes, tenantID)
	}

	// 3. Notify the creator that the work is completed and needs verification
	if order.RequestorID.Valid {
		requestorUUID := uuid.UUID(order.RequestorID.Bytes)
		w.createNotification(ctx, tenantID, requestorUUID,
			"work_completed",
			fmt.Sprintf("Work order %s completed", order.Code),
			fmt.Sprintf("Work order \"%s\" has been completed and is ready for verification.", order.Title),
			"work_order", orderID)
	}

	return nil
}

func (w *WorkflowSubscriber) onAlertFired(ctx context.Context, event eventbus.Event) error {
	var payload struct {
		AlertID  string `json:"alert_id"`
		Severity string `json:"severity"`
		AssetID  string `json:"asset_id"`
		Message  string `json:"message"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil
	}

	tenantID, _ := uuid.Parse(event.TenantID)

	// Notify ops-admins about ALL alert severities
	if tenantID != uuid.Nil {
		for _, uid := range w.opsAdminUserIDs(ctx, tenantID) {
			alertID, _ := uuid.Parse(payload.AlertID)
			w.createNotification(ctx, tenantID, uid,
				"alert_fired",
				fmt.Sprintf("Alert: %s", payload.Message),
				fmt.Sprintf("A %s alert has been triggered: %s", payload.Severity, payload.Message),
				"alert", alertID)
		}
	}

	// Only auto-create work orders for critical alerts
	if payload.Severity != "critical" {
		return nil
	}

	// Validate asset ID first
	assetUUID, err := uuid.Parse(payload.AssetID)
	if err != nil {
		zap.L().Warn("workflow: invalid asset_id in alert", zap.String("asset_id", payload.AssetID))
		return nil
	}

	// Dedup: check if an open work order already exists for this asset
	var existingCount int
	err = w.pool.QueryRow(ctx,
		"SELECT count(*) FROM work_orders WHERE asset_id = $1 AND status NOT IN ('completed','verified','rejected') AND deleted_at IS NULL AND tenant_id = $2",
		assetUUID, tenantID).Scan(&existingCount)
	if err != nil {
		zap.L().Warn("workflow: dedup check failed", zap.Error(err))
		return nil
	}

	if existingCount > 0 {
		return nil // already has an open work order
	}

	// Create emergency work order via service layer
	order, createErr := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
		Title:       fmt.Sprintf("Emergency: %s", payload.Message),
		Type:        "emergency",
		Priority:    "critical",
		AssetID:     &assetUUID,
		Description: payload.Message,
	})
	if createErr != nil {
		zap.L().Debug("workflow: emergency WO creation skipped", zap.Error(createErr))
		return nil
	}

	// Auto-advance to in_progress (skip approval for emergencies)
	// uuid.Nil as operatorID signals a system operation, bypassing self-approval checks
	if _, err := w.maintenanceSvc.Transition(ctx, tenantID, order.ID, uuid.Nil, []string{"super-admin"}, maintenance.TransitionRequest{
		Status:  "approved",
		Comment: "Auto-approved: emergency work order",
	}); err != nil {
		zap.L().Error("workflow: failed to auto-approve emergency WO", zap.String("order_id", order.ID.String()), zap.Error(err))
	}
	if _, err := w.maintenanceSvc.Transition(ctx, tenantID, order.ID, uuid.Nil, nil, maintenance.TransitionRequest{
		Status:  "in_progress",
		Comment: "Auto-started: emergency work order",
	}); err != nil {
		zap.L().Error("workflow: failed to auto-start emergency WO", zap.String("order_id", order.ID.String()), zap.Error(err))
	}

	zap.L().Info("workflow: auto-created emergency work order",
		zap.String("asset_id", payload.AssetID),
		zap.String("order_id", order.ID.String()))

	return nil
}

func (w *WorkflowSubscriber) createNotification(ctx context.Context, tenantID, userID uuid.UUID, notifType, title, body, resourceType string, resourceID uuid.UUID) {
	w.pool.Exec(ctx,
		"INSERT INTO notifications (tenant_id, user_id, type, title, body, resource_type, resource_id) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		tenantID, userID, notifType, title, body, resourceType, resourceID)

	// Publish for WebSocket delivery
	if w.bus != nil {
		payload, _ := json.Marshal(map[string]string{
			"user_id": userID.String(),
			"type":    notifType,
			"title":   title,
		})
		w.bus.Publish(ctx, eventbus.Event{
			Subject:  eventbus.SubjectNotificationCreated,
			TenantID: tenantID.String(),
			Payload:  payload,
		})
	}
}

// opsAdminUserIDs returns user IDs with the ops-admin or super-admin role for a tenant.
func (w *WorkflowSubscriber) opsAdminUserIDs(ctx context.Context, tenantID uuid.UUID) []uuid.UUID {
	rows, err := w.pool.Query(ctx,
		`SELECT DISTINCT u.id FROM users u
		 JOIN user_roles ur ON ur.user_id = u.id
		 JOIN roles r ON r.id = ur.role_id
		 WHERE r.name IN ('ops-admin', 'super-admin') AND u.tenant_id = $1 AND u.status = 'active'`,
		tenantID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

func (w *WorkflowSubscriber) onAssetCreatedNotify(ctx context.Context, event eventbus.Event) error {
	var payload struct {
		AssetID string `json:"asset_id"`
		Name    string `json:"name"`
		Type    string `json:"type"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil
	}
	tenantID, _ := uuid.Parse(event.TenantID)
	assetID, _ := uuid.Parse(payload.AssetID)
	if tenantID == uuid.Nil {
		return nil
	}

	for _, uid := range w.opsAdminUserIDs(ctx, tenantID) {
		w.createNotification(ctx, tenantID, uid,
			"asset_created",
			fmt.Sprintf("New asset: %s", payload.Name),
			fmt.Sprintf("A new %s asset \"%s\" has been added to the inventory.", payload.Type, payload.Name),
			"asset", assetID)
	}
	return nil
}

func (w *WorkflowSubscriber) onInventoryCompletedNotify(ctx context.Context, event eventbus.Event) error {
	var payload struct {
		TaskID   string `json:"task_id"`
		TenantID string `json:"tenant_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil
	}
	tenantID, _ := uuid.Parse(event.TenantID)
	taskID, _ := uuid.Parse(payload.TaskID)
	if tenantID == uuid.Nil || taskID == uuid.Nil {
		return nil
	}

	// Look up who is assigned to the task
	var assignedTo uuid.UUID
	var code string
	err := w.pool.QueryRow(ctx,
		"SELECT assigned_to, code FROM inventory_tasks WHERE id = $1 AND tenant_id = $2",
		taskID, tenantID).Scan(&assignedTo, &code)
	if err != nil || assignedTo == uuid.Nil {
		return nil
	}

	w.createNotification(ctx, tenantID, assignedTo,
		"inventory_completed",
		fmt.Sprintf("Inventory task %s completed", code),
		fmt.Sprintf("Inventory task \"%s\" has been completed. Please review the results.", code),
		"inventory_task", taskID)

	// Auto-create work order if discrepancies exceed threshold
	var discrepancyCount int
	err2 := w.pool.QueryRow(ctx,
		"SELECT count(*) FROM inventory_items WHERE task_id = $1 AND status IN ('discrepancy', 'missing')",
		taskID).Scan(&discrepancyCount)

	if err2 == nil && discrepancyCount > 5 {
		_, woErr := w.maintenanceSvc.Create(ctx, tenantID, uuid.Nil, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Inventory discrepancies: %s (%d items)", code, discrepancyCount),
			Type:        "inspection",
			Priority:    "high",
			Description: fmt.Sprintf("Inventory task %s completed with %d discrepancies requiring investigation.", code, discrepancyCount),
		})
		if woErr != nil {
			zap.L().Debug("workflow: auto work order for inventory skipped", zap.Error(woErr))
		} else {
			zap.L().Info("workflow: auto-created work order for inventory discrepancies",
				zap.String("task_code", code), zap.Int("discrepancies", discrepancyCount))
		}
	}

	return nil
}

func (w *WorkflowSubscriber) onImportCompletedNotify(ctx context.Context, event eventbus.Event) error {
	var payload struct {
		JobID   string `json:"job_id"`
		Created int    `json:"created"`
		Updated int    `json:"updated"`
		Errors  int    `json:"errors"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return nil
	}
	tenantID, _ := uuid.Parse(event.TenantID)
	if tenantID == uuid.Nil {
		return nil
	}

	// Notify all ops-admins about import completion
	for _, uid := range w.opsAdminUserIDs(ctx, tenantID) {
		w.createNotification(ctx, tenantID, uid,
			"import_completed",
			"Asset import completed",
			fmt.Sprintf("Import finished: %d created, %d updated, %d errors.", payload.Created, payload.Updated, payload.Errors),
			"import", uuid.Nil)
	}
	return nil
}

// StartSessionCleanup runs a background ticker that cleans up expired and old sessions.
func (w *WorkflowSubscriber) StartSessionCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.cleanupSessions(ctx)
			}
		}
	}()
	zap.L().Info("Session cleanup started (1h interval)")
}

func (w *WorkflowSubscriber) cleanupSessions(ctx context.Context) {
	// 1. Mark sessions inactive for 7+ days as expired
	res1, _ := w.pool.Exec(ctx,
		"UPDATE user_sessions SET expired_at = now() WHERE expired_at IS NULL AND last_active_at < now() - interval '7 days'")

	// 2. Delete sessions older than 30 days
	res2, _ := w.pool.Exec(ctx,
		"DELETE FROM user_sessions WHERE created_at < now() - interval '30 days'")

	// 3. Keep only latest 20 sessions per user (delete excess)
	res3, _ := w.pool.Exec(ctx,
		`DELETE FROM user_sessions WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) AS rn
				FROM user_sessions
			) ranked WHERE rn > 20
		)`)

	expired := res1.RowsAffected()
	deleted := res2.RowsAffected()
	trimmed := res3.RowsAffected()

	if expired+deleted+trimmed > 0 {
		zap.L().Info("session cleanup completed",
			zap.Int64("expired", expired),
			zap.Int64("deleted", deleted),
			zap.Int64("trimmed", trimmed))
	}
}

// StartConflictAndDiscoveryCleanup runs a background ticker for conflict SLA and discovery TTL.
func (w *WorkflowSubscriber) StartConflictAndDiscoveryCleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.autoResolveStaleConflicts(ctx)
				w.expireStaleDiscoveries(ctx)
			}
		}
	}()
	zap.L().Info("Conflict SLA + Discovery TTL checker started (1h interval)")
}

// autoResolveStaleConflicts resolves import conflicts older than 7 days
// by accepting the higher-priority source value.
func (w *WorkflowSubscriber) autoResolveStaleConflicts(ctx context.Context) {
	// Notify ops-admins about conflicts approaching 3-day SLA warning
	rows, _ := w.pool.Query(ctx,
		`SELECT tenant_id, count(*) FROM sync_conflicts
		 WHERE resolution = 'pending' AND created_at < now() - interval '3 days' AND created_at >= now() - interval '4 days'
		 GROUP BY tenant_id`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var tid uuid.UUID
			var cnt int
			if rows.Scan(&tid, &cnt) == nil {
				for _, uid := range w.opsAdminUserIDs(ctx, tid) {
					w.createNotification(ctx, tid, uid,
						"conflict_sla_warning",
						fmt.Sprintf("%d sync conflicts approaching SLA deadline", cnt),
						"These conflicts will be auto-resolved in 4 days if not manually addressed.",
						"sync_conflict", uuid.Nil)
				}
			}
		}
	}

	// Auto-resolve sync_conflicts older than 7 days
	res1, _ := w.pool.Exec(ctx,
		`UPDATE sync_conflicts SET resolution = 'auto_expired', resolved_at = now()
		 WHERE resolution = 'pending' AND created_at < now() - interval '7 days'`)

	// Also handle import_conflicts if the table exists (created by ingestion-engine)
	res2, _ := w.pool.Exec(ctx,
		`UPDATE import_conflicts SET status = 'auto_resolved', resolved_at = now()
		 WHERE status = 'pending' AND created_at < now() - interval '7 days'`)

	expired1 := res1.RowsAffected()
	expired2 := res2.RowsAffected()
	if expired1+expired2 > 0 {
		zap.L().Info("auto-resolved stale conflicts",
			zap.Int64("sync_conflicts", expired1),
			zap.Int64("import_conflicts", expired2))
	}
}

// expireStaleDiscoveries marks discovered assets pending for >14 days as expired.
func (w *WorkflowSubscriber) expireStaleDiscoveries(ctx context.Context) {
	res, _ := w.pool.Exec(ctx,
		`UPDATE discovered_assets SET status = 'expired'
		 WHERE status = 'pending' AND discovered_at < now() - interval '14 days'`)

	expired := res.RowsAffected()
	if expired > 0 {
		zap.L().Info("expired stale discoveries", zap.Int64("count", expired))
	}
}

// StartSLAChecker runs a background ticker that checks for SLA warnings and breaches.
func (w *WorkflowSubscriber) StartSLAChecker(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				w.checkSLABreaches(ctx)
				w.checkSLAWarnings(ctx)
			}
		}
	}()
	zap.L().Info("SLA checker started (60s interval)")
}

func (w *WorkflowSubscriber) checkSLABreaches(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		"SELECT id, tenant_id, code, assignee_id FROM work_orders WHERE status IN ('approved','in_progress') AND sla_deadline IS NOT NULL AND sla_deadline < now() AND sla_breached = false AND deleted_at IS NULL")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, tenantID uuid.UUID
		var code string
		var assigneeID pgtype.UUID
		if rows.Scan(&id, &tenantID, &code, &assigneeID) != nil {
			continue
		}
		w.pool.Exec(ctx, "UPDATE work_orders SET sla_breached = true WHERE id = $1 AND tenant_id = $2", id, tenantID)
		if assigneeID.Valid {
			w.createNotification(ctx, tenantID, uuid.UUID(assigneeID.Bytes),
				"sla_breach",
				fmt.Sprintf("SLA Breached: %s", code),
				fmt.Sprintf("Work order %s has exceeded its SLA deadline.", code),
				"work_order", id)
		}
		zap.L().Warn("SLA breached", zap.String("order", code))
	}
}

func (w *WorkflowSubscriber) checkSLAWarnings(ctx context.Context) {
	rows, err := w.pool.Query(ctx,
		"SELECT id, tenant_id, code, assignee_id FROM work_orders WHERE status IN ('approved','in_progress') AND sla_deadline IS NOT NULL AND sla_warning_sent = false AND sla_deadline - (sla_deadline - approved_at) * 0.25 < now() AND approved_at IS NOT NULL AND deleted_at IS NULL")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id, tenantID uuid.UUID
		var code string
		var assigneeID pgtype.UUID
		if rows.Scan(&id, &tenantID, &code, &assigneeID) != nil {
			continue
		}
		w.pool.Exec(ctx, "UPDATE work_orders SET sla_warning_sent = true WHERE id = $1 AND tenant_id = $2", id, tenantID)
		if assigneeID.Valid {
			w.createNotification(ctx, tenantID, uuid.UUID(assigneeID.Bytes),
				"sla_warning",
				fmt.Sprintf("SLA Warning: %s", code),
				fmt.Sprintf("Work order %s is approaching its SLA deadline.", code),
				"work_order", id)
		}
	}
}

// StartMetricsPuller periodically pulls metrics from active inbound integration adapters.
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
		`SELECT id, tenant_id, name, type, endpoint, config FROM integration_adapters
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
		var configRaw []byte
		if rows.Scan(&id, &tenantID, &name, &adapterType, &endpoint, &configRaw) != nil {
			continue
		}
		if endpoint == nil || *endpoint == "" {
			continue
		}

		pullErr := w.pullFromAdapter(ctx, id, tenantID, name, adapterType, *endpoint, configRaw)
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
			Title:    fmt.Sprintf("Dedup Audit: Serial %s duplicated", serial),
			Type:     "dedup_audit",
			Priority: "high",
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
			Title:    fmt.Sprintf("Firmware Upgrade: %s (%s)", name, assetTag),
			Type:     "firmware_upgrade",
			Priority: "low",
			AssetID:  &assetID,
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
		Title:    fmt.Sprintf("Security: Default BMC Password — %s (%s)", name, assetTag),
		Type:     "security_hardening",
		Priority: "critical",
		AssetID:  &assetID,
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
