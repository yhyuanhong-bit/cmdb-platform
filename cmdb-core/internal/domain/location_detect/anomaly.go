package location_detect

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
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
	rows, err := dbgen.New(s.pool).DetectFrequentRelocations(ctx, tenantID)
	if err != nil {
		zap.L().Warn("detectFrequentRelocations query failed", zap.Error(err))
		return nil
	}

	var anomalies []Anomaly
	for _, r := range rows {
		assetID := r.AssetID
		anomalies = append(anomalies, Anomaly{
			Type:       AnomalyFrequentRelocation,
			Severity:   "warning",
			Message:    fmt.Sprintf("Asset %s (%s) relocated %d times in 30 days", r.AssetTag, r.Name, r.MoveCount),
			AssetID:    &assetID,
			Details:    map[string]interface{}{"move_count": int(r.MoveCount), "period_days": 30},
			DetectedAt: time.Now(),
		})
	}
	return anomalies
}

// detectOffHoursRelocations finds relocations between 22:00-06:00.
func (s *Service) detectOffHoursRelocations(ctx context.Context, tenantID uuid.UUID) []Anomaly {
	rows, err := dbgen.New(s.pool).DetectOffHoursRelocations(ctx, tenantID)
	if err != nil {
		zap.L().Warn("detectOffHoursRelocations query failed", zap.Error(err))
		return nil
	}

	var anomalies []Anomaly
	for _, r := range rows {
		assetID := r.AssetID
		detectedAt := r.DetectedAt.Time
		anomalies = append(anomalies, Anomaly{
			Type:       AnomalyOffHoursRelocation,
			Severity:   "warning",
			Message:    fmt.Sprintf("Off-hours relocation: %s (%s) moved at %s", r.AssetTag, r.Name, detectedAt.Format("15:04")),
			AssetID:    &assetID,
			Details:    map[string]interface{}{"time": detectedAt.Format(time.RFC3339)},
			DetectedAt: time.Now(),
		})
	}
	return anomalies
}

// detectBulkDisappearance finds racks where 3+ devices disappeared within 1 hour.
func (s *Service) detectBulkDisappearance(ctx context.Context, tenantID uuid.UUID) []Anomaly {
	rows, err := dbgen.New(s.pool).DetectBulkDisappearance(ctx, tenantID)
	if err != nil {
		zap.L().Warn("detectBulkDisappearance query failed", zap.Error(err))
		return nil
	}

	var anomalies []Anomaly
	for _, r := range rows {
		if !r.FromRackID.Valid {
			// NOT NULL on the row guard above keeps this unreachable,
			// but the pgtype.UUID nullable wrapper means the compiler
			// requires an explicit check before we take the address.
			continue
		}
		rackID := uuid.UUID(r.FromRackID.Bytes)
		anomalies = append(anomalies, Anomaly{
			Type:       AnomalyBulkDisappearance,
			Severity:   "critical",
			Message:    fmt.Sprintf("Bulk disappearance: %d devices left %s within 1 hour", r.DeviceCount, r.Name),
			RackID:     &rackID,
			Details:    map[string]interface{}{"device_count": int(r.DeviceCount), "rack_name": r.Name},
			DetectedAt: time.Now(),
		})
	}
	return anomalies
}
