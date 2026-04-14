package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	location_detect "github.com/cmdb-platform/cmdb-core/internal/domain/location_detect"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// LocationDetectGetDiffs returns current location differences.
func (s *APIServer) LocationDetectGetDiffs(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	diffs, err := s.locationDetectSvc.CompareLocations(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to compare locations")
		return
	}
	if diffs == nil {
		diffs = []location_detect.LocationDiff{}
	}
	response.OK(c, diffs)
}

// LocationDetectGetHistory returns location change history for an asset.
func (s *APIServer) LocationDetectGetHistory(c *gin.Context) {
	assetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid asset ID")
		return
	}
	history, err := s.locationDetectSvc.GetLocationHistory(c.Request.Context(), assetID, 50)
	if err != nil {
		response.InternalError(c, "failed to get location history")
		return
	}
	if history == nil {
		history = []location_detect.LocationChange{}
	}
	response.OK(c, history)
}

// LocationDetectGetSummary returns a summary of location detection status.
func (s *APIServer) LocationDetectGetSummary(c *gin.Context) {
	tenantID := tenantIDFromContext(c)

	var totalAssets, consistentCount, relocatedCount, newDeviceCount int64

	_ = s.pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM assets WHERE tenant_id = $1 AND deleted_at IS NULL", tenantID).Scan(&totalAssets)
	_ = s.pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM mac_address_cache WHERE tenant_id = $1 AND asset_id IS NOT NULL", tenantID).Scan(&consistentCount)
	_ = s.pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM asset_location_history WHERE tenant_id = $1 AND detected_at > now() - interval '24 hours'", tenantID).Scan(&relocatedCount)
	_ = s.pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM mac_address_cache WHERE tenant_id = $1 AND asset_id IS NULL", tenantID).Scan(&newDeviceCount)

	missingCount := totalAssets - consistentCount
	if missingCount < 0 {
		missingCount = 0
	}

	response.OK(c, gin.H{
		"total_assets":       totalAssets,
		"tracked_by_network": consistentCount,
		"relocations_24h":    relocatedCount,
		"missing":            missingCount,
		"unregistered":       newDeviceCount,
		"coverage_pct":       float64(consistentCount) / float64(max(totalAssets, 1)) * 100,
	})
}
