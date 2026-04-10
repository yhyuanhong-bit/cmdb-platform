package workflows

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// WorkflowSubscriber handles cross-module reactions to domain events.
type WorkflowSubscriber struct {
	pool    *pgxpool.Pool
	queries *dbgen.Queries
	bus     eventbus.Bus
}

// New creates a WorkflowSubscriber.
func New(pool *pgxpool.Pool, queries *dbgen.Queries, bus eventbus.Bus) *WorkflowSubscriber {
	return &WorkflowSubscriber{pool: pool, queries: queries, bus: bus}
}

// Register subscribes to all relevant event subjects.
func (w *WorkflowSubscriber) Register() {
	if w.bus == nil {
		return
	}

	w.bus.Subscribe(eventbus.SubjectOrderTransitioned, w.onOrderTransitioned)
	w.bus.Subscribe("alert.fired", w.onAlertFired)

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

	// Only auto-create work orders for critical alerts
	if payload.Severity != "critical" {
		return nil
	}

	tenantID, _ := uuid.Parse(event.TenantID)

	// Dedup: check if an open work order already exists for this asset
	var existingCount int
	w.pool.QueryRow(ctx,
		"SELECT count(*) FROM work_orders WHERE asset_id = $1 AND status NOT IN ('completed','verified','rejected') AND deleted_at IS NULL AND tenant_id = $2",
		payload.AssetID, tenantID).Scan(&existingCount)

	if existingCount > 0 {
		return nil // already has an open work order
	}

	// Create emergency work order
	assetID, _ := uuid.Parse(payload.AssetID)
	code := fmt.Sprintf("WO-EMG-%d", uuid.New().ID())

	w.pool.Exec(ctx, `
		INSERT INTO work_orders (tenant_id, code, title, type, status, priority, asset_id, description, created_at, updated_at)
		VALUES ($1, $2, $3, 'emergency', 'submitted', 'critical', $4, $5, now(), now())
	`, tenantID, code, fmt.Sprintf("Emergency: %s", payload.Message), assetID, payload.Message)

	zap.L().Info("workflow: auto-created emergency work order",
		zap.String("asset_id", payload.AssetID),
		zap.String("code", code))

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
