package location_detect

import (
	"context"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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
	sc := database.Scope(s.pool, tenantID)
	rows, err := sc.Query(ctx, `
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
	`)
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

		if scanErr := rows.Scan(
			&d.AssetID, &d.AssetTag, &d.AssetName,
			&d.MACAddress,
			&cmdbRackID, &cmdbRackName,
			&actualRackID, &actualRackName,
			&d.HasWorkOrder,
		); scanErr != nil {
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
	newRows, err := sc.Query(ctx, `
		SELECT mc.mac_address, mc.detected_rack_id, r.name
		FROM mac_address_cache mc
		LEFT JOIN racks r ON mc.detected_rack_id = r.id
		WHERE mc.tenant_id = $1 AND mc.asset_id IS NULL
	`)
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
	sc := database.Scope(s.pool, tenantID)
	for _, e := range entries {
		// Look up which rack this switch port maps to
		var rackID *uuid.UUID
		if err := sc.QueryRow(ctx,
			"SELECT connected_rack_id FROM switch_port_mapping WHERE switch_asset_id = $2 AND port_name = $3 AND tenant_id = $1",
			e.SwitchAssetID, e.PortName).Scan(&rackID); err != nil {
			zap.L().Debug("mac cache: switch port mapping not found", zap.String("mac", e.MACAddress), zap.Error(err))
		}

		// Try to match MAC to an existing asset
		var assetID *uuid.UUID
		if err := sc.QueryRow(ctx,
			"SELECT id FROM assets WHERE attributes->>'mac_address' = $2 AND tenant_id = $1 AND deleted_at IS NULL",
			e.MACAddress).Scan(&assetID); err != nil {
			zap.L().Debug("mac cache: asset lookup by mac failed", zap.String("mac", e.MACAddress), zap.Error(err))
		}

		// Upsert into cache
		_, err := sc.Exec(ctx, `
			INSERT INTO mac_address_cache (tenant_id, mac_address, switch_asset_id, port_name, vlan_id, asset_id, detected_rack_id, last_seen)
			VALUES ($1, $2, $3, $4, $5, $6, $7, now())
			ON CONFLICT (tenant_id, mac_address) DO UPDATE SET
				switch_asset_id = $3, port_name = $4, vlan_id = $5,
				asset_id = COALESCE($6, mac_address_cache.asset_id),
				detected_rack_id = COALESCE($7, mac_address_cache.detected_rack_id),
				last_seen = now()
		`, e.MACAddress, e.SwitchAssetID, e.PortName, e.VLANID, assetID, rackID)
		if err != nil {
			zap.L().Warn("mac cache upsert failed", zap.String("mac", e.MACAddress), zap.Error(err))
		}
	}
	return nil
}

// nullableUUID wraps a *uuid.UUID as a pgtype.UUID suitable for sqlc params.
func nullableUUID(u *uuid.UUID) pgtype.UUID {
	if u == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *u, Valid: true}
}

// RecordLocationChange records a location change in history.
func (s *Service) RecordLocationChange(ctx context.Context, tenantID, assetID uuid.UUID, fromRackID, toRackID *uuid.UUID, detectedBy string, workOrderID *uuid.UUID) {
	if err := dbgen.New(s.pool).RecordLocationChange(ctx, dbgen.RecordLocationChangeParams{
		TenantID:    tenantID,
		AssetID:     assetID,
		FromRackID:  nullableUUID(fromRackID),
		ToRackID:    nullableUUID(toRackID),
		DetectedBy:  detectedBy,
		WorkOrderID: nullableUUID(workOrderID),
	}); err != nil {
		zap.L().Error("location detect: failed to record location change", zap.Error(err))
	}
}

// GetLocationHistory returns location change history for an asset.
func (s *Service) GetLocationHistory(ctx context.Context, assetID uuid.UUID, limit int) ([]LocationChange, error) {
	rows, err := dbgen.New(s.pool).GetLocationHistory(ctx, dbgen.GetLocationHistoryParams{
		AssetID: assetID,
		Limit:   int32(limit),
	})
	if err != nil {
		return nil, err
	}

	changes := make([]LocationChange, 0, len(rows))
	for _, r := range rows {
		c := LocationChange{
			ID:         r.ID,
			DetectedBy: r.DetectedBy,
			DetectedAt: r.DetectedAt.Time,
		}
		if r.FromRackID.Valid {
			id := uuid.UUID(r.FromRackID.Bytes)
			c.FromRackID = &id
		}
		if r.ToRackID.Valid {
			id := uuid.UUID(r.ToRackID.Bytes)
			c.ToRackID = &id
		}
		if r.WorkOrderID.Valid {
			id := uuid.UUID(r.WorkOrderID.Bytes)
			c.WorkOrderID = &id
		}
		if r.FromRackName.Valid {
			c.FromRackName = r.FromRackName.String
		}
		if r.ToRackName.Valid {
			c.ToRackName = r.ToRackName.String
		}
		changes = append(changes, c)
	}
	return changes, nil
}
