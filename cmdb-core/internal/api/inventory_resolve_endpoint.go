package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// ResolveInventoryDiscrepancy handles POST /inventory/tasks/:id/items/:itemId/resolve
// Resolves a discrepancy on an inventory item by applying the given action.
func (s *APIServer) ResolveInventoryDiscrepancy(c *gin.Context, id IdPath, itemId openapi_types.UUID) {
	taskID := uuid.UUID(id)
	itemID := uuid.UUID(itemId)

	var req struct {
		Action string `json:"action" binding:"required"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	var newStatus string
	switch req.Action {
	case "verify", "clear":
		newStatus = "scanned"
	case "add_findings":
		newStatus = "discrepancy"
	case "register":
		newStatus = "scanned"
	default:
		response.BadRequest(c, "invalid action: must be verify, add_findings, register, or clear")
		return
	}

	ctx := c.Request.Context()

	// Update item status
	tag, err := s.pool.Exec(ctx,
		"UPDATE inventory_items SET status = $1, scanned_at = now() WHERE id = $2",
		newStatus, itemID)
	if err != nil {
		response.InternalError(c, "failed to resolve discrepancy")
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "inventory item not found")
		return
	}

	userID := userIDFromContext(c)

	// Create a note for the resolution
	noteText := req.Note
	if noteText == "" {
		noteText = "Resolved via action: " + req.Action
	}
	noteID := uuid.New()
	s.pool.Exec(ctx,
		"INSERT INTO inventory_notes (id, item_id, author_id, severity, text, created_at) VALUES ($1, $2, $3, 'info', $4, now())",
		noteID, itemID, userID, noteText)

	// Create scan history record
	scanID := uuid.New()
	s.pool.Exec(ctx,
		"INSERT INTO inventory_scan_history (id, item_id, scanned_by, method, result, note, scanned_at) VALUES ($1, $2, $3, 'manual', $4, $5, now())",
		scanID, itemID, userID, req.Action, req.Note)

	// Auto-activate task if still planned. Tenant-scoped so a cross-tenant
	// task UUID (or one leaked via an item-ID resolve) cannot flip another
	// tenant's task state.
	s.pool.Exec(ctx,
		"UPDATE inventory_tasks SET status = 'in_progress' WHERE id = $1 AND tenant_id = $2 AND status = 'planned'",
		taskID, tenantIDFromContext(c))

	s.recordAudit(c, "item.discrepancy_resolved", "inventory", "inventory_item", itemID, map[string]any{
		"task_id": taskID.String(),
		"action":  req.Action,
		"status":  newStatus,
	})
	response.OK(c, gin.H{"status": newStatus, "action": req.Action})
}
