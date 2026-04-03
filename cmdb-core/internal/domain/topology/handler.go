package topology

import (
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler exposes topology HTTP endpoints.
type Handler struct {
	svc *Service
}

// NewHandler creates a new topology handler.
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// Register mounts all topology routes on the given router group.
func (h *Handler) Register(r *gin.RouterGroup) {
	r.GET("/locations", h.listRootLocations)
	r.GET("/locations/:id", h.getLocation)
	r.GET("/locations/:id/children", h.listChildren)
	r.GET("/locations/:id/ancestors", h.listAncestors)
	r.GET("/locations/:id/stats", h.getLocationStats)
	r.GET("/locations/:id/racks", h.listRacksByLocation)
	r.GET("/racks/:id", h.getRack)
	r.GET("/racks/:id/assets", h.listAssetsByRack)
}

func (h *Handler) listRootLocations(c *gin.Context) {
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}
	locs, err := h.svc.ListRootLocations(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list root locations")
		return
	}
	response.OK(c, locs)
}

func (h *Handler) getLocation(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	loc, err := h.svc.GetLocation(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}
	response.OK(c, loc)
}

func (h *Handler) listChildren(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	children, err := h.svc.ListChildren(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to list children")
		return
	}
	response.OK(c, children)
}

func (h *Handler) listAncestors(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	tenantID, ok := parseTenantID(c)
	if !ok {
		return
	}
	loc, err := h.svc.GetLocation(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "location not found")
		return
	}
	path := loc.Path.String
	if !loc.Path.Valid || path == "" {
		response.OK(c, []any{})
		return
	}
	ancestors, err := h.svc.ListAncestors(c.Request.Context(), tenantID, path)
	if err != nil {
		response.InternalError(c, "failed to list ancestors")
		return
	}
	response.OK(c, ancestors)
}

func (h *Handler) getLocationStats(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	stats, err := h.svc.GetLocationStats(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to get location stats")
		return
	}
	response.OK(c, stats)
}

func (h *Handler) listRacksByLocation(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	racks, err := h.svc.ListRacksByLocation(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to list racks")
		return
	}
	response.OK(c, racks)
}

func (h *Handler) getRack(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	rack, err := h.svc.GetRack(c.Request.Context(), id)
	if err != nil {
		response.NotFound(c, "rack not found")
		return
	}
	response.OK(c, rack)
}

func (h *Handler) listAssetsByRack(c *gin.Context) {
	id, ok := parseUUIDParam(c, "id")
	if !ok {
		return
	}
	assets, err := h.svc.ListAssetsByRack(c.Request.Context(), id)
	if err != nil {
		response.InternalError(c, "failed to list assets by rack")
		return
	}
	response.OK(c, assets)
}

// parseUUIDParam extracts a UUID path parameter. Returns false on failure after
// writing an error response.
func parseUUIDParam(c *gin.Context, name string) (uuid.UUID, bool) {
	raw := c.Param(name)
	id, err := uuid.Parse(raw)
	if err != nil {
		response.BadRequest(c, "invalid uuid: "+name)
		return uuid.Nil, false
	}
	return id, true
}

// parseTenantID reads the tenant_id set by middleware in the gin context.
func parseTenantID(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get("tenant_id")
	if !exists {
		response.BadRequest(c, "missing tenant_id")
		return uuid.Nil, false
	}
	switch v := val.(type) {
	case uuid.UUID:
		return v, true
	case string:
		id, err := uuid.Parse(v)
		if err != nil {
			response.BadRequest(c, "invalid tenant_id")
			return uuid.Nil, false
		}
		return id, true
	default:
		response.BadRequest(c, "invalid tenant_id type")
		return uuid.Nil, false
	}
}
