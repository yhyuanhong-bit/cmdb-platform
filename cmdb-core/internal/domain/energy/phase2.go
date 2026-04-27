package energy

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/shopspring/decimal"
)

// Wave 6.2: PUE rollup, anomaly detection, and the scheduler that runs
// both nightly. PUE math is computed on read (see ListLocationDailyPue
// query) so the persisted row is just the kWh inputs — there's no risk
// of a stored PUE drifting from a recomputed daily kWh.

// AnomalyConfig tunes the rule-based anomaly detector. The defaults match
// the values that DC-engineering teams typically set on day one — high
// threshold of 2.0 catches a doubled load (often a runaway process or
// stuck fan), low threshold of 0.3 catches a hung asset that's drawing
// far less power than normal.
type AnomalyConfig struct {
	// HighThreshold — observed / median ≥ this triggers a 'high' anomaly.
	HighThreshold decimal.Decimal
	// LowThreshold — observed / median ≤ this AND observed > 0 triggers
	// a 'low' anomaly. Zero-kWh days are not anomalies; the lack of a
	// power_kw sample is owned by the alerts pipeline.
	LowThreshold decimal.Decimal
	// BaselineWindowDays — how far back to look for the median baseline.
	// 7 days is a sensible default that captures a full weekly pattern;
	// shorter windows (e.g. 3) are noisier on assets with weekday-only
	// load shapes.
	BaselineWindowDays int
	// MinSampleCount — require at least this many days of history before
	// scoring; with too few baselines the median is meaningless and we'd
	// alert on every new asset for its first week.
	MinSampleCount int
}

// DefaultAnomalyConfig returns sensible production defaults.
func DefaultAnomalyConfig() AnomalyConfig {
	return AnomalyConfig{
		HighThreshold:      decimal.NewFromFloat(2.0),
		LowThreshold:       decimal.NewFromFloat(0.3),
		BaselineWindowDays: 7,
		MinSampleCount:     3,
	}
}

// AggregateLocationDay rolls up energy_daily_kwh rows for the given
// (tenant, day) into one energy_location_daily row per location, split
// by IT vs non-IT asset type. Idempotent.
func (s *Service) AggregateLocationDay(ctx context.Context, tenantID uuid.UUID, day time.Time) error {
	return s.queries.AggregateLocationDayPue(ctx, dbgen.AggregateLocationDayPueParams{
		TenantID: tenantID,
		Day:      pgtype.Date{Time: truncateToDay(day), Valid: true},
	})
}

// LocationDailyRow is the API-shape result of ListLocationDailyPue,
// re-exported from the domain so handlers and tests share one type.
type LocationDailyRow = dbgen.ListLocationDailyPueRow

// ListLocationDailyPue returns per-location daily PUE rows. PUE column
// is NULL when it_kwh is zero (operator sees that as "no IT load
// recorded" rather than ∞).
func (s *Service) ListLocationDailyPue(ctx context.Context, tenantID uuid.UUID, locationID *uuid.UUID, dayFrom, dayTo time.Time) ([]LocationDailyRow, error) {
	params := dbgen.ListLocationDailyPueParams{
		TenantID: tenantID,
		DayFrom:  pgtype.Date{Time: truncateToDay(dayFrom), Valid: true},
		DayTo:    pgtype.Date{Time: truncateToDay(dayTo), Valid: true},
	}
	if locationID != nil && *locationID != uuid.Nil {
		params.LocationID = pgtype.UUID{Bytes: *locationID, Valid: true}
	}
	return s.queries.ListLocationDailyPue(ctx, params)
}

// DetectAnomaliesForDay runs the rule-based anomaly detector for every
// asset that has a daily row on `day`, comparing observed kWh against
// the trailing-window median. Persists one energy_anomalies row per
// flagged (asset, day). Idempotent on re-run.
//
// The rule set matches AnomalyConfig:
//   - score = observed / median
//   - high anomaly when score ≥ HighThreshold
//   - low  anomaly when score ≤ LowThreshold AND observed > 0
//   - skip assets with fewer than MinSampleCount baseline days
//   - skip days where median is zero (would force divide-by-zero)
//
// Returns the count of anomalies persisted on this run (new + overwrites
// of existing rows).
func (s *Service) DetectAnomaliesForDay(ctx context.Context, tenantID uuid.UUID, day time.Time, cfg AnomalyConfig) (int, error) {
	dayDate := truncateToDay(day)
	assets, err := s.queries.ListAssetsWithDayKwh(ctx, dbgen.ListAssetsWithDayKwhParams{
		TenantID: tenantID,
		Day:      pgtype.Date{Time: dayDate, Valid: true},
	})
	if err != nil {
		return 0, fmt.Errorf("list assets with day kwh: %w", err)
	}

	flagged := 0
	for _, a := range assets {
		observed := pgNumericToDecimal(a.KwhTotal)
		baseline, err := s.queries.ComputeAssetBaselineMedian(ctx, dbgen.ComputeAssetBaselineMedianParams{
			TenantID:   tenantID,
			AssetID:    a.AssetID,
			Day:        pgtype.Date{Time: dayDate, Valid: true},
			WindowDays: int32(cfg.BaselineWindowDays),
		})
		if err != nil {
			return flagged, fmt.Errorf("baseline asset=%s: %w", a.AssetID, err)
		}
		if int(baseline.SampleCount) < cfg.MinSampleCount {
			continue // not enough history to score
		}
		median := pgNumericToDecimal(baseline.MedianKwh)
		if !median.IsPositive() {
			continue // can't divide by zero baseline
		}
		score := observed.Div(median)

		// Determine if this row crosses a threshold.
		var kind string
		switch {
		case score.Cmp(cfg.HighThreshold) >= 0:
			kind = "high"
		case observed.IsPositive() && score.Cmp(cfg.LowThreshold) <= 0:
			kind = "low"
		default:
			continue
		}

		obsNumeric, err := decimalToPgNumeric(observed)
		if err != nil {
			return flagged, fmt.Errorf("encode observed: %w", err)
		}
		medNumeric, err := decimalToPgNumeric(median)
		if err != nil {
			return flagged, fmt.Errorf("encode median: %w", err)
		}
		scoreNumeric, err := decimalToPgNumeric(score)
		if err != nil {
			return flagged, fmt.Errorf("encode score: %w", err)
		}

		if err := s.queries.UpsertEnergyAnomaly(ctx, dbgen.UpsertEnergyAnomalyParams{
			TenantID:       tenantID,
			AssetID:        a.AssetID,
			Day:            pgtype.Date{Time: dayDate, Valid: true},
			Kind:           kind,
			BaselineMedian: medNumeric,
			ObservedKwh:    obsNumeric,
			Score:          scoreNumeric,
		}); err != nil {
			return flagged, fmt.Errorf("upsert anomaly asset=%s: %w", a.AssetID, err)
		}
		flagged++
	}
	return flagged, nil
}

// ListAnomalies returns anomalies in the requested window, optionally
// filtered by status (open/ack/resolved). Joined with the asset row so
// the UI can render names without a second roundtrip.
func (s *Service) ListAnomalies(ctx context.Context, tenantID uuid.UUID, status *string, dayFrom, dayTo time.Time, limit, offset int32) ([]dbgen.ListEnergyAnomaliesRow, int64, error) {
	listParams := dbgen.ListEnergyAnomaliesParams{
		TenantID: tenantID,
		DayFrom:  pgtype.Date{Time: truncateToDay(dayFrom), Valid: true},
		DayTo:    pgtype.Date{Time: truncateToDay(dayTo), Valid: true},
		Limit:    limit,
		Offset:   offset,
	}
	countParams := dbgen.CountEnergyAnomaliesParams{
		TenantID: tenantID,
		DayFrom:  pgtype.Date{Time: truncateToDay(dayFrom), Valid: true},
		DayTo:    pgtype.Date{Time: truncateToDay(dayTo), Valid: true},
	}
	if status != nil && *status != "" {
		listParams.Status = pgtype.Text{String: *status, Valid: true}
		countParams.Status = pgtype.Text{String: *status, Valid: true}
	}

	rows, err := s.queries.ListEnergyAnomalies(ctx, listParams)
	if err != nil {
		return nil, 0, fmt.Errorf("list anomalies: %w", err)
	}
	total, err := s.queries.CountEnergyAnomalies(ctx, countParams)
	if err != nil {
		return nil, 0, fmt.Errorf("count anomalies: %w", err)
	}
	return rows, total, nil
}

// TransitionAnomaly flips an anomaly's status (open / ack / resolved).
// reviewerID is stamped on the row; note is appended (replaces).
func (s *Service) TransitionAnomaly(ctx context.Context, tenantID, assetID uuid.UUID, day time.Time, status string, reviewerID uuid.UUID, note string) (*dbgen.EnergyAnomaly, error) {
	if status != "open" && status != "ack" && status != "resolved" {
		return nil, errors.New("invalid anomaly status")
	}
	row, err := s.queries.TransitionEnergyAnomaly(ctx, dbgen.TransitionEnergyAnomalyParams{
		TenantID:   tenantID,
		AssetID:    assetID,
		Day:        pgtype.Date{Time: truncateToDay(day), Valid: true},
		Status:     status,
		ReviewedBy: pgtype.UUID{Bytes: reviewerID, Valid: reviewerID != uuid.Nil},
		Note:       pgtype.Text{String: note, Valid: note != ""},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("transition anomaly: %w", err)
	}
	return &row, nil
}

// ---------------------------------------------------------------------------
// Scheduler — runs the daily aggregator + PUE rollup + anomaly detector
// once per tick (default: hourly). Each tick processes "yesterday" (in
// UTC) for every tenant that has recent metrics. Idempotent on re-run
// because all three steps use UPSERT semantics.
// ---------------------------------------------------------------------------

// ScheduleTickResult is what one tick of RunDailyTick returns. Useful for
// telemetry counters and for the "did the scheduler run today?" health
// check.
type ScheduleTickResult struct {
	TenantsScanned    int
	AssetDaysAggregated int
	LocationsRolled   int
	AnomaliesFlagged  int
	Errors            []error
}

// RunDailyTick is the scheduler's per-tick entry point. It processes
// "yesterday" for every tenant that has power_kw samples in the last
// 48h. All three steps are idempotent so a duplicate tick on the same
// hour is a no-op rather than a double-count.
//
// Caller is expected to wrap this in a goroutine + ticker and to log
// the returned errors. We don't fail the tick on per-tenant error —
// one bad tenant shouldn't block the rest.
func (s *Service) RunDailyTick(ctx context.Context, anomalyCfg AnomalyConfig) ScheduleTickResult {
	res := ScheduleTickResult{}
	yesterday := truncateToDay(time.Now().UTC().AddDate(0, 0, -1))

	tenants, err := s.queries.ListTenantsWithRecentMetrics(ctx)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Errorf("list tenants: %w", err))
		return res
	}
	res.TenantsScanned = len(tenants)

	for _, t := range tenants {
		if !t.Valid {
			continue
		}
		tenantID := uuid.UUID(t.Bytes)
		count, err := s.AggregateRange(ctx, tenantID, yesterday, yesterday)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("tenant=%s aggregate: %w", tenantID, err))
			continue
		}
		res.AssetDaysAggregated += count

		if err := s.AggregateLocationDay(ctx, tenantID, yesterday); err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("tenant=%s pue: %w", tenantID, err))
			continue
		}
		// Best-effort count via re-querying — if this errors we still log
		// the tick result; the count is informational.
		var nLoc int
		_ = s.pool.QueryRow(ctx,
			`SELECT count(*) FROM energy_location_daily WHERE tenant_id=$1 AND day=$2`,
			tenantID, yesterday,
		).Scan(&nLoc)
		res.LocationsRolled += nLoc

		flagged, err := s.DetectAnomaliesForDay(ctx, tenantID, yesterday, anomalyCfg)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("tenant=%s anomalies: %w", tenantID, err))
			continue
		}
		res.AnomaliesFlagged += flagged
	}
	return res
}
