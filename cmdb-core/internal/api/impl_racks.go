package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ---------------------------------------------------------------------------
// Rack endpoints
// ---------------------------------------------------------------------------

// CreateRack creates a new rack.
// (POST /racks)
func (s *APIServer) CreateRack(c *gin.Context) {
	var req CreateRackJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var totalU int32
	if req.TotalU != nil {
		totalU = int32(*req.TotalU)
	}
	// Fix #7: validate total_u > 0
	if totalU <= 0 {
		response.BadRequest(c, "total_u must be greater than 0")
		return
	}

	var powerKw pgtype.Numeric
	if req.PowerCapacityKw != nil {
		powerKw = float32ToNumeric(*req.PowerCapacityKw)
	}

	var tags []string
	if req.Tags != nil {
		tags = *req.Tags
	}

	params := dbgen.CreateRackParams{
		TenantID:        tenantID,
		LocationID:      uuid.UUID(req.LocationId),
		Name:            req.Name,
		RowLabel:        textFromPtr(req.RowLabel),
		TotalU:          totalU,
		PowerCapacityKw: powerKw,
		Status:          req.Status,
		Tags:            tags,
	}

	created, err := s.topologySvc.CreateRack(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			response.Err(c, 409, "DUPLICATE", "A rack with this name already exists in this location")
			return
		}
		response.InternalError(c, "failed to create rack")
		return
	}
	s.recordAudit(c, "rack.created", "topology", "rack", created.ID, map[string]any{
		"name": created.Name, "location_id": created.LocationID.String(),
	})
	s.publishEvent(c.Request.Context(), eventbus.SubjectRackCreated, tenantID.String(), map[string]any{
		"rack_id": created.ID.String(), "tenant_id": tenantID.String(), "name": created.Name,
	})
	response.Created(c, toAPIRack(*created))
}

// GetRack returns a single rack by ID.
// (GET /racks/{id})
func (s *APIServer) GetRack(c *gin.Context, id IdPath) {
	rack, err := s.topologySvc.GetRack(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "rack not found")
		return
	}
	// Fix #13: compute used_u from rack_slots
	var usedU int
	occ, err := s.topologySvc.GetRackOccupancy(c.Request.Context(), uuid.UUID(id))
	if err == nil {
		usedU = int(occ.UsedU)
	}
	response.OK(c, toAPIRackWithOccupancy(rack, usedU))
}

// ListRackAssets returns all assets in a rack.
// (GET /racks/{id}/assets)
func (s *APIServer) ListRackAssets(c *gin.Context, id IdPath) {
	assets, err := s.topologySvc.ListAssetsByRack(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list rack assets")
		return
	}
	response.OK(c, convertSlice(assets, toAPIAsset))
}

// UpdateRack updates an existing rack.
// (PUT /racks/{id})
func (s *APIServer) UpdateRack(c *gin.Context, id IdPath) {
	// Use a superset struct to handle location_id (#22) which isn't in the generated spec
	var req struct {
		UpdateRackJSONRequestBody
		LocationId *string `json:"location_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	params := dbgen.UpdateRackParams{
		ID:       uuid.UUID(id),
		TenantID: tenantIDFromContext(c), // Fix #8: ensure tenant_id is set for security
	}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.RowLabel != nil {
		params.RowLabel = pgtype.Text{String: *req.RowLabel, Valid: true}
	}
	if req.TotalU != nil {
		params.TotalU = pgtype.Int4{Int32: int32(*req.TotalU), Valid: true}
	}
	if req.PowerCapacityKw != nil {
		params.PowerCapacityKw = float32ToNumeric(*req.PowerCapacityKw)
	}
	if req.Status != nil {
		params.Status = pgtype.Text{String: *req.Status, Valid: true}
	}
	if req.Tags != nil {
		params.Tags = *req.Tags
	}
	// Fix #22: support moving rack to different location
	if req.LocationId != nil {
		locUUID, err := uuid.Parse(*req.LocationId)
		if err != nil {
			response.BadRequest(c, "invalid location_id")
			return
		}
		params.LocationID = pgtype.UUID{Bytes: locUUID, Valid: true}
	}

	updated, err := s.topologySvc.UpdateRack(c.Request.Context(), params)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			response.NotFound(c, "rack not found")
		} else {
			response.InternalError(c, "failed to update rack")
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
	s.recordAudit(c, "rack.updated", "topology", "rack", updated.ID, diff)
	s.publishEvent(c.Request.Context(), eventbus.SubjectRackUpdated, tenantIDFromContext(c).String(), map[string]any{
		"rack_id": updated.ID.String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	response.OK(c, toAPIRack(*updated))
}

// DeleteRack deletes a rack.
// (DELETE /racks/{id})
func (s *APIServer) DeleteRack(c *gin.Context, id IdPath) {
	err := s.topologySvc.DeleteRack(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "rack not found")
		return
	}
	s.recordAudit(c, "rack.deleted", "topology", "rack", uuid.UUID(id), nil)
	s.publishEvent(c.Request.Context(), eventbus.SubjectRackDeleted, tenantIDFromContext(c).String(), map[string]any{
		"rack_id": uuid.UUID(id).String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	c.Status(204)
}

// ---------------------------------------------------------------------------
// Rack Slot endpoints
// ---------------------------------------------------------------------------

// ListRackSlots returns all slot assignments for a rack.
// (GET /racks/{id}/slots)
func (s *APIServer) ListRackSlots(c *gin.Context, id IdPath) {
	slots, err := s.topologySvc.ListRackSlots(c.Request.Context(), uuid.UUID(id))
	if err != nil {
		response.InternalError(c, "failed to list rack slots")
		return
	}
	response.OK(c, convertSlice(slots, toAPIRackSlot))
}

// CreateRackSlot assigns an asset to a rack slot with conflict detection.
// (POST /racks/{id}/slots)
func (s *APIServer) CreateRackSlot(c *gin.Context, id IdPath) {
	var req CreateRackSlotJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	side := "front"
	if req.Side != nil {
		side = *req.Side
	}

	// Fix #9: validate U-position against rack's total_u
	rack, err := s.topologySvc.GetRack(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(id))
	if err != nil {
		response.NotFound(c, "rack not found")
		return
	}
	if req.StartU < 1 || int32(req.EndU) > rack.TotalU {
		response.BadRequest(c, fmt.Sprintf("U position out of range: rack has %dU, requested U%d-U%d", rack.TotalU, req.StartU, req.EndU))
		return
	}
	if req.StartU > req.EndU {
		response.BadRequest(c, "start_u must be <= end_u")
		return
	}

	// Check for U-position conflict
	conflictCount, err := s.topologySvc.CheckSlotConflict(c.Request.Context(), uuid.UUID(id), side, int32(req.StartU), int32(req.EndU))
	if err != nil {
		response.InternalError(c, "failed to check slot conflict")
		return
	}
	if conflictCount > 0 {
		response.BadRequest(c, fmt.Sprintf("U position conflict: U%d-U%d on %s is occupied", req.StartU, req.EndU, side))
		return
	}

	slot, err := s.topologySvc.CreateRackSlot(c.Request.Context(), dbgen.CreateRackSlotParams{
		RackID:  uuid.UUID(id),
		AssetID: uuid.UUID(req.AssetId),
		StartU:  int32(req.StartU),
		EndU:    int32(req.EndU),
		Side:    side,
	})
	if err != nil {
		response.InternalError(c, "failed to create rack slot")
		return
	}

	s.recordAudit(c, "rack_slot.created", "topology", "rack_slot", slot.ID, map[string]any{
		"rack_id":  uuid.UUID(id).String(),
		"asset_id": uuid.UUID(req.AssetId).String(),
		"start_u":  req.StartU,
		"end_u":    req.EndU,
		"side":     side,
	})
	// Fix #12: publish rack.occupancy_changed event
	s.publishEvent(c.Request.Context(), eventbus.SubjectRackOccupancyChanged, tenantIDFromContext(c).String(), map[string]any{
		"rack_id": uuid.UUID(id).String(), "tenant_id": tenantIDFromContext(c).String(),
	})

	// Convert the created slot to API format
	apiSlot := RackSlot{
		Id:      &slot.ID,
		RackId:  &slot.RackID,
		AssetId: &slot.AssetID,
		StartU:  func() *int { v := int(slot.StartU); return &v }(),
		EndU:    func() *int { v := int(slot.EndU); return &v }(),
		Side:    &slot.Side,
	}
	response.Created(c, apiSlot)
}

// DeleteRackSlot removes an asset from a rack slot.
// (DELETE /racks/{id}/slots/{slotId})
func (s *APIServer) DeleteRackSlot(c *gin.Context, id IdPath, slotId openapi_types.UUID) {
	err := s.topologySvc.DeleteRackSlot(c.Request.Context(), tenantIDFromContext(c), uuid.UUID(slotId))
	if err != nil {
		response.NotFound(c, "rack slot not found")
		return
	}
	s.recordAudit(c, "rack_slot.deleted", "topology", "rack_slot", uuid.UUID(slotId), map[string]any{
		"rack_id": uuid.UUID(id).String(),
	})
	// Fix #12: publish rack.occupancy_changed event
	s.publishEvent(c.Request.Context(), eventbus.SubjectRackOccupancyChanged, tenantIDFromContext(c).String(), map[string]any{
		"rack_id": uuid.UUID(id).String(), "tenant_id": tenantIDFromContext(c).String(),
	})
	c.Status(204)
}

// ListRackNetworkConnections handles GET /racks/{id}/network-connections
// Returns all network connections for a given rack.
func (s *APIServer) ListRackNetworkConnections(c *gin.Context, id IdPath) {
	rackID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	rows, err := dbgen.New(s.pool).ListRackNetworkConnections(c.Request.Context(), dbgen.ListRackNetworkConnectionsParams{
		RackID:   rackID,
		TenantID: tenantID,
	})
	if err != nil {
		response.InternalError(c, "failed to query network connections")
		return
	}

	connections := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		var connectedAssetID *string
		if r.ConnectedAssetID.Valid {
			s := uuid.UUID(r.ConnectedAssetID.Bytes).String()
			connectedAssetID = &s
		}
		vlans := r.Vlans
		if vlans == nil {
			vlans = []int32{}
		}
		connections = append(connections, gin.H{
			"id":                 r.ID.String(),
			"port":               r.SourcePort,
			"device":             r.Device,
			"connected_asset_id": connectedAssetID,
			"external_device":    r.ExternalDevice,
			"speed":              r.Speed,
			"status":             r.Status,
			"vlan":               formatVlans(vlans),
			"connection_type":    r.ConnectionType,
		})
	}

	response.OK(c, gin.H{"connections": connections})
}

// CreateRackNetworkConnection handles POST /racks/{id}/network-connections
// Adds a new network connection record for the rack.
func (s *APIServer) CreateRackNetworkConnection(c *gin.Context, id IdPath) {
	rackID := uuid.UUID(id)
	tenantID := tenantIDFromContext(c)

	var body struct {
		SourcePort       string  `json:"source_port"`
		ConnectedAssetID *string `json:"connected_asset_id"`
		ExternalDevice   *string `json:"external_device"`
		Speed            *string `json:"speed"`
		Status           *string `json:"status"`
		Vlans            []int32 `json:"vlans"`
		ConnectionType   *string `json:"connection_type"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if body.SourcePort == "" {
		response.BadRequest(c, "source_port is required")
		return
	}
	if body.ConnectedAssetID != nil && *body.ConnectedAssetID != "" &&
		body.ExternalDevice != nil && *body.ExternalDevice != "" {
		response.BadRequest(c, "connected_asset_id and external_device are mutually exclusive")
		return
	}

	connectedAssetID := pgtype.UUID{}
	if body.ConnectedAssetID != nil && *body.ConnectedAssetID != "" {
		parsed, err := uuid.Parse(*body.ConnectedAssetID)
		if err != nil {
			response.BadRequest(c, "invalid connected_asset_id")
			return
		}
		connectedAssetID = pgtype.UUID{Bytes: parsed, Valid: true}
	}

	newID := uuid.New()
	err := dbgen.New(s.pool).CreateRackNetworkConnection(c.Request.Context(), dbgen.CreateRackNetworkConnectionParams{
		ID:               newID,
		RackID:           rackID,
		TenantID:         tenantID,
		SourcePort:       body.SourcePort,
		ConnectedAssetID: connectedAssetID,
		ExternalDevice:   nullableTextPtr(body.ExternalDevice),
		Speed:            nullableTextPtr(body.Speed),
		Status:           nullableTextPtr(body.Status),
		Vlans:            body.Vlans,
		ConnectionType:   nullableTextPtr(body.ConnectionType),
	})
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "unique") {
			response.Err(c, http.StatusConflict, "CONFLICT", "connection already exists")
			return
		}
		response.InternalError(c, "failed to create network connection")
		return
	}

	s.recordAudit(c, "network_connection.created", "topology", "rack_network_connection", newID, map[string]any{
		"rack_id":     rackID.String(),
		"source_port": body.SourcePort,
	})
	response.Created(c, gin.H{"id": newID.String()})
}

// DeleteRackNetworkConnection handles DELETE /racks/{id}/network-connections/{connectionId}
// Removes a specific network connection by its ID.
func (s *APIServer) DeleteRackNetworkConnection(c *gin.Context, id IdPath, connectionId openapi_types.UUID) {
	rackID := uuid.UUID(id)
	connID := uuid.UUID(connectionId)
	tenantID := tenantIDFromContext(c)

	rowsAffected, err := dbgen.New(s.pool).DeleteRackNetworkConnection(c.Request.Context(), dbgen.DeleteRackNetworkConnectionParams{
		ID:       connID,
		RackID:   rackID,
		TenantID: tenantID,
	})
	if err != nil {
		response.InternalError(c, "failed to delete network connection")
		return
	}
	if rowsAffected == 0 {
		response.NotFound(c, "connection not found")
		return
	}

	s.recordAudit(c, "network_connection.deleted", "topology", "rack_network_connection", connID, nil)
	c.Status(http.StatusNoContent)
}

// nullableTextPtr converts a *string into a pgtype.Text, collapsing
// both nil and "" to a NULL. Shared helper keeps the conversion
// consistent across the rack handlers.
func nullableTextPtr(s *string) pgtype.Text {
	if s == nil || *s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: *s, Valid: true}
}
