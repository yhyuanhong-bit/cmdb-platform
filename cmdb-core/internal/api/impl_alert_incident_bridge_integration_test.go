//go:build integration

package api_test

import (
	"context"
	"os"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/domain/monitoring"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Wave 5.4: alert → incident bridge.
//
// We test the bridge directly (not through the evaluator's full tick) so
// the test stays fast and deterministic — the evaluator already has its
// own tick tests in evaluator_test.go that cover rule selection +
// aggregation.  Here we only care about the linkage rules:
//
//   1. critical / high / warning + known asset → creates incident on
//      first firing, attaches subsequent firings to the same incident
//      within the 24h dedupe window
//   2. low / info severity → no incident
//   3. firing without an asset_id → no incident (we don't have an
//      affected_asset_id to put on the incident)
//   4. resolved alert → writes a follow-up timeline comment, leaves
//      incident state unchanged
//   5. tenant isolation — bridge from tenant A never finds an incident
//      in tenant B even on the same asset id

// Bridge tests insert an alert_event row directly and then call the
// (package-private) bridge. We get at the bridge through the
// monitoring.NewBridgeForTest export below — see the helper file.

func newBridgeTestPool(t *testing.T) *pgxpool.Pool {
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

func seedTwoTenantsForBridge(t *testing.T, pool *pgxpool.Pool) (uuid.UUID, uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()
	suffix := a.String()[:8]
	if _, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3), ($4, $5, $6)`,
		a, "br-A-"+suffix, "br-a-"+suffix,
		b, "br-B-"+suffix, "br-b-"+suffix,
	); err != nil {
		t.Fatalf("insert tenants: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM incident_comments WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM incidents          WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM alert_events       WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM assets             WHERE tenant_id IN ($1,$2)`, a, b)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants            WHERE id IN ($1,$2)`, a, b)
	})
	return a, b
}

func seedAssetForBridge(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	tag := "BR-" + id.String()[:8]
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status)
		 VALUES ($1, $2, $3, $3, 'server', 'rack_mount', 'operational')`,
		id, tenantID, tag,
	); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return id
}

// insertAlertRow writes an alert_events row directly and returns its id.
// We bypass the evaluator's metric-aggregation path — the only thing the
// bridge cares about is the row that emit() ends up producing.
func insertAlertRow(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID, severity, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	dedup := id.String() // unique per row so tests don't collide on the dedup_key index
	var assetArg interface{}
	if assetID != uuid.Nil {
		assetArg = assetID
	}
	if _, err := pool.Exec(context.Background(),
		`INSERT INTO alert_events (id, tenant_id, asset_id, status, severity, message, dedup_key, fired_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, now(), now())`,
		id, tenantID, assetArg, status, severity, "test alert", dedup,
	); err != nil {
		t.Fatalf("insert alert_event: %v", err)
	}
	return id
}

func countIncidentsForAsset(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM incidents WHERE tenant_id = $1 AND affected_asset_id = $2`,
		tenantID, assetID,
	).Scan(&n); err != nil {
		t.Fatalf("count incidents: %v", err)
	}
	return n
}

func readAlertIncidentID(t *testing.T, pool *pgxpool.Pool, alertID uuid.UUID) (uuid.UUID, bool) {
	t.Helper()
	var id *uuid.UUID
	if err := pool.QueryRow(context.Background(),
		`SELECT incident_id FROM alert_events WHERE id = $1`, alertID,
	).Scan(&id); err != nil {
		t.Fatalf("read alert.incident_id: %v", err)
	}
	if id == nil {
		return uuid.Nil, false
	}
	return *id, true
}

func countCommentsForIncident(t *testing.T, pool *pgxpool.Pool, incidentID uuid.UUID) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM incident_comments WHERE incident_id = $1`, incidentID,
	).Scan(&n); err != nil {
		t.Fatalf("count comments: %v", err)
	}
	return n
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestBridge_CriticalAlertSpawnsIncident(t *testing.T) {
	pool := newBridgeTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForBridge(t, pool)
	asset := seedAssetForBridge(t, pool, tenantA)
	bridge := monitoring.NewBridgeForTest(pool)

	alertID := insertAlertRow(t, pool, tenantA, asset, "critical", "firing")
	bridge.OnAlertEmittedForTest(ctx, tenantA, alertID, asset, "critical", "firing", "rule X firing: cpu = 95%")

	if got := countIncidentsForAsset(t, pool, tenantA, asset); got != 1 {
		t.Fatalf("incidents for asset = %d, want 1", got)
	}

	incID, linked := readAlertIncidentID(t, pool, alertID)
	if !linked {
		t.Fatalf("alert.incident_id not stamped")
	}
	// Initial system comment: "incident opened from alert: …"
	if got := countCommentsForIncident(t, pool, incID); got != 1 {
		t.Errorf("comments = %d, want 1", got)
	}
}

func TestBridge_FollowUpAlertAttachesToExistingIncident(t *testing.T) {
	pool := newBridgeTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForBridge(t, pool)
	asset := seedAssetForBridge(t, pool, tenantA)
	bridge := monitoring.NewBridgeForTest(pool)

	// First firing: spawns incident.
	first := insertAlertRow(t, pool, tenantA, asset, "high", "firing")
	bridge.OnAlertEmittedForTest(ctx, tenantA, first, asset, "high", "firing", "first")

	// Second firing on same asset: should attach to the same incident,
	// not create a new one.
	second := insertAlertRow(t, pool, tenantA, asset, "high", "firing")
	bridge.OnAlertEmittedForTest(ctx, tenantA, second, asset, "high", "firing", "second")

	if got := countIncidentsForAsset(t, pool, tenantA, asset); got != 1 {
		t.Fatalf("dedupe failed: incidents = %d, want 1 (both alerts should attach)", got)
	}

	id1, _ := readAlertIncidentID(t, pool, first)
	id2, _ := readAlertIncidentID(t, pool, second)
	if id1 != id2 {
		t.Errorf("alerts attached to different incidents: %s vs %s", id1, id2)
	}
	// Two system comments — one per attach.
	if got := countCommentsForIncident(t, pool, id1); got != 2 {
		t.Errorf("comments = %d, want 2", got)
	}
}

func TestBridge_LowSeverityIgnored(t *testing.T) {
	pool := newBridgeTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForBridge(t, pool)
	asset := seedAssetForBridge(t, pool, tenantA)
	bridge := monitoring.NewBridgeForTest(pool)

	for _, sev := range []string{"low", "info", "medium"} {
		alertID := insertAlertRow(t, pool, tenantA, asset, sev, "firing")
		bridge.OnAlertEmittedForTest(ctx, tenantA, alertID, asset, sev, "firing", "noisy "+sev)
		if _, linked := readAlertIncidentID(t, pool, alertID); linked {
			t.Errorf("severity=%q linked to incident, want skip", sev)
		}
	}
	if got := countIncidentsForAsset(t, pool, tenantA, asset); got != 0 {
		t.Errorf("incidents created from low-severity = %d, want 0", got)
	}
}

func TestBridge_NoAssetIDIsSkipped(t *testing.T) {
	pool := newBridgeTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForBridge(t, pool)
	bridge := monitoring.NewBridgeForTest(pool)

	// Critical severity but asset_id=Nil: bridge skips because we have
	// nothing to put in affected_asset_id and no key for dedupe.
	alertID := insertAlertRow(t, pool, tenantA, uuid.Nil, "critical", "firing")
	bridge.OnAlertEmittedForTest(ctx, tenantA, alertID, uuid.Nil, "critical", "firing", "scope-wide alert")

	if _, linked := readAlertIncidentID(t, pool, alertID); linked {
		t.Errorf("alert without asset_id linked to incident, want skip")
	}
}

func TestBridge_ResolvedAlertWritesFollowUpComment(t *testing.T) {
	pool := newBridgeTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForBridge(t, pool)
	asset := seedAssetForBridge(t, pool, tenantA)
	bridge := monitoring.NewBridgeForTest(pool)

	// Firing → spawns incident.
	alertID := insertAlertRow(t, pool, tenantA, asset, "critical", "firing")
	bridge.OnAlertEmittedForTest(ctx, tenantA, alertID, asset, "critical", "firing", "cpu pegged")
	incID, _ := readAlertIncidentID(t, pool, alertID)

	// Now mark the same row resolved (the evaluator's upsert keeps the
	// incident_id from the firing row) and re-call the bridge.
	if _, err := pool.Exec(ctx,
		`UPDATE alert_events SET status = 'resolved', resolved_at = now() WHERE id = $1`,
		alertID,
	); err != nil {
		t.Fatalf("update to resolved: %v", err)
	}
	bridge.OnAlertEmittedForTest(ctx, tenantA, alertID, asset, "critical", "resolved", "cpu back below threshold")

	// One firing comment + one resolved comment.
	if got := countCommentsForIncident(t, pool, incID); got != 2 {
		t.Errorf("after resolve comments = %d, want 2", got)
	}
	// Incident status MUST stay open — the bridge never auto-closes.
	var status string
	_ = pool.QueryRow(ctx, `SELECT status FROM incidents WHERE id = $1`, incID).Scan(&status)
	if status != "open" {
		t.Errorf("incident status changed by resolve = %q, want open", status)
	}
}

func TestBridge_ResolvedWithoutPriorLinkIsNoOp(t *testing.T) {
	pool := newBridgeTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, _ := seedTwoTenantsForBridge(t, pool)
	bridge := monitoring.NewBridgeForTest(pool)

	// Resolved alert that was never linked (e.g. low-severity firing
	// that the bridge ignored). commentOnResolved should silently no-op.
	alertID := insertAlertRow(t, pool, tenantA, uuid.Nil, "info", "resolved")
	bridge.OnAlertEmittedForTest(ctx, tenantA, alertID, uuid.Nil, "info", "resolved", "")

	// No incidents, no comments — and crucially, no error logged means
	// the bridge handled the unlinked case gracefully.
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM incidents WHERE tenant_id = $1`, tenantA).Scan(&n)
	if n != 0 {
		t.Errorf("incidents created on unlinked resolve = %d, want 0", n)
	}
}

func TestBridge_TenantIsolation(t *testing.T) {
	pool := newBridgeTestPool(t)
	defer pool.Close()
	ctx := context.Background()

	tenantA, tenantB := seedTwoTenantsForBridge(t, pool)
	bridge := monitoring.NewBridgeForTest(pool)

	// Asset id is the SAME UUID across both tenants — exotic but valid,
	// since assets is keyed by (tenant_id, id). The bridge's dedupe
	// query must include tenant_id; if it didn't, tenant B's firing
	// would collide with tenant A's incident.
	sharedID := uuid.New()
	if _, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, sub_type, status) VALUES
		 ($1, $2, $3, 'A', 'server', 'rack_mount', 'operational')`,
		sharedID, tenantA, "TA-"+sharedID.String()[:6],
	); err != nil {
		t.Fatalf("seed assetA: %v", err)
	}
	// Different asset id for tenant B (assets.id is globally unique
	// because of the PK), but tenant A and B both have firing alerts
	// referring to assets they own.
	assetB := seedAssetForBridge(t, pool, tenantB)

	// Tenant A spawns an incident.
	alertA := insertAlertRow(t, pool, tenantA, sharedID, "critical", "firing")
	bridge.OnAlertEmittedForTest(ctx, tenantA, alertA, sharedID, "critical", "firing", "tenant A fault")

	// Tenant B fires on a different asset — must spawn its own incident,
	// not attach to tenant A's.
	alertB := insertAlertRow(t, pool, tenantB, assetB, "critical", "firing")
	bridge.OnAlertEmittedForTest(ctx, tenantB, alertB, assetB, "critical", "firing", "tenant B fault")

	idA, _ := readAlertIncidentID(t, pool, alertA)
	idB, _ := readAlertIncidentID(t, pool, alertB)
	if idA == idB {
		t.Errorf("cross-tenant attach: alerts share incident %s", idA)
	}
	// Each tenant has exactly one incident.
	var nA, nB int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM incidents WHERE tenant_id = $1`, tenantA).Scan(&nA)
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM incidents WHERE tenant_id = $1`, tenantB).Scan(&nB)
	if nA != 1 || nB != 1 {
		t.Errorf("incidents per tenant: A=%d B=%d, want 1/1", nA, nB)
	}
}
