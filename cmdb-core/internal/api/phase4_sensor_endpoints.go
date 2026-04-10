package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// sensorRecord represents one sensor row in list/create responses.
type sensorRecord struct {
	ID              string  `json:"id"`
	AssetID         *string `json:"asset_id"`
	AssetName       *string `json:"asset_name"`
	Name            string  `json:"name"`
	Type            string  `json:"type"`
	Location        *string `json:"location"`
	PollingInterval int     `json:"pollingInterval"`
	Enabled         bool    `json:"enabled"`
	Status          string  `json:"status"`
	Icon            string  `json:"icon"`
	LastSeen        *string `json:"lastSeen"`
}

// sensorIcon maps a sensor type to a Material icon name.
func sensorIcon(sensorType string) string {
	switch strings.ToLower(sensorType) {
	case "temperature":
		return "thermostat"
	case "humidity":
		return "water_drop"
	case "power":
		return "bolt"
	case "network":
		return "lan"
	case "cpu":
		return "memory"
	case "memory":
		return "memory"
	case "disk":
		return "storage"
	default:
		return "sensors"
	}
}

// ListSensors handles GET /sensors
// Returns all sensors for the current tenant with asset name and derived icon.
func (s *APIServer) ListSensors(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT s.id, s.asset_id, a.name AS asset_name, s.name, s.type, s.location,
		       s.polling_interval, s.enabled, s.status, s.last_heartbeat
		FROM sensors s
		LEFT JOIN assets a ON s.asset_id = a.id
		WHERE s.tenant_id = $1
		ORDER BY s.name
	`, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to query sensors"})
		return
	}
	defer rows.Close()

	sensors := []sensorRecord{}
	for rows.Next() {
		var (
			id              string
			assetID         *string
			assetName       *string
			name            string
			sType           string
			location        *string
			pollingInterval int
			enabled         bool
			status          string
			lastHeartbeat   *time.Time
		)
		if err := rows.Scan(&id, &assetID, &assetName, &name, &sType, &location,
			&pollingInterval, &enabled, &status, &lastHeartbeat); err != nil {
			continue
		}

		// Capitalize status
		displayStatus := strings.ToUpper(status[:1]) + status[1:]

		var lastSeen *string
		if lastHeartbeat != nil {
			s := lastHeartbeat.Format(time.RFC3339)
			lastSeen = &s
		}

		sensors = append(sensors, sensorRecord{
			ID:              id,
			AssetID:         assetID,
			AssetName:       assetName,
			Name:            name,
			Type:            sType,
			Location:        location,
			PollingInterval: pollingInterval,
			Enabled:         enabled,
			Status:          displayStatus,
			Icon:            sensorIcon(sType),
			LastSeen:        lastSeen,
		})
	}

	c.JSON(http.StatusOK, gin.H{"sensors": sensors})
}

// CreateSensor handles POST /sensors
// Inserts a new sensor and returns its generated ID.
func (s *APIServer) CreateSensor(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	var body struct {
		AssetID         *string `json:"asset_id"`
		Name            string  `json:"name"`
		Type            string  `json:"type"`
		Location        *string `json:"location"`
		PollingInterval *int    `json:"polling_interval"`
		Enabled         *bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if body.Name == "" || body.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name and type are required"})
		return
	}

	pollingInterval := 30
	if body.PollingInterval != nil {
		pollingInterval = *body.PollingInterval
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	newID := uuid.New()
	_, err := s.pool.Exec(c.Request.Context(), `
		INSERT INTO sensors (id, tenant_id, asset_id, name, type, location, polling_interval, enabled, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 'offline', now(), now())
	`, newID, tenantID, body.AssetID, body.Name, body.Type, body.Location, pollingInterval, enabled)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create sensor"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": newID.String()})
}

// UpdateSensor handles PUT /sensors/:id
// Updates sensor fields using a dynamic SET clause.
func (s *APIServer) UpdateSensor(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	sensorID := c.Param("id")
	if _, err := uuid.Parse(sensorID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sensor ID"})
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	allowed := map[string]bool{
		"asset_id":         true,
		"name":             true,
		"type":             true,
		"location":         true,
		"polling_interval": true,
		"enabled":          true,
		"status":           true,
	}

	var setClauses []string
	var args []interface{}
	idx := 1

	for _, col := range []string{"asset_id", "name", "type", "location", "polling_interval", "enabled", "status"} {
		if val, ok := body[col]; ok && allowed[col] {
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, idx))
			args = append(args, val)
			idx++
		}
	}

	if len(setClauses) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no fields to update"})
		return
	}

	setClauses = append(setClauses, fmt.Sprintf("updated_at = now()"))
	args = append(args, sensorID)
	idIdx := idx
	idx++
	args = append(args, tenantID)

	query := fmt.Sprintf("UPDATE sensors SET %s WHERE id = $%d AND tenant_id = $%d",
		strings.Join(setClauses, ", "), idIdx, idx)

	result, err := s.pool.Exec(c.Request.Context(), query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update sensor"})
		return
	}
	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "sensor not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "sensor updated"})
}

// DeleteSensor handles DELETE /sensors/:id
// Removes a sensor by ID, scoped to the current tenant.
func (s *APIServer) DeleteSensor(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	sensorID := c.Param("id")
	if _, err := uuid.Parse(sensorID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sensor ID"})
		return
	}

	result, err := s.pool.Exec(c.Request.Context(), `
		DELETE FROM sensors WHERE id = $1 AND tenant_id = $2
	`, sensorID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete sensor"})
		return
	}
	if result.RowsAffected() == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "sensor not found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// SensorHeartbeat handles POST /sensors/:id/heartbeat
// Updates a sensor's last_heartbeat timestamp and optional status, scoped to the current tenant.
func (s *APIServer) SensorHeartbeat(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	sensorID := c.Param("id")
	if _, err := uuid.Parse(sensorID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sensor ID"})
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	// Body is optional; ignore bind errors
	_ = c.ShouldBindJSON(&body)
	if body.Status == "" {
		body.Status = "online"
	}

	_, err := s.pool.Exec(c.Request.Context(), `
		UPDATE sensors SET last_heartbeat = now(), status = $1, updated_at = now()
		WHERE id = $2 AND tenant_id = $3
	`, body.Status, sensorID, tenantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update sensor heartbeat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}
