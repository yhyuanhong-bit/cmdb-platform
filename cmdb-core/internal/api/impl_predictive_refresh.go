package api

import (
	"errors"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/predictive"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// formatPgNumeric stringifies a pgx Numeric for JSON. Decimal precision
// is preserved; NULL collapses to "0". (Originally lived in the energy
// billing handlers; moved here when those were retired.)
func formatPgNumeric(n pgtype.Numeric) string {
	if !n.Valid {
		return "0"
	}
	v, err := n.Value()
	if err != nil {
		return "0"
	}
	if s, ok := v.(string); ok {
		return s
	}
	return "0"
}

// ListPredictiveRefresh — GET /predictive/refresh
func (s *APIServer) ListPredictiveRefresh(c *gin.Context, params ListPredictiveRefreshParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)
	p := predictive.ListParams{TenantID: tenantID, Limit: limit, Offset: offset}
	if params.Status != nil {
		v := string(*params.Status)
		p.Status = &v
	}
	if params.Kind != nil {
		v := string(*params.Kind)
		p.Kind = &v
	}
	rows, total, err := s.predictiveSvc.List(c.Request.Context(), p)
	if err != nil {
		response.InternalError(c, "failed to list recommendations")
		return
	}
	out := make([]PredictiveRefresh, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIPredictiveRefresh(r))
	}
	response.OKList(c, out, page, pageSize, int(total))
}

// GetPredictiveRefreshAggregate — GET /predictive/refresh/aggregate
//
// Server-side roll-up so the capex chart doesn't have to fetch the
// whole list and bucket in JS. Only open recommendations with a
// non-NULL target_date are counted (see SQL doc for the rationale).
func (s *APIServer) GetPredictiveRefreshAggregate(c *gin.Context, params GetPredictiveRefreshAggregateParams) {
	tenantID := tenantIDFromContext(c)

	arg := dbgen.AggregatePredictiveRefreshByMonthParams{TenantID: tenantID}
	if params.From != nil {
		arg.FromMonth = pgtype.Date{Time: params.From.Time, Valid: true}
	}
	if params.To != nil {
		arg.ToMonth = pgtype.Date{Time: params.To.Time, Valid: true}
	}

	rows, err := dbgen.New(s.pool).AggregatePredictiveRefreshByMonth(c.Request.Context(), arg)
	if err != nil {
		response.InternalError(c, "failed to aggregate recommendations")
		return
	}

	out := make([]PredictiveRefreshAggregateBucket, 0, len(rows))
	for _, r := range rows {
		out = append(out, PredictiveRefreshAggregateBucket{
			Month:            openapi_types.Date{Time: r.Month.Time},
			Count:            r.Count,
			WarrantyExpiring: r.WarrantyExpiring,
			WarrantyExpired:  r.WarrantyExpired,
			EolApproaching:   r.EolApproaching,
			EolPassed:        r.EolPassed,
			AgedOut:          r.AgedOut,
		})
	}
	response.OK(c, out)
}

// RunPredictiveRefreshScan — POST /predictive/refresh/run
func (s *APIServer) RunPredictiveRefreshScan(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	scan, err := s.predictiveSvc.ScanAndUpsert(c.Request.Context(), tenantID, predictive.DefaultRuleConfig())
	if err != nil {
		response.InternalError(c, "scan failed")
		return
	}
	s.recordAudit(c, "predictive.refresh_scan_run", "predictive", "tenant", tenantID, map[string]any{
		"assets_scanned": scan.AssetsScanned,
		"rows_upserted":  scan.RowsUpserted,
	})
	response.OK(c, gin.H{
		"assets_scanned": scan.AssetsScanned,
		"rows_upserted":  scan.RowsUpserted,
	})
}

// TransitionPredictiveRefresh — POST /predictive/refresh/{assetId}/{kind}/transition
func (s *APIServer) TransitionPredictiveRefresh(c *gin.Context, assetId openapi_types.UUID, kind TransitionPredictiveRefreshParamsKind) {
	var body TransitionPredictiveRefreshJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tenantID := tenantIDFromContext(c)
	userID := userIDFromContext(c)
	note := ""
	if body.Note != nil {
		note = *body.Note
	}
	row, err := s.predictiveSvc.Transition(
		c.Request.Context(),
		tenantID,
		uuid.UUID(assetId),
		string(kind),
		string(body.Status),
		userID,
		note,
	)
	if err != nil {
		if errors.Is(err, predictive.ErrNotFound) {
			response.NotFound(c, "recommendation not found")
			return
		}
		if strings.Contains(err.Error(), "invalid status") {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to transition recommendation")
		return
	}
	s.recordAudit(c, "predictive.refresh_transitioned", "predictive", "asset", uuid.UUID(assetId), map[string]any{
		"kind":   kind,
		"status": body.Status,
	})
	response.OK(c, toAPIPredictiveRefreshFromRecord(*row))
}

// ---------------------------------------------------------------------------
// Converters.
// ---------------------------------------------------------------------------

func toAPIPredictiveRefresh(db dbgen.ListPredictiveRefreshRow) PredictiveRefresh {
	out := PredictiveRefresh{
		AssetId:    openapi_types.UUID(db.AssetID),
		Kind:       PredictiveRefreshKind(db.Kind),
		RiskScore:  formatPgNumeric(db.RiskScore),
		Reason:     db.Reason,
		Status:     PredictiveRefreshStatus(db.Status),
		DetectedAt: db.DetectedAt,
	}
	if db.AssetTag != "" {
		s := db.AssetTag
		out.AssetTag = &s
	}
	if db.AssetName != "" {
		s := db.AssetName
		out.AssetName = &s
	}
	if db.AssetType != "" {
		s := db.AssetType
		out.AssetType = &s
	}
	if db.LocationID.Valid {
		u := uuid.UUID(db.LocationID.Bytes)
		oid := openapi_types.UUID(u)
		out.LocationId = &oid
	}
	if db.RecommendedAction.Valid {
		s := db.RecommendedAction.String
		out.RecommendedAction = &s
	}
	if db.TargetDate.Valid {
		d := openapi_types.Date{Time: db.TargetDate.Time}
		out.TargetDate = &d
	}
	if db.ReviewedBy.Valid {
		u := uuid.UUID(db.ReviewedBy.Bytes)
		oid := openapi_types.UUID(u)
		out.ReviewedBy = &oid
	}
	if db.ReviewedAt.Valid {
		t := db.ReviewedAt.Time
		out.ReviewedAt = &t
	}
	if db.Note.Valid {
		s := db.Note.String
		out.Note = &s
	}
	if db.PurchaseDate.Valid {
		d := openapi_types.Date{Time: db.PurchaseDate.Time}
		out.PurchaseDate = &d
	}
	if db.WarrantyEnd.Valid {
		d := openapi_types.Date{Time: db.WarrantyEnd.Time}
		out.WarrantyEnd = &d
	}
	if db.EolDate.Valid {
		d := openapi_types.Date{Time: db.EolDate.Time}
		out.EolDate = &d
	}
	return out
}

func toAPIPredictiveRefreshFromRecord(db dbgen.PredictiveRefreshRecommendation) PredictiveRefresh {
	out := PredictiveRefresh{
		AssetId:    openapi_types.UUID(db.AssetID),
		Kind:       PredictiveRefreshKind(db.Kind),
		RiskScore:  formatPgNumeric(db.RiskScore),
		Reason:     db.Reason,
		Status:     PredictiveRefreshStatus(db.Status),
		DetectedAt: db.DetectedAt,
	}
	if db.RecommendedAction.Valid {
		s := db.RecommendedAction.String
		out.RecommendedAction = &s
	}
	if db.TargetDate.Valid {
		d := openapi_types.Date{Time: db.TargetDate.Time}
		out.TargetDate = &d
	}
	if db.ReviewedBy.Valid {
		u := uuid.UUID(db.ReviewedBy.Bytes)
		oid := openapi_types.UUID(u)
		out.ReviewedBy = &oid
	}
	if db.ReviewedAt.Valid {
		t := db.ReviewedAt.Time
		out.ReviewedAt = &t
	}
	if db.Note.Valid {
		s := db.Note.String
		out.Note = &s
	}
	return out
}
