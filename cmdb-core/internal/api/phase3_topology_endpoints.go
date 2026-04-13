package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetAssetDependencies handles GET /topology/dependencies?asset_id=
// Returns all dependency edges where the asset is either the source or target.
func (s *APIServer) GetAssetDependencies(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	assetID := c.Query("asset_id")
	if assetID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "asset_id query param required"})
		return
	}

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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query dependencies"})
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

	c.JSON(http.StatusOK, gin.H{"dependencies": deps})
}

// CreateAssetDependency handles POST /topology/dependencies
// Creates a new directed dependency edge between two assets.
func (s *APIServer) CreateAssetDependency(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	var body struct {
		SourceAssetID string `json:"source_asset_id"`
		TargetAssetID string `json:"target_asset_id"`
		DependencyType string `json:"dependency_type"`
		Description   string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if body.SourceAssetID == "" || body.TargetAssetID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_asset_id and target_asset_id are required"})
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
			c.JSON(http.StatusConflict, gin.H{"error": "dependency already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create dependency"})
		return
	}

	depID, _ := uuid.Parse(newID)
	s.recordAudit(c, "dependency.created", "topology", "asset_dependency", depID, map[string]any{
		"source_asset_id": body.SourceAssetID,
		"target_asset_id": body.TargetAssetID,
		"dependency_type": body.DependencyType,
	})
	c.JSON(http.StatusCreated, gin.H{"id": newID})
}

// DeleteAssetDependency handles DELETE /topology/dependencies/:id
// Removes a dependency edge by its ID.
func (s *APIServer) DeleteAssetDependency(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}

	tag, err := s.pool.Exec(c.Request.Context(), `
		DELETE FROM asset_dependencies WHERE id = $1
	`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete dependency"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "dependency not found"})
		return
	}

	depID, _ := uuid.Parse(id)
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
func (s *APIServer) GetTopologyGraph(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	locationID := c.Query("location_id")
	if locationID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "location_id query param required"})
		return
	}

	// Step 1: fetch assets in the location (up to 50)
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
		  AND a.location_id = $2
		LIMIT 50
	`, tenantID, locationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query assets"})
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

	// Step 2: fetch dependencies for those assets
	edges := []gin.H{}
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
			}
		}
	}

	// Step 3: build nodes (metrics = null for now)
	nodes := []gin.H{}
	for _, a := range assets {
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
			"metrics":          nil,
		})
	}

	_ = time.Now() // satisfy time import if needed elsewhere in package

	c.JSON(http.StatusOK, gin.H{
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
