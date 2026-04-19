package api

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ---------------------------------------------------------------------------
// Inventory endpoints
// ---------------------------------------------------------------------------

// ListInventoryTasks returns a paginated list of inventory tasks.
// (GET /inventory/tasks)
func (s *APIServer) ListInventoryTasks(c *gin.Context, params ListInventoryTasksParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	var scopeLocationID *uuid.UUID
	if params.ScopeLocationId != nil {
		u := uuid.UUID(*params.ScopeLocationId)
		scopeLocationID = &u
	}
	tasks, total, err := s.inventorySvc.List(c.Request.Context(), tenantID, scopeLocationID, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list inventory tasks")
		return
	}
	response.OKList(c, convertSlice(tasks, toAPIInventoryTask), page, pageSize, int(total))
}

// GetInventoryTask returns a single inventory task by ID.
// (GET /inventory/tasks/{id})
func (s *APIServer) GetInventoryTask(c *gin.Context, id IdPath) {
	task, err := s.inventorySvc.GetByID(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "inventory task not found")
		return
	}
	response.OK(c, toAPIInventoryTask(*task))
}

// ListInventoryItems returns a paginated list of items in an inventory task.
// (GET /inventory/tasks/{id}/items)
func (s *APIServer) ListInventoryItems(c *gin.Context, id IdPath, params ListInventoryItemsParams) {
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	items, total, err := s.inventorySvc.ListItems(c.Request.Context(), uuid.UUID(id), limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list inventory items")
		return
	}
	response.OKList(c, convertSlice(items, toAPIInventoryItem), page, pageSize, int(total))
}

// CreateInventoryTask creates a new inventory task.
// (POST /inventory/tasks)
func (s *APIServer) CreateInventoryTask(c *gin.Context) {
	var req CreateInventoryTaskJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	code := fmt.Sprintf("INV-%d-%04d", time.Now().Year(), rand.Intn(10000))

	params := dbgen.CreateInventoryTaskParams{
		TenantID: tenantID,
		Code:     code,
		Name:     req.Name,
		Method:   pgtype.Text{String: req.Method, Valid: true},
	}
	if req.PlannedDate != "" {
		t, err := time.Parse("2006-01-02", req.PlannedDate)
		if err == nil {
			params.PlannedDate = pgtype.Date{Time: t, Valid: true}
		}
	}
	if req.ScopeLocationId != nil {
		params.ScopeLocationID = pgtype.UUID{Bytes: uuid.UUID(*req.ScopeLocationId), Valid: true}
	}
	if req.AssignedTo != nil {
		params.AssignedTo = pgtype.UUID{Bytes: uuid.UUID(*req.AssignedTo), Valid: true}
	}

	task, err := s.inventorySvc.Create(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to create inventory task")
		return
	}
	s.recordAudit(c, "task.created", "inventory", "inventory_task", task.ID, map[string]any{
		"code": task.Code,
		"name": task.Name,
	})
	response.Created(c, toAPIInventoryTask(*task))
}

// CompleteInventoryTask marks an inventory task as completed.
// (POST /inventory/tasks/{id}/complete)
//
// Scoped to the caller's tenant: the underlying UPDATE filters by
// `tenant_id` so a tenant-B caller holding a tenant-A task UUID can
// never flip its status. A miss returns 404 (not 403) to avoid
// leaking "exists in another tenant".
func (s *APIServer) CompleteInventoryTask(c *gin.Context, id IdPath) {
	task, err := s.inventorySvc.Complete(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "inventory task not found")
		return
	}
	s.recordAudit(c, "task.completed", "inventory", "inventory_task", uuid.UUID(id), map[string]any{
		"code": task.Code,
	})
	response.OK(c, toAPIInventoryTask(*task))
}

// UpdateInventoryTask updates an inventory task.
// (PUT /inventory/tasks/{id})
func (s *APIServer) UpdateInventoryTask(c *gin.Context, id openapi_types.UUID) {
	var req UpdateInventoryTaskJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	var assignedTo *uuid.UUID
	if req.AssignedTo != nil {
		u := uuid.UUID(*req.AssignedTo)
		assignedTo = &u
	}

	task, err := s.inventorySvc.Update(c.Request.Context(), tenantID, uuid.UUID(id), req.Name, req.PlannedDate, assignedTo)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(c, "inventory task not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	s.recordAudit(c, "task.updated", "inventory", "inventory_task", uuid.UUID(id), map[string]any{
		"name": req.Name,
	})
	response.OK(c, toAPIInventoryTask(*task))
}

// DeleteInventoryTask soft-deletes an inventory task.
// (DELETE /inventory/tasks/{id})
func (s *APIServer) DeleteInventoryTask(c *gin.Context, id openapi_types.UUID) {
	tenantID := tenantIDFromContext(c)
	err := s.inventorySvc.Delete(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			response.NotFound(c, "inventory task not found")
			return
		}
		response.BadRequest(c, err.Error())
		return
	}
	s.recordAudit(c, "task.deleted", "inventory", "inventory_task", uuid.UUID(id), nil)
	c.Status(204)
}

// ScanInventoryItem records a scan result for an inventory item.
// (POST /inventory/tasks/{id}/items/{itemId}/scan)
func (s *APIServer) ScanInventoryItem(c *gin.Context, id IdPath, itemId openapi_types.UUID) {
	var req ScanInventoryItemJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	actualJSON, _ := json.Marshal(req.Actual)
	userID := userIDFromContext(c)

	params := dbgen.ScanInventoryItemParams{
		ID:        uuid.UUID(itemId),
		Actual:    actualJSON,
		Status:    req.Status,
		ScannedBy: pgtype.UUID{Bytes: userID, Valid: true},
	}

	ctx := c.Request.Context()
	item, err := s.inventorySvc.ScanItem(ctx, params)
	if err != nil {
		response.NotFound(c, "inventory item not found")
		return
	}

	// Auto-activate task if still planned. Tenant-scoped: a cross-tenant
	// item.id (if one ever leaked here) must not be able to flip another
	// tenant's task status.
	taskID := uuid.UUID(id)
	s.pool.Exec(ctx,
		"UPDATE inventory_tasks SET status = 'in_progress' WHERE id = $1 AND tenant_id = $2 AND status = 'planned'",
		taskID, tenantIDFromContext(c))

	s.recordAudit(c, "item.scanned", "inventory", "inventory_item", uuid.UUID(itemId), map[string]any{
		"status": req.Status,
	})
	response.OK(c, toAPIInventoryItem(*item))
}

// ImportInventoryItems accepts a JSON batch of items, matches them against
// existing assets by serial_number/asset_tag, and returns match statistics.
// (POST /inventory/tasks/{id}/import)
func (s *APIServer) ImportInventoryItems(c *gin.Context, id IdPath) {
	var req ImportInventoryItemsJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)
	taskID := uuid.UUID(id)
	ctx := c.Request.Context()

	stats := map[string]int{"matched": 0, "discrepancy": 0, "not_found": 0, "total": 0}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		response.InternalError(c, "failed to start transaction")
		return
	}
	defer tx.Rollback(ctx)

	for _, item := range req.Items {
		stats["total"]++
		tag := ""
		serial := ""
		if item.AssetTag != nil {
			tag = *item.AssetTag
		}
		if item.SerialNumber != nil {
			serial = *item.SerialNumber
		}

		asset, err := s.assetSvc.FindBySerialOrTag(ctx, tenantID, serial, tag)

		// Fallback: try property_number
		if (err != nil || asset == nil) && item.PropertyNumber != nil && *item.PropertyNumber != "" {
			row := s.pool.QueryRow(ctx,
				"SELECT id FROM assets WHERE tenant_id = $1 AND property_number = $2 LIMIT 1",
				tenantID, *item.PropertyNumber)
			var assetID uuid.UUID
			if row.Scan(&assetID) == nil {
				a, e := s.assetSvc.GetByID(ctx, tenantID, assetID)
				if e == nil {
					asset = a
					err = nil
				}
			}
		}

		// Fallback: try control_number
		if (err != nil || asset == nil) && item.ControlNumber != nil && *item.ControlNumber != "" {
			row := s.pool.QueryRow(ctx,
				"SELECT id FROM assets WHERE tenant_id = $1 AND control_number = $2 LIMIT 1",
				tenantID, *item.ControlNumber)
			var assetID uuid.UUID
			if row.Scan(&assetID) == nil {
				a, e := s.assetSvc.GetByID(ctx, tenantID, assetID)
				if e == nil {
					asset = a
					err = nil
				}
			}
		}

		// Build expected JSON
		expectedData := map[string]string{}
		if item.AssetTag != nil {
			expectedData["asset_tag"] = *item.AssetTag
		}
		if item.SerialNumber != nil {
			expectedData["serial_number"] = *item.SerialNumber
		}
		if item.ExpectedLocation != nil {
			expectedData["expected_location"] = *item.ExpectedLocation
		}
		if item.PropertyNumber != nil {
			expectedData["property_number"] = *item.PropertyNumber
		}
		if item.ControlNumber != nil {
			expectedData["control_number"] = *item.ControlNumber
		}
		expectedJSON, _ := json.Marshal(expectedData)

		if err != nil || asset == nil {
			stats["not_found"]++
			// Insert as missing item (no asset_id)
			tx.Exec(ctx,
				"INSERT INTO inventory_items (task_id, expected, status) VALUES ($1, $2, 'missing')",
				taskID, expectedJSON)
			continue
		}

		stats["matched"]++
		// Insert matched item
		tx.Exec(ctx,
			"INSERT INTO inventory_items (task_id, asset_id, rack_id, expected, status) VALUES ($1, $2, $3, $4, 'pending')",
			taskID, asset.ID, asset.RackID, expectedJSON)
	}

	// Auto-transition task: planned → in_progress (inside transaction).
	// Tenant-scoped so a cross-tenant task UUID cannot be flipped.
	tx.Exec(ctx,
		"UPDATE inventory_tasks SET status = 'in_progress' WHERE id = $1 AND tenant_id = $2 AND status = 'planned'",
		taskID, tenantID)

	if err := tx.Commit(ctx); err != nil {
		response.InternalError(c, "failed to commit import")
		return
	}

	s.recordAudit(c, "inventory.imported", "inventory", "inventory_task", taskID, map[string]any{
		"matched":   stats["matched"],
		"not_found": stats["not_found"],
		"total":     stats["total"],
	})
	response.OK(c, stats)
}

// GetInventorySummary returns scan progress counts for an inventory task.
// (GET /inventory/tasks/{id}/summary)
func (s *APIServer) GetInventorySummary(c *gin.Context, id IdPath) {
	summary, err := s.inventorySvc.GetSummary(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "inventory task not found")
		return
	}
	response.OK(c, map[string]any{
		"total":       summary.Total,
		"scanned":     summary.Scanned,
		"pending":     summary.Pending,
		"discrepancy": summary.Discrepancy,
	})
}
