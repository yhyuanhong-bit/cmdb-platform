package workflows

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"go.uber.org/zap"
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
