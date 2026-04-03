package topology

// LocationStats aggregates key metrics for a single location.
type LocationStats struct {
	TotalAssets    int64   `json:"total_assets"`
	TotalRacks     int64   `json:"total_racks"`
	CriticalAlerts int64   `json:"critical_alerts"`
	AvgOccupancy   float64 `json:"avg_occupancy"`
}
