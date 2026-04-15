package api

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

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

	if err := s.pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM assets WHERE tenant_id = $1 AND deleted_at IS NULL", tenantID).Scan(&totalAssets); err != nil {
		zap.L().Error("location detect: failed to count assets", zap.Error(err))
	}
	if err := s.pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM mac_address_cache WHERE tenant_id = $1 AND asset_id IS NOT NULL", tenantID).Scan(&consistentCount); err != nil {
		zap.L().Error("location detect: failed to count tracked devices", zap.Error(err))
	}
	if err := s.pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM asset_location_history WHERE tenant_id = $1 AND detected_at > now() - interval '24 hours'", tenantID).Scan(&relocatedCount); err != nil {
		zap.L().Error("location detect: failed to count relocations", zap.Error(err))
	}
	if err := s.pool.QueryRow(c.Request.Context(),
		"SELECT count(*) FROM mac_address_cache WHERE tenant_id = $1 AND asset_id IS NULL", tenantID).Scan(&newDeviceCount); err != nil {
		zap.L().Error("location detect: failed to count new devices", zap.Error(err))
	}

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

// LocationDetectGetAnomalies returns detected anomaly patterns.
// GET /api/v1/location-detect/anomalies
func (s *APIServer) LocationDetectGetAnomalies(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	anomalies := s.locationDetectSvc.DetectAnomalies(c.Request.Context(), tenantID)
	if anomalies == nil {
		anomalies = []location_detect.Anomaly{}
	}
	response.OK(c, anomalies)
}

// LocationDetectGetReport returns a monthly location governance report.
// GET /api/v1/location-detect/report?days=30
func (s *APIServer) LocationDetectGetReport(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	ctx := c.Request.Context()

	// Parse period (default 30 days)
	days := 30
	if p := c.Query("days"); p != "" {
		if d, err := strconv.Atoi(p); err == nil && d > 0 && d <= 365 {
			days = d
		}
	}
	interval := fmt.Sprintf("%d days", days)

	var totalAssets, trackedAssets int64
	_ = s.pool.QueryRow(ctx, "SELECT count(*) FROM assets WHERE tenant_id = $1 AND deleted_at IS NULL", tenantID).Scan(&totalAssets)
	_ = s.pool.QueryRow(ctx, "SELECT count(*) FROM mac_address_cache WHERE tenant_id = $1 AND asset_id IS NOT NULL", tenantID).Scan(&trackedAssets)

	var totalRelocations, authorizedRelocations, unauthorizedRelocations int64
	_ = s.pool.QueryRow(ctx,
		"SELECT count(*) FROM asset_location_history WHERE tenant_id = $1 AND detected_at > now() - $2::interval",
		tenantID, interval).Scan(&totalRelocations)
	_ = s.pool.QueryRow(ctx,
		"SELECT count(*) FROM asset_location_history WHERE tenant_id = $1 AND detected_at > now() - $2::interval AND work_order_id IS NOT NULL",
		tenantID, interval).Scan(&authorizedRelocations)
	unauthorizedRelocations = totalRelocations - authorizedRelocations

	var unregisteredDevices int64
	_ = s.pool.QueryRow(ctx, "SELECT count(*) FROM mac_address_cache WHERE tenant_id = $1 AND asset_id IS NULL", tenantID).Scan(&unregisteredDevices)

	var locationAlerts int64
	_ = s.pool.QueryRow(ctx,
		"SELECT count(*) FROM alert_events WHERE tenant_id = $1 AND (message LIKE '%relocation%' OR message LIKE '%missing%' OR message LIKE '%Unregistered%') AND fired_at > now() - $2::interval",
		tenantID, interval).Scan(&locationAlerts)

	coveragePct := float64(0)
	if totalAssets > 0 {
		coveragePct = float64(trackedAssets) / float64(totalAssets) * 100
	}

	response.OK(c, gin.H{
		"period_days":              days,
		"total_assets":             totalAssets,
		"tracked_by_network":       trackedAssets,
		"coverage_pct":             fmt.Sprintf("%.1f", coveragePct),
		"total_relocations":        totalRelocations,
		"authorized_relocations":   authorizedRelocations,
		"unauthorized_relocations": unauthorizedRelocations,
		"unregistered_devices":     unregisteredDevices,
		"location_alerts":          locationAlerts,
		"auto_corrections":         authorizedRelocations,
	})
}
