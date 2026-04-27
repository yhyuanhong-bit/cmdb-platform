package api

//tenantlint:allow-direct-pool — SyncResolveConflict builds dynamic SQL across tenant/non-tenant tables; scoped via conflict ownership check

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// allowedResolveColumns is the per-entity_type whitelist of columns that a
// sync_conflicts.remote_diff JSON blob is allowed to write when resolving
// with remote_wins. Any JSON key outside this set is rejected with
// INVALID_FIELD — this is the only defense against attacker-controlled
// column names being concatenated into UPDATE statements.
//
// System-owned columns (id, tenant_id, created_at, updated_at,
// sync_version) are intentionally excluded: the handler sets updated_at
// itself and bumps sync_version separately, and rewriting tenant_id or id
// would be a tenancy-break or identity-swap.
var allowedResolveColumns = map[string]map[string]bool{
	"assets": {
		"name":            true,
		"type":            true,
		"sub_type":        true,
		"status":          true,
		"bia_level":       true,
		"location_id":     true,
		"rack_id":         true,
		"vendor":          true,
		"model":           true,
		"serial_number":   true,
		"asset_tag":       true,
		"property_number": true,
		"control_number":  true,
		"attributes":      true,
		"tags":            true,
	},
	"locations": {
		"name":       true,
		"name_en":    true,
		"slug":       true,
		"level":      true,
		"parent_id":  true,
		"status":     true,
		"metadata":   true,
		"sort_order": true,
	},
	"racks": {
		"name":              true,
		"row_label":         true,
		"total_u":           true,
		"power_capacity_kw": true,
		"status":            true,
		"tags":              true,
		"location_id":       true,
	},
	"work_orders": {
		"title":             true,
		"type":              true,
		"status":            true,
		"priority":          true,
		"description":       true,
		"reason":            true,
		"location_id":       true,
		"asset_id":          true,
		"requestor_id":      true,
		"assignee_id":       true,
		"scheduled_start":   true,
		"scheduled_end":     true,
		"actual_start":      true,
		"actual_end":        true,
		"execution_status":  true,
		"governance_status": true,
	},
	"alert_events": {
		"status":        true,
		"severity":      true,
		"message":       true,
		"trigger_value": true,
		"acked_at":      true,
		"resolved_at":   true,
	},
	"alert_rules": {
		"name":        true,
		"metric_name": true,
		"condition":   true,
		"severity":    true,
		"enabled":     true,
	},
	"inventory_tasks": {
		"name":              true,
		"code":              true,
		"scope_location_id": true,
		"status":            true,
		"method":            true,
		"planned_date":      true,
		"completed_date":    true,
		"assigned_to":       true,
	},
	"inventory_items": {
		"actual":     true,
		"status":     true,
		"scanned_at": true,
		"scanned_by": true,
	},
}

// tablesWithUpdatedAt tracks which syncable tables carry an `updated_at`
// column. Only those get `updated_at = now()` appended to the SET clause.
// alert_events, alert_rules, inventory_tasks, and inventory_items have no
// such column in the current schema (see db/migrations/000006, 000007).
var tablesWithUpdatedAt = map[string]bool{
	"assets":      true,
	"locations":   true,
	"racks":       true, // added in migration 000037
	"work_orders": true,
}

// tablesWithoutTenantID names entity tables that do not carry a tenant_id
// column directly. For these, we skip the `AND tenant_id = $N` guard on the
// follow-up entity UPDATE — the preceding sync_conflicts SELECT already
// confirmed the conflict belongs to the caller's tenant, which transitively
// authorizes mutating the referenced entity row. inventory_items is scoped
// to a tenant only via its parent inventory_tasks.task_id relationship.
var tablesWithoutTenantID = map[string]bool{
	"inventory_items": true,
}

// validateResolveColumns ensures every key in diff is a column we're
// willing to write on the given entityType. The returned error embeds the
// INVALID_FIELD marker so the HTTP layer can surface a machine-readable
// code to the client.
func validateResolveColumns(entityType string, diff map[string]any) error {
	allowed, ok := allowedResolveColumns[entityType]
	if !ok {
		return fmt.Errorf("INVALID_FIELD: entity_type %q is not resolvable", entityType)
	}
	for key := range diff {
		if !allowed[key] {
			return fmt.Errorf("INVALID_FIELD: field %q is not resolvable for entity_type %q", key, entityType)
		}
	}
	return nil
}

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

	sc := database.Scope(s.pool, tenantID)
	rows, err := sc.Query(c.Request.Context(), query, sinceVersion, limit+1)
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
	rows, err := dbgen.New(s.pool).ListSyncStatesByTenant(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to query sync state")
		return
	}

	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		items = append(items, gin.H{
			"node_id":           r.NodeID,
			"entity_type":       r.EntityType,
			"last_sync_version": nullableInt8(r.LastSyncVersion),
			"last_sync_at":      nullableTimestamp(r.LastSyncAt),
			"status":            nullableText(r.Status),
			"error_message":     nullableText(r.ErrorMessage),
		})
	}
	response.OK(c, items)
}

// nullableInt8 converts a pgtype.Int8 to a *int64 for JSON encoding.
// Callers that previously scanned into plain int64 got 0 on NULL; we preserve
// that shape by returning 0 when invalid.
func nullableInt8(v pgtype.Int8) int64 {
	if !v.Valid {
		return 0
	}
	return v.Int64
}

// nullableTimestamp converts a pgtype.Timestamptz to a JSON-friendly value.
// Returns nil when NULL so the JSON emits `null` (matching prior behavior,
// where the interface{} scan held a nil).
func nullableTimestamp(v pgtype.Timestamptz) interface{} {
	if !v.Valid {
		return nil
	}
	return v.Time
}

// nullableText converts a pgtype.Text to a string or nil.
func nullableText(v pgtype.Text) interface{} {
	if !v.Valid {
		return nil
	}
	return v.String
}

// SyncGetConflicts returns pending sync conflicts.
// GET /api/v1/sync/conflicts
//
// sync_conflicts is a manual-intervention channel — rows are filed by
// operators, not by the sync agent. See SyncResolveConflict and
// docs/SYNC_CONFLICT.md for the full policy.
func (s *APIServer) SyncGetConflicts(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := dbgen.New(s.pool).ListPendingSyncConflicts(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to query conflicts")
		return
	}

	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		items = append(items, gin.H{
			"id":             r.ID,
			"entity_type":    r.EntityType,
			"entity_id":      r.EntityID,
			"local_version":  r.LocalVersion,
			"remote_version": r.RemoteVersion,
			"local_diff":     r.LocalDiff,
			"remote_diff":    r.RemoteDiff,
			"created_at":     nullableTimestamp(r.CreatedAt),
		})
	}
	response.OK(c, items)
}

// SyncResolveConflict resolves a sync conflict and applies the resolution to the entity.
// POST /api/v1/sync/conflicts/:id/resolve
//
// Conflict lifecycle (IMPORTANT):
//
// This handler is the resolution half of a MANUAL-INTERVENTION channel.
// Nothing in the sync pipeline inserts rows into sync_conflicts automatically
// — the default conflict strategy for automatic sync is last-write-wins
// (see internal/domain/sync/agent.go and docs/SYNC_CONFLICT.md). Rows land
// in sync_conflicts only when an operator explicitly files one via admin
// tooling / a support workflow to arbitrate a human-reported dispute.
//
// That is why there is no auto-detection counterpart to this handler: a
// proper version-based detection scheme would require every write path to
// stamp a version/hash and diff it against the incoming envelope, which is
// non-trivial and not yet designed. Until that design lands, sync_conflicts
// is the operator escape hatch for edge-case divergence.
//
// Security notes:
//   - The SELECT and UPDATE on sync_conflicts are scoped by tenant_id, so a
//     user cannot read or resolve a conflict belonging to another tenant.
//   - remote_diff keys are validated against a per-entity column whitelist
//     (allowedResolveColumns) before any UPDATE is built. Column names are
//     further sanitized via pgx.Identifier{}.Sanitize(); values always flow
//     through positional placeholders.
func (s *APIServer) SyncResolveConflict(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
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
	q := dbgen.New(s.pool)

	// 1. Read the conflict to get entity info. Scoped by tenant_id — a
	//    conflict owned by another tenant must surface as a 404.
	conflict, err := q.GetPendingSyncConflict(ctx, dbgen.GetPendingSyncConflictParams{
		ID:       conflictID,
		TenantID: tenantID,
	})
	if err != nil {
		response.NotFound(c, "conflict not found or already resolved")
		return
	}
	entityType := conflict.EntityType
	entityID := conflict.EntityID.String()
	remoteDiff := conflict.RemoteDiff

	// 2. If remote_wins, validate the diff keys BEFORE marking the
	//    conflict resolved. If validation fails, the conflict stays
	//    pending so an operator can inspect it.
	var diffMap map[string]interface{}
	if req.Resolution == "remote_wins" && remoteDiff != nil {
		if err := json.Unmarshal(remoteDiff, &diffMap); err != nil {
			response.Err(c, http.StatusBadRequest, "INVALID_FIELD",
				"remote_diff is not a valid JSON object")
			return
		}
		if err := validateResolveColumns(entityType, diffMap); err != nil {
			response.Err(c, http.StatusBadRequest, "INVALID_FIELD", err.Error())
			return
		}
	}

	// 3. Mark conflict as resolved. Scoped by tenant_id as a second guard.
	if err := q.ResolveSyncConflict(ctx, dbgen.ResolveSyncConflictParams{
		Resolution: pgtype.Text{String: req.Resolution, Valid: true},
		ResolvedBy: pgtype.UUID{Bytes: userID, Valid: true},
		ID:         conflictID,
		TenantID:   tenantID,
	}); err != nil {
		response.InternalError(c, "failed to resolve conflict")
		return
	}

	// 4. If remote_wins, apply remote_diff to the entity. Column names are
	//    whitelisted (step 2) and further sanitized via pgx.Identifier;
	//    values are bound through positional placeholders.
	if req.Resolution == "remote_wins" && len(diffMap) > 0 {
		setClauses := make([]string, 0, len(diffMap))
		args := make([]interface{}, 0, len(diffMap)+1)
		argIdx := 1
		for key, val := range diffMap {
			colIdent := pgx.Identifier{key}.Sanitize()
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", colIdent, argIdx))
			args = append(args, val)
			argIdx++
		}
		if tablesWithUpdatedAt[entityType] {
			setClauses = append(setClauses, "updated_at = now()")
		}
		tableIdent := pgx.Identifier{entityType}.Sanitize()
		var query, versionQuery string
		if tablesWithoutTenantID[entityType] {
			query = fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d",
				tableIdent, strings.Join(setClauses, ", "), argIdx)
			args = append(args, entityID)
			versionQuery = fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $1", tableIdent)
		} else {
			query = fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d AND tenant_id = $%d",
				tableIdent, strings.Join(setClauses, ", "), argIdx, argIdx+1)
			args = append(args, entityID, tenantID)
			versionQuery = fmt.Sprintf("UPDATE %s SET sync_version = sync_version + 1 WHERE id = $1 AND tenant_id = $2", tableIdent)
		}
		if _, err := s.pool.Exec(ctx, query, args...); err != nil {
			response.InternalError(c, "failed to apply remote diff")
			return
		}

		// Increment sync_version separately; tenant-scoped where applicable.
		// A failed sync_version bump leaves the row's version behind its
		// actual data, so downstream pullers will repeatedly re-apply the
		// remote diff — idempotent but wasteful, and a persistent pattern
		// means conflict resolution is stuck. Warn so it surfaces.
		if tablesWithoutTenantID[entityType] {
			if _, err := s.pool.Exec(ctx, versionQuery, entityID); err != nil {
				zap.L().Warn("sync resolve: version bump failed",
					zap.String("entity_type", entityType),
					zap.String("entity_id", fmt.Sprint(entityID)),
					zap.Error(err))
			}
		} else {
			if _, err := s.pool.Exec(ctx, versionQuery, entityID, tenantID); err != nil {
				zap.L().Warn("sync resolve: version bump failed",
					zap.String("entity_type", entityType),
					zap.String("entity_id", fmt.Sprint(entityID)),
					zap.Error(err))
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
	sc := database.Scope(s.pool, tenantID)

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
			err = sc.QueryRow(ctx,
				"SELECT COALESCE(MAX(EXTRACT(EPOCH FROM created_at))::bigint, 0) FROM audit_events WHERE tenant_id = $1",
			).Scan(&maxVersion)
		case "inventory_items":
			err = sc.QueryRow(ctx,
				"SELECT COALESCE(MAX(t.sync_version), 0) FROM inventory_items t JOIN inventory_tasks it ON t.task_id = it.id WHERE it.tenant_id = $1",
			).Scan(&maxVersion)
		default:
			err = sc.QueryRow(ctx,
				fmt.Sprintf("SELECT COALESCE(MAX(sync_version), 0) FROM %s WHERE tenant_id = $1", table),
			).Scan(&maxVersion)
		}
		if err != nil {
			continue
		}

		stateRows, err := dbgen.New(s.pool).ListSyncStatesByTenantEntity(ctx, dbgen.ListSyncStatesByTenantEntityParams{
			TenantID:   tenantID,
			EntityType: table,
		})
		if err != nil {
			results = append(results, entityStats{EntityType: table, MaxVersion: maxVersion, Nodes: []nodeGap{}})
			continue
		}

		nodes := make([]nodeGap, 0, len(stateRows))
		for _, sr := range stateRows {
			last := nullableInt8(sr.LastSyncVersion)
			gap := maxVersion - last
			if gap < 0 {
				gap = 0
			}
			nodes = append(nodes, nodeGap{NodeID: sr.NodeID, LastSyncVersion: last, Gap: gap})
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
		// entityType already passed the allowedTables map above; pgx.Identifier
		// is a defence-in-depth quote+escape so a future allow-list edit can't
		// silently introduce SQL injection.
		query = fmt.Sprintf("SELECT row_to_json(t) FROM %s t WHERE t.tenant_id = $1 ORDER BY t.sync_version", pgx.Identifier{entityType}.Sanitize())
	}
	sc2 := database.Scope(s.pool, tenantID)
	rows, err := sc2.Query(c.Request.Context(), query)
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
