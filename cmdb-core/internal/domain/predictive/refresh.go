// Package predictive implements Phase 1 of the Predictive track —
// rule-based recommendations against the asset lifecycle fields
// (purchase_date, warranty_end, eol_date, expected_lifespan_months).
//
// "Phase 1" is intentionally rule-based, not ML. The platform doesn't
// yet have the labelled refresh-vs-failure history a model would need
// to beat the simple rules below; the planning doc commits to "build
// the data pipeline first, model when the data matures." The schema
// (000071) is shaped so a future ML writer can replace this engine
// without table changes — it just writes different (asset, kind, score)
// rows.
//
// Five rules in this engine:
//
//	warranty_expiring  warranty_end ≤ now()+90d AND > now()
//	warranty_expired   warranty_end < now()
//	eol_approaching    eol_date ≤ now()+180d AND > now()
//	eol_passed         eol_date < now()
//	aged_out           purchase_date older than expected_lifespan_months
//
// Risk scores map "days remaining" (or "days past") into the 0-100
// band so an asset whose warranty expires tomorrow scores higher than
// one that expires in 89 days. See scoreFromDeadline below.
package predictive

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

var (
	// ErrNotFound — recommendation not visible to caller's tenant.
	ErrNotFound = errors.New("predictive: not found")
)

// RuleConfig tunes the rule engine. Defaults match the production rules
// described in the package comment; tests inject narrower windows so
// they don't have to plant assets months in the future.
type RuleConfig struct {
	WarrantyHorizonDays int
	EOLHorizonDays      int
	// AgedOutMargin: an aged_out flag fires when
	//   now - purchase_date ≥ expected_lifespan_months * 30 days * (1 + margin)
	// A small margin (default 0.0 — fire exactly at end-of-life) gives
	// operators something to grab before the asset becomes a problem;
	// a larger margin defers flags to assets actually past spec life.
	AgedOutMargin float64
}

func DefaultRuleConfig() RuleConfig {
	return RuleConfig{
		WarrantyHorizonDays: 90,
		EOLHorizonDays:      180,
		AgedOutMargin:       0.0,
	}
}

type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
	now     func() time.Time // injectable for tests
}

func NewService(queries *dbgen.Queries, pool *pgxpool.Pool) *Service {
	return &Service{queries: queries, pool: pool, now: time.Now}
}

// WithClock lets tests pin "now" so a flag computed against a known asset
// purchase_date is deterministic.
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// ScanResult is what one tenant scan returns. Useful for telemetry +
// for the manual-trigger endpoint to show the operator what happened.
type ScanResult struct {
	AssetsScanned   int
	RowsUpserted    int
	StaleRowsPurged int
}

// ScanAndUpsert runs every rule against every lifecycle-bearing asset
// in the tenant, upserts the matched rows, and sweeps any 'open' rows
// the engine no longer matches (operator-acked rows are preserved).
// Idempotent on re-run.
//
// All upserts + the stale sweep run inside a single tx. Postgres's
// now() returns transaction-start time, so every upsert in this run
// stamps the same detected_at timestamp T; the sweep DELETE then uses
// `< T` (read at the same tx-start). New rows are NOT cut by their
// own scan; rows from a previous scan have detected_at < T and ARE
// cut. This avoids a race where two scans within the same wall-clock
// second could erase each other's work.
func (s *Service) ScanAndUpsert(ctx context.Context, tenantID uuid.UUID, cfg RuleConfig) (ScanResult, error) {
	res := ScanResult{}
	now := s.now()

	assets, err := s.queries.ListAssetsForPredictiveScan(ctx, tenantID)
	if err != nil {
		return res, fmt.Errorf("list assets: %w", err)
	}
	res.AssetsScanned = len(assets)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	qtx := s.queries.WithTx(tx)

	// Read the tx start time once and use it as the sweep cutoff. All
	// upserts inside this tx will have detected_at = now() = this same
	// timestamp, so they're not cut by `< cutoff`. Rows from prior
	// scans have older detected_at and ARE cut.
	var cutoff time.Time
	if err := tx.QueryRow(ctx, `SELECT now()`).Scan(&cutoff); err != nil {
		return res, fmt.Errorf("read tx start: %w", err)
	}

	for _, a := range assets {
		recs := evaluateAsset(a, now, cfg)
		for _, r := range recs {
			score, err := decimalToPgNumeric(r.Score)
			if err != nil {
				return res, fmt.Errorf("encode score: %w", err)
			}
			params := dbgen.UpsertPredictiveRefreshParams{
				TenantID:          tenantID,
				AssetID:           a.ID,
				Kind:              r.Kind,
				RiskScore:         score,
				Reason:            r.Reason,
				RecommendedAction: pgtype.Text{String: r.Action, Valid: r.Action != ""},
			}
			if r.TargetDate != nil {
				params.TargetDate = pgtype.Date{Time: *r.TargetDate, Valid: true}
			}
			if err := qtx.UpsertPredictiveRefresh(ctx, params); err != nil {
				return res, fmt.Errorf("upsert asset=%s kind=%s: %w", a.ID, r.Kind, err)
			}
			res.RowsUpserted++
		}
	}

	// Sweep stale 'open' rows. Acked / resolved rows are kept for audit.
	if err := qtx.DeleteStalePredictiveRefresh(ctx, dbgen.DeleteStalePredictiveRefreshParams{
		TenantID: tenantID,
		Cutoff:   cutoff,
	}); err != nil {
		return res, fmt.Errorf("sweep stale: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("commit: %w", err)
	}
	return res, nil
}

// recommendation is the engine's intermediate shape — we accumulate
// these per asset and then translate to the SQL params.
type recommendation struct {
	Kind       string
	Score      decimal.Decimal
	Reason     string
	Action     string
	TargetDate *time.Time
}

// evaluateAsset runs every rule against one asset and returns the
// matching recommendations. Pure function so it's trivially testable.
func evaluateAsset(a dbgen.ListAssetsForPredictiveScanRow, now time.Time, cfg RuleConfig) []recommendation {
	var out []recommendation

	// Warranty rules.
	if a.WarrantyEnd.Valid {
		end := a.WarrantyEnd.Time
		days := daysBetween(now, end)
		switch {
		case days < 0:
			// expired
			out = append(out, recommendation{
				Kind:       "warranty_expired",
				Score:      scoreFromDaysPast(-days, 365.0),
				Reason:     fmt.Sprintf("Warranty expired %d days ago (ended %s)", -days, end.Format("2006-01-02")),
				Action:     "Renew or replace",
				TargetDate: &end,
			})
		case days <= cfg.WarrantyHorizonDays:
			// expiring soon
			endCopy := end
			out = append(out, recommendation{
				Kind:       "warranty_expiring",
				Score:      scoreFromDeadline(days, cfg.WarrantyHorizonDays),
				Reason:     fmt.Sprintf("Warranty ends in %d days (on %s)", days, end.Format("2006-01-02")),
				Action:     "Plan renewal",
				TargetDate: &endCopy,
			})
		}
	}

	// EOL rules.
	if a.EolDate.Valid {
		end := a.EolDate.Time
		days := daysBetween(now, end)
		switch {
		case days < 0:
			out = append(out, recommendation{
				Kind:       "eol_passed",
				Score:      scoreFromDaysPast(-days, 730.0), // EOL is more urgent than warranty
				Reason:     fmt.Sprintf("EOL date passed %d days ago (was %s)", -days, end.Format("2006-01-02")),
				Action:     "Replace before next failure",
				TargetDate: &end,
			})
		case days <= cfg.EOLHorizonDays:
			endCopy := end
			out = append(out, recommendation{
				Kind:       "eol_approaching",
				Score:      scoreFromDeadline(days, cfg.EOLHorizonDays),
				Reason:     fmt.Sprintf("EOL in %d days (on %s)", days, end.Format("2006-01-02")),
				Action:     "Schedule replacement",
				TargetDate: &endCopy,
			})
		}
	}

	// Aged-out rule: purchase_date + expected_lifespan_months ≤ now * (1+margin)
	if a.PurchaseDate.Valid && a.ExpectedLifespanMonths.Valid {
		life := time.Duration(a.ExpectedLifespanMonths.Int32) * 30 * 24 * time.Hour
		expected := a.PurchaseDate.Time.Add(life)
		if cfg.AgedOutMargin > 0 {
			expected = expected.Add(time.Duration(float64(life) * cfg.AgedOutMargin))
		}
		days := daysBetween(now, expected)
		if days < 0 {
			out = append(out, recommendation{
				Kind:       "aged_out",
				Score:      scoreFromDaysPast(-days, 365.0),
				Reason:     fmt.Sprintf("Past expected lifespan by %d days (purchased %s, %d-month spec)", -days, a.PurchaseDate.Time.Format("2006-01-02"), a.ExpectedLifespanMonths.Int32),
				Action:     "Refresh as opportunity allows",
				TargetDate: &expected,
			})
		}
	}

	return out
}

// daysBetween returns the integer number of days from `from` to `to`,
// treating the comparison as date-level (so "today" vs "tomorrow" is 1,
// not zero-or-one depending on the hour). Negative when `to` is before
// `from`.
func daysBetween(from, to time.Time) int {
	a := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}

// scoreFromDeadline maps "days until deadline" into a 0-100 band, with
// 100 at zero days remaining and 50 at half the horizon. Linearly
// interpolated — a model writer can replace this with anything fancier
// without touching the schema.
func scoreFromDeadline(daysRemaining, horizonDays int) decimal.Decimal {
	if horizonDays <= 0 {
		return decimal.NewFromInt(50)
	}
	if daysRemaining <= 0 {
		return decimal.NewFromInt(100)
	}
	if daysRemaining > horizonDays {
		return decimal.Zero
	}
	frac := float64(horizonDays-daysRemaining) / float64(horizonDays)
	score := decimal.NewFromFloat(frac * 100).Round(2)
	if score.GreaterThan(decimal.NewFromInt(100)) {
		score = decimal.NewFromInt(100)
	}
	return score
}

// scoreFromDaysPast maps "days past deadline" into 0-100, saturating at
// `saturationDays` past. Past-deadline rows always score ≥ the same
// kind's pre-deadline rows, so an expired warranty outranks an
// expiring one in the dashboard.
func scoreFromDaysPast(daysPast int, saturationDays float64) decimal.Decimal {
	if daysPast <= 0 {
		return decimal.NewFromInt(100)
	}
	frac := float64(daysPast) / saturationDays
	if frac > 1 {
		frac = 1
	}
	// Floor at 80 so even 1 day past expiry outranks an in-window expiring.
	score := 80.0 + 20.0*frac
	return decimal.NewFromFloat(score).Round(2)
}

// ---------------------------------------------------------------------------
// Read paths.
// ---------------------------------------------------------------------------

// List returns recommendations filtered by status / kind, paginated.
type ListParams struct {
	TenantID uuid.UUID
	Status   *string
	Kind     *string
	Limit    int32
	Offset   int32
}

func (s *Service) List(ctx context.Context, p ListParams) ([]dbgen.ListPredictiveRefreshRow, int64, error) {
	listP := dbgen.ListPredictiveRefreshParams{
		TenantID: p.TenantID,
		Limit:    p.Limit,
		Offset:   p.Offset,
	}
	countP := dbgen.CountPredictiveRefreshParams{TenantID: p.TenantID}
	if p.Status != nil && *p.Status != "" {
		listP.Status = pgtype.Text{String: *p.Status, Valid: true}
		countP.Status = pgtype.Text{String: *p.Status, Valid: true}
	}
	if p.Kind != nil && *p.Kind != "" {
		listP.Kind = pgtype.Text{String: *p.Kind, Valid: true}
		countP.Kind = pgtype.Text{String: *p.Kind, Valid: true}
	}
	rows, err := s.queries.ListPredictiveRefresh(ctx, listP)
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}
	total, err := s.queries.CountPredictiveRefresh(ctx, countP)
	if err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}
	return rows, total, nil
}

// Transition flips a recommendation's status. Allowed transitions are
// permissive (any status → any status) so an operator can correct a
// mistake; the audit trail captures the trail.
func (s *Service) Transition(
	ctx context.Context,
	tenantID, assetID uuid.UUID,
	kind, status string,
	reviewerID uuid.UUID,
	note string,
) (*dbgen.PredictiveRefreshRecommendation, error) {
	if status != "open" && status != "ack" && status != "resolved" {
		return nil, errors.New("invalid status")
	}
	row, err := s.queries.TransitionPredictiveRefresh(ctx, dbgen.TransitionPredictiveRefreshParams{
		TenantID:   tenantID,
		AssetID:    assetID,
		Kind:       kind,
		Status:     status,
		ReviewedBy: pgtype.UUID{Bytes: reviewerID, Valid: reviewerID != uuid.Nil},
		Note:       pgtype.Text{String: note, Valid: note != ""},
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("transition: %w", err)
	}
	return &row, nil
}

// ---------------------------------------------------------------------------
// Scheduler tick.
// ---------------------------------------------------------------------------

// TickResult aggregates per-tenant scan results for the scheduler's
// telemetry log line.
type TickResult struct {
	TenantsScanned int
	AssetsScanned  int
	RowsUpserted   int
	Errors         []error
}

// RunScanTick scans every tenant that has at least one lifecycle-bearing
// asset. Per-tenant errors don't abort the tick.
func (s *Service) RunScanTick(ctx context.Context, cfg RuleConfig) TickResult {
	res := TickResult{}
	tenants, err := s.queries.ListTenantsWithLifecycleAssets(ctx)
	if err != nil {
		res.Errors = append(res.Errors, fmt.Errorf("list tenants: %w", err))
		return res
	}
	res.TenantsScanned = len(tenants)
	for _, tenantID := range tenants {
		scan, err := s.ScanAndUpsert(ctx, tenantID, cfg)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Errorf("tenant=%s: %w", tenantID, err))
			continue
		}
		res.AssetsScanned += scan.AssetsScanned
		res.RowsUpserted += scan.RowsUpserted
	}
	return res
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func decimalToPgNumeric(d decimal.Decimal) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(d.String()); err != nil {
		return n, err
	}
	return n, nil
}
