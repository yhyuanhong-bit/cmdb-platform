package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// GetAssetQRData returns QR code data for an asset (JSON content to encode into QR).
// GET /api/v1/assets/:id/qr-data
func (s *APIServer) GetAssetQRData(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	assetID := uuid.UUID(id)

	var tag, sn, name string
	err := s.pool.QueryRow(c.Request.Context(),
		"SELECT asset_tag, COALESCE(serial_number, ''), name FROM assets WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL",
		assetID, tenantID).Scan(&tag, &sn, &name)
	if err != nil {
		response.NotFound(c, "asset not found")
		return
	}

	qrData := map[string]string{
		"t":    "asset",
		"id":   assetID.String(),
		"tag":  tag,
		"sn":   sn,
		"name": name,
	}
	response.OK(c, qrData)
}

// GetRackQRData returns QR code data for a rack.
// GET /api/v1/racks/:id/qr-data
func (s *APIServer) GetRackQRData(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	rackID := uuid.UUID(id)

	var rackName, locName string
	err := s.pool.QueryRow(c.Request.Context(),
		`SELECT r.name, COALESCE(l.name, '')
		 FROM racks r LEFT JOIN locations l ON r.location_id = l.id
		 WHERE r.id = $1 AND r.tenant_id = $2`,
		rackID, tenantID).Scan(&rackName, &locName)
	if err != nil {
		response.NotFound(c, "rack not found")
		return
	}

	qrData := map[string]string{
		"t":    "rack",
		"id":   rackID.String(),
		"name": rackName,
		"loc":  locName,
	}
	response.OK(c, qrData)
}

// ConfirmAssetLocation updates asset location via QR scan.
// POST /api/v1/assets/:id/confirm-location
// Body: {"rack_id": "uuid"}
func (s *APIServer) ConfirmAssetLocation(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	assetID := uuid.UUID(id)

	var req struct {
		RackID string `json:"rack_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "rack_id is required")
		return
	}

	newRackID, err := uuid.Parse(req.RackID)
	if err != nil {
		response.BadRequest(c, "invalid rack_id")
		return
	}

	// Get current rack
	var currentRackID *uuid.UUID
	if err := s.pool.QueryRow(c.Request.Context(),
		"SELECT rack_id FROM assets WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL",
		assetID, tenantID).Scan(&currentRackID); err != nil {
		zap.L().Error("qr: failed to get current rack", zap.Error(err))
	}

	// Update location
	_, err = s.pool.Exec(c.Request.Context(),
		"UPDATE assets SET rack_id = $1, sync_version = sync_version + 1, updated_at = now() WHERE id = $2 AND tenant_id = $3",
		newRackID, assetID, tenantID)
	if err != nil {
		response.InternalError(c, "failed to update location")
		return
	}

	// Record history
	if s.locationDetectSvc != nil {
		s.locationDetectSvc.RecordLocationChange(c.Request.Context(), tenantID, assetID, currentRackID, &newRackID, "qr_scan", nil)
	}

	// Audit
	s.recordAudit(c, "asset.location_confirmed", "asset", "asset", assetID, map[string]any{
		"from_rack": currentRackID,
		"to_rack":   newRackID,
		"source":    "qr_scan",
		"operator":  userID,
	})

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "location_updated", "rack_id": newRackID}})
}
