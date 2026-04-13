package api

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// SyncGetChanges returns incremental changes for a given entity type since a version.
// GET /api/v1/sync/changes?entity_type=assets&since_version=0&limit=100
func (s *APIServer) SyncGetChanges(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	entityType := c.Query("entity_type")
	if entityType == "" {
		response.BadRequest(c, "entity_type is required")
		return
	}

	sinceVersion, _ := strconv.ParseInt(c.DefaultQuery("since_version", "0"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit > 1000 {
		limit = 1000
	}

	allowedTables := map[string]bool{
		"assets": true, "locations": true, "racks": true,
		"work_orders": true, "alert_events": true, "inventory_tasks": true,
		"alert_rules": true,
	}
	if !allowedTables[entityType] {
		response.BadRequest(c, "invalid entity_type")
		return
	}

	query := fmt.Sprintf(
		"SELECT row_to_json(t) AS data, t.sync_version FROM %s t WHERE t.tenant_id = $1 AND t.sync_version > $2 AND t.deleted_at IS NULL ORDER BY t.sync_version LIMIT $3",
		entityType)

	// alert_rules and alert_events don't have deleted_at
	if entityType == "alert_rules" || entityType == "alert_events" {
		query = fmt.Sprintf(
			"SELECT row_to_json(t) AS data, t.sync_version FROM %s t WHERE t.tenant_id = $1 AND t.sync_version > $2 ORDER BY t.sync_version LIMIT $3",
			entityType)
	}

	rows, err := s.pool.Query(c.Request.Context(), query, tenantID, sinceVersion, limit+1)
	if err != nil {
		response.InternalError(c, "failed to query changes")
		return
	}
	defer rows.Close()

	var items []json.RawMessage
	var lastVersion int64
	count := 0
	for rows.Next() {
		var data json.RawMessage
		var version int64
		if rows.Scan(&data, &version) == nil {
			count++
			if count <= limit {
				items = append(items, data)
				lastVersion = version
			}
		}
	}
	if items == nil {
		items = []json.RawMessage{}
	}

	response.OK(c, gin.H{
		"changes":        items,
		"has_more":       count > limit,
		"latest_version": lastVersion,
	})
}

// SyncGetState returns sync state for all entity types.
// GET /api/v1/sync/state
func (s *APIServer) SyncGetState(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.pool.Query(c.Request.Context(),
		"SELECT node_id, entity_type, last_sync_version, last_sync_at, status, error_message FROM sync_state WHERE tenant_id = $1 ORDER BY node_id, entity_type",
		tenantID)
	if err != nil {
		response.InternalError(c, "failed to query sync state")
		return
	}
	defer rows.Close()

	var items []gin.H
	for rows.Next() {
		var nodeID, entityType, status string
		var version int64
		var lastSync, errMsg interface{}
		if rows.Scan(&nodeID, &entityType, &version, &lastSync, &status, &errMsg) == nil {
			items = append(items, gin.H{
				"node_id": nodeID, "entity_type": entityType,
				"last_sync_version": version, "last_sync_at": lastSync,
				"status": status, "error_message": errMsg,
			})
		}
	}
	if items == nil {
		items = []gin.H{}
	}
	response.OK(c, items)
}

// SyncGetConflicts returns pending sync conflicts.
// GET /api/v1/sync/conflicts
func (s *APIServer) SyncGetConflicts(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.pool.Query(c.Request.Context(),
		"SELECT id, entity_type, entity_id, local_version, remote_version, local_diff, remote_diff, created_at FROM sync_conflicts WHERE tenant_id = $1 AND resolution = 'pending' ORDER BY created_at",
		tenantID)
	if err != nil {
		response.InternalError(c, "failed to query conflicts")
		return
	}
	defer rows.Close()

	var items []gin.H
	for rows.Next() {
		var id, entityID uuid.UUID
		var entityType string
		var localV, remoteV int64
		var localDiff, remoteDiff, createdAt interface{}
		if rows.Scan(&id, &entityType, &entityID, &localV, &remoteV, &localDiff, &remoteDiff, &createdAt) == nil {
			items = append(items, gin.H{
				"id": id, "entity_type": entityType, "entity_id": entityID,
				"local_version": localV, "remote_version": remoteV,
				"local_diff": localDiff, "remote_diff": remoteDiff,
				"created_at": createdAt,
			})
		}
	}
	if items == nil {
		items = []gin.H{}
	}
	response.OK(c, items)
}

// SyncResolveConflict resolves a sync conflict.
// POST /api/v1/sync/conflicts/:id/resolve
func (s *APIServer) SyncResolveConflict(c *gin.Context) {
	userID := userIDFromContext(c)
	conflictID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid conflict ID")
		return
	}

	var req struct {
		Resolution string `json:"resolution" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "resolution is required (local_wins or remote_wins)")
		return
	}
	if req.Resolution != "local_wins" && req.Resolution != "remote_wins" {
		response.BadRequest(c, "resolution must be local_wins or remote_wins")
		return
	}

	_, err = s.pool.Exec(c.Request.Context(),
		"UPDATE sync_conflicts SET resolution = $1, resolved_by = $2, resolved_at = now() WHERE id = $3",
		req.Resolution, userID, conflictID)
	if err != nil {
		response.InternalError(c, "failed to resolve conflict")
		return
	}

	s.recordAudit(c, "sync.conflict_resolved", "sync", "sync_conflict", conflictID, map[string]any{
		"resolution": req.Resolution,
	})
	c.Status(204)
}

// SyncSnapshot streams a full snapshot of an entity type for initial sync.
// GET /api/v1/sync/snapshot?entity_type=assets
func (s *APIServer) SyncSnapshot(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	entityType := c.Query("entity_type")

	allowedTables := map[string]bool{
		"assets": true, "locations": true, "racks": true,
		"work_orders": true, "alert_events": true, "inventory_tasks": true,
		"alert_rules": true,
	}
	if !allowedTables[entityType] {
		response.BadRequest(c, "invalid entity_type")
		return
	}

	query := fmt.Sprintf("SELECT row_to_json(t) FROM %s t WHERE tenant_id = $1 ORDER BY sync_version", entityType)
	rows, err := s.pool.Query(c.Request.Context(), query, tenantID)
	if err != nil {
		response.InternalError(c, "failed to query snapshot")
		return
	}
	defer rows.Close()

	c.Header("Content-Type", "application/x-ndjson")
	c.Status(200)

	for rows.Next() {
		var jsonRow []byte
		if rows.Scan(&jsonRow) == nil {
			c.Writer.Write(jsonRow)
			c.Writer.Write([]byte("\n"))
		}
	}
}
