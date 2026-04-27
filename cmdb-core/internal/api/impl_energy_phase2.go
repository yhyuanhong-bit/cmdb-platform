package api

import (
	"errors"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/energy"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
)

// ListEnergyPue — GET /energy/pue
func (s *APIServer) ListEnergyPue(c *gin.Context, params ListEnergyPueParams) {
	tenantID := tenantIDFromContext(c)
	var locPtr *uuid.UUID
	if params.LocationId != nil {
		u := uuid.UUID(*params.LocationId)
		locPtr = &u
	}
	rows, err := s.energySvc.ListLocationDailyPue(c.Request.Context(), tenantID, locPtr, params.DayFrom.Time, params.DayTo.Time)
	if err != nil {
		response.InternalError(c, "failed to list pue rows")
		return
	}
	out := make([]EnergyLocationPue, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIEnergyPue(r))
	}
	response.OK(c, out)
}

// ListEnergyAnomalies — GET /energy/anomalies
func (s *APIServer) ListEnergyAnomalies(c *gin.Context, params ListEnergyAnomaliesParams) {
	tenantID := tenantIDFromContext(c)
	page, pageSize, limit, offset := paginationDefaults(params.Page, params.PageSize)
	var statusPtr *string
	if params.Status != nil {
		s := string(*params.Status)
		statusPtr = &s
	}
	rows, total, err := s.energySvc.ListAnomalies(c.Request.Context(), tenantID, statusPtr, params.DayFrom.Time, params.DayTo.Time, limit, offset)
	if err != nil {
		response.InternalError(c, "failed to list anomalies")
		return
	}
	out := make([]EnergyAnomaly, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIEnergyAnomaly(r))
	}
	response.OKList(c, out, page, pageSize, int(total))
}

// TransitionEnergyAnomaly — POST /energy/anomalies/{assetId}/{day}/transition
func (s *APIServer) TransitionEnergyAnomaly(c *gin.Context, assetId openapi_types.UUID, day openapi_types.Date) {
	var body TransitionEnergyAnomalyJSONRequestBody
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
	row, err := s.energySvc.TransitionAnomaly(
		c.Request.Context(),
		tenantID,
		uuid.UUID(assetId),
		day.Time,
		string(body.Status),
		userID,
		note,
	)
	if err != nil {
		if errors.Is(err, energy.ErrNotFound) {
			response.NotFound(c, "anomaly not found")
			return
		}
		if strings.Contains(err.Error(), "invalid anomaly status") {
			response.BadRequest(c, err.Error())
			return
		}
		response.InternalError(c, "failed to transition anomaly")
		return
	}
	s.recordAudit(c, "energy.anomaly_transitioned", "energy", "anomaly", uuid.UUID(assetId), map[string]any{
		"day": day.Time.Format("2006-01-02"), "status": body.Status,
	})
	response.OK(c, toAPIEnergyAnomalyFromRecord(*row))
}

// ---------------------------------------------------------------------------
// Converters.
// ---------------------------------------------------------------------------

func toAPIEnergyPue(db dbgen.ListLocationDailyPueRow) EnergyLocationPue {
	out := EnergyLocationPue{
		LocationId:      openapi_types.UUID(db.LocationID),
		Day:             openapi_types.Date{Time: db.Day.Time},
		ItKwh:           formatPgNumeric(db.ItKwh),
		NonItKwh:        formatPgNumeric(db.NonItKwh),
		TotalKwh:        formatPgNumeric(db.TotalKwh),
		ItAssetCount:    int(db.ItAssetCount),
		NonItAssetCount: int(db.NonItAssetCount),
		ComputedAt:      ptrTime(db.ComputedAt),
	}
	if db.LocationName != "" {
		s := db.LocationName
		out.LocationName = &s
	}
	if db.LocationLevel != "" {
		s := db.LocationLevel
		out.LocationLevel = &s
	}
	if db.Pue.Valid {
		s := formatPgNumeric(db.Pue)
		out.Pue = &s
	}
	return out
}

func toAPIEnergyAnomaly(db dbgen.ListEnergyAnomaliesRow) EnergyAnomaly {
	out := EnergyAnomaly{
		AssetId:        openapi_types.UUID(db.AssetID),
		Day:            openapi_types.Date{Time: db.Day.Time},
		Kind:           EnergyAnomalyKind(db.Kind),
		BaselineMedian: formatPgNumeric(db.BaselineMedian),
		ObservedKwh:    formatPgNumeric(db.ObservedKwh),
		Score:          formatPgNumeric(db.Score),
		Status:         EnergyAnomalyStatus(db.Status),
		DetectedAt:     db.DetectedAt,
	}
	if db.AssetTag != "" {
		s := db.AssetTag
		out.AssetTag = &s
	}
	if db.AssetName != "" {
		s := db.AssetName
		out.AssetName = &s
	}
	if db.LocationID.Valid {
		u := uuid.UUID(db.LocationID.Bytes)
		oid := openapi_types.UUID(u)
		out.LocationId = &oid
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

func toAPIEnergyAnomalyFromRecord(db dbgen.EnergyAnomaly) EnergyAnomaly {
	out := EnergyAnomaly{
		AssetId:        openapi_types.UUID(db.AssetID),
		Day:            openapi_types.Date{Time: db.Day.Time},
		Kind:           EnergyAnomalyKind(db.Kind),
		BaselineMedian: formatPgNumeric(db.BaselineMedian),
		ObservedKwh:    formatPgNumeric(db.ObservedKwh),
		Score:          formatPgNumeric(db.Score),
		Status:         EnergyAnomalyStatus(db.Status),
		DetectedAt:     db.DetectedAt,
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
