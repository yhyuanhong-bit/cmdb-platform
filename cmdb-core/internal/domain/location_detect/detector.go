package location_detect

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// StartPeriodicDetection runs location detection every 5 minutes.
func (s *Service) StartPeriodicDetection(ctx context.Context, tenantID uuid.UUID) {
	ticker := time.NewTicker(5 * time.Minute)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				s.RunDetection(ctx, tenantID)
			}
		}
	}()
	zap.L().Info("Location detection started (5m interval)")
}

// RunDetection performs a single location detection cycle: compares locations,
// processes diffs (auto-confirm, alerts, discovery), and runs anomaly detection.
// It is called both by the periodic ticker and immediately after MAC cache updates.
func (s *Service) RunDetection(ctx context.Context, tenantID uuid.UUID) {
	diffs, err := s.CompareLocations(ctx, tenantID)
	if err != nil {
		zap.L().Warn("location detection failed", zap.Error(err))
		return
	}

	var relocated, missing, newDevice int
	for _, d := range diffs {
		switch d.DiffType {
		case "relocated":
			relocated++
			if d.HasWorkOrder {
				// Auto-confirm relocation: update CMDB
				s.autoConfirmRelocation(ctx, tenantID, d)
			} else {
				// Unauthorized relocation: alert
				s.createLocationAlert(ctx, tenantID, d, "warning",
					fmt.Sprintf("Unauthorized relocation detected: %s moved from %s to %s",
						d.AssetTag, d.CMDBRackName, d.ActualRackName))
			}

		case "missing":
			missing++
			s.createLocationAlert(ctx, tenantID, d, "warning",
				fmt.Sprintf("Device missing from network: %s (last known: %s)",
					d.AssetTag, d.CMDBRackName))

		case "new_device":
			newDevice++
			s.createLocationAlert(ctx, tenantID, d, "info",
				fmt.Sprintf("Unregistered device detected: MAC %s at %s",
					d.MACAddress, d.ActualRackName))

			// Also create discovery candidate for review
			s.pool.Exec(ctx, `
				INSERT INTO discovered_assets (tenant_id, source, hostname, ip_address, raw_data, status, discovered_at)
				VALUES ($1, 'snmp_mac_detect', $2, '', $3, 'pending', now())
				ON CONFLICT DO NOTHING`,
				tenantID,
				fmt.Sprintf("MAC-%s", d.MACAddress),
				fmt.Sprintf(`{"mac_address":"%s","detected_rack":"%s","detected_at":"%s"}`, d.MACAddress, d.ActualRackName, d.DetectedAt.Format(time.RFC3339)))
		}
	}

	if relocated+missing+newDevice > 0 {
		zap.L().Info("location detection completed",
			zap.Int("relocated", relocated),
			zap.Int("missing", missing),
			zap.Int("new_device", newDevice),
			zap.Int("consistent", len(diffs)-relocated-missing-newDevice))
	}

	// Run anomaly detection
	anomalies := s.DetectAnomalies(ctx, tenantID)
	for _, a := range anomalies {
		s.createLocationAlert(ctx, tenantID, LocationDiff{
			DiffType: string(a.Type),
		}, a.Severity, a.Message)
		zap.L().Warn("location anomaly detected", zap.String("type", string(a.Type)), zap.String("message", a.Message))
	}
}

func (s *Service) autoConfirmRelocation(ctx context.Context, tenantID uuid.UUID, d LocationDiff) {
	// Update asset location in CMDB
	_, err := s.pool.Exec(ctx,
		"UPDATE assets SET rack_id = $1, sync_version = sync_version + 1 WHERE id = $2 AND tenant_id = $3",
		d.ActualRackID, d.AssetID, tenantID)
	if err != nil {
		zap.L().Warn("auto-confirm relocation failed", zap.Error(err))
		return
	}

	// Record history
	s.RecordLocationChange(ctx, tenantID, d.AssetID, d.CMDBRackID, d.ActualRackID, "snmp_auto", nil)

	// Publish event
	if s.bus != nil {
		payload, _ := json.Marshal(map[string]string{
			"asset_id":  d.AssetID.String(),
			"asset_tag": d.AssetTag,
			"from_rack": d.CMDBRackName,
			"to_rack":   d.ActualRackName,
			"source":    "snmp_auto",
		})
		s.bus.Publish(ctx, eventbus.Event{
			Subject:  eventbus.SubjectAssetLocationChanged,
			TenantID: tenantID.String(),
			Payload:  payload,
		})
	}

	// Auto-close matching relocation work orders
	rows, _ := s.pool.Query(ctx,
		`SELECT id FROM work_orders
		 WHERE asset_id = $1 AND type = 'relocation'
		 AND status NOT IN ('completed','verified','rejected')
		 AND tenant_id = $2 AND deleted_at IS NULL`,
		d.AssetID, tenantID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var woID uuid.UUID
			if rows.Scan(&woID) == nil {
				s.pool.Exec(ctx,
					"UPDATE work_orders SET status = 'completed', actual_end = now(), sync_version = sync_version + 1 WHERE id = $1",
					woID)
				s.pool.Exec(ctx,
					"INSERT INTO work_order_logs (order_id, action, from_status, to_status) VALUES ($1, 'auto_completed_by_location_detect', $2, 'completed')",
					woID, "in_progress")
				zap.L().Info("auto-closed relocation work order", zap.String("order_id", woID.String()))
			}
		}
	}

	zap.L().Info("auto-confirmed relocation",
		zap.String("asset", d.AssetTag),
		zap.String("from", d.CMDBRackName),
		zap.String("to", d.ActualRackName))
}

func (s *Service) createLocationAlert(ctx context.Context, tenantID uuid.UUID, d LocationDiff, severity, message string) {
	// Insert alert event
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO alert_events (tenant_id, asset_id, severity, status, message, fired_at)
		VALUES ($1, $2, $3, 'firing', $4, now())
	`, tenantID, d.AssetID, severity, message)

	// Publish event for WebSocket/notification
	if s.bus != nil {
		payload, _ := json.Marshal(map[string]string{
			"asset_id": d.AssetID.String(),
			"severity": severity,
			"message":  message,
			"type":     "location_" + d.DiffType,
		})
		s.bus.Publish(ctx, eventbus.Event{
			Subject:  eventbus.SubjectAlertFired,
			TenantID: tenantID.String(),
			Payload:  payload,
		})
	}
}
