package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// ListRackNetworkConnections handles GET /racks/{id}/network-connections
// Returns all network connections for a given rack.
func (s *APIServer) ListRackNetworkConnections(c *gin.Context, id IdPath) {
	rackID := uuid.UUID(id).String()
	tenantID := tenantIDFromContext(c)

	// Fix #11: verify tenant ownership via JOIN with racks table
	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT
			rnc.id,
			rnc.source_port,
			COALESCE(a.name, rnc.external_device, '') AS device,
			rnc.connected_asset_id,
			COALESCE(rnc.external_device, '')          AS external_device,
			COALESCE(rnc.speed, '')                    AS speed,
			COALESCE(rnc.status, '')                   AS status,
			COALESCE(rnc.vlans, '{}')                  AS vlans,
			COALESCE(rnc.connection_type, '')          AS connection_type
		FROM rack_network_connections rnc
		JOIN racks r ON rnc.rack_id = r.id AND r.tenant_id = $2 AND r.deleted_at IS NULL
		LEFT JOIN assets a ON rnc.connected_asset_id = a.id
		WHERE rnc.rack_id = $1
		ORDER BY rnc.source_port
	`, rackID, tenantID)
	if err != nil {
		response.InternalError(c, "failed to query network connections")
		return
	}
	defer rows.Close()

	connections := []gin.H{}
	for rows.Next() {
		var id, port, device, externalDevice, speed, status, connType string
		var connectedAssetID *string
		var vlans []int32

		if err := rows.Scan(&id, &port, &device, &connectedAssetID, &externalDevice, &speed, &status, &vlans, &connType); err != nil {
			continue
		}
		if vlans == nil {
			vlans = []int32{}
		}

		connections = append(connections, gin.H{
			"id":                 id,
			"port":               port,
			"device":             device,
			"connected_asset_id": connectedAssetID,
			"external_device":    externalDevice,
			"speed":              speed,
			"status":             status,
			"vlan":               formatVlans(vlans),
			"connection_type":    connType,
		})
	}

	response.OK(c, gin.H{"connections": connections})
}

// CreateRackNetworkConnection handles POST /racks/{id}/network-connections
// Adds a new network connection record for the rack.
func (s *APIServer) CreateRackNetworkConnection(c *gin.Context, id IdPath) {
	rackID := uuid.UUID(id).String()
	tenantID := tenantIDFromContext(c)

	var body struct {
		SourcePort       string   `json:"source_port"`
		ConnectedAssetID *string  `json:"connected_asset_id"`
		ExternalDevice   *string  `json:"external_device"`
		Speed            *string  `json:"speed"`
		Status           *string  `json:"status"`
		Vlans            []int32  `json:"vlans"`
		ConnectionType   *string  `json:"connection_type"`
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

	newID := uuid.New().String()
	_, err := s.pool.Exec(c.Request.Context(), `
		INSERT INTO rack_network_connections
			(id, rack_id, tenant_id, source_port, connected_asset_id, external_device, speed, status, vlans, connection_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, newID, rackID, tenantID, body.SourcePort,
		body.ConnectedAssetID, body.ExternalDevice,
		body.Speed, body.Status, body.Vlans, body.ConnectionType)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "duplicate") || strings.Contains(errStr, "unique") {
			response.Err(c, http.StatusConflict, "CONFLICT", "connection already exists")
			return
		}
		response.InternalError(c, "failed to create network connection")
		return
	}

	connID, _ := uuid.Parse(newID)
	s.recordAudit(c, "network_connection.created", "topology", "rack_network_connection", connID, map[string]any{
		"rack_id":     rackID,
		"source_port": body.SourcePort,
	})
	response.Created(c, gin.H{"id": newID})
}

// DeleteRackNetworkConnection handles DELETE /racks/{id}/network-connections/{connectionId}
// Removes a specific network connection by its ID.
func (s *APIServer) DeleteRackNetworkConnection(c *gin.Context, id IdPath, connectionId openapi_types.UUID) {
	rackID := uuid.UUID(id).String()
	connID := uuid.UUID(connectionId)
	tenantID := tenantIDFromContext(c)

	// Fix #10: verify the connection belongs to the specified rack and tenant
	tag, err := s.pool.Exec(c.Request.Context(), `
		DELETE FROM rack_network_connections
		WHERE id = $1
		  AND rack_id = $2
		  AND rack_id IN (SELECT r.id FROM racks r WHERE r.tenant_id = $3 AND r.deleted_at IS NULL)
	`, connID, rackID, tenantID)
	if err != nil {
		response.InternalError(c, "failed to delete network connection")
		return
	}
	if tag.RowsAffected() == 0 {
		response.NotFound(c, "connection not found")
		return
	}

	s.recordAudit(c, "network_connection.deleted", "topology", "rack_network_connection", connID, nil)
	c.Status(http.StatusNoContent)
}
