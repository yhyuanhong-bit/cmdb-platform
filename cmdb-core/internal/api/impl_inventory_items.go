package api

import (
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// GetItemScanHistory handles GET /inventory/tasks/:id/items/:itemId/scan-history
// Returns the scan history for a specific inventory item. Task id is unused —
// item_id alone uniquely identifies the scan history rows.
//
// Migration 000076 added tenant_id to inventory_scan_history. Queries route
// through database.Scope, which prepends the bound tenantID as $1 and refuses
// SQL that doesn't reference tenant_id (audit finding H5, 2026-04-28).
func (s *APIServer) GetItemScanHistory(c *gin.Context, _ IdPath, itemId openapi_types.UUID) {
	itemID := uuid.UUID(itemId)
	tenantID := tenantIDFromContext(c)
	sc := database.Scope(s.pool, tenantID)

	rows, err := sc.Query(c.Request.Context(), `
		SELECT ish.id, ish.scanned_at, u.display_name, ish.method, ish.result, ish.note
		FROM inventory_scan_history ish
		LEFT JOIN users u ON ish.scanned_by = u.id
		WHERE ish.tenant_id = $1 AND ish.item_id = $2
		ORDER BY ish.scanned_at DESC
	`, itemID)
	if err != nil {
		response.InternalError(c, "failed to query scan history")
		return
	}
	defer rows.Close()

	history := []gin.H{}
	for rows.Next() {
		var (
			id          uuid.UUID
			scannedAt   time.Time
			displayName *string
			method      string
			result      string
			note        *string
		)
		if err := rows.Scan(&id, &scannedAt, &displayName, &method, &result, &note); err != nil {
			response.InternalError(c, "failed to scan row")
			return
		}
		history = append(history, gin.H{
			"id":        id.String(),
			"timestamp": scannedAt.UTC().Format(time.RFC3339),
			"operator":  displayName,
			"method":    method,
			"result":    result,
			"note":      note,
		})
	}
	if err := rows.Err(); err != nil {
		response.InternalError(c, "error reading scan history rows")
		return
	}

	response.OK(c, gin.H{"scan_history": history})
}

// CreateItemScanRecord handles POST /inventory/tasks/:id/items/:itemId/scan-history
// Creates a new scan record for a specific inventory item. Task id is unused —
// item_id alone uniquely identifies the target.
func (s *APIServer) CreateItemScanRecord(c *gin.Context, _ IdPath, itemId openapi_types.UUID) {
	itemID := uuid.UUID(itemId)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)

	var body struct {
		Method string  `json:"method" binding:"required"`
		Result string  `json:"result" binding:"required"`
		Note   *string `json:"note"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// Refuse to write a scan history row whose item belongs to another
	// tenant. The SELECT is gated by tenant_id so a cross-tenant item_id
	// returns 0 rows → 404.
	sc := database.Scope(s.pool, tenantID)
	var ownedItemID uuid.UUID
	if err := sc.QueryRow(c.Request.Context(), `
		SELECT ii.id
		FROM inventory_items ii
		JOIN inventory_tasks it ON ii.task_id = it.id
		WHERE ii.id = $2 AND it.tenant_id = $1
	`, itemID).Scan(&ownedItemID); err != nil {
		response.NotFound(c, "inventory item not found")
		return
	}

	newID := uuid.New()
	if _, err := sc.Exec(c.Request.Context(), `
		INSERT INTO inventory_scan_history (id, tenant_id, item_id, scanned_by, method, result, note, scanned_at)
		VALUES ($2, $1, $3, $4, $5, $6, $7, now())
	`, newID, itemID, userID, body.Method, body.Result, body.Note); err != nil {
		response.InternalError(c, "failed to create scan record")
		return
	}

	s.recordAudit(c, "scan_record.created", "inventory", "inventory_scan_history", newID, map[string]any{
		"item_id": itemID.String(),
		"method":  body.Method,
		"result":  body.Result,
	})
	response.Created(c, gin.H{"id": newID.String()})
}

// GetItemNotes handles GET /inventory/tasks/{id}/items/{itemId}/notes
// Returns notes for a specific inventory item. The {id} (task id) is unused
// in the query because item_id alone is unique.
func (s *APIServer) GetItemNotes(c *gin.Context, _ IdPath, itemId openapi_types.UUID) {
	itemID := uuid.UUID(itemId)
	tenantID := tenantIDFromContext(c)
	sc := database.Scope(s.pool, tenantID)

	rows, err := sc.Query(c.Request.Context(), `
		SELECT n.id, n.created_at, u.display_name, n.severity, n.text
		FROM inventory_notes n
		LEFT JOIN users u ON n.author_id = u.id
		WHERE n.tenant_id = $1 AND n.item_id = $2
		ORDER BY n.created_at DESC
	`, itemID)
	if err != nil {
		response.InternalError(c, "failed to query notes")
		return
	}
	defer rows.Close()

	notes := []gin.H{}
	for rows.Next() {
		var (
			id          uuid.UUID
			createdAt   time.Time
			displayName *string
			severity    string
			text        string
		)
		if err := rows.Scan(&id, &createdAt, &displayName, &severity, &text); err != nil {
			response.InternalError(c, "failed to scan row")
			return
		}
		notes = append(notes, gin.H{
			"id":        id.String(),
			"timestamp": createdAt.UTC().Format(time.RFC3339),
			"author":    displayName,
			"severity":  severity,
			"text":      text,
		})
	}
	if err := rows.Err(); err != nil {
		response.InternalError(c, "error reading notes rows")
		return
	}

	response.OK(c, gin.H{"notes": notes})
}

// CreateItemNote handles POST /inventory/tasks/{id}/items/{itemId}/notes
// Creates a new note for a specific inventory item.
func (s *APIServer) CreateItemNote(c *gin.Context, _ IdPath, itemId openapi_types.UUID) {
	itemID := uuid.UUID(itemId)
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)

	var body struct {
		Severity string `json:"severity"`
		Text     string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if body.Severity == "" {
		body.Severity = "info"
	}

	// Mirror CreateItemScanRecord's tenant gate on the parent item.
	sc := database.Scope(s.pool, tenantID)
	var ownedItemID uuid.UUID
	if err := sc.QueryRow(c.Request.Context(), `
		SELECT ii.id
		FROM inventory_items ii
		JOIN inventory_tasks it ON ii.task_id = it.id
		WHERE ii.id = $2 AND it.tenant_id = $1
	`, itemID).Scan(&ownedItemID); err != nil {
		response.NotFound(c, "inventory item not found")
		return
	}

	newID := uuid.New()
	if _, err := sc.Exec(c.Request.Context(), `
		INSERT INTO inventory_notes (id, tenant_id, item_id, author_id, severity, text, created_at)
		VALUES ($2, $1, $3, $4, $5, $6, now())
	`, newID, itemID, userID, body.Severity, body.Text); err != nil {
		response.InternalError(c, "failed to create note")
		return
	}

	s.recordAudit(c, "item_note.created", "inventory", "inventory_note", newID, map[string]any{
		"item_id":  itemID.String(),
		"severity": body.Severity,
	})
	response.Created(c, gin.H{"id": newID.String()})
}
