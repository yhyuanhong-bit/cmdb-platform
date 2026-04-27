// Package energy implements the billing accounting layer over the existing
// metrics-based energy telemetry. The platform already collects power_kw
// samples per asset; this package turns them into:
//
//   - tariffs       — $/kWh per location, valid in a date range, with
//                     no-overlap enforcement at the domain layer
//   - daily kWh    — pre-rolled (asset, day) energy totals from the
//                     metrics hypertable, idempotent re-aggregation
//   - monthly bill — per-asset cost for a date range, applying each
//                     asset's location-specific tariff (or tenant default)
//
// The tariff overlap rule deserves an explicit comment: two tariffs
// overlap when their date ranges intersect on the same (tenant,
// location) — open-ended ranges (effective_to NULL) overlap with any
// later range. We don't use a btree_gist EXCLUDE constraint because
// btree_gist isn't installed on the managed Postgres environment, so
// the rule lives in Go and is exercised by the unit tests.
package energy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var (
	// ErrNotFound — row isn't visible to the caller's tenant. Mapped to 404.
	ErrNotFound = errors.New("energy: not found")

	// ErrTariffOverlap — caller tried to insert/update a tariff whose
	// effective range overlaps with another tariff on the same
	// (tenant, location). Mapped to 409.
	ErrTariffOverlap = errors.New("energy: tariff range overlaps an existing tariff")

	// ErrInvalidRange — effective_to before effective_from, or rate <= 0.
	// Mapped to 400.
	ErrInvalidRange = errors.New("energy: invalid tariff range or rate")

	// ErrNoTariff — no tariff (location-specific or tenant default)
	// covers the requested day. Mapped to 409 with a hint on what to fix.
	ErrNoTariff = errors.New("energy: no tariff applies to the requested day")
)

type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

func NewService(queries *dbgen.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool}
}

// ---------------------------------------------------------------------------
// Tariffs.
// ---------------------------------------------------------------------------

type CreateTariffParams struct {
	TenantID      uuid.UUID
	LocationID    *uuid.UUID // nil = tenant default
	Currency      string     // "USD", "TWD", … defaults to "USD"
	RatePerKWh    decimal.Decimal
	EffectiveFrom time.Time // truncated to DATE
	EffectiveTo   *time.Time
	Notes         string
}

func (s *Service) CreateTariff(ctx context.Context, p CreateTariffParams) (*dbgen.EnergyTariff, error) {
	if !p.RatePerKWh.IsPositive() {
		return nil, ErrInvalidRange
	}
	from := truncateToDay(p.EffectiveFrom)
	var to pgtype.Date
	if p.EffectiveTo != nil {
		t := truncateToDay(*p.EffectiveTo)
		if t.Before(from) {
			return nil, ErrInvalidRange
		}
		to = pgtype.Date{Time: t, Valid: true}
	}

	if err := s.assertNoOverlap(ctx, p.TenantID, p.LocationID, uuid.Nil, from, to); err != nil {
		return nil, err
	}

	currency := p.Currency
	if currency == "" {
		currency = "USD"
	}

	rate, err := decimalToPgNumeric(p.RatePerKWh)
	if err != nil {
		return nil, fmt.Errorf("encode rate: %w", err)
	}

	row, err := s.queries.CreateEnergyTariff(ctx, dbgen.CreateEnergyTariffParams{
		TenantID:      p.TenantID,
		LocationID:    pgtype.UUID{Bytes: derefOrNil(p.LocationID), Valid: p.LocationID != nil && *p.LocationID != uuid.Nil},
		Currency:      currency,
		RatePerKwh:    rate,
		EffectiveFrom: pgtype.Date{Time: from, Valid: true},
		EffectiveTo:   to,
		Notes:         pgtype.Text{String: p.Notes, Valid: p.Notes != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("create tariff: %w", err)
	}
	return &row, nil
}

type UpdateTariffParams struct {
	TenantID      uuid.UUID
	ID            uuid.UUID
	LocationID    *uuid.UUID // ignored — locality of a tariff is immutable
	Currency      *string
	RatePerKWh    *decimal.Decimal
	EffectiveFrom *time.Time
	EffectiveTo   *time.Time
	ClearEffectiveTo bool
	Notes         *string
}

// UpdateTariff lets the caller adjust rate / effective dates / notes.
// Locality of a tariff (which location it covers) is immutable: if you
// need to retarget a tariff, delete and recreate. That keeps the
// no-overlap reasoning simpler — moving locality + dates atomically
// while preserving overlap invariants is a footgun we don't need.
func (s *Service) UpdateTariff(ctx context.Context, p UpdateTariffParams) (*dbgen.EnergyTariff, error) {
	current, err := s.queries.GetEnergyTariff(ctx, dbgen.GetEnergyTariffParams{ID: p.ID, TenantID: p.TenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get tariff: %w", err)
	}

	from := current.EffectiveFrom.Time
	to := current.EffectiveTo
	if p.EffectiveFrom != nil {
		from = truncateToDay(*p.EffectiveFrom)
	}
	if p.EffectiveTo != nil {
		t := truncateToDay(*p.EffectiveTo)
		to = pgtype.Date{Time: t, Valid: true}
	} else if p.ClearEffectiveTo {
		to = pgtype.Date{Valid: false}
	}
	if to.Valid && to.Time.Before(from) {
		return nil, ErrInvalidRange
	}
	if p.RatePerKWh != nil && !p.RatePerKWh.IsPositive() {
		return nil, ErrInvalidRange
	}

	loc := current.LocationID
	var locPtr *uuid.UUID
	if loc.Valid {
		u := uuid.UUID(loc.Bytes)
		locPtr = &u
	}
	if err := s.assertNoOverlap(ctx, p.TenantID, locPtr, p.ID, from, to); err != nil {
		return nil, err
	}

	params := dbgen.UpdateEnergyTariffParams{ID: p.ID, TenantID: p.TenantID}
	if p.Currency != nil {
		params.Currency = pgtype.Text{String: *p.Currency, Valid: true}
	}
	if p.RatePerKWh != nil {
		rate, err := decimalToPgNumeric(*p.RatePerKWh)
		if err != nil {
			return nil, fmt.Errorf("encode rate: %w", err)
		}
		params.RatePerKwh = rate
	}
	if p.EffectiveFrom != nil {
		params.EffectiveFrom = pgtype.Date{Time: from, Valid: true}
	}
	if to.Valid {
		params.EffectiveTo = to
	} else if p.ClearEffectiveTo {
		// pgtype.Date zero value is invalid, so passing it through
		// COALESCE keeps the existing value. Direct UPDATE for the
		// clear-to-NULL case:
		if _, err := database.Scope(s.pool, p.TenantID).Exec(ctx,
			`UPDATE energy_tariffs SET effective_to = NULL, updated_at = now() WHERE tenant_id = $1 AND id = $2`,
			p.ID,
		); err != nil {
			return nil, fmt.Errorf("clear effective_to: %w", err)
		}
	}
	if p.Notes != nil {
		params.Notes = pgtype.Text{String: *p.Notes, Valid: true}
	}

	row, err := s.queries.UpdateEnergyTariff(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("update tariff: %w", err)
	}
	return &row, nil
}

func (s *Service) DeleteTariff(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := s.queries.DeleteEnergyTariff(ctx, dbgen.DeleteEnergyTariffParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("delete tariff: %w", err)
	}
	return nil
}

func (s *Service) ListTariffs(ctx context.Context, tenantID uuid.UUID) ([]dbgen.EnergyTariff, error) {
	return s.queries.ListEnergyTariffs(ctx, tenantID)
}

func (s *Service) GetTariff(ctx context.Context, tenantID, id uuid.UUID) (*dbgen.EnergyTariff, error) {
	row, err := s.queries.GetEnergyTariff(ctx, dbgen.GetEnergyTariffParams{ID: id, TenantID: tenantID})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("get tariff: %w", err)
	}
	return &row, nil
}

// assertNoOverlap rejects the (tenant, location, [from, to]) range if
// any other tariff's range intersects it. excludeID is the row being
// updated (uuid.Nil for inserts). Open-ended ranges are treated as
// extending to '9999-12-31' for the purposes of the overlap check.
func (s *Service) assertNoOverlap(
	ctx context.Context,
	tenantID uuid.UUID,
	locationID *uuid.UUID,
	excludeID uuid.UUID,
	from time.Time,
	to pgtype.Date,
) error {
	params := dbgen.CountOverlappingTariffsParams{
		TenantID:   tenantID,
		WindowFrom: pgtype.Date{Time: from, Valid: true},
	}
	if locationID != nil && *locationID != uuid.Nil {
		params.LocationID = pgtype.UUID{Bytes: *locationID, Valid: true}
	}
	if excludeID != uuid.Nil {
		params.ExcludeID = pgtype.UUID{Bytes: excludeID, Valid: true}
	}
	if to.Valid {
		params.WindowTo = to
	}
	n, err := s.queries.CountOverlappingTariffs(ctx, params)
	if err != nil {
		return fmt.Errorf("check overlap: %w", err)
	}
	if n > 0 {
		return ErrTariffOverlap
	}
	return nil
}

// ResolveTariff returns the rate that applies to a given (location, day),
// falling back to the tenant default. Returns ErrNoTariff if neither
// covers the day so the caller can surface a clear error rather than a
// silent zero-cost bill.
func (s *Service) ResolveTariff(ctx context.Context, tenantID uuid.UUID, locationID *uuid.UUID, day time.Time) (*dbgen.EnergyTariff, error) {
	params := dbgen.ResolveTariffForDayParams{
		TenantID: tenantID,
		Day:      pgtype.Date{Time: truncateToDay(day), Valid: true},
	}
	if locationID != nil && *locationID != uuid.Nil {
		params.LocationID = pgtype.UUID{Bytes: *locationID, Valid: true}
	}
	row, err := s.queries.ResolveTariffForDay(ctx, params)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNoTariff
		}
		return nil, fmt.Errorf("resolve tariff: %w", err)
	}
	return &row, nil
}

// ---------------------------------------------------------------------------
// Daily aggregation.
// ---------------------------------------------------------------------------

// AggregateDay rolls up one asset's energy use for one day from the
// metrics hypertable into energy_daily_kwh. Idempotent — re-running
// for the same (asset, day) overwrites the row.
//
// Callers typically run this for "yesterday" once a day from a cron
// job, but it's also safe to invoke ad-hoc when filling backfill gaps.
func (s *Service) AggregateDay(ctx context.Context, tenantID, assetID uuid.UUID, day time.Time) error {
	return s.queries.AggregateAssetDayKwh(ctx, dbgen.AggregateAssetDayKwhParams{
		TenantID: tenantID,
		AssetID:  assetID,
		Day:      pgtype.Date{Time: truncateToDay(day), Valid: true},
	})
}

// AggregateRange runs AggregateDay for each (asset, day) pair derived
// from distinct (asset_id) values present in the metrics table within
// [dayFrom, dayTo]. Used by the catch-up backfill workflow when the
// daily cron didn't run for a stretch.
func (s *Service) AggregateRange(ctx context.Context, tenantID uuid.UUID, dayFrom, dayTo time.Time) (int, error) {
	from := truncateToDay(dayFrom)
	to := truncateToDay(dayTo)
	if to.Before(from) {
		return 0, ErrInvalidRange
	}
	rows, err := database.Scope(s.pool, tenantID).Query(ctx, `
		SELECT DISTINCT m.asset_id
		FROM metrics m
		WHERE m.tenant_id = $1
		  AND m.name = 'power_kw'
		  AND m.time >= $2::date
		  AND m.time <  ($3::date + INTERVAL '1 day')
	`, from, to)
	if err != nil {
		return 0, fmt.Errorf("scan assets: %w", err)
	}
	defer rows.Close()
	var assetIDs []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return 0, fmt.Errorf("scan asset: %w", err)
		}
		assetIDs = append(assetIDs, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		for _, aid := range assetIDs {
			if err := s.AggregateDay(ctx, tenantID, aid, d); err != nil {
				return count, err
			}
			count++
		}
	}
	return count, nil
}

// ListDaily returns the per-(asset, day) rollup rows in the requested
// window. Capped server-side by the date range; if the caller needs
// pagination they should narrow the window.
func (s *Service) ListDaily(ctx context.Context, tenantID uuid.UUID, dayFrom, dayTo time.Time) ([]dbgen.ListDailyKwhForTenantRow, error) {
	return s.queries.ListDailyKwhForTenant(ctx, dbgen.ListDailyKwhForTenantParams{
		TenantID: tenantID,
		DayFrom:  pgtype.Date{Time: truncateToDay(dayFrom), Valid: true},
		DayTo:    pgtype.Date{Time: truncateToDay(dayTo), Valid: true},
	})
}

// ---------------------------------------------------------------------------
// Bill computation.
// ---------------------------------------------------------------------------

// AssetBillLine is the per-asset cost breakdown for a date range.
type AssetBillLine struct {
	AssetID    uuid.UUID
	LocationID *uuid.UUID
	KWh        decimal.Decimal
	RatePerKWh decimal.Decimal
	Cost       decimal.Decimal
	Currency   string
}

// Bill is the result of CalculateBill — the total plus per-asset lines.
// Currency is whatever the resolved tariffs use; if assets in the range
// span tariffs in different currencies, the total is undefined and
// CurrencyMixed is set to true so the caller can refuse to display
// "$25 + €18 = 43".
type Bill struct {
	TenantID      uuid.UUID
	DayFrom       time.Time
	DayTo         time.Time
	TotalKWh      decimal.Decimal
	TotalCost     decimal.Decimal
	Currency      string
	CurrencyMixed bool
	Lines         []AssetBillLine
}

// CalculateBill iterates each asset that has kWh in the window, resolves
// the tariff applying to (location, midpoint of window), and produces a
// per-asset cost line. The midpoint heuristic keeps the math simple while
// correctly handling the common case of a single tariff applying to the
// whole month; for windows that straddle a renewal you should split the
// query.
func (s *Service) CalculateBill(ctx context.Context, tenantID uuid.UUID, dayFrom, dayTo time.Time) (*Bill, error) {
	from := truncateToDay(dayFrom)
	to := truncateToDay(dayTo)
	if to.Before(from) {
		return nil, ErrInvalidRange
	}

	rows, err := s.queries.ListAssetsWithKwhInRange(ctx, dbgen.ListAssetsWithKwhInRangeParams{
		TenantID: tenantID,
		DayFrom:  pgtype.Date{Time: from, Valid: true},
		DayTo:    pgtype.Date{Time: to, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("list bill assets: %w", err)
	}

	midpoint := from.Add(to.Sub(from) / 2)

	bill := &Bill{
		TenantID: tenantID,
		DayFrom:  from,
		DayTo:    to,
		TotalKWh: decimal.Zero,
		TotalCost: decimal.Zero,
	}
	currencies := make(map[string]struct{})

	for _, r := range rows {
		var locPtr *uuid.UUID
		if r.LocationID.Valid {
			u := uuid.UUID(r.LocationID.Bytes)
			locPtr = &u
		}
		tariff, err := s.ResolveTariff(ctx, tenantID, locPtr, midpoint)
		if err != nil {
			if errors.Is(err, ErrNoTariff) {
				// Skip this asset — caller will see fewer lines than
				// SumDailyKwh and can investigate. We don't want to
				// silently zero-cost it.
				continue
			}
			return nil, err
		}
		kwh := pgNumericToDecimal(r.AssetKwh)
		rate := pgNumericToDecimal(tariff.RatePerKwh)
		cost := kwh.Mul(rate)

		bill.TotalKWh = bill.TotalKWh.Add(kwh)
		bill.TotalCost = bill.TotalCost.Add(cost)
		currencies[tariff.Currency] = struct{}{}

		bill.Lines = append(bill.Lines, AssetBillLine{
			AssetID:    r.AssetID,
			LocationID: locPtr,
			KWh:        kwh,
			RatePerKWh: rate,
			Cost:       cost,
			Currency:   tariff.Currency,
		})
	}

	switch len(currencies) {
	case 0:
		bill.Currency = "USD" // default when no rows; total is zero anyway
	case 1:
		for c := range currencies {
			bill.Currency = c
		}
	default:
		bill.CurrencyMixed = true
		bill.Currency = "MIXED"
	}

	return bill, nil
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func derefOrNil(p *uuid.UUID) uuid.UUID {
	if p == nil {
		return uuid.Nil
	}
	return *p
}

// decimalToPgNumeric converts a shopspring/decimal to pgtype.Numeric. We
// do this via the canonical string form because pgtype.Numeric's direct
// constructor takes the int128 mantissa + exponent which is brittle for
// fractional rates like 0.1234.
func decimalToPgNumeric(d decimal.Decimal) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(d.String()); err != nil {
		return n, err
	}
	return n, nil
}

func pgNumericToDecimal(n pgtype.Numeric) decimal.Decimal {
	if !n.Valid {
		return decimal.Zero
	}
	v, err := n.Value()
	if err != nil {
		return decimal.Zero
	}
	s, ok := v.(string)
	if !ok {
		return decimal.Zero
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}
