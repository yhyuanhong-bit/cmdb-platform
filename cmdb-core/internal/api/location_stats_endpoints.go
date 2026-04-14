package api

import (
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// locationAssetCountsResponse is the payload returned by GetLocationAssetCounts.
type locationAssetCountsResponse struct {
	Counts map[string]int64 `json:"counts"`
	Alerts map[string]int64 `json:"alerts"`
}

// GetLocationAssetCounts handles GET /api/v1/locations/asset-counts.
// Returns real-time asset counts and critical alert counts per location for the
// current tenant. Both maps are keyed by location UUID string.
func (s *APIServer) GetLocationAssetCounts(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	ctx := c.Request.Context()
	q := dbgen.New(s.pool)

	assetRows, err := q.CountAssetsByLocation(ctx, tenantID)
	if err != nil {
		response.InternalError(c, "failed to count assets by location")
		return
	}

	alertRows, err := q.CountAlertsByLocation(ctx, tenantID)
	if err != nil {
		response.InternalError(c, "failed to count alerts by location")
		return
	}

	counts := make(map[string]int64, len(assetRows))
	for _, row := range assetRows {
		counts[row.ID.String()] = row.TotalAssets
	}

	alerts := make(map[string]int64, len(alertRows))
	for _, row := range alertRows {
		if row.ID.Valid {
			alerts[uuid.UUID(row.ID.Bytes).String()] = row.CriticalAlerts
		}
	}

	response.OK(c, locationAssetCountsResponse{
		Counts: counts,
		Alerts: alerts,
	})
}
