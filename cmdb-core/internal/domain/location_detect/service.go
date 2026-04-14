package location_detect

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// LocationDiff represents a difference between CMDB record and detected location.
type LocationDiff struct {
	AssetID        uuid.UUID  `json:"asset_id"`
	AssetTag       string     `json:"asset_tag"`
	AssetName      string     `json:"asset_name"`
	MACAddress     string     `json:"mac_address"`
	CMDBRackID     *uuid.UUID `json:"cmdb_rack_id"`
	CMDBRackName   string     `json:"cmdb_rack_name"`
	ActualRackID   *uuid.UUID `json:"actual_rack_id"`
	ActualRackName string     `json:"actual_rack_name"`
	DiffType       string     `json:"diff_type"` // consistent | relocated | missing | new_device
	HasWorkOrder   bool       `json:"has_work_order"`
	DetectedAt     time.Time  `json:"detected_at"`
}

// MACEntry represents a single MAC table entry from SNMP scan.
type MACEntry struct {
	SwitchAssetID uuid.UUID
	PortName      string
	MACAddress    string
	VLANID        *int
}

// LocationChange represents a historical location change.
type LocationChange struct {
	ID           uuid.UUID  `json:"id"`
	FromRackID   *uuid.UUID `json:"from_rack_id"`
	FromRackName string     `json:"from_rack_name"`
	ToRackID     *uuid.UUID `json:"to_rack_id"`
	ToRackName   string     `json:"to_rack_name"`
	DetectedBy   string     `json:"detected_by"`
	WorkOrderID  *uuid.UUID `json:"work_order_id"`
	DetectedAt   time.Time  `json:"detected_at"`
}

// Service handles location detection via SNMP MAC table comparison.
type Service struct {
	pool *pgxpool.Pool
	bus  eventbus.Bus
}

// NewService creates a LocationDetect service.
func NewService(pool *pgxpool.Pool, bus eventbus.Bus) *Service {
	return &Service{pool: pool, bus: bus}
}

// CompareLocations compares CMDB records with MAC table data for a tenant.
func (s *Service) CompareLocations(ctx context.Context, tenantID uuid.UUID) ([]LocationDiff, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			a.id, a.asset_tag, a.name,
			mc.mac_address,
			a.rack_id AS cmdb_rack_id,
			cr.name AS cmdb_rack_name,
			mc.detected_rack_id AS actual_rack_id,
			ar.name AS actual_rack_name,
			EXISTS(
				SELECT 1 FROM work_orders wo
				WHERE wo.asset_id = a.id
				AND wo.type = 'relocation'
				AND wo.status NOT IN ('completed','verified','rejected')
				AND wo.deleted_at IS NULL
			) AS has_work_order
		FROM mac_address_cache mc
		JOIN assets a ON mc.asset_id = a.id AND a.tenant_id = $1 AND a.deleted_at IS NULL
		LEFT JOIN racks cr ON a.rack_id = cr.id
		LEFT JOIN racks ar ON mc.detected_rack_id = ar.id
		WHERE mc.tenant_id = $1
	`, tenantID)
	if err != nil {
		return nil, fmt.Errorf("compare locations: %w", err)
	}
	defer rows.Close()

	var diffs []LocationDiff
	now := time.Now()

	for rows.Next() {
		var d LocationDiff
		var cmdbRackID, actualRackID *uuid.UUID
		var cmdbRackName, actualRackName *string

		if err := rows.Scan(
			&d.AssetID, &d.AssetTag, &d.AssetName,
			&d.MACAddress,
			&cmdbRackID, &cmdbRackName,
			&actualRackID, &actualRackName,
			&d.HasWorkOrder,
		); err != nil {
			continue
		}

		d.CMDBRackID = cmdbRackID
		d.ActualRackID = actualRackID
		d.DetectedAt = now
		if cmdbRackName != nil {
			d.CMDBRackName = *cmdbRackName
		}
		if actualRackName != nil {
			d.ActualRackName = *actualRackName
		}

		// Determine diff type
		switch {
		case cmdbRackID != nil && actualRackID != nil && *cmdbRackID == *actualRackID:
			d.DiffType = "consistent"
		case cmdbRackID != nil && actualRackID != nil && *cmdbRackID != *actualRackID:
			d.DiffType = "relocated"
		case actualRackID == nil:
			d.DiffType = "missing"
		default:
			d.DiffType = "consistent"
		}

		diffs = append(diffs, d)
	}

	// Also find MACs in cache with no asset_id (new/unregistered devices)
	newRows, err := s.pool.Query(ctx, `
		SELECT mc.mac_address, mc.detected_rack_id, r.name
		FROM mac_address_cache mc
		LEFT JOIN racks r ON mc.detected_rack_id = r.id
		WHERE mc.tenant_id = $1 AND mc.asset_id IS NULL
	`, tenantID)
	if err == nil {
		defer newRows.Close()
		for newRows.Next() {
			var mac string
			var rackID *uuid.UUID
			var rackName *string
			if newRows.Scan(&mac, &rackID, &rackName) == nil {
				d := LocationDiff{
					MACAddress: mac,
					DiffType:   "new_device",
					DetectedAt: now,
				}
				d.ActualRackID = rackID
				if rackName != nil {
					d.ActualRackName = *rackName
				}
				diffs = append(diffs, d)
			}
		}
	}

	return diffs, nil
}

// UpdateMACCache updates the MAC address cache from SNMP scan results.
func (s *Service) UpdateMACCache(ctx context.Context, tenantID uuid.UUID, entries []MACEntry) error {
	for _, e := range entries {
		// Look up which rack this switch port maps to
		var rackID *uuid.UUID
		_ = s.pool.QueryRow(ctx,
			"SELECT connected_rack_id FROM switch_port_mapping WHERE switch_asset_id = $1 AND port_name = $2 AND tenant_id = $3",
			e.SwitchAssetID, e.PortName, tenantID).Scan(&rackID)

		// Try to match MAC to an existing asset
		var assetID *uuid.UUID
		_ = s.pool.QueryRow(ctx,
			"SELECT id FROM assets WHERE attributes->>'mac_address' = $1 AND tenant_id = $2 AND deleted_at IS NULL",
			e.MACAddress, tenantID).Scan(&assetID)

		// Upsert into cache
		_, err := s.pool.Exec(ctx, `
			INSERT INTO mac_address_cache (tenant_id, mac_address, switch_asset_id, port_name, vlan_id, asset_id, detected_rack_id, last_seen)
			VALUES ($1, $2, $3, $4, $5, $6, $7, now())
			ON CONFLICT (tenant_id, mac_address) DO UPDATE SET
				switch_asset_id = $3, port_name = $4, vlan_id = $5,
				asset_id = COALESCE($6, mac_address_cache.asset_id),
				detected_rack_id = COALESCE($7, mac_address_cache.detected_rack_id),
				last_seen = now()
		`, tenantID, e.MACAddress, e.SwitchAssetID, e.PortName, e.VLANID, assetID, rackID)
		if err != nil {
			zap.L().Warn("mac cache upsert failed", zap.String("mac", e.MACAddress), zap.Error(err))
		}
	}
	return nil
}

// RecordLocationChange records a location change in history.
func (s *Service) RecordLocationChange(ctx context.Context, tenantID, assetID uuid.UUID, fromRackID, toRackID *uuid.UUID, detectedBy string, workOrderID *uuid.UUID) {
	_, _ = s.pool.Exec(ctx, `
		INSERT INTO asset_location_history (tenant_id, asset_id, from_rack_id, to_rack_id, detected_by, work_order_id)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, tenantID, assetID, fromRackID, toRackID, detectedBy, workOrderID)
}

// GetLocationHistory returns location change history for an asset.
func (s *Service) GetLocationHistory(ctx context.Context, assetID uuid.UUID, limit int) ([]LocationChange, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT h.id, h.from_rack_id, fr.name, h.to_rack_id, tr.name, h.detected_by, h.work_order_id, h.detected_at
		FROM asset_location_history h
		LEFT JOIN racks fr ON h.from_rack_id = fr.id
		LEFT JOIN racks tr ON h.to_rack_id = tr.id
		WHERE h.asset_id = $1
		ORDER BY h.detected_at DESC
		LIMIT $2
	`, assetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var changes []LocationChange
	for rows.Next() {
		var c LocationChange
		var fromName, toName *string
		if err := rows.Scan(&c.ID, &c.FromRackID, &fromName, &c.ToRackID, &toName, &c.DetectedBy, &c.WorkOrderID, &c.DetectedAt); err != nil {
			continue
		}
		if fromName != nil {
			c.FromRackName = *fromName
		}
		if toName != nil {
			c.ToRackName = *toName
		}
		changes = append(changes, c)
	}
	return changes, nil
}
