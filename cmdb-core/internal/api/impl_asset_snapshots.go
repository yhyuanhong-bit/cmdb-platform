package api

import (
	"encoding/json"
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

// ---------------------------------------------------------------------------
// D10-P2 field-by-field diff between two point-in-time asset states
// ---------------------------------------------------------------------------

// DiffAssetState resolves the snapshots at two timestamps and returns the
// per-field change list. 400 when `to` is not strictly after `from` — the
// endpoint represents an open interval and an inverted range would have
// no meaning. 404 when either anchor has no snapshot, matching the
// semantics of the point-in-time endpoint.
// (GET /assets/{id}/diff)
func (s *APIServer) DiffAssetState(c *gin.Context, id IdPath, params DiffAssetStateParams) {
	if !params.To.After(params.From) {
		response.BadRequest(c, "`to` must be strictly after `from`")
		return
	}
	result, err := s.assetSvc.DiffStateAt(
		c.Request.Context(),
		tenantIDFromContext(c),
		uuid.UUID(id),
		params.From,
		params.To,
	)
	if err != nil {
		if errors.Is(err, asset.ErrSnapshotNotFound) {
			response.NotFound(c, "no asset state at or before one of the requested times")
			return
		}
		response.InternalError(c, "failed to diff asset state")
		return
	}
	response.OK(c, toAPIAssetDiff(uuid.UUID(id), result))
}

// toAPIAssetDiff converts a service DiffResult into the API AssetDiff shape.
// Attribute JSON bytes are decoded here so the wire response carries a
// real object/array rather than a base64 blob.
func toAPIAssetDiff(assetID uuid.UUID, r *asset.DiffResult) AssetDiff {
	changes := make([]AssetDiffField, 0, len(r.Changes))
	for _, ch := range r.Changes {
		changes = append(changes, AssetDiffField{
			Field: ch.Field,
			From:  normalizeDiffValue(ch.From),
			To:    normalizeDiffValue(ch.To),
		})
	}
	return AssetDiff{
		AssetId: assetID,
		FromAt:  r.From.ValidAt,
		ToAt:    r.To.ValidAt,
		Changes: changes,
	}
}

// normalizeDiffValue decodes raw JSON bytes coming from JSONB columns so
// the API response contains structured JSON rather than a base64 string.
// Non-[]byte values (string, uuid.UUID, []string, nil) pass through.
func normalizeDiffValue(v any) any {
	b, ok := v.([]byte)
	if !ok {
		return v
	}
	if len(b) == 0 {
		return nil
	}
	var decoded any
	if err := json.Unmarshal(b, &decoded); err != nil {
		return string(b)
	}
	return decoded
}
