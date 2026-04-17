package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// SyncGetChanges returns incremental changes for a given entity type since a version.
// GET /api/v1/sync/changes?entity_type=assets&since_version=0&limit=100
func (s *APIServer) SyncGetChanges(c *gin.Context, params SyncGetChangesParams) {
	tenantID := tenantIDFromContext(c)
	entityType := string(params.EntityType)

	var sinceVersion int64
	if params.SinceVersion != nil {
		sinceVersion = *params.SinceVersion
	}
	limit := 100
	if params.Limit != nil {
		limit = *params.Limit
	}
	if limit > 1000 {
		limit = 1000
	}

	// oapi-codegen validates EntityType against the spec enum, so the allowlist
	// check is redundant — but keep it as a defense-in-depth guard against future
	// enum drift.
	allowedTables := map[string]bool{
		"assets": true, "locations": true, "racks": true,
		"work_orders": true, "alert_events": true, "inventory_tasks": true,
		"alert_rules": true, "inventory_items": true, "audit_events": true,
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

	// inventory_items: no tenant_id, no deleted_at — join via task to scope by tenant
	if entityType == "inventory_items" {
		query = "SELECT row_to_json(t) AS data, t.sync_version FROM inventory_items t JOIN inventory_tasks it ON t.task_id = it.id WHERE it.tenant_id = $1 AND t.sync_version > $2 ORDER BY t.sync_version LIMIT $3"
	}

	// audit_events: no sync_version, use created_at for incremental pull
	if entityType == "audit_events" {
		query = "SELECT row_to_json(t) AS data, EXTRACT(EPOCH FROM t.created_at)::bigint AS sync_version FROM audit_events t WHERE t.tenant_id = $1 AND t.created_at > to_timestamp($2::bigint) ORDER BY t.created_at LIMIT $3"
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

// SyncResolveConflict resolves a sync conflict and applies the resolution to the entity.
// POST /api/v1/sync/conflicts/:id/resolve
func (s *APIServer) SyncResolveConflict(c *gin.Context, id IdPath) {
	userID := userIDFromContext(c)
	conflictID := uuid.UUID(id)

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

	ctx := c.Request.Context()

	// 1. Read the conflict to get entity info
	var entityType, entityID string
	var remoteDiff json.RawMessage
	err := s.pool.QueryRow(ctx,
		"SELECT entity_type, entity_id, remote_diff FROM sync_conflicts WHERE id = $1 AND resolution = 'pending'",
		conflictID).Scan(&entityType, &entityID, &remoteDiff)
	if err != nil {
		response.NotFound(c, "conflict not found or already resolved")
		return
	}

	// 2. Mark conflict as resolved
	_, err = s.pool.Exec(ctx,
		"UPDATE sync_conflicts SET resolution = $1, resolved_by = $2, resolved_at = now() WHERE id = $3",
		req.Resolution, userID, conflictID)
	if err != nil {
		response.InternalError(c, "failed to resolve conflict")
		return
	}

	// 3. If remote_wins, apply remote_diff to entity
	if req.Resolution == "remote_wins" && remoteDiff != nil {
		var diffMap map[string]interface{}
		if json.Unmarshal(remoteDiff, &diffMap) == nil && len(diffMap) > 0 {
			setClauses := []string{}
			args := []interface{}{}
			argIdx := 1
			for key, val := range diffMap {
				setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, argIdx))
				args = append(args, val)
				argIdx++
			}
			if len(setClauses) > 0 {
				query := fmt.Sprintf("UPDATE %s SET %s, updated_at = now() WHERE id = $%d",
					entityType, strings.Join(setClauses, ", "), argIdx)
				args = append(args, entityID)
				s.pool.Exec(ctx, query, args...)

				// Increment sync_version
				s.pool.Exec(ctx,
					fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $1", entityType),
					entityID)
			}
		}
	}

	s.recordAudit(c, "sync.conflict_resolved", "sync", "sync_conflict", conflictID, map[string]any{
		"resolution":  req.Resolution,
		"entity_type": entityType,
		"entity_id":   entityID,
	})
	c.Status(204)
}

// SyncStats returns per-entity-type max versions and per-node sync gaps.
// GET /api/v1/sync/stats
func (s *APIServer) SyncStats(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	ctx := c.Request.Context()

	syncableTables := []string{"assets", "locations", "racks", "work_orders", "alert_events", "inventory_tasks", "alert_rules", "inventory_items"}

	type nodeGap struct {
		NodeID          string `json:"node_id"`
		LastSyncVersion int64  `json:"last_sync_version"`
		Gap             int64  `json:"gap"`
	}
	type entityStats struct {
		EntityType string    `json:"entity_type"`
		MaxVersion int64     `json:"max_version"`
		Nodes      []nodeGap `json:"nodes"`
	}

	var results []entityStats

	for _, table := range syncableTables {
		var maxVersion int64
		var err error

		switch table {
		case "audit_events":
			err = s.pool.QueryRow(ctx,
				"SELECT COALESCE(MAX(EXTRACT(EPOCH FROM created_at))::bigint, 0) FROM audit_events WHERE tenant_id = $1",
				tenantID).Scan(&maxVersion)
		case "inventory_items":
			err = s.pool.QueryRow(ctx,
				"SELECT COALESCE(MAX(t.sync_version), 0) FROM inventory_items t JOIN inventory_tasks it ON t.task_id = it.id WHERE it.tenant_id = $1",
				tenantID).Scan(&maxVersion)
		default:
			err = s.pool.QueryRow(ctx,
				fmt.Sprintf("SELECT COALESCE(MAX(sync_version), 0) FROM %s WHERE tenant_id = $1", table),
				tenantID).Scan(&maxVersion)
		}
		if err != nil {
			continue
		}

		rows, err := s.pool.Query(ctx,
			"SELECT node_id, last_sync_version FROM sync_state WHERE tenant_id = $1 AND entity_type = $2",
			tenantID, table)
		if err != nil {
			results = append(results, entityStats{EntityType: table, MaxVersion: maxVersion, Nodes: []nodeGap{}})
			continue
		}

		var nodes []nodeGap
		for rows.Next() {
			var ng nodeGap
			if rows.Scan(&ng.NodeID, &ng.LastSyncVersion) == nil {
				ng.Gap = maxVersion - ng.LastSyncVersion
				if ng.Gap < 0 {
					ng.Gap = 0
				}
				nodes = append(nodes, ng)
			}
		}
		rows.Close()
		if nodes == nil {
			nodes = []nodeGap{}
		}

		results = append(results, entityStats{EntityType: table, MaxVersion: maxVersion, Nodes: nodes})
	}

	response.OK(c, results)
}

// SyncSnapshot streams a full snapshot of an entity type for initial sync.
// GET /api/v1/sync/snapshot?entity_type=assets
func (s *APIServer) SyncSnapshot(c *gin.Context, params SyncSnapshotParams) {
	tenantID := tenantIDFromContext(c)
	entityType := string(params.EntityType)

	// oapi-codegen enforces the enum; this allowlist is a defense-in-depth
	// guard against future schema drift.
	allowedTables := map[string]bool{
		"assets": true, "locations": true, "racks": true,
		"work_orders": true, "alert_events": true, "inventory_tasks": true,
		"alert_rules": true, "inventory_items": true, "audit_events": true,
	}
	if !allowedTables[entityType] {
		response.BadRequest(c, "invalid entity_type")
		return
	}

	var query string
	switch entityType {
	case "inventory_items":
		query = "SELECT row_to_json(t) FROM inventory_items t JOIN inventory_tasks it ON t.task_id = it.id WHERE it.tenant_id = $1 ORDER BY t.sync_version"
	case "audit_events":
		query = "SELECT row_to_json(t) FROM audit_events t WHERE t.tenant_id = $1 ORDER BY t.created_at"
	default:
		query = fmt.Sprintf("SELECT row_to_json(t) FROM %s t WHERE t.tenant_id = $1 ORDER BY t.sync_version", entityType)
	}
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
