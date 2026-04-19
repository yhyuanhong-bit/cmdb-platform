package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/discovery"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// Discovery (Auto-Discovery Staging Area)
// ---------------------------------------------------------------------------

// ListDiscoveredAssets lists discovered assets with optional status filter.
// (GET /discovery/pending)
func (s *APIServer) ListDiscoveredAssets(c *gin.Context, params ListDiscoveredAssetsParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)

	items, total, err := s.discoverySvc.List(c.Request.Context(), tenantID, params.Status, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list discovered assets")
		return
	}
	response.OKList(c, convertSlice(items, toAPIDiscoveredAsset), page, pageSize, int(total))
}

// IngestDiscoveredAsset ingests a newly discovered asset into the staging area.
// (POST /discovery/ingest)
func (s *APIServer) IngestDiscoveredAsset(c *gin.Context) {
	var req IngestDiscoveredAssetJSONRequestBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	tenantID := tenantIDFromContext(c)

	var rawDataJSON json.RawMessage
	if req.RawData != nil {
		rawDataJSON, _ = json.Marshal(req.RawData)
	} else {
		rawDataJSON = json.RawMessage("{}")
	}

	params := dbgen.CreateDiscoveredAssetParams{
		TenantID: tenantID,
		Source:   req.Source,
		Hostname: textFromPtr(&req.Hostname),
		RawData:  rawDataJSON,
		Status:   "pending",
	}
	if req.ExternalId != nil {
		params.ExternalID = textFromPtr(req.ExternalId)
	}
	if req.IpAddress != nil {
		params.IpAddress = textFromPtr(req.IpAddress)
	}

	// Auto-match by IP if possible
	if req.IpAddress != nil && *req.IpAddress != "" {
		matched, matchErr := s.discoverySvc.Queries().FindAssetByIP(c.Request.Context(), dbgen.FindAssetByIPParams{
			TenantID:  tenantID,
			IpAddress: pgtype.Text{String: *req.IpAddress, Valid: true},
		})
		if matchErr == nil {
			params.MatchedAssetID = pgtype.UUID{Bytes: matched.ID, Valid: true}
			params.Status = "conflict"
		}
	}

	item, err := s.discoverySvc.Ingest(c.Request.Context(), params)
	if err != nil {
		response.InternalError(c, "failed to ingest discovered asset")
		return
	}
	response.Created(c, toAPIDiscoveredAsset(*item))
}

// ApproveDiscoveredAsset approves a discovered asset AND creates the
// canonical row in `assets`. This is the action that "commits" a staged
// discovery finding into the CMDB.
//
// Contract (see REMEDIATION-ROADMAP 2.2):
//   - Runs inside a single Postgres transaction. INSERT into assets,
//     UPDATE discovered_assets.status='approved', and audit write are all
//     atomic — if any step fails the whole thing rolls back.
//   - Tenant-scoped end-to-end. Cross-tenant approve returns 404 (not 403)
//     to avoid leaking row existence.
//   - Idempotent on approved_asset_id. A repeated POST returns 200 with
//     the existing asset instead of double-creating.
//   - Duplicate identifier (asset_tag/property_number collision) returns
//     409 and leaves discovered_assets.status unchanged.
//   - asset.created event is published AFTER commit. Inside the tx would
//     mean a commit-failure emits a ghost event subscribers can't undo.
//
// (POST /discovery/{id}/approve)
func (s *APIServer) ApproveDiscoveredAsset(c *gin.Context, id IdPath) {
	ctx := c.Request.Context()
	tenantID := tenantIDFromContext(c)
	reviewerID := userIDFromContext(c)

	result, err := s.discoverySvc.ApproveAndCreateAsset(ctx, uuid.UUID(id), tenantID, reviewerID)
	if err != nil {
		switch {
		case errors.Is(err, discovery.ErrNotFound):
			response.NotFound(c, "discovered asset not found")
		case errors.Is(err, discovery.ErrAssetAlreadyExists):
			response.Err(c, http.StatusConflict, "ASSET_ALREADY_EXISTS",
				"an asset with the same identifier already exists — rename or reconcile before approval")
		default:
			zap.L().Error("discovery approve failed",
				zap.String("discovered_asset_id", uuid.UUID(id).String()),
				zap.String("tenant_id", tenantID.String()),
				zap.Error(err))
			response.InternalError(c, "failed to approve discovered asset")
		}
		return
	}

	// Event publish happens POST-commit. If this fails, the data is still
	// consistent — subscribers just miss the notification. A cron reconciler
	// could replay, but for now we log and continue.
	if result.Created {
		s.publishEvent(ctx, eventbus.SubjectAssetCreated, tenantID.String(), map[string]any{
			"asset_id":            result.Asset.ID.String(),
			"tenant_id":           tenantID.String(),
			"asset_tag":           result.Asset.AssetTag,
			"type":                result.Asset.Type,
			"source":              "discovery",
			"discovered_asset_id": uuid.UUID(id).String(),
		})
	}

	response.OK(c, toAPIDiscoveredAsset(result.Discovered))
}

// IgnoreDiscoveredAsset ignores a discovered asset.
// (POST /discovery/{id}/ignore)
func (s *APIServer) IgnoreDiscoveredAsset(c *gin.Context, id IdPath) {
	reviewerID := userIDFromContext(c)
	item, err := s.discoverySvc.Ignore(c.Request.Context(), uuid.UUID(id), reviewerID)
	if err != nil {
		response.InternalError(c, "failed to ignore discovered asset")
		return
	}
	response.OK(c, toAPIDiscoveredAsset(*item))
}

// GetDiscoveryStats returns discovery statistics for the last 24 hours.
// (GET /discovery/stats)
func (s *APIServer) GetDiscoveryStats(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	row, err := s.discoverySvc.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to get discovery stats")
		return
	}
	total := int(row.Total)
	pending := int(row.Pending)
	conflict := int(row.Conflict)
	approved := int(row.Approved)
	ignored := int(row.Ignored)
	matched := int(row.Matched)
	response.OK(c, DiscoveryStats{
		Total:    &total,
		Pending:  &pending,
		Conflict: &conflict,
		Approved: &approved,
		Ignored:  &ignored,
		Matched:  &matched,
	})
}
