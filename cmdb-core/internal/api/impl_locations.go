package api

import (
	"encoding/json"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Location endpoints
// ---------------------------------------------------------------------------

// ListLocations returns root locations, or looks up a location by slug+level.
// Supports ?all=true to return all locations (flat list for tree building).
// (GET /locations)
func (s *APIServer) ListLocations(c *gin.Context, params ListLocationsParams) {
	tenantID := tenantIDFromContext(c)

	if params.Slug != nil && params.Level != nil {
		loc, err := s.topologySvc.GetBySlug(c.Request.Context(), tenantID, *params.Slug, *params.Level)
		if err != nil {
			response.NotFound(c, "location not found")
			return
		}
		response.OK(c, toAPILocation(*loc))
		return
	}

	// ?all=true → return every location for this tenant
	if c.Query("all") == "true" {
		locations, err := s.topologySvc.ListAllLocations(c.Request.Context(), tenantID)
		if err != nil {
			response.InternalError(c, "failed to list all locations")
			return
		}
		response.OK(c, convertSlice(locations, toAPILocation))
		return
	}

	locations, err := s.topologySvc.ListRootLocations(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list locations")
		return
	}
	response.OK(c, convertSlice(locations, toAPILocation))
}

// GetLocation returns a single location by ID.
// (GET /locations/{id})
func (s *APIServer) GetLocation(c *gin.Context, id IdPath) {
	loc, err := s.topologySvc.GetLocation(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}
	response.OK(c, toAPILocation(loc))
}

// ListLocationAncestors returns ancestor locations for a given location.
// (GET /locations/{id}/ancestors)
func (s *APIServer) ListLocationAncestors(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)

	loc, err := s.topologySvc.GetLocation(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}

	path := pgtextToStr(loc.Path)
	ancestors, err := s.topologySvc.ListAncestors(c.Request.Context(), tenantID, path)
	if err != nil {
		response.InternalError(c, "failed to list ancestors")
		return
	}
	response.OK(c, convertSlice(ancestors, toAPILocation))
}

// ListLocationChildren returns child locations for a given parent.
// (GET /locations/{id}/children)
func (s *APIServer) ListLocationChildren(c *gin.Context, id IdPath) {
	children, err := s.topologySvc.ListChildren(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list children")
		return
	}
	response.OK(c, convertSlice(children, toAPILocation))
}

// ListLocationRacks returns racks at a location.
// (GET /locations/{id}/racks)
func (s *APIServer) ListLocationRacks(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	racks, err := s.topologySvc.ListRacksByLocation(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list racks")
		return
	}
	// Fix #3: batch-fetch occupancy to avoid N+1
	occupancyMap := map[uuid.UUID]int32{}
	occupancies, err := s.topologySvc.GetRackOccupanciesByLocation(c.Request.Context(), tenantID, uuid.UUID(id))
	if err == nil {
		for _, o := range occupancies {
			occupancyMap[o.RackID] = o.UsedU
		}
	}
	apiRacks := make([]Rack, len(racks))
	for i, r := range racks {
		apiRacks[i] = toAPIRackWithOccupancy(r, int(occupancyMap[r.ID]))
	}
	response.OK(c, apiRacks)
}

// GetLocationStats returns aggregate statistics for a location.
// (GET /locations/{id}/stats)
func (s *APIServer) GetLocationStats(c *gin.Context, id IdPath) {
	stats, err := s.topologySvc.GetLocationStats(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to get location stats")
		return
	}
	response.OK(c, LocationStats{
		TotalAssets:    int(stats.TotalAssets),
		TotalRacks:     int(stats.TotalRacks),
		CriticalAlerts: int(stats.CriticalAlerts),
		AvgOccupancy:   float32(stats.AvgOccupancy),
	})
}

// CreateLocation creates a new location.
// (POST /locations)
func (s *APIServer) CreateLocation(c *gin.Context) {
	var req CreateLocationJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	metadataJSON := json.RawMessage(`{}`)
	if req.Metadata != nil {
		metadataJSON, _ = json.Marshal(req.Metadata)
	}

	var sortOrder int32
	if req.SortOrder != nil {
		sortOrder = int32(*req.SortOrder)
	}

	// Build ltree path: for root locations use slug, for children use parent.path + "." + slug
	path := req.Slug
	if req.ParentId != nil {
		parent, err := s.topologySvc.GetLocation(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(*req.ParentId))
		if err != nil {
			response.BadRequest(c, "parent location not found")
			return
		}
		parentPath := pgtextToStr(parent.Path)
		if parentPath != "" {
			path = parentPath + "." + req.Slug
		}
	}

	params := dbgen.CreateLocationParams{
		TenantID:  tenantID,
		Name:      req.Name,
		NameEn:    textFromPtr(req.NameEn),
		Slug:      req.Slug,
		Level:     req.Level,
		ParentID:  pguuidFromPtr(uuidPtrFromOAPI(req.ParentId)),
		Column7:   path,
		Status:    req.Status,
		Metadata:  metadataJSON,
		SortOrder: sortOrder,
	}
	// Latitude/Longitude: read from metadata if provided by frontend
	// (frontend sends them as top-level fields, but they get captured in metadata)
	if req.Metadata != nil {
		if lat, ok := (*req.Metadata)["latitude"]; ok {
			if v, ok := lat.(float64); ok {
				params.Latitude = pgtype.Float8{Float64: v, Valid: true}
			}
		}
		if lng, ok := (*req.Metadata)["longitude"]; ok {
			if v, ok := lng.(float64); ok {
				params.Longitude = pgtype.Float8{Float64: v, Valid: true}
			}
		}
	}

	created, err := s.topologySvc.CreateLocation(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			response.Err(c, 409, "DUPLICATE", "A location with this slug already exists")
			return
		}
		zap.L().Error("failed to create location", zap.Error(err))
		response.InternalError(c, "failed to create location")
		return
	}
	s.recordAudit(c, "location.created", "topology", "location", created.ID, map[string]any{
		"name": created.Name, "slug": created.Slug, "level": created.Level,
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectLocationCreated, tenantID.String(), map[string]any{
		"location_id": created.ID.String(), "tenant_id": tenantID.String(), "name": created.Name,
	})
	response.Created(c, toAPILocation(*created))
}

// UpdateLocation updates an existing location.
// (PUT /locations/{id})
func (s *APIServer) UpdateLocation(c *gin.Context, id IdPath) {
	var req UpdateLocationJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateLocationParams{
		ID: uuid.UUID(id),
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.NameEn != nil {
		params.NameEn = pgtype.Text{String: *req.NameEn, Valid: true}
	}
	if req.Slug != nil {
		params.Slug = pgtype.Text{String: *req.Slug, Valid: true}
	}
	if req.Level != nil {
		params.Level = pgtype.Text{String: *req.Level, Valid: true}
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.Metadata != nil {
		b, _ := json.Marshal(req.Metadata)
		params.Metadata = b
	}
	if req.SortOrder != nil {
		params.SortOrder = pgtype.Int4{Int32: int32(*req.SortOrder), Valid: true}
	}

	updated, err := s.topologySvc.UpdateLocation(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "location not found")
		} else {
			response.InternalError(c, "failed to update location")
		}
		return
	}
	diff := map[string]any{}
	if req.Name != nil {
		diff["name"] = *req.Name
	}
	if req.Status != nil {
		diff["status"] = *req.Status
	}
	if req.Level != nil {
		diff["level"] = *req.Level
	}
	s.recordAudit(c, "location.updated", "topology", "location", updated.ID, diff)
	s.publishEvent(c.Request.Context(), eventbus.SubjectLocationUpdated, tenantIDFromContext(c).String(), map[string]any{
		"location_id": updated.ID.String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	response.OK(c, toAPILocation(*updated))
}

// DeleteLocation deletes a location.
// Supports ?recursive=true to delete all descendants.
// Returns 409 if location has dependencies and recursive is not set.
// (DELETE /locations/{id})
func (s *APIServer) DeleteLocation(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	locID := uuid.UUID(id)
	recursive := c.Query("recursive") == "true"

	// Preflight check: if ?preflight=true, return dependency info without deleting
	if c.Query("preflight") == "true" {
		info, err := s.topologySvc.PreflightDeleteLocation(c.Request.Context(), tenantID, locID)
		if err != nil {
			response.NotFound(c, "location not found")
			return
		}
		response.OK(c, gin.H{
			"child_locations": info.ChildLocations,
			"racks":           info.Racks,
			"assets":          info.Assets,
			"safe_to_delete":  info.ChildLocations == 0 && info.Racks == 0 && info.Assets == 0,
		})
		return
	}

	err := s.topologySvc.DeleteLocation(c.Request.Context(), tenantID, locID, recursive)
	if err != nil {
		if strings.Contains(err.Error(), "use recursive=true") {
			c.JSON(409, gin.H{"error": gin.H{"code": "HAS_DEPENDENCIES", "message": err.Error()}})
			return
		}
		response.NotFound(c, "location not found or delete failed")
		return
	}
	s.recordAudit(c, "location.deleted", "topology", "location", locID, map[string]any{"recursive": recursive})
	s.publishEvent(c.Request.Context(), eventbus.SubjectLocationDeleted, tenantID.String(), map[string]any{
		"location_id": locID.String(), "tenant_id": tenantID.String(), "recursive": recursive,
	})
	c.Status(204)
}

// ListLocationDescendants returns all descendant locations for a given location.
// (GET /locations/{id}/descendants)
func (s *APIServer) ListLocationDescendants(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)

	loc, err := s.topologySvc.GetLocation(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}

	path := pgtextToStr(loc.Path)
	descendants, err := s.topologySvc.ListDescendants(c.Request.Context(), tenantID, path)
	if err != nil {
		response.InternalError(c, "failed to list descendants")
		return
	}
	response.OK(c, convertSlice(descendants, toAPILocation))
}
