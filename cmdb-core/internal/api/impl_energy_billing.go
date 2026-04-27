package api

import (
	"errors"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/energy"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/shopspring/decimal"
)

// ---------------------------------------------------------------------------
// Wave 6.1: energy billing handlers.
//
// Money values cross the wire as strings (per the OpenAPI schema) so we
// avoid float drift on multi-asset totals. The decimal package handles
// parse + arithmetic; pgtype.Numeric is the storage type.
// ---------------------------------------------------------------------------

func (s *APIServer) ListEnergyTariffs(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.energySvc.ListTariffs(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to list tariffs")
		return
	}
	out := make([]EnergyTariff, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIEnergyTariff(r))
	}
	response.OK(c, out)
}

func (s *APIServer) GetEnergyTariff(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	row, err := s.energySvc.GetTariff(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, energy.ErrNotFound) {
			response.NotFound(c, "tariff not found")
			return
		}
		response.InternalError(c, "failed to load tariff")
		return
	}
	response.OK(c, toAPIEnergyTariff(*row))
}

func (s *APIServer) CreateEnergyTariff(c *gin.Context) {
	var body CreateEnergyTariffJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if body.RatePerKwh == "" {
		response.BadRequest(c, "rate_per_kwh required")
		return
	}
	rate, err := decimal.NewFromString(body.RatePerKwh)
	if err != nil {
		response.BadRequest(c, "rate_per_kwh must be a decimal string")
		return
	}

	p := energy.CreateTariffParams{
		TenantID:      tenantIDFromContext(c),
		RatePerKWh:    rate,
		EffectiveFrom: body.EffectiveFrom.Time,
	}
	if body.LocationId != nil {
		u := uuid.UUID(*body.LocationId)
		p.LocationID = &u
	}
	if body.Currency != nil {
		p.Currency = *body.Currency
	}
	if body.EffectiveTo != nil {
		t := body.EffectiveTo.Time
		p.EffectiveTo = &t
	}
	if body.Notes != nil {
		p.Notes = *body.Notes
	}

	row, err := s.energySvc.CreateTariff(c.Request.Context(), p)
	if err != nil {
		s.writeEnergyTariffErr(c, err)
		return
	}
	s.recordAudit(c, "energy.tariff_created", "energy", "tariff", row.ID, nil)
	response.Created(c, toAPIEnergyTariff(*row))
}

func (s *APIServer) UpdateEnergyTariff(c *gin.Context, id IdPath) {
	var body UpdateEnergyTariffJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	p := energy.UpdateTariffParams{
		TenantID: tenantIDFromContext(c),
		ID:       uuid.UUID(id),
	}
	if body.Currency != nil {
		p.Currency = body.Currency
	}
	if body.RatePerKwh != nil {
		rate, err := decimal.NewFromString(*body.RatePerKwh)
		if err != nil {
			response.BadRequest(c, "rate_per_kwh must be a decimal string")
			return
		}
		p.RatePerKWh = &rate
	}
	if body.EffectiveFrom != nil {
		t := body.EffectiveFrom.Time
		p.EffectiveFrom = &t
	}
	if body.EffectiveTo != nil {
		t := body.EffectiveTo.Time
		p.EffectiveTo = &t
	}
	if body.ClearEffectiveTo != nil && *body.ClearEffectiveTo {
		p.ClearEffectiveTo = true
	}
	if body.Notes != nil {
		p.Notes = body.Notes
	}

	row, err := s.energySvc.UpdateTariff(c.Request.Context(), p)
	if err != nil {
		s.writeEnergyTariffErr(c, err)
		return
	}
	s.recordAudit(c, "energy.tariff_updated", "energy", "tariff", row.ID, nil)
	response.OK(c, toAPIEnergyTariff(*row))
}

func (s *APIServer) DeleteEnergyTariff(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	if err := s.energySvc.DeleteTariff(c.Request.Context(), tenantID, uuid.UUID(id)); err != nil {
		response.InternalError(c, "failed to delete tariff")
		return
	}
	s.recordAudit(c, "energy.tariff_deleted", "energy", "tariff", uuid.UUID(id), nil)
	c.Status(204)
}

func (s *APIServer) writeEnergyTariffErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, energy.ErrNotFound):
		response.NotFound(c, "tariff not found")
	case errors.Is(err, energy.ErrTariffOverlap):
		response.Err(c, 409, "ENERGY_TARIFF_OVERLAP",
			"tariff date range overlaps an existing tariff for the same location")
	case errors.Is(err, energy.ErrInvalidRange):
		response.BadRequest(c, "invalid tariff range or rate")
	default:
		response.InternalError(c, "failed to apply tariff change")
	}
}

// AggregateEnergyDaily — POST /energy/billing/aggregate
func (s *APIServer) AggregateEnergyDaily(c *gin.Context) {
	var body AggregateEnergyDailyJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tenantID := tenantIDFromContext(c)
	count, err := s.energySvc.AggregateRange(c.Request.Context(), tenantID, body.DayFrom.Time, body.DayTo.Time)
	if err != nil {
		if errors.Is(err, energy.ErrInvalidRange) {
			response.BadRequest(c, "day_to must be on or after day_from")
			return
		}
		response.InternalError(c, "failed to aggregate")
		return
	}
	response.OK(c, gin.H{"aggregated_count": count})
}

// ListEnergyDailyKwh — GET /energy/billing/daily
func (s *APIServer) ListEnergyDailyKwh(c *gin.Context, params ListEnergyDailyKwhParams) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.energySvc.ListDaily(c.Request.Context(), tenantID, params.DayFrom.Time, params.DayTo.Time)
	if err != nil {
		response.InternalError(c, "failed to list daily kwh")
		return
	}
	out := make([]EnergyDailyKwh, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIEnergyDailyKwh(r))
	}
	response.OK(c, out)
}

// GetEnergyBill — GET /energy/billing/bill
func (s *APIServer) GetEnergyBill(c *gin.Context, params GetEnergyBillParams) {
	tenantID := tenantIDFromContext(c)
	bill, err := s.energySvc.CalculateBill(c.Request.Context(), tenantID, params.DayFrom.Time, params.DayTo.Time)
	if err != nil {
		if errors.Is(err, energy.ErrInvalidRange) {
			response.BadRequest(c, "day_to must be on or after day_from")
			return
		}
		response.InternalError(c, "failed to compute bill")
		return
	}
	response.OK(c, toAPIEnergyBill(bill))
}

// ---------------------------------------------------------------------------
// Converters.
// ---------------------------------------------------------------------------

func toAPIEnergyTariff(db dbgen.EnergyTariff) EnergyTariff {
	out := EnergyTariff{
		Id:            db.ID,
		Currency:      db.Currency,
		RatePerKwh:    formatPgNumeric(db.RatePerKwh),
		EffectiveFrom: openapi_types.Date{Time: db.EffectiveFrom.Time},
		CreatedAt:     db.CreatedAt,
	}
	if db.LocationID.Valid {
		u := uuid.UUID(db.LocationID.Bytes)
		oid := openapi_types.UUID(u)
		out.LocationId = &oid
	}
	if db.EffectiveTo.Valid {
		d := openapi_types.Date{Time: db.EffectiveTo.Time}
		out.EffectiveTo = &d
	}
	if db.Notes.Valid {
		s := db.Notes.String
		out.Notes = &s
	}
	if !db.UpdatedAt.IsZero() {
		t := db.UpdatedAt
		out.UpdatedAt = &t
	}
	return out
}

func toAPIEnergyDailyKwh(db dbgen.ListDailyKwhForTenantRow) EnergyDailyKwh {
	out := EnergyDailyKwh{
		AssetId:      db.AssetID,
		Day:          openapi_types.Date{Time: db.Day.Time},
		KwhTotal:     formatPgNumeric(db.KwhTotal),
		KwPeak:       formatPgNumeric(db.KwPeak),
		KwAvg:        formatPgNumeric(db.KwAvg),
		SampleCount:  int(db.SampleCount),
		ComputedAt:   ptrTime(db.ComputedAt),
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
	return out
}

func toAPIEnergyBill(b *energy.Bill) EnergyBill {
	out := EnergyBill{
		DayFrom:       openapi_types.Date{Time: b.DayFrom},
		DayTo:         openapi_types.Date{Time: b.DayTo},
		TotalKwh:      b.TotalKWh.String(),
		TotalCost:     b.TotalCost.String(),
		Currency:      b.Currency,
		CurrencyMixed: b.CurrencyMixed,
	}
	lines := make([]EnergyBillLine, 0, len(b.Lines))
	for _, l := range b.Lines {
		line := EnergyBillLine{
			AssetId:    openapi_types.UUID(l.AssetID),
			Kwh:        l.KWh.String(),
			RatePerKwh: l.RatePerKWh.String(),
			Cost:       l.Cost.String(),
			Currency:   l.Currency,
		}
		if l.LocationID != nil {
			oid := openapi_types.UUID(*l.LocationID)
			line.LocationId = &oid
		}
		lines = append(lines, line)
	}
	out.Lines = &lines
	return out
}

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

func ptrTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	tc := t
	return &tc
}
