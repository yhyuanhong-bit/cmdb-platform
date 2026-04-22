package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/topology"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ListAssetDependencies handles GET /topology/dependencies?asset_id=
// Returns all dependency edges where the asset is either the source or target.
// asset_id is optional in the OpenAPI spec but required here — the handler
// always scopes to one asset.
func (s *APIServer) ListAssetDependencies(c *gin.Context, params ListAssetDependenciesParams) {
	tenantID := tenantIDFromContext(c)
	if params.AssetId == nil {
		response.BadRequest(c, "asset_id query param required")
		return
	}
	assetID := uuid.UUID(*params.AssetId)

	rows, err := dbgen.New(s.pool).ListAssetDependencies(c.Request.Context(), dbgen.ListAssetDependenciesParams{
		TenantID:      tenantID,
		SourceAssetID: assetID,
	})
	if err != nil {
		response.InternalError(c, "failed to query dependencies")
		return
	}

	deps := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		deps = append(deps, gin.H{
			"id":                  r.ID.String(),
			"source_asset_id":     r.SourceAssetID.String(),
			"source_asset_name":   r.SourceAssetName,
			"target_asset_id":     r.TargetAssetID.String(),
			"target_asset_name":   r.TargetAssetName,
			"dependency_type":     r.DependencyType,
			"dependency_category": string(r.DependencyCategory),
			"description":         r.Description,
		})
	}

	response.OK(c, gin.H{"dependencies": deps})
}

// CreateAssetDependency handles POST /topology/dependencies.
// Creates a directed dependency edge between two assets.
//
// dependency_category (migration 000054) is optional in the request; when
// absent we default to "dependency" to match the DB column default and
// pre-category handler behavior. dependency_type defaults to "depends_on"
// so pre-migration clients keep working; any caller that supplies a
// dependency_type but not a category gets "dependency" — this is the
// conservative bucket for legacy verbs like 'depends_on' / 'requires'.
func (s *APIServer) CreateAssetDependency(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	var body CreateAssetDependencyRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	srcUUID := uuid.UUID(body.SourceAssetId)
	tgtUUID := uuid.UUID(body.TargetAssetId)
	if srcUUID == uuid.Nil || tgtUUID == uuid.Nil {
		response.BadRequest(c, "source_asset_id and target_asset_id are required")
		return
	}

	depType := "depends_on"
	if body.DependencyType != nil && *body.DependencyType != "" {
		depType = *body.DependencyType
	}

	category := Dependency
	if body.DependencyCategory != nil {
		if !body.DependencyCategory.Valid() {
			response.BadRequest(c, "dependency_category must be containment, dependency, communication, or custom")
			return
		}
		category = *body.DependencyCategory
	}

	description := ""
	if body.Description != nil {
		description = *body.Description
	}

	newID := uuid.New()
	err := s.topologySvc.CreateDependency(c.Request.Context(), topology.CreateDependencyParams{
		ID:             newID,
		TenantID:       tenantID,
		SourceAssetID:  srcUUID,
		TargetAssetID:  tgtUUID,
		DependencyType: depType,
		Category:       string(category),
		Description:    description,
	})
	if err != nil {
		switch {
		case errors.Is(err, topology.ErrSelfDependency):
			response.Err(c, http.StatusConflict, "dependency_self", "asset cannot depend on itself")
			return
		case errors.Is(err, topology.ErrCycleDetected):
			response.Err(c, http.StatusConflict, "dependency_cycle", "adding this edge would create a dependency cycle")
			return
		case errors.Is(err, topology.ErrDependencyExists):
			response.Err(c, http.StatusConflict, "CONFLICT", "dependency already exists")
			return
		case errors.Is(err, topology.ErrInvalidCategory):
			response.BadRequest(c, "dependency_category must be containment, dependency, communication, or custom")
			return
		}
		response.InternalError(c, "failed to create dependency")
		return
	}

	s.recordAudit(c, "dependency.created", "topology", "asset_dependency", newID, map[string]any{
		"source_asset_id":     srcUUID.String(),
		"target_asset_id":     tgtUUID.String(),
		"dependency_type":     depType,
		"dependency_category": string(category),
	})
	response.Created(c, gin.H{"id": newID.String()})
}

// DeleteAssetDependency handles DELETE /topology/dependencies/:id
// Removes a dependency edge by its ID.
func (s *APIServer) DeleteAssetDependency(c *gin.Context, id IdPath) {
	depID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	rowsAffected, err := dbgen.New(s.pool).DeleteAssetDependency(c.Request.Context(), dbgen.DeleteAssetDependencyParams{
		ID:       depID,
		TenantID: tenantID,
	})
	if err != nil {
		response.InternalError(c, "failed to delete dependency")
		return
	}
	if rowsAffected == 0 {
		response.NotFound(c, "dependency not found")
		return
	}

	s.recordAudit(c, "dependency.deleted", "topology", "asset_dependency", depID, nil)
	c.Status(http.StatusNoContent)
}

// topologyAsset holds node data for GetTopologyGraph.
type topologyAsset struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	SubType        string   `json:"sub_type"`
	Status         string   `json:"status"`
	BIALevel       string   `json:"bia_level"`
	IPAddress      string   `json:"ip_address"`
	Model          string   `json:"model"`
	RackName       string   `json:"rack_name"`
	Tags           []string `json:"tags"`
	HasActiveAlert bool     `json:"has_active_alert"`
}

// GetTopologyGraph handles GET /topology/graph?location_id=
// Returns a graph of asset nodes and dependency edges for a given location.
// location_id is optional in the OpenAPI spec but required here — the graph
// is always scoped to one location.
func (s *APIServer) GetTopologyGraph(c *gin.Context, params GetTopologyGraphParams) {
	tenantID := tenantIDFromContext(c)
	if params.LocationId == nil {
		response.BadRequest(c, "location_id query param required")
		return
	}
	locationID := uuid.UUID(*params.LocationId).String()

	// Step 1: fetch assets in the location and its descendants (up to 200)
	assetRows, err := s.pool.Query(c.Request.Context(), `
		SELECT
			a.id,
			a.name,
			a.type,
			COALESCE(a.sub_type, '')  AS sub_type,
			a.status,
			COALESCE(a.bia_level, '') AS bia_level,
			COALESCE(a.ip_address, '') AS ip_address,
			COALESCE(a.model, '')     AS model,
			COALESCE(r.name, '')      AS rack_name,
			COALESCE(a.tags, '{}')    AS tags,
			EXISTS(
				SELECT 1 FROM alert_events ae
				WHERE ae.asset_id = a.id AND ae.status = 'firing'
			) AS has_active_alert
		FROM assets a
		LEFT JOIN racks r ON a.rack_id = r.id
		WHERE a.tenant_id = $1
		  AND a.deleted_at IS NULL
		  AND a.location_id IN (
			SELECT id FROM locations
			WHERE tenant_id = $1 AND path <@ (SELECT path FROM locations WHERE id = $2)
		  )
		ORDER BY a.name
		LIMIT 200
	`, tenantID, locationID)
	if err != nil {
		response.InternalError(c, "failed to query assets")
		return
	}
	defer assetRows.Close()

	assets := []topologyAsset{}
	assetIDs := []string{}
	alertSet := map[string]bool{}

	for assetRows.Next() {
		var a topologyAsset
		var tags []string
		if err := assetRows.Scan(
			&a.ID, &a.Name, &a.Type, &a.SubType, &a.Status,
			&a.BIALevel, &a.IPAddress, &a.Model, &a.RackName,
			&tags, &a.HasActiveAlert,
		); err != nil {
			continue
		}
		if tags == nil {
			tags = []string{}
		}
		a.Tags = tags
		assets = append(assets, a)
		assetIDs = append(assetIDs, a.ID)
		if a.HasActiveAlert {
			alertSet[a.ID] = true
		}
	}

	// Step 2: fetch dependencies and include external nodes
	edges := []gin.H{}
	assetIDSet := map[string]bool{}
	for _, id := range assetIDs {
		assetIDSet[id] = true
	}
	externalIDs := []string{}

	if len(assetIDs) > 0 {
		assetUUIDs := make([]uuid.UUID, 0, len(assetIDs))
		for _, idStr := range assetIDs {
			if u, perr := uuid.Parse(idStr); perr == nil {
				assetUUIDs = append(assetUUIDs, u)
			}
		}
		depRows, err := dbgen.New(s.pool).ListAssetDependenciesByAssetIDs(c.Request.Context(), assetUUIDs)
		if err == nil {
			for _, d := range depRows {
				src := d.SourceAssetID.String()
				tgt := d.TargetAssetID.String()
				isFaultPath := alertSet[src] || alertSet[tgt]
				edges = append(edges, gin.H{
					"id":              d.ID.String(),
					"source":          src,
					"target":          tgt,
					"dependency_type": d.DependencyType,
					"is_fault_path":   isFaultPath,
				})
				// Track external asset IDs (not in this location's asset set)
				if !assetIDSet[src] {
					externalIDs = append(externalIDs, src)
				}
				if !assetIDSet[tgt] {
					externalIDs = append(externalIDs, tgt)
				}
			}
		}
	}

	// Fetch external assets that participate in cross-location dependencies
	if len(externalIDs) > 0 {
		extRows, err := s.pool.Query(c.Request.Context(), `
			SELECT a.id, a.name, a.type, COALESCE(a.sub_type,''), a.status,
			       COALESCE(a.bia_level,''), COALESCE(a.ip_address,''),
			       COALESCE(a.model,''), COALESCE(r.name,''), COALESCE(a.tags,'{}'),
			       EXISTS(SELECT 1 FROM alert_events ae WHERE ae.asset_id=a.id AND ae.status='firing')
			FROM assets a LEFT JOIN racks r ON a.rack_id=r.id
			WHERE a.id = ANY($1) AND a.deleted_at IS NULL
		`, externalIDs)
		if err == nil {
			defer extRows.Close()
			for extRows.Next() {
				var a topologyAsset
				var tags []string
				if err := extRows.Scan(&a.ID, &a.Name, &a.Type, &a.SubType, &a.Status,
					&a.BIALevel, &a.IPAddress, &a.Model, &a.RackName, &tags, &a.HasActiveAlert); err != nil {
					continue
				}
				if tags == nil {
					tags = []string{}
				}
				a.Tags = tags
				assets = append(assets, a)
				assetIDs = append(assetIDs, a.ID)
			}
		}
	}

	// Step 3: batch-fetch latest metrics for all assets (cpu, memory, disk)
	metricsMap := map[string]gin.H{} // asset_id → { cpu, memory, disk_io }
	if len(assetIDs) > 0 {
		metricRows, err := s.pool.Query(c.Request.Context(), `
			SELECT asset_id::text, name, COALESCE(avg(value), 0) AS avg_val
			FROM metrics
			WHERE asset_id = ANY($1)
			  AND name IN ('cpu_usage', 'memory_usage', 'disk_usage')
			  AND time > now() - interval '1 hour'
			GROUP BY asset_id, name
		`, assetIDs)
		if err == nil {
			defer metricRows.Close()
			for metricRows.Next() {
				var aid, mname string
				var avgVal float64
				if err := metricRows.Scan(&aid, &mname, &avgVal); err != nil {
					continue
				}
				if _, ok := metricsMap[aid]; !ok {
					metricsMap[aid] = gin.H{"cpu": 0.0, "memory": 0.0, "disk_io": 0.0}
				}
				switch mname {
				case "cpu_usage":
					metricsMap[aid]["cpu"] = avgVal
				case "memory_usage":
					metricsMap[aid]["memory"] = avgVal
				case "disk_usage":
					metricsMap[aid]["disk_io"] = avgVal
				}
			}
		}
	}

	// Build node list with metrics
	nodes := []gin.H{}
	for _, a := range assets {
		m := metricsMap[a.ID]
		if m == nil {
			m = gin.H{"cpu": 0.0, "memory": 0.0, "disk_io": 0.0}
		}
		nodes = append(nodes, gin.H{
			"id":               a.ID,
			"name":             a.Name,
			"type":             a.Type,
			"sub_type":         a.SubType,
			"status":           a.Status,
			"bia_level":        a.BIALevel,
			"ip_address":       a.IPAddress,
			"model":            a.Model,
			"rack_name":        a.RackName,
			"tags":             a.Tags,
			"has_active_alert": a.HasActiveAlert,
			"metrics":          m,
		})
	}

	response.OK(c, gin.H{
		"nodes": nodes,
		"edges": edges,
	})
}

// formatVlans converts a []int32 slice to a comma-separated string.
func formatVlans(vlans []int32) string {
	parts := make([]string, len(vlans))
	for i, v := range vlans {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, ",")
}

// defaultImpactMaxDepth matches api/openapi.yaml. Duplicated here so the
// handler's default does not silently drift from the schema's default.
const defaultImpactMaxDepth = 5

// GetTopologyImpact handles GET /topology/impact?root_asset_id=...&max_depth=...&direction=...
//
// Multi-hop transitive impact analysis. Cycle-safe via the recursive
// CTE's path accumulator (see db/queries/asset_dependencies.sql).
// tenant_id is enforced at every hop inside the CTE, not just here,
// so a caller who forges a root_asset_id from another tenant gets an
// empty result rather than leaked edges.
func (s *APIServer) GetTopologyImpact(c *gin.Context, params GetTopologyImpactParams) {
	tenantID := tenantIDFromContext(c)
	rootID := uuid.UUID(params.RootAssetId)

	maxDepth := defaultImpactMaxDepth
	if params.MaxDepth != nil {
		maxDepth = *params.MaxDepth
	}

	direction := topology.ImpactDirectionDownstream
	if params.Direction != nil {
		direction = topology.ImpactDirection(*params.Direction)
	}

	edges, err := s.topologySvc.GetImpactPath(c.Request.Context(), tenantID, rootID, maxDepth, direction)
	if err != nil {
		// Validation errors from the service surface as 400; anything
		// else is treated as an internal failure. The validation layer
		// returns messages safe to echo to clients.
		if strings.HasPrefix(err.Error(), "max_depth must be") ||
			strings.HasPrefix(err.Error(), "direction must be") {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to compute impact graph")
		return
	}

	apiEdges := make([]ImpactEdge, 0, len(edges))
	for _, e := range edges {
		path := make([]openapi_types.UUID, len(e.Path))
		for i, p := range e.Path {
			path[i] = openapi_types.UUID(p)
		}
		apiEdges = append(apiEdges, ImpactEdge{
			Id:                 openapi_types.UUID(e.ID),
			SourceAssetId:      openapi_types.UUID(e.SourceAssetID),
			SourceAssetName:    e.SourceAssetName,
			TargetAssetId:      openapi_types.UUID(e.TargetAssetID),
			TargetAssetName:    e.TargetAssetName,
			DependencyType:     e.DependencyType,
			DependencyCategory: DependencyCategory(e.DependencyCategory),
			Depth:              e.Depth,
			Path:               path,
			Direction:          ImpactEdgeDirection(e.Direction),
		})
	}

	response.OK(c, TopologyImpactResponse{
		RootAssetId: openapi_types.UUID(rootID),
		Direction:   TopologyImpactResponseDirection(direction),
		MaxDepth:    maxDepth,
		Edges:       apiEdges,
	})
}
