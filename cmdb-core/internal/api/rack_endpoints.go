package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

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
