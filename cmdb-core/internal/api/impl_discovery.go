package api

import (
	"encoding/json"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// ApproveDiscoveredAsset approves a discovered asset.
// (POST /discovery/{id}/approve)
func (s *APIServer) ApproveDiscoveredAsset(c *gin.Context, id IdPath) {
	reviewerID := userIDFromContext(c)
	item, err := s.discoverySvc.Approve(c.Request.Context(), uuid.UUID(id), reviewerID)
	if err != nil {
		response.InternalError(c, "failed to approve discovered asset")
		return
	}
	response.OK(c, toAPIDiscoveredAsset(*item))
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
