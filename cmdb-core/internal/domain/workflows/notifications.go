package workflows

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

//tenantlint:allow-direct-pool — cross-tenant notification dispatcher

// Source labels for telemetry.ErrorsSuppressedTotal. One const per
// module-function pair so the label value cannot drift between call
// sites and operators see a stable series name in Grafana.
const (
	sourceNotifyOrderDone = "workflows.notifications.orderTransitioned"
	sourceNotifyAlert     = "workflows.notifications.alertFired"
	sourceNotifyCreate    = "workflows.notifications.createNotification"
	sourceNotifyInventory = "workflows.notifications.inventoryCompleted"
)

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
		if _, resolveErr := w.pool.Exec(ctx,
			"UPDATE alert_events SET status = 'resolved', resolved_at = now() WHERE asset_id = $1 AND status = 'firing' AND tenant_id = $2",
			order.AssetID.Bytes, tenantID,
		); resolveErr != nil {
			zap.L().Warn("workflow: auto-resolve alerts failed", zap.Error(resolveErr),
				zap.String("order_id", payload.OrderID))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceNotifyOrderDone, telemetry.ReasonDBExecFailed).Inc()
		} else {
			assetUUID := uuid.UUID(order.AssetID.Bytes)
			zap.L().Info("workflow: auto-resolved alerts for completed work order",
				zap.String("order_id", payload.OrderID),
				zap.String("asset_id", assetUUID.String()))
		}
	}

	// 2. If order is linked to an asset, update the asset's updated_at.
	// Timestamp bump is best-effort — a failure just means the sync
	// stream will pick up the real mutation on the next tick. Still
	// logged + counted so sustained failures show up as an alert.
	if order.AssetID.Valid {
		if _, err := w.pool.Exec(ctx,
			"UPDATE assets SET updated_at = now() WHERE id = $1 AND tenant_id = $2",
			order.AssetID.Bytes, tenantID,
		); err != nil {
			zap.L().Warn("workflow: asset updated_at bump failed", zap.Error(err),
				zap.String("order_id", payload.OrderID))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceNotifyOrderDone, telemetry.ReasonDBExecFailed).Inc()
		}
	}

	// 3. Notify the creator that the work is completed and needs
	// verification. createNotification now returns an error so
	// transient INSERT failures surface instead of vanishing silently.
	if order.RequestorID.Valid {
		requestorUUID := uuid.UUID(order.RequestorID.Bytes)
		if err := w.createNotification(ctx, tenantID, requestorUUID,
			"work_completed",
			fmt.Sprintf("Work order %s completed", order.Code),
			fmt.Sprintf("Work order \"%s\" has been completed and is ready for verification.", order.Title),
			"work_order", orderID,
		); err != nil {
			zap.L().Warn("workflow: notify requestor failed", zap.Error(err),
				zap.String("order_id", payload.OrderID))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceNotifyOrderDone, telemetry.ReasonNotificationFailed).Inc()
		}
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
			w.warnNotify(ctx, sourceNotifyAlert, tenantID, uid,
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

	// Dedup: only suppress if an open emergency work order already exists
	// for this asset in this tenant. A pending maintenance/inspection WO
	// addresses a different lifecycle and MUST NOT mute an incoming
	// critical alert — before this narrowing, a single stale routine WO
	// silently swallowed every downstream critical alert for the same
	// asset. Tenant_id stays in the predicate for isolation.
	var existingCount int
	err = w.pool.QueryRow(ctx,
		`SELECT count(*) FROM work_orders
		  WHERE asset_id = $1
		    AND tenant_id = $2
		    AND type = 'emergency'
		    AND status IN ('approved','in_progress','submitted')
		    AND deleted_at IS NULL`,
		assetUUID, tenantID).Scan(&existingCount)
	if err != nil {
		zap.L().Warn("workflow: dedup check failed", zap.Error(err))
		return nil
	}

	if existingCount > 0 {
		return nil // already has an open emergency work order for this asset
	}

	// Create emergency work order via service layer. Resolver returns the
	// per-tenant system user UUID so both work_orders.requestor_id and the
	// downstream work_order_logs.operator_id FKs resolve — uuid.Nil would
	// violate both (migration 000052).
	sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceNotifyAlert)
	if !ok {
		return nil
	}
	order, createErr := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
		Title:       fmt.Sprintf("Emergency: %s", payload.Message),
		Type:        "emergency",
		Priority:    "critical",
		AssetID:     &assetUUID,
		Description: payload.Message,
	})
	if createErr != nil {
		// Emergency-WO creation failure used to log at Debug,
		// hiding the failure behind the default log level so
		// operators only noticed days later when a real incident
		// escaped through the same path. Promoted to Warn +
		// counter so sustained failures become a dashboard signal.
		zap.L().Warn("workflow: emergency WO creation skipped", zap.Error(createErr),
			zap.String("asset_id", payload.AssetID))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceNotifyAlert, telemetry.ReasonWOCreationFailed).Inc()
		return nil
	}

	// Atomically flip governance_status='approved' AND execution_status='working'
	// in a single SQL UPDATE. Previously this was a two-step Transition(approved) +
	// Transition(in_progress) flow — a crash, DB timeout, or retry between the two
	// steps could strand the WO half-approved (approved-but-not-started), which
	// then trips the SLA scanner and hides real emergencies behind phantom tickets.
	//
	// Idempotency: the service returns (nil, nil) if 0 rows matched (already
	// transitioned, concurrent winner, or stale retry). That is success — the
	// caller's intent ("ensure this emergency WO is approved+in_progress") is
	// already satisfied.
	//
	// sysUID (the per-tenant 'system' user) stands in as the auto-approver.
	// work_order_logs.operator_id has a FK to users(id), so uuid.Nil here
	// would trip the same FK violation we just fixed on .requestor_id.
	if _, err := w.maintenanceSvc.TransitionEmergencyAtomic(ctx, tenantID, order.ID, sysUID); err != nil {
		zap.L().Error("workflow: failed to atomic-approve emergency WO",
			zap.String("order_id", order.ID.String()), zap.Error(err))
		// No compensation path: the atomic UPDATE either fully succeeded, fully
		// failed (no state change), or matched 0 rows (nothing to undo). Log and
		// move on — the next alert-fire event will retry idempotently.
	}

	zap.L().Info("workflow: auto-created emergency work order",
		zap.String("asset_id", payload.AssetID),
		zap.String("order_id", order.ID.String()))

	return nil
}

// warnNotify is the background-safe wrapper around createNotification:
// every auto-check / ticker / event subscriber already swallows per-row
// errors to keep the outer loop alive, so instead of teaching every
// call site to handle the new error return, we route them through one
// place that Warn-logs + counters and drops. Use createNotification
// directly only when the caller needs to propagate the failure to a
// user-facing request path (none today).
func (w *WorkflowSubscriber) warnNotify(
	ctx context.Context,
	source string,
	tenantID, userID uuid.UUID,
	notifType, title, body, resourceType string,
	resourceID uuid.UUID,
) {
	if err := w.createNotification(ctx, tenantID, userID, notifType, title, body, resourceType, resourceID); err != nil {
		zap.L().Warn("workflow: createNotification failed",
			zap.String("source", source),
			zap.String("notif_type", notifType),
			zap.Error(err))
		telemetry.ErrorsSuppressedTotal.WithLabelValues(source, telemetry.ReasonNotificationFailed).Inc()
	}
}

// createNotification inserts a notification row and (best-effort)
// publishes a WebSocket event. Returns the INSERT error so background
// callers can Warn + counter and carry on. The publish step is still
// fire-and-forget — a failed WebSocket broadcast must not stop the DB
// write from being visible, but we log it so a broken bus is still
// diagnosable.
func (w *WorkflowSubscriber) createNotification(
	ctx context.Context,
	tenantID, userID uuid.UUID,
	notifType, title, body, resourceType string,
	resourceID uuid.UUID,
) error {
	if _, err := w.pool.Exec(ctx,
		"INSERT INTO notifications (tenant_id, user_id, type, title, body, resource_type, resource_id) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		tenantID, userID, notifType, title, body, resourceType, resourceID,
	); err != nil {
		return fmt.Errorf("insert notification: %w", err)
	}

	// Publish for WebSocket delivery
	if w.bus != nil {
		payload, err := json.Marshal(map[string]string{
			"user_id": userID.String(),
			"type":    notifType,
			"title":   title,
		})
		if err != nil {
			zap.L().Warn("createNotification: marshal payload failed", zap.Error(err))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceNotifyCreate, telemetry.ReasonJSONUnmarshal).Inc()
			return nil
		}
		if pubErr := w.bus.Publish(ctx, eventbus.Event{
			Subject:  eventbus.SubjectNotificationCreated,
			TenantID: tenantID.String(),
			Payload:  payload,
		}); pubErr != nil {
			zap.L().Warn("createNotification: bus publish failed", zap.Error(pubErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceNotifyCreate, telemetry.ReasonNotificationFailed).Inc()
		}
	}
	return nil
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
		w.warnNotify(ctx, "workflows.notifications.assetCreated", tenantID, uid,
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

	w.warnNotify(ctx, sourceNotifyInventory, tenantID, assignedTo,
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
		sysUID, ok := w.resolveSystemUser(ctx, tenantID, sourceNotifyInventory)
		if !ok {
			return nil
		}
		_, woErr := w.maintenanceSvc.Create(ctx, tenantID, sysUID, maintenance.CreateOrderRequest{
			Title:       fmt.Sprintf("Inventory discrepancies: %s (%d items)", code, discrepancyCount),
			Type:        "inspection",
			Priority:    "high",
			Description: fmt.Sprintf("Inventory task %s completed with %d discrepancies requiring investigation.", code, discrepancyCount),
		})
		if woErr != nil {
			// maintenanceSvc.Create failures used to log at Debug,
			// hiding real failures behind the default log level.
			// Promoted to Warn + counter so repeated failures show
			// up in Grafana without having to flip to debug mode.
			zap.L().Warn("workflow: auto work order for inventory skipped", zap.Error(woErr))
			telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceNotifyInventory, telemetry.ReasonWOCreationFailed).Inc()
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
		w.warnNotify(ctx, "workflows.notifications.importCompleted", tenantID, uid,
			"import_completed",
			"Asset import completed",
			fmt.Sprintf("Import finished: %d created, %d updated, %d errors.", payload.Created, payload.Updated, payload.Errors),
			"import", uuid.Nil)
	}
	return nil
}

// StartSessionCleanup runs a background ticker that cleans up expired and old sessions.
