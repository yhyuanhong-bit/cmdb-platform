package api

import (
	"errors"

	"github.com/cmdb-platform/cmdb-core/internal/domain/asset"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// D10-P0 point-in-time asset state (review-2026-04-21-v2)
// ---------------------------------------------------------------------------

// GetAssetStateAt returns the snapshot of an asset at or before the
// query time. 404s when nothing predates the requested instant — which
// means either the asset was created later, or it predates the
// 000056 backfill and has never been rewritten since.
// (GET /assets/{id}/state-at)
func (s *APIServer) GetAssetStateAt(c *gin.Context, id IdPath, params GetAssetStateAtParams) {
	snap, err := s.assetSvc.GetStateAt(
		c.Request.Context(),
		tenantIDFromContext(c),
		uuid.UUID(id),
		params.At,
	)
	if err != nil {
		if errors.Is(err, asset.ErrSnapshotNotFound) {
			response.NotFound(c, "no asset state at or before requested time")
			return
		}
		response.InternalError(c, "failed to fetch asset state")
		return
	}
	response.OK(c, toAPIAssetSnapshot(snap))
}

// ListAssetSnapshots returns the full snapshot timeline for an asset
// newest-first. Feeds the history timeline UI.
// (GET /assets/{id}/snapshots)
func (s *APIServer) ListAssetSnapshots(c *gin.Context, id IdPath, params ListAssetSnapshotsParams) {
	limit := int32(100)
	if params.Limit != nil {
		limit = int32(*params.Limit)
	}
	snaps, err := s.assetSvc.ListSnapshots(
		c.Request.Context(),
		tenantIDFromContext(c),
		uuid.UUID(id),
		limit,
	)
	if err != nil {
		response.InternalError(c, "failed to list asset snapshots")
		return
	}
	response.OK(c, convertSlice(snaps, toAPIAssetSnapshot))
}
