package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
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
// The spec's optional tenant_id query param is ignored; the handler always
// scopes to the authenticated tenant to preserve isolation.
func (s *APIServer) ListSensors(c *gin.Context, _ ListSensorsParams) {
	tenantID := tenantIDFromContext(c)

	rows, err := dbgen.New(s.pool).ListSensorsByTenant(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to query sensors")
		return
	}

	sensors := make([]sensorRecord, 0, len(rows))
	for _, r := range rows {
		// Capitalize status (guard empty string)
		displayStatus := r.Status
		if len(displayStatus) > 0 {
			displayStatus = strings.ToUpper(displayStatus[:1]) + displayStatus[1:]
		}

		var lastSeen *string
		if r.LastHeartbeat.Valid {
			formatted := r.LastHeartbeat.Time.Format(time.RFC3339)
			lastSeen = &formatted
		}

		var assetID *string
		if r.AssetID.Valid {
			idStr := uuid.UUID(r.AssetID.Bytes).String()
			assetID = &idStr
		}
		var assetName *string
		if r.AssetName.Valid {
			name := r.AssetName.String
			assetName = &name
		}
		var location *string
		if r.Location.Valid {
			loc := r.Location.String
			location = &loc
		}

		sensors = append(sensors, sensorRecord{
			ID:              r.ID.String(),
			AssetID:         assetID,
			AssetName:       assetName,
			Name:            r.Name,
			Type:            r.Type,
			Location:        location,
			PollingInterval: int(r.PollingInterval),
			Enabled:         r.Enabled,
			Status:          displayStatus,
			Icon:            sensorIcon(r.Type),
			LastSeen:        lastSeen,
		})
	}

	response.OK(c, gin.H{"sensors": sensors})
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
		response.BadRequest(c, err.Error())
		return
	}
	if body.Name == "" || body.Type == "" {
		response.BadRequest(c, "name and type are required")
		return
	}

	pollingInterval := int32(30)
	if body.PollingInterval != nil {
		pollingInterval = int32(*body.PollingInterval)
	}
	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	assetIDParam := pgtype.UUID{}
	if body.AssetID != nil && *body.AssetID != "" {
		parsed, err := uuid.Parse(*body.AssetID)
		if err != nil {
			response.BadRequest(c, "invalid asset_id")
			return
		}
		assetIDParam = pgtype.UUID{Bytes: parsed, Valid: true}
	}
	locationParam := pgtype.Text{}
	if body.Location != nil {
		locationParam = pgtype.Text{String: *body.Location, Valid: true}
	}

	newID := uuid.New()
	if err := dbgen.New(s.pool).CreateSensor(c.Request.Context(), dbgen.CreateSensorParams{
		ID:              newID,
		TenantID:        tenantID,
		AssetID:         assetIDParam,
		Name:            body.Name,
		Type:            body.Type,
		Location:        locationParam,
		PollingInterval: pollingInterval,
		Enabled:         enabled,
	}); err != nil {
		response.InternalError(c, "failed to create sensor")
		return
	}

	s.recordAudit(c, "sensor.created", "monitoring", "sensor", newID, map[string]any{
		"name": body.Name,
		"type": body.Type,
	})
	response.Created(c, gin.H{"id": newID.String()})
}

// UpdateSensor handles PUT /sensors/{id}.
// Updates sensor fields using a dynamic SET clause. This one stays as raw
// SQL because sqlc has no native "partial update with arbitrary columns"
// story that composes with the COALESCE(sqlc.narg) idiom once the column
// set grows — writing each column as a separate UPDATE would be worse.
// The allow-list below constrains what `body` keys can reach the SQL.
func (s *APIServer) UpdateSensor(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	sensorID := uuid.UUID(id)

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
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
		response.BadRequest(c, "no fields to update")
		return
	}

	setClauses = append(setClauses, "updated_at = now()")
	args = append(args, sensorID)
	idIdx := idx
	idx++
	args = append(args, tenantID)

	query := fmt.Sprintf("UPDATE sensors SET %s WHERE id = $%d AND tenant_id = $%d",
		strings.Join(setClauses, ", "), idIdx, idx)

	result, err := s.pool.Exec(c.Request.Context(), query, args...)
	if err != nil {
		response.InternalError(c, "failed to update sensor")
		return
	}
	if result.RowsAffected() == 0 {
		response.NotFound(c, "sensor not found")
		return
	}

	s.recordAudit(c, "sensor.updated", "monitoring", "sensor", sensorID, body)
	response.OK(c, gin.H{"message": "sensor updated"})
}

// DeleteSensor handles DELETE /sensors/{id}
// Removes a sensor by ID, scoped to the current tenant.
func (s *APIServer) DeleteSensor(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	sensorID := uuid.UUID(id)

	rowsAffected, err := dbgen.New(s.pool).DeleteSensor(c.Request.Context(), dbgen.DeleteSensorParams{
		ID:       sensorID,
		TenantID: tenantID,
	})
	if err != nil {
		response.InternalError(c, "failed to delete sensor")
		return
	}
	if rowsAffected == 0 {
		response.NotFound(c, "sensor not found")
		return
	}

	s.recordAudit(c, "sensor.deleted", "monitoring", "sensor", sensorID, nil)
	c.Status(http.StatusNoContent)
}

// SensorHeartbeat handles POST /sensors/{id}/heartbeat
// Updates a sensor's last_heartbeat timestamp and optional status, scoped to the current tenant.
func (s *APIServer) SensorHeartbeat(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	sensorID := uuid.UUID(id)

	var body struct {
		Status string `json:"status"`
	}
	// Body is optional; ignore bind errors
	_ = c.ShouldBindJSON(&body)
	if body.Status == "" {
		body.Status = "online"
	}

	if err := dbgen.New(s.pool).UpdateSensorHeartbeat(c.Request.Context(), dbgen.UpdateSensorHeartbeatParams{
		ID:       sensorID,
		TenantID: tenantID,
		Status:   body.Status,
	}); err != nil {
		response.InternalError(c, "failed to update sensor heartbeat")
		return
	}

	response.OK(c, gin.H{"message": "ok"})
}
