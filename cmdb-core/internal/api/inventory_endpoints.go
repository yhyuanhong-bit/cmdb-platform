package api

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// ---------------------------------------------------------------------------
// Custom inventory endpoints (not in generated OpenAPI spec)
// ---------------------------------------------------------------------------

// rackSummaryRow holds per-rack counts for an inventory task.
type rackSummaryRow struct {
	RackName    string  `json:"rack_name"`
	RackID      *string `json:"rack_id"`
	Total       int64   `json:"total"`
	Scanned     int64   `json:"scanned"`
	Pending     int64   `json:"pending"`
	Discrepancy int64   `json:"discrepancy"`
	Status      string  `json:"status"`
}

// discrepancyRow holds an inventory item flagged as discrepancy or missing.
type discrepancyRow struct {
	ID           string  `json:"id"`
	Status       string  `json:"status"`
	ScannedAt    *string `json:"scanned_at"`
	AssetName    *string `json:"asset_name"`
	AssetTag     *string `json:"asset_tag"`
	SerialNumber *string `json:"serial_number"`
	RackName     *string `json:"rack_name"`
	LocationName *string `json:"location_name"`
}

// GetInventoryRacksSummary returns per-rack progress totals for an inventory task.
// GET /inventory/tasks/:id/racks-summary
func (s *APIServer) GetInventoryRacksSummary(c *gin.Context) {
	taskID := c.Param("id")

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT
			COALESCE(r.name, 'Unassigned') as rack_name,
			r.id as rack_id,
			count(ii.id) as total,
			count(ii.id) FILTER (WHERE ii.status = 'scanned') as scanned,
			count(ii.id) FILTER (WHERE ii.status = 'pending') as pending,
			count(ii.id) FILTER (WHERE ii.status = 'discrepancy') as discrepancy
		FROM inventory_items ii
		LEFT JOIN assets a ON ii.asset_id = a.id
		LEFT JOIN racks r ON a.rack_id = r.id
		WHERE ii.task_id = $1
		GROUP BY r.id, r.name
		ORDER BY r.name
	`, taskID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to query racks summary"})
		return
	}
	defer rows.Close()

	results := make([]rackSummaryRow, 0)
	for rows.Next() {
		var (
			rackName    string
			rackID      pgtype.Text
			total       int64
			scanned     int64
			pending     int64
			discrepancy int64
		)
		if err := rows.Scan(&rackName, &rackID, &total, &scanned, &pending, &discrepancy); err != nil {
			c.JSON(500, gin.H{"error": "failed to scan rack summary row"})
			return
		}

		status := "not_started"
		if scanned == total && total > 0 {
			status = "scanned"
		} else if scanned > 0 {
			status = "in_progress"
		}

		row := rackSummaryRow{
			RackName:    rackName,
			Total:       total,
			Scanned:     scanned,
			Pending:     pending,
			Discrepancy: discrepancy,
			Status:      status,
		}
		if rackID.Valid {
			row.RackID = &rackID.String
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		c.JSON(500, gin.H{"error": "error reading rack summary rows"})
		return
	}

	c.JSON(200, gin.H{"racks": results})
}

// GetInventoryDiscrepancies returns inventory items with status discrepancy or missing.
// GET /inventory/tasks/:id/discrepancies
func (s *APIServer) GetInventoryDiscrepancies(c *gin.Context) {
	taskID := c.Param("id")

	rows, err := s.pool.Query(c.Request.Context(), `
		SELECT ii.id, ii.status, ii.scanned_at,
		       a.name as asset_name, a.asset_tag, a.serial_number,
		       r.name as rack_name, l.name as location_name
		FROM inventory_items ii
		LEFT JOIN assets a ON ii.asset_id = a.id
		LEFT JOIN racks r ON a.rack_id = r.id
		LEFT JOIN locations l ON a.location_id = l.id
		WHERE ii.task_id = $1 AND ii.status IN ('discrepancy', 'missing')
		ORDER BY ii.scanned_at DESC
	`, taskID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to query discrepancies"})
		return
	}
	defer rows.Close()

	results := make([]discrepancyRow, 0)
	for rows.Next() {
		var (
			id           pgtype.UUID
			status       string
			scannedAt    pgtype.Timestamptz
			assetName    pgtype.Text
			assetTag     pgtype.Text
			serialNumber pgtype.Text
			rackName     pgtype.Text
			locationName pgtype.Text
		)
		if err := rows.Scan(&id, &status, &scannedAt, &assetName, &assetTag, &serialNumber, &rackName, &locationName); err != nil {
			c.JSON(500, gin.H{"error": "failed to scan discrepancy row"})
			return
		}

		row := discrepancyRow{
			Status: status,
		}

		// id (UUID → string)
		if id.Valid {
			row.ID = uuid.UUID(id.Bytes).String()
		}

		if scannedAt.Valid {
			t := scannedAt.Time.UTC().Format("2006-01-02T15:04:05Z")
			row.ScannedAt = &t
		}
		if assetName.Valid {
			row.AssetName = &assetName.String
		}
		if assetTag.Valid {
			row.AssetTag = &assetTag.String
		}
		if serialNumber.Valid {
			row.SerialNumber = &serialNumber.String
		}
		if rackName.Valid {
			row.RackName = &rackName.String
		}
		if locationName.Valid {
			row.LocationName = &locationName.String
		}

		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		c.JSON(500, gin.H{"error": "error reading discrepancy rows"})
		return
	}

	c.JSON(200, gin.H{"discrepancies": results})
}
