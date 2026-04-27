//go:build integration

package api_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/change"
	"github.com/cmdb-platform/cmdb-core/internal/domain/energy"
	"github.com/cmdb-platform/cmdb-core/internal/domain/metricsource"
	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/cmdb-platform/cmdb-core/internal/domain/predictive"
	"github.com/cmdb-platform/cmdb-core/internal/domain/problem"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Wave 11: cross-track end-to-end scenarios.
//
// Each domain has its own integration tests, but the riskiest seams are
// between domains. These tests exercise full operational scenarios that
// touch multiple waves so a regression in any seam fails fast — not
// silently in production six weeks later.
//
// Each scenario runs in its own tenant so they don't pollute each other,
// and the helpers below seed the minimum schema needed (tenant + asset +
// users) without leaning on package-private state from the per-domain
// tests.

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newCrossTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://cmdb:cmdb@localhost:5432/cmdb?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Skipf("test database unreachable: %v", err)
	}
	return pool
}

// seedCrossTenant inserts a tenant and registers a cleanup that wipes
// every wave's tables for that tenant. Adding a new wave that creates
// per-tenant rows means adding the table to this cleanup so the test
// suite stays self-cleaning.
func seedCrossTenant(t *testing.T, pool *pgxpool.Pool, label string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	a := uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		a, "cross-"+label+"-"+suffix, "cross-"+label+"-"+suffix,
	); err != nil {
		t.Fatalf("insert tenant: %v", err)
	}
	t.Cleanup(func() {
		// Cleanup order matters — child tables before parents. The
		// tables not present in earlier waves are tolerated via
		// "IF EXISTS"-style behaviour: pgx returns "relation does not
		// exist" if the migration didn't run, and we ignore the error.
		queries := []string{
			`DELETE FROM change_comments    WHERE tenant_id = $1`,
			`DELETE FROM change_problems    WHERE tenant_id = $1`,
			`DELETE FROM change_services    WHERE tenant_id = $1`,
			`DELETE FROM change_assets      WHERE tenant_id = $1`,
			`DELETE FROM change_approvals   WHERE tenant_id = $1`,
			`DELETE FROM changes            WHERE tenant_id = $1`,
			`DELETE FROM problem_comments   WHERE tenant_id = $1`,
			`DELETE FROM incident_problem_links WHERE tenant_id = $1`,
			`DELETE FROM problems           WHERE tenant_id = $1`,
			`DELETE FROM incident_comments  WHERE tenant_id = $1`,
			`DELETE FROM alert_events       WHERE tenant_id = $1`,
			`DELETE FROM incidents          WHERE tenant_id = $1`,
			`DELETE FROM predictive_refresh_recommendations WHERE tenant_id = $1`,
			`DELETE FROM energy_anomalies   WHERE tenant_id = $1`,
			`DELETE FROM energy_location_daily WHERE tenant_id = $1`,
			`DELETE FROM energy_daily_kwh   WHERE tenant_id = $1`,
			`DELETE FROM energy_tariffs     WHERE tenant_id = $1`,
			`DELETE FROM metric_sources     WHERE tenant_id = $1`,
			`DELETE FROM metrics            WHERE tenant_id = $1`,
			`DELETE FROM assets             WHERE tenant_id = $1`,
			`DELETE FROM tenants            WHERE id = $1`,
		}
		for _, q := range queries {
			_, _ = pool.Exec(ctx, q, a)
		}
	})
	return a
}

func seedCrossUser(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, label string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO users (id, tenant_id, username, display_name, email, password_hash, status, source)
		 VALUES ($1, $2, $3, $4, $5, '!', 'active', 'test')`,
		id, tenantID, label+"-"+id.String()[:8], label, label+"@example.com",
	); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return id
}

func seedCrossAsset(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, kind string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status)
		 VALUES ($1, $2, $3, $3, $4, 'rack_mount', 'operational')`,
		id, tenantID, "X-"+id.String()[:8], kind,
	); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return id
}

// ---------------------------------------------------------------------------
// Scenario A: alert → bridge → incident → problem link → change → CAB
//
// The full ITSM flow that's the headline value of waves 5.x. A critical
// alert on an asset spawns an incident via the Wave 5.4 bridge. The
// operator links it to a recurring problem and creates a change to
// address it. The CAB approves, the change runs to success, and every
// system comment lands on the right timeline.
// ---------------------------------------------------------------------------

func TestCrossTrack_Scenario_A_AlertToChangeExecution(t *testing.T) {
	pool := newCrossTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA := seedCrossTenant(t, pool, "scenA")
	asset := seedCrossAsset(t, pool, tenantA, "server")

	q := dbgen.New(pool)
	bridge := monitoring.NewBridgeForTest(pool)
	psvc := problem.NewService(q, pool)
	csvc := change.NewService(q, pool)

	// Step 1: alert event arrives. Insert an alert_events row directly,
	// then call the bridge — same path the production evaluator uses.
	alertID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO alert_events (id, tenant_id, asset_id, status, severity, message, dedup_key, fired_at, updated_at)
		 VALUES ($1, $2, $3, 'firing', 'critical', 'cpu pinned at 100%%', $4, now(), now())`,
		alertID, tenantA, asset, alertID.String(),
	); err != nil {
		t.Fatalf("insert alert: %v", err)
	}
	bridge.OnAlertEmittedForTest(ctx, tenantA, alertID, asset, "critical", "firing", "cpu pinned at 100%")

	// Bridge should have spawned an incident on the affected asset.
	var incidentID uuid.UUID
	if err := pool.QueryRow(ctx,
		`SELECT id FROM incidents WHERE tenant_id=$1 AND affected_asset_id=$2 AND status='open'`,
		tenantA, asset,
	).Scan(&incidentID); err != nil {
		t.Fatalf("bridge did not spawn incident: %v", err)
	}

	// Step 2: operator opens a problem for the recurring root cause and
	// links the incident to it.
	prob, err := psvc.Create(ctx, problem.CreateParams{
		TenantID: tenantA, Title: "auth service memory leak", Severity: "high",
	})
	if err != nil {
		t.Fatalf("create problem: %v", err)
	}
	if err := psvc.LinkIncident(ctx, tenantA, incidentID, prob.ID, uuid.Nil); err != nil {
		t.Fatalf("link incident to problem: %v", err)
	}

	// Reverse lookup must show the problem as covering this incident.
	probs, err := psvc.ListProblemsForIncident(ctx, tenantA, incidentID)
	if err != nil {
		t.Fatalf("list problems for incident: %v", err)
	}
	if len(probs) != 1 || probs[0].ID != prob.ID {
		t.Errorf("expected problem %s linked, got %v", prob.ID, probs)
	}

	// Step 3: operator opens a normal change to roll out the fix. CAB
	// threshold of 1 keeps the test small but exercises the full flow.
	chg, err := csvc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "patch auth-svc to v2.4.1", Type: "normal",
		Risk: "medium", ApprovalThreshold: 1,
	})
	if err != nil {
		t.Fatalf("create change: %v", err)
	}
	if err := csvc.LinkProblem(ctx, tenantA, chg.ID, prob.ID); err != nil {
		t.Fatalf("link change to problem: %v", err)
	}
	if err := csvc.LinkAsset(ctx, tenantA, chg.ID, asset); err != nil {
		t.Fatalf("link change to asset: %v", err)
	}

	// Step 4: submit, vote, execute.
	submitted, err := csvc.Submit(ctx, tenantA, chg.ID, uuid.Nil)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if submitted.Status != "submitted" {
		t.Fatalf("after submit status=%q want submitted", submitted.Status)
	}

	voter := seedCrossUser(t, pool, tenantA, "cab1")
	approved, err := csvc.CastVote(ctx, tenantA, chg.ID, voter, "approve", "looks good")
	if err != nil {
		t.Fatalf("vote: %v", err)
	}
	if approved.Status != "approved" {
		t.Errorf("threshold-met status=%q want approved", approved.Status)
	}
	if _, err := csvc.Start(ctx, tenantA, chg.ID, uuid.Nil); err != nil {
		t.Fatalf("start: %v", err)
	}
	finished, err := csvc.MarkSucceeded(ctx, tenantA, chg.ID, uuid.Nil, "rolled out cleanly")
	if err != nil {
		t.Fatalf("succeeded: %v", err)
	}
	if finished.Status != "succeeded" {
		t.Errorf("final status=%q want succeeded", finished.Status)
	}

	// Step 5: spot-check that the timeline (incident comments) shows the
	// bridge's "incident opened from alert" line — it's the seam that
	// connects the alert pipeline to the ITSM track and is the most
	// likely thing to silently break under refactoring.
	var commentBody string
	if err := pool.QueryRow(ctx,
		`SELECT body FROM incident_comments WHERE incident_id=$1 ORDER BY created_at ASC LIMIT 1`,
		incidentID,
	).Scan(&commentBody); err != nil {
		t.Fatalf("read incident timeline: %v", err)
	}
	if !contains(commentBody, "incident opened from alert") {
		t.Errorf("first comment = %q, expected bridge marker", commentBody)
	}
}

// contains is a tiny helper to avoid importing strings.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Scenario B: energy daily aggregate → anomaly detect → operator review
//
// Exercises the Wave 6 energy track end-to-end: 7 days of baseline kWh
// + a spike day → AggregateLocationDay rolls up PUE → anomaly detector
// flags the spike → operator acks → re-running the detector preserves
// the ack.
// ---------------------------------------------------------------------------

func TestCrossTrack_Scenario_B_EnergyAnomalyLifecycle(t *testing.T) {
	pool := newCrossTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA := seedCrossTenant(t, pool, "scenB")
	loc := seedCrossLocation(t, pool, tenantA, "DC-1")
	asset := seedCrossAssetAtLocation(t, pool, tenantA, loc, "server")

	q := dbgen.New(pool)
	esvc := energy.NewService(q, pool)

	// 7 days of baseline at 10 kWh.
	for i := 1; i <= 7; i++ {
		day := time.Date(2026, 5, i, 0, 0, 0, 0, time.UTC)
		upsertCrossDailyKwh(t, pool, tenantA, asset, day, 10.0)
	}
	// Spike day at 30 kWh — score 3.0, well over the 2.0 threshold.
	spikeDay := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	upsertCrossDailyKwh(t, pool, tenantA, asset, spikeDay, 30.0)

	// PUE rollup: aggregator should produce one location_daily row.
	if err := esvc.AggregateLocationDay(ctx, tenantA, spikeDay); err != nil {
		t.Fatalf("aggregate location day: %v", err)
	}
	pueRows, err := esvc.ListLocationDailyPue(ctx, tenantA, &loc, spikeDay, spikeDay)
	if err != nil {
		t.Fatalf("list pue: %v", err)
	}
	if len(pueRows) != 1 {
		t.Fatalf("pue rows = %d, want 1", len(pueRows))
	}

	// Detector flags the spike.
	flagged, err := esvc.DetectAnomaliesForDay(ctx, tenantA, spikeDay, energy.DefaultAnomalyConfig())
	if err != nil {
		t.Fatalf("detect: %v", err)
	}
	if flagged != 1 {
		t.Fatalf("flagged = %d, want 1", flagged)
	}

	// Operator acks. Re-running the detector must preserve the ack.
	if _, err := esvc.TransitionAnomaly(ctx, tenantA, asset, spikeDay, "ack", uuid.Nil, "load test"); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if _, err := esvc.DetectAnomaliesForDay(ctx, tenantA, spikeDay, energy.DefaultAnomalyConfig()); err != nil {
		t.Fatalf("detect rerun: %v", err)
	}
	var status, note string
	_ = pool.QueryRow(ctx,
		`SELECT status, COALESCE(note, '') FROM energy_anomalies WHERE tenant_id=$1 AND asset_id=$2 AND day=$3`,
		tenantA, asset, spikeDay,
	).Scan(&status, &note)
	if status != "ack" {
		t.Errorf("status after rerun = %q, want ack", status)
	}
	if note != "load test" {
		t.Errorf("note clobbered: %q", note)
	}
}

// seedCrossLocation, seedCrossAssetAtLocation, upsertCrossDailyKwh — small
// helpers local to this file so the tests stay self-contained.
func seedCrossLocation(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, name string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	slug := name + "-" + id.String()[:8]
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO locations (id, tenant_id, name, slug, level) VALUES ($1, $2, $3, $4, 'idc')`,
		id, tenantID, name, slug,
	); err != nil {
		t.Fatalf("seed location: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM locations WHERE id = $1`, id)
	})
	return id
}

func seedCrossAssetAtLocation(t *testing.T, pool *pgxpool.Pool, tenantID, locID uuid.UUID, kind string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status, location_id)
		 VALUES ($1, $2, $3, $3, $4, 'rack_mount', 'operational', $5)`,
		id, tenantID, "X-"+id.String()[:8], kind, locID,
	); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return id
}

func upsertCrossDailyKwh(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID, day time.Time, kwh float64) {
	t.Helper()
	if _, err := pool.Exec(context.Background(), `
		INSERT INTO energy_daily_kwh (tenant_id, asset_id, day, kwh_total, kw_peak, kw_avg, sample_count)
		VALUES ($1, $2, $3, $4, $5, $6, 24)
		ON CONFLICT (tenant_id, asset_id, day) DO UPDATE SET
			kwh_total = EXCLUDED.kwh_total,
			kw_peak = EXCLUDED.kw_peak,
			kw_avg = EXCLUDED.kw_avg
	`, tenantID, assetID, day, kwh, kwh/24+0.1, kwh/24); err != nil {
		t.Fatalf("upsert daily kwh: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Scenario C: predictive recommendation → operator ack → change creation
//
// Asset's warranty expires soon → predictive scanner produces a refresh
// recommendation → operator acks (planning ahead) → re-scan preserves
// ack → operator opens a change to drive the refresh and links the
// asset. The seam under test is "ack survives re-scan" combined with
// the change ↔ asset linkage from Wave 5.3.
// ---------------------------------------------------------------------------

func TestCrossTrack_Scenario_C_PredictiveAckThenChange(t *testing.T) {
	pool := newCrossTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA := seedCrossTenant(t, pool, "scenC")

	// Asset whose warranty ends in 30 days.
	asset := uuid.New()
	end := time.Now().AddDate(0, 0, 30)
	if _, err := pool.Exec(ctx, `
		INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status, warranty_end)
		VALUES ($1, $2, $3, $3, 'server', 'rack_mount', 'operational', $4)
	`, asset, tenantA, "X-"+asset.String()[:8], end); err != nil {
		t.Fatalf("seed asset: %v", err)
	}

	q := dbgen.New(pool)
	predSvc := predictive.NewService(q, pool)
	csvc := change.NewService(q, pool)

	// First scan flags the asset.
	if _, err := predSvc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan 1: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx,
		`SELECT status FROM predictive_refresh_recommendations
		 WHERE tenant_id=$1 AND asset_id=$2 AND kind='warranty_expiring'`,
		tenantA, asset,
	).Scan(&status); err != nil {
		t.Fatalf("read rec: %v", err)
	}
	if status != "open" {
		t.Errorf("first scan status = %q, want open", status)
	}

	// Operator acks.
	if _, err := predSvc.Transition(ctx, tenantA, asset, "warranty_expiring", "ack", uuid.Nil, "ordered renewal"); err != nil {
		t.Fatalf("ack: %v", err)
	}

	// Re-scan must preserve the ack.
	if _, err := predSvc.ScanAndUpsert(ctx, tenantA, predictive.DefaultRuleConfig()); err != nil {
		t.Fatalf("scan 2: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT status FROM predictive_refresh_recommendations
		 WHERE tenant_id=$1 AND asset_id=$2 AND kind='warranty_expiring'`,
		tenantA, asset,
	).Scan(&status); err != nil {
		t.Fatalf("re-read rec: %v", err)
	}
	if status != "ack" {
		t.Errorf("status after rescan = %q, want ack", status)
	}

	// Operator opens a refresh change linked to the asset.
	chg, err := csvc.Create(ctx, change.CreateParams{
		TenantID: tenantA, Title: "warranty refresh", Type: "standard", Risk: "low",
	})
	if err != nil {
		t.Fatalf("create change: %v", err)
	}
	if err := csvc.LinkAsset(ctx, tenantA, chg.ID, asset); err != nil {
		t.Fatalf("link asset: %v", err)
	}

	// Standard changes auto-approve on submit (Wave 5.3 rule).
	out, err := csvc.Submit(ctx, tenantA, chg.ID, uuid.Nil)
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if out.Status != "approved" {
		t.Errorf("standard change after submit = %q, want approved", out.Status)
	}

	// Verify asset shows up under the change's linked assets.
	assets, err := csvc.ListAssets(ctx, tenantA, chg.ID)
	if err != nil {
		t.Fatalf("list assets: %v", err)
	}
	if len(assets) != 1 || assets[0].ID != asset {
		t.Errorf("change asset linkage broken: %v", assets)
	}
}

// ---------------------------------------------------------------------------
// Scenario D: metric source heartbeat → freshness clears
//
// A source registered with a 60s interval starts stale (no heartbeat
// yet). One heartbeat moves it out of the freshness list. Backdating
// past 2× interval brings it back. This pins the data-plane health
// signal that everything else relies on.
// ---------------------------------------------------------------------------

func TestCrossTrack_Scenario_D_MetricSourceFreshnessLifecycle(t *testing.T) {
	pool := newCrossTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA := seedCrossTenant(t, pool, "scenD")
	q := dbgen.New(pool)
	msvc := metricsource.NewService(q, pool)

	src, err := msvc.Create(ctx, metricsource.CreateParams{
		TenantID: tenantA, Name: "switch-a3-snmp", Kind: "snmp", ExpectedIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("create source: %v", err)
	}

	// New source has never heartbeated → stale.
	stale, err := msvc.ListStale(ctx, tenantA)
	if err != nil {
		t.Fatalf("list stale: %v", err)
	}
	if !containsStale(stale, src.ID) {
		t.Errorf("never-heartbeated source should be in freshness list")
	}

	// Heartbeat with 100 samples → not stale anymore.
	if _, err := msvc.Heartbeat(ctx, tenantA, src.ID, 100); err != nil {
		t.Fatalf("heartbeat: %v", err)
	}
	stale, err = msvc.ListStale(ctx, tenantA)
	if err != nil {
		t.Fatalf("list stale 2: %v", err)
	}
	if containsStale(stale, src.ID) {
		t.Errorf("just-heartbeated source should not be stale")
	}

	// Backdate the heartbeat past 2× interval → stale again.
	if _, err := pool.Exec(ctx,
		`UPDATE metric_sources SET last_heartbeat_at = now() - interval '5 minutes' WHERE id = $1`,
		src.ID,
	); err != nil {
		t.Fatalf("backdate: %v", err)
	}
	stale, err = msvc.ListStale(ctx, tenantA)
	if err != nil {
		t.Fatalf("list stale 3: %v", err)
	}
	if !containsStale(stale, src.ID) {
		t.Errorf("backdated source should be stale")
	}

	// Disabling the source removes it from freshness even when overdue.
	disabledStatus := "disabled"
	if _, err := msvc.Update(ctx, metricsource.UpdateParams{
		TenantID: tenantA, ID: src.ID, Status: &disabledStatus,
	}); err != nil {
		t.Fatalf("disable: %v", err)
	}
	stale, err = msvc.ListStale(ctx, tenantA)
	if err != nil {
		t.Fatalf("list stale 4: %v", err)
	}
	if containsStale(stale, src.ID) {
		t.Errorf("disabled source should be excluded from freshness")
	}

	// Confirm nothing leaked across via a sanity check that sources are
	// found via Get for the same tenant. Cross-tenant isolation already
	// covered by per-domain tests; this is just a wiring sanity.
	got, err := msvc.Get(ctx, tenantA, src.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "switch-a3-snmp" {
		t.Errorf("name = %q, want switch-a3-snmp", got.Name)
	}

	// Cross-tenant Get must fail.
	otherTenant := seedCrossTenant(t, pool, "other")
	if _, err := msvc.Get(ctx, otherTenant, src.ID); !errors.Is(err, metricsource.ErrNotFound) {
		t.Errorf("cross-tenant Get: want ErrNotFound, got %v", err)
	}
}

func containsStale(rows []metricsource.StaleSource, id uuid.UUID) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}
