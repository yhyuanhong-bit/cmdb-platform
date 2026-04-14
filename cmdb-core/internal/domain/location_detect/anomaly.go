package location_detect

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// AnomalyType represents types of location anomalies.
type AnomalyType string

const (
	AnomalyFrequentRelocation AnomalyType = "frequent_relocation"
	AnomalyOffHoursRelocation AnomalyType = "off_hours_relocation"
	AnomalyBulkDisappearance  AnomalyType = "bulk_disappearance"
)

// Anomaly represents a detected location anomaly.
type Anomaly struct {
	Type       AnomalyType            `json:"type"`
	Severity   string                 `json:"severity"`
	Message    string                 `json:"message"`
	AssetID    *uuid.UUID             `json:"asset_id,omitempty"`
	RackID     *uuid.UUID             `json:"rack_id,omitempty"`
	Details    map[string]interface{} `json:"details"`
	DetectedAt time.Time              `json:"detected_at"`
}

// DetectAnomalies runs all anomaly detection checks and returns findings.
func (s *Service) DetectAnomalies(ctx context.Context, tenantID uuid.UUID) []Anomaly {
	var anomalies []Anomaly

	anomalies = append(anomalies, s.detectFrequentRelocations(ctx, tenantID)...)
	anomalies = append(anomalies, s.detectOffHoursRelocations(ctx, tenantID)...)
	anomalies = append(anomalies, s.detectBulkDisappearance(ctx, tenantID)...)

	return anomalies
}

// detectFrequentRelocations finds assets moved 3+ times in 30 days.
func (s *Service) detectFrequentRelocations(ctx context.Context, tenantID uuid.UUID) []Anomaly {
	rows, err := s.pool.Query(ctx, `
		SELECT h.asset_id, a.asset_tag, a.name, count(*) as move_count
		FROM asset_location_history h
		JOIN assets a ON h.asset_id = a.id
		WHERE h.tenant_id = $1 AND h.detected_at > now() - interval '30 days'
		GROUP BY h.asset_id, a.asset_tag, a.name
		HAVING count(*) >= 3
		ORDER BY count(*) DESC
	`, tenantID)
	if err != nil {
		zap.L().Warn("detectFrequentRelocations query failed", zap.Error(err))
		return nil
	}
	defer rows.Close()

	var anomalies []Anomaly
	for rows.Next() {
		var assetID uuid.UUID
		var tag, name string
		var count int
		if rows.Scan(&assetID, &tag, &name, &count) == nil {
			anomalies = append(anomalies, Anomaly{
				Type:       AnomalyFrequentRelocation,
				Severity:   "warning",
				Message:    fmt.Sprintf("Asset %s (%s) relocated %d times in 30 days", tag, name, count),
				AssetID:    &assetID,
				Details:    map[string]interface{}{"move_count": count, "period_days": 30},
				DetectedAt: time.Now(),
			})
		}
	}
	return anomalies
}

// detectOffHoursRelocations finds relocations between 22:00-06:00.
func (s *Service) detectOffHoursRelocations(ctx context.Context, tenantID uuid.UUID) []Anomaly {
	rows, err := s.pool.Query(ctx, `
		SELECT h.asset_id, a.asset_tag, a.name, h.detected_at
		FROM asset_location_history h
		JOIN assets a ON h.asset_id = a.id
		WHERE h.tenant_id = $1
		AND h.detected_at > now() - interval '24 hours'
		AND (EXTRACT(HOUR FROM h.detected_at) >= 22 OR EXTRACT(HOUR FROM h.detected_at) < 6)
		ORDER BY h.detected_at DESC
	`, tenantID)
	if err != nil {
		zap.L().Warn("detectOffHoursRelocations query failed", zap.Error(err))
		return nil
	}
	defer rows.Close()

	var anomalies []Anomaly
	for rows.Next() {
		var assetID uuid.UUID
		var tag, name string
		var detectedAt time.Time
		if rows.Scan(&assetID, &tag, &name, &detectedAt) == nil {
			anomalies = append(anomalies, Anomaly{
				Type:       AnomalyOffHoursRelocation,
				Severity:   "warning",
				Message:    fmt.Sprintf("Off-hours relocation: %s (%s) moved at %s", tag, name, detectedAt.Format("15:04")),
				AssetID:    &assetID,
				Details:    map[string]interface{}{"time": detectedAt.Format(time.RFC3339)},
				DetectedAt: time.Now(),
			})
		}
	}
	return anomalies
}

// detectBulkDisappearance finds racks where 3+ devices disappeared within 1 hour.
func (s *Service) detectBulkDisappearance(ctx context.Context, tenantID uuid.UUID) []Anomaly {
	rows, err := s.pool.Query(ctx, `
		SELECT h.from_rack_id, r.name, count(*) as device_count
		FROM asset_location_history h
		JOIN racks r ON h.from_rack_id = r.id
		WHERE h.tenant_id = $1
		AND h.detected_at > now() - interval '1 hour'
		AND h.from_rack_id IS NOT NULL
		GROUP BY h.from_rack_id, r.name
		HAVING count(*) >= 3
	`, tenantID)
	if err != nil {
		zap.L().Warn("detectBulkDisappearance query failed", zap.Error(err))
		return nil
	}
	defer rows.Close()

	var anomalies []Anomaly
	for rows.Next() {
		var rackID uuid.UUID
		var rackName string
		var count int
		if rows.Scan(&rackID, &rackName, &count) == nil {
			anomalies = append(anomalies, Anomaly{
				Type:       AnomalyBulkDisappearance,
				Severity:   "critical",
				Message:    fmt.Sprintf("Bulk disappearance: %d devices left %s within 1 hour", count, rackName),
				RackID:     &rackID,
				Details:    map[string]interface{}{"device_count": count, "rack_name": rackName},
				DetectedAt: time.Now(),
			})
		}
	}
	return anomalies
}
