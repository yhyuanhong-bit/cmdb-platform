package api

import (
	"context"
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
)

// ---------------------------------------------------------------------------
// Asset compliance / scan endpoint
//
// Today the platform does NOT write a dedicated module='compliance' audit
// event. Rather than invent compliance findings out of thin air (the previous
// frontend placeholder rendered a fabricated ISO 27001 / Security Patching
// list), this endpoint surfaces the most recent "scan-style" audit_events
// targeting the asset — i.e. discovery + integration sync events. That gives
// the UI a real "last scanned at" timestamp it can show, and an empty state
// when the asset has never been touched by a scan.
//
// If/when a real compliance scanner ships, the SQL filter below is the only
// thing that needs to change (extend complianceModulesFilter).
// ---------------------------------------------------------------------------

// scanAuditModules lists the audit_events.module values that count as a
// "scan" for the purposes of the compliance endpoint. Centralised so we can
// extend it (e.g. adding 'compliance' once a scanner exists) without hunting
// through the SQL.
var scanAuditModules = []string{"discovery", "integration"}

// maxComplianceEvents bounds the number of recent events returned. Keeps the
// payload predictable and prevents a noisy asset from blowing up the response.
const maxComplianceEvents = 20

// auditScanRow is the shape returned from the audit_events query. Kept
// internal so the SQL → API conversion is in one place and unit-testable.
type auditScanRow struct {
	Action     string
	Module     *string
	Source     *string
	OperatorID *uuid.UUID
	CreatedAt  time.Time
}

// scanQuerier is the narrow interface needed to fetch scan rows. Letting us
// swap a fake implementation in unit tests without standing up a real DB.
type scanQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// fetchAssetScanEvents loads the most recent scan-style audit events for the
// given asset, newest first. The SQL filters on the bound tenant ($1) and the
// asset id ($2), and limits to maxComplianceEvents.
func fetchAssetScanEvents(ctx context.Context, sc scanQuerier, assetID uuid.UUID) ([]auditScanRow, error) {
	rows, err := sc.Query(ctx, `
		SELECT action, module, source, operator_id, created_at
		FROM audit_events
		WHERE tenant_id = $1
		  AND target_type = 'asset'
		  AND target_id  = $2
		  AND module = ANY($3::varchar[])
		ORDER BY created_at DESC
		LIMIT $4
	`, assetID, scanAuditModules, int32(maxComplianceEvents))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// pgUUID matches the wire format of pgtype.UUID for scanning.
	type pgUUID struct {
		Bytes [16]byte
		Valid bool
	}

	var out []auditScanRow
	for rows.Next() {
		var r auditScanRow
		var module, source *string
		var opID pgUUID
		if err := rows.Scan(&r.Action, &module, &source, &opID, &r.CreatedAt); err != nil {
			zap.L().Warn("compliance scan: row scan failed", zap.Error(err))
			continue
		}
		r.Module = module
		r.Source = source
		if opID.Valid {
			id := uuid.UUID(opID.Bytes)
			r.OperatorID = &id
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// buildComplianceScanResponse converts raw scan rows into the API payload.
// Pure function — no DB, no gin — so it is trivially unit-testable.
func buildComplianceScanResponse(assetID uuid.UUID, rows []auditScanRow) AssetComplianceScan {
	events := make([]AssetComplianceScanEvent, 0, len(rows))
	for _, r := range rows {
		evt := AssetComplianceScanEvent{
			Action:    r.Action,
			Module:    r.Module,
			Source:    r.Source,
			ScannedAt: r.CreatedAt,
		}
		if r.OperatorID != nil {
			id := openapi_types.UUID(*r.OperatorID)
			evt.OperatorId = &id
		}
		events = append(events, evt)
	}

	resp := AssetComplianceScan{
		AssetId:    openapi_types.UUID(assetID),
		EventCount: len(events),
		Events:     events,
	}
	if len(rows) > 0 {
		latest := rows[0]
		resp.LastScanAt = &latest.CreatedAt
		action := latest.Action
		resp.LastScanAction = &action
		if latest.Module != nil {
			m := *latest.Module
			resp.LastScanModule = &m
		}
		if latest.Source != nil {
			s := *latest.Source
			resp.LastScanSource = &s
		}
	}
	return resp
}

// errNoAssetService is sentinel for when the handler is constructed without
// an asset service (defensive — should never happen in production wiring).
var errNoAssetService = errors.New("compliance: asset service not wired")

// GetAssetComplianceScan handles GET /assets/:id/compliance-scan.
//
// Returns a 404 when the asset is unknown to this tenant. Returns a 200 with
// last_scan_at=null and an empty events array when the asset exists but has
// no scan-style audit events recorded yet — the UI renders an empty state in
// that case rather than fabricating compliance findings.
func (s *APIServer) GetAssetComplianceScan(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	assetID := uuid.UUID(id)
	ctx := c.Request.Context()

	if s.assetSvc == nil {
		zap.L().Error("compliance scan: asset service not wired", zap.Error(errNoAssetService))
		response.InternalError(c, "asset service unavailable")
		return
	}

	// 1. Confirm the asset exists for this tenant. This is what makes /404
	//    distinguishable from "no scans yet" (200 with empty list).
	if _, err := s.assetSvc.GetByID(ctx, tenantID, assetID); err != nil {
		response.NotFound(c, "asset not found")
		return
	}

	// 2. Pull the most recent scan-style audit events for this asset.
	sc := database.Scope(s.pool, tenantID)
	rows, err := fetchAssetScanEvents(ctx, sc, assetID)
	if err != nil {
		zap.L().Error("compliance scan: query failed",
			zap.String("asset_id", assetID.String()),
			zap.Error(err))
		response.InternalError(c, "failed to query compliance scan history")
		return
	}

	response.OK(c, buildComplianceScanResponse(assetID, rows))
}
