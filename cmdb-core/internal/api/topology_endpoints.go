package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
	assetID := uuid.UUID(*params.AssetId).String()

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT
			ad.id,
			ad.source_asset_id,
			sa.name AS source_asset_name,
			ad.target_asset_id,
			ta.name AS target_asset_name,
			ad.dependency_type,
			COALESCE(ad.description, '') AS description
		FROM asset_dependencies ad
		JOIN assets sa ON ad.source_asset_id = sa.id
		JOIN assets ta ON ad.target_asset_id = ta.id
		WHERE ad.tenant_id = $1
		  AND (ad.source_asset_id = $2 OR ad.target_asset_id = $2)
	`, tenantID, assetID)
	if err != nil {
		response.InternalError(c, "failed to query dependencies")
		return
	}
	defer rows.Close()

	deps := []gin.H{}
	for rows.Next() {
		var id, sourceID, sourceName, targetID, targetName, depType, desc string
		if err := rows.Scan(&id, &sourceID, &sourceName, &targetID, &targetName, &depType, &desc); err != nil {
			continue
		}
		deps = append(deps, gin.H{
			"id":                id,
			"source_asset_id":   sourceID,
			"source_asset_name": sourceName,
			"target_asset_id":   targetID,
			"target_asset_name": targetName,
			"dependency_type":   depType,
			"description":       desc,
		})
	}

	response.OK(c, gin.H{"dependencies": deps})
}

// CreateAssetDependency handles POST /topology/dependencies
// Creates a new directed dependency edge between two assets.
func (s *APIServer) CreateAssetDependency(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	var body struct {
		SourceAssetID  string `json:"source_asset_id"`
		TargetAssetID  string `json:"target_asset_id"`
		DependencyType string `json:"dependency_type"`
		Description    string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if body.SourceAssetID == "" || body.TargetAssetID == "" {
		response.BadRequest(c, "source_asset_id and target_asset_id are required")
		return
	}
	if body.DependencyType == "" {
		body.DependencyType = "depends_on"
	}

	newID := uuid.New().String()
	_, err := s.pool.Exec(c.Request.Context(), `
		INSERT INTO asset_dependencies (id, tenant_id, source_asset_id, target_asset_id, dependency_type, description)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, newID, tenantID, body.SourceAssetID, body.TargetAssetID, body.DependencyType, body.Description)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "unique") {
			response.Err(c, http.StatusConflict, "CONFLICT", "dependency already exists")
			return
		}
		response.InternalError(c, "failed to create dependency")
		return
	}

	depID, _ := uuid.Parse(newID)
	s.recordAudit(c, "dependency.created", "topology", "asset_dependency", depID, map[string]any{
		"source_asset_id": body.SourceAssetID,
		"target_asset_id": body.TargetAssetID,
		"dependency_type": body.DependencyType,
	})
	response.Created(c, gin.H{"id": newID})
}

// DeleteAssetDependency handles DELETE /topology/dependencies/:id
// Removes a dependency edge by its ID.
func (s *APIServer) DeleteAssetDependency(c *gin.Context, id IdPath) {
	depID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	tag, err := s.pool.Exec(c.Request.Context(), `
		DELETE FROM asset_dependencies WHERE id = $1 AND tenant_id = $2
	`, depID, tenantID)
	if err != nil {
		response.InternalError(c, "failed to delete dependency")
		return
	}
	if tag.RowsAffected() == 0 {
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
		depRows, err := s.pool.Query(c.Request.Context(), `
			SELECT id, source_asset_id, target_asset_id, dependency_type
			FROM asset_dependencies
			WHERE source_asset_id = ANY($1) OR target_asset_id = ANY($1)
		`, assetIDs)
		if err == nil {
			defer depRows.Close()
			for depRows.Next() {
				var edgeID, src, tgt, depType string
				if err := depRows.Scan(&edgeID, &src, &tgt, &depType); err != nil {
					continue
				}
				isFaultPath := alertSet[src] || alertSet[tgt]
				edges = append(edges, gin.H{
					"id":              edgeID,
					"source":          src,
					"target":          tgt,
					"dependency_type": depType,
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
