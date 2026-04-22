package discovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors the handler layer maps to HTTP status codes. Callers must
// use errors.Is to branch — the underlying Postgres error is wrapped, so the
// sentinel identity survives formatting.
var (
	// ErrNotFound is returned when a discovered_asset is absent, OR when it
	// exists in another tenant. We deliberately collapse both cases so a
	// tenant probing IDs can't distinguish "your row" from "another tenant's
	// row" by timing or error text.
	ErrNotFound = errors.New("discovered asset not found")

	// ErrAssetAlreadyExists is returned when promoting a discovered_asset to
	// an asset collides with an existing unique constraint (asset_tag,
	// property_number, etc). The transaction is rolled back so the
	// discovered_asset.status is unchanged.
	ErrAssetAlreadyExists = errors.New("asset with matching identifier already exists")
)

// ApproveResult bundles the approval output for the handler: the canonical
// asset row (whether freshly created or returned idempotently) and the
// discovered_asset row as-of-commit.
type ApproveResult struct {
	Asset            dbgen.Asset
	Discovered       dbgen.DiscoveredAsset
	Created          bool // false on idempotent retry — asset already existed.
	ReviewerID       uuid.UUID
	ApprovedAssetTag string
}

type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

// NewService builds the discovery domain service. The pool is needed so
// Approve can own its own transaction boundary (see ApproveAndCreateAsset).
// It is optional (may be nil) for list-only use cases and tests that don't
// exercise approval.
func NewService(queries *dbgen.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool}
}

// Queries returns the underlying queries for direct access (e.g. auto-match).
func (s *Service) Queries() *dbgen.Queries {
	return s.queries
}

func (s *Service) List(ctx context.Context, tenantID uuid.UUID, status *string, limit, offset int32) ([]dbgen.DiscoveredAsset, int64, error) {
	params := dbgen.ListDiscoveredAssetsParams{TenantID: tenantID, Limit: limit, Offset: offset}
	countParams := dbgen.CountDiscoveredAssetsParams{TenantID: tenantID}
	if status != nil {
		params.Status = pgtype.Text{String: *status, Valid: true}
		countParams.Status = pgtype.Text{String: *status, Valid: true}
	}
	items, err := s.queries.ListDiscoveredAssets(ctx, params)
	if err != nil {
		return nil, 0, fmt.Errorf("list discovered: %w", err)
	}
	total, err := s.queries.CountDiscoveredAssets(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count discovered: %w", err)
	}
	return items, total, nil
}

func (s *Service) Ingest(ctx context.Context, params dbgen.CreateDiscoveredAssetParams) (*dbgen.DiscoveredAsset, error) {
	item, err := s.queries.CreateDiscoveredAsset(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create discovered: %w", err)
	}
	return &item, nil
}

// ApproveAndCreateAsset promotes a discovered_asset to a canonical asset
// inside a single Postgres transaction.
//
// Contract:
//  1. Lookup is tenant-scoped. A row owned by another tenant returns
//     ErrNotFound — we do NOT leak existence by returning 403.
//  2. Idempotent on approved_asset_id. A second call after a successful
//     approve returns the existing asset with Created=false.
//  3. Duplicate assets (asset_tag collision, etc) roll back the whole tx so
//     discovered_assets.status is unchanged. Caller sees ErrAssetAlreadyExists.
//  4. Audit event is written INSIDE the tx so a rollback loses the audit
//     trail too — no phantom "approved" audit if the INSERT failed.
//  5. Domain event publish is the caller's job AFTER commit — publishing
//     inside the tx would emit a ghost event on commit failure.
//
// NOTE: the asset_tag generated here is synthetic. The discovered record
// does not carry a human-supplied tag; operators are expected to rename
// later via UpdateAsset. We use a `DSC-<short-id>` prefix so the row is
// visually distinguishable in UIs and searchable.
func (s *Service) ApproveAndCreateAsset(
	ctx context.Context,
	discoveredID, tenantID, reviewerID uuid.UUID,
) (*ApproveResult, error) {
	if s.pool == nil {
		return nil, fmt.Errorf("discovery: pool not configured")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // safe to call after Commit — no-op.

	qtx := s.queries.WithTx(tx)

	// 1. Load + tenant scope.
	da, err := qtx.GetDiscoveredAsset(ctx, dbgen.GetDiscoveredAssetParams{
		ID:       discoveredID,
		TenantID: tenantID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get discovered: %w", err)
	}
	// Defensive: the query is already tenant-scoped, but the struct carries
	// tenant_id so we double-check. Cheap and catches a future bug where the
	// query gets edited.
	if da.TenantID != tenantID {
		return nil, ErrNotFound
	}

	// 2. Idempotency fast-path.
	if da.ApprovedAssetID.Valid {
		existingID := uuid.UUID(da.ApprovedAssetID.Bytes)
		existing, getErr := qtx.GetAsset(ctx, dbgen.GetAssetParams{
			ID:       existingID,
			TenantID: tenantID,
		})
		if getErr != nil {
			// The link is dangling (asset was deleted). Treat as "not approved"
			// would be dangerous — it could silently create a duplicate. Surface
			// the inconsistency to the caller instead.
			return nil, fmt.Errorf("idempotent lookup: discovered=%s approved_asset_id=%s: %w",
				discoveredID, existingID, getErr)
		}
		if commitErr := tx.Commit(ctx); commitErr != nil {
			return nil, fmt.Errorf("commit idempotent: %w", commitErr)
		}
		return &ApproveResult{
			Asset:            existing,
			Discovered:       da,
			Created:          false,
			ReviewerID:       reviewerID,
			ApprovedAssetTag: existing.AssetTag,
		}, nil
	}

	// 3. Synthesize asset params. Mapping rules:
	//    - hostname → name (fall back to source+external_id if hostname empty)
	//    - ip_address → ip_address
	//    - asset_tag: synthetic "DSC-<first-8-of-uuid>"; asset_tag is globally
	//      unique so it must be deterministic per discovered_asset to keep
	//      idempotency honest under partial-commit replay.
	//    - status: 'inventoried' (matches chk_assets_status; a just-promoted
	//      row has not yet been physically deployed/verified).
	//    - type: 'server' is the safest default — discovery currently only
	//      sees host-like things. Future: infer from raw_data if present.
	//    - attributes: echo raw_data + discovery provenance (source,
	//      external_id, discovered_at) so the audit trail is not lost.
	assetTag := synthAssetTag(da.ID)
	name := displayName(da)

	attrs, err := buildAttributes(da)
	if err != nil {
		return nil, fmt.Errorf("build attributes: %w", err)
	}

	createParams := dbgen.CreateAssetParams{
		TenantID:   tenantID,
		AssetTag:   assetTag,
		Name:       name,
		Type:       "server",
		Status:     "inventoried",
		BiaLevel:   "normal",
		Attributes: attrs,
	}
	if da.IpAddress.Valid {
		// The generated struct does not expose ip_address as a direct column
		// on CreateAssetParams (historical gap in the INSERT query), so we
		// set it via UPDATE after create. Easier than regenerating the whole
		// CreateAsset query.
	}

	newAsset, err := qtx.CreateAsset(ctx, createParams)
	if err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrAssetAlreadyExists
		}
		return nil, fmt.Errorf("create asset: %w", err)
	}

	// 3a. Patch ip_address onto the fresh asset if we have one. Done here
	// (still inside the tx) so the post-commit row is consistent.
	if da.IpAddress.Valid && da.IpAddress.String != "" {
		if _, ipErr := tx.Exec(ctx,
			`UPDATE assets SET ip_address = $1, updated_at = now() WHERE id = $2 AND tenant_id = $3`,
			da.IpAddress.String, newAsset.ID, tenantID,
		); ipErr != nil {
			return nil, fmt.Errorf("set asset ip_address: %w", ipErr)
		}
		newAsset.IpAddress = da.IpAddress
	}

	// 4. Link discovered_asset → asset, flip status.
	updatedDA, err := qtx.ApproveDiscoveredAsset(ctx, dbgen.ApproveDiscoveredAssetParams{
		ID:              discoveredID,
		TenantID:        tenantID,
		ApprovedAssetID: pgtype.UUID{Bytes: newAsset.ID, Valid: true},
		ReviewedBy:      pgtype.UUID{Bytes: reviewerID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("approve discovered: %w", err)
	}

	// 5. Audit INSIDE the tx so a commit failure takes the audit event down
	// with it. The handler will NOT record a second audit on return.
	diff := map[string]any{
		"discovered_asset_id": discoveredID.String(),
		"asset_id":            newAsset.ID.String(),
		"asset_tag":           newAsset.AssetTag,
		"source":              da.Source,
	}
	diffJSON, _ := json.Marshal(diff)
	if _, err := qtx.CreateAuditEvent(ctx, dbgen.CreateAuditEventParams{
		TenantID:     tenantID,
		Action:       "discovery.approved",
		Module:       pgtype.Text{String: "discovery", Valid: true},
		TargetType:   pgtype.Text{String: "discovered_asset", Valid: true},
		TargetID:     pgtype.UUID{Bytes: discoveredID, Valid: true},
		OperatorType: dbgen.AuditOperatorTypeUser,
		OperatorID:   pgtype.UUID{Bytes: reviewerID, Valid: true},
		Diff:         diffJSON,
		Source:       "api",
	}); err != nil {
		return nil, fmt.Errorf("audit: %w", err)
	}

	// 6. Commit. Only after this succeeds does the caller publish the event.
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	return &ApproveResult{
		Asset:            newAsset,
		Discovered:       updatedDA,
		Created:          true,
		ReviewerID:       reviewerID,
		ApprovedAssetTag: newAsset.AssetTag,
	}, nil
}

// Approve is the legacy thin wrapper. Preserved so callers that only want
// the status flip (e.g. if a non-creation "approved" path ever appears) keep
// compiling. The API handler now uses ApproveAndCreateAsset.
//
// Deprecated: use ApproveAndCreateAsset.
func (s *Service) Approve(ctx context.Context, id, tenantID, reviewerID uuid.UUID) (*dbgen.DiscoveredAsset, error) {
	item, err := s.queries.ApproveDiscoveredAsset(ctx, dbgen.ApproveDiscoveredAssetParams{
		ID:         id,
		TenantID:   tenantID,
		ReviewedBy: pgtype.UUID{Bytes: reviewerID, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("approve discovered: %w", err)
	}
	return &item, nil
}

func (s *Service) Ignore(ctx context.Context, id, reviewerID uuid.UUID) (*dbgen.DiscoveredAsset, error) {
	item, err := s.queries.IgnoreDiscoveredAsset(ctx, dbgen.IgnoreDiscoveredAssetParams{ID: id, ReviewedBy: pgtype.UUID{Bytes: reviewerID, Valid: true}})
	if err != nil {
		return nil, fmt.Errorf("ignore discovered: %w", err)
	}
	return &item, nil
}

func (s *Service) GetStats(ctx context.Context, tenantID uuid.UUID) (*dbgen.GetDiscoveryStatsRow, error) {
	row, err := s.queries.GetDiscoveryStats(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get discovery stats: %w", err)
	}
	return &row, nil
}

// synthAssetTag builds a deterministic asset_tag for an auto-approved
// discovery row. Deterministic so retries of the handler before the FIRST
// commit finishes can't mint two different tags. "DSC-" prefix makes the
// provenance obvious in UIs and searches.
func synthAssetTag(discoveredID uuid.UUID) string {
	return "DSC-" + strings.ToUpper(discoveredID.String()[:8])
}

// displayName picks a human-readable name for the new asset. Priority:
// hostname → external_id → source+short id.
func displayName(da dbgen.DiscoveredAsset) string {
	if da.Hostname.Valid && strings.TrimSpace(da.Hostname.String) != "" {
		return da.Hostname.String
	}
	if da.ExternalID.Valid && strings.TrimSpace(da.ExternalID.String) != "" {
		return da.ExternalID.String
	}
	return fmt.Sprintf("%s-%s", da.Source, da.ID.String()[:8])
}

// buildAttributes preserves the discovery provenance and the raw_data blob
// on the promoted asset. Shape:
//
//	{
//	  "discovery": {
//	    "source":        "...",
//	    "external_id":   "...",
//	    "discovered_at": "...",
//	    "discovered_asset_id": "...",
//	    "raw_data":      { ... original ingestion payload ... }
//	  }
//	}
func buildAttributes(da dbgen.DiscoveredAsset) (json.RawMessage, error) {
	prov := map[string]any{
		"discovered_asset_id": da.ID.String(),
		"source":              da.Source,
		"discovered_at":       da.DiscoveredAt,
	}
	if da.ExternalID.Valid {
		prov["external_id"] = da.ExternalID.String
	}
	if len(da.RawData) > 0 && !bytes_equal(da.RawData, []byte("{}")) {
		var raw any
		if err := json.Unmarshal(da.RawData, &raw); err == nil {
			prov["raw_data"] = raw
		}
	}
	out := map[string]any{"discovery": prov}
	data, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func bytes_equal(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// isDuplicateKeyError classifies Postgres unique-violation errors without
// requiring a pgconn.PgError dependency surface in the caller. Follows the
// same pattern as impl_assets.go::CreateAsset.
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "SQLSTATE 23505")
}
