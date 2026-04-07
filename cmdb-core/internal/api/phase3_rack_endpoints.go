package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetRackNetworkConnections handles GET /racks/:id/network-connections
// Returns all network connections for a given rack.
func (s *APIServer) GetRackNetworkConnections(c *gin.Context) {
	rackID := c.Param("id")
	if rackID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing rack id"})
		return
	}

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
		LEFT JOIN assets a ON rnc.connected_asset_id = a.id
		WHERE rnc.rack_id = $1
		ORDER BY rnc.source_port
	`, rackID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query network connections"})
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

	c.JSON(http.StatusOK, gin.H{"connections": connections})
}

// CreateRackNetworkConnection handles POST /racks/:id/network-connections
// Adds a new network connection record for the rack.
func (s *APIServer) CreateRackNetworkConnection(c *gin.Context) {
	rackID := c.Param("id")
	if rackID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing rack id"})
		return
	}

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
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	if body.SourcePort == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source_port is required"})
		return
	}
	if body.ConnectedAssetID != nil && *body.ConnectedAssetID != "" &&
		body.ExternalDevice != nil && *body.ExternalDevice != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "connected_asset_id and external_device are mutually exclusive"})
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
			c.JSON(http.StatusConflict, gin.H{"error": "connection already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create network connection"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": newID})
}

// DeleteRackNetworkConnection handles DELETE /racks/:id/network-connections/:connectionId
// Removes a specific network connection by its ID.
func (s *APIServer) DeleteRackNetworkConnection(c *gin.Context) {
	connectionID := c.Param("connectionId")
	if connectionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing connection id"})
		return
	}

	tag, err := s.pool.Exec(c.Request.Context(), `
		DELETE FROM rack_network_connections WHERE id = $1
	`, connectionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete network connection"})
		return
	}
	if tag.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "connection not found"})
		return
	}

	c.Status(http.StatusNoContent)
}
