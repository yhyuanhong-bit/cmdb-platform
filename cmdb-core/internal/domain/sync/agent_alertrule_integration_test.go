//go:build integration

package sync

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests exercise the applyAlertRule path against a real Postgres to
// verify the sync_version guard added in Bug #1 fix: an older-version
// envelope arriving after a newer one MUST NOT overwrite the local row.
//
// Run with:
//
//	go test -tags integration -race ./internal/domain/sync/...
//
// TEST_DATABASE_URL can override the default docker-compose connection.
// Tests Skip when the DB is unreachable so `go test ./...` stays green on
// machines without the stack up.

func alertRuleTestDBURL() string {
	if u := os.Getenv("TEST_DATABASE_URL"); u != "" {
		return u
	}
	return "postgres://cmdb:changeme@localhost:5432/cmdb?sslmode=disable"
}

func newAlertRuleTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), alertRuleTestDBURL())
	if err != nil {
		t.Skipf("no test database: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("test database unreachable: %v", err)
	}
	return pool
}

// seedAlertRule inserts a fresh tenant and alert_rule at the given
// sync_version. The returned IDs are used to assert that subsequent
// applyAlertRule calls behave correctly.
func seedAlertRule(t *testing.T, pool *pgxpool.Pool, name string, syncVersion int64) (tenantID, ruleID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	tenantID = uuid.New()
	ruleID = uuid.New()

	// Tenant must exist for FK.
	_, err := pool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)`,
		tenantID, "t-"+tenantID.String()[:8], "sl-"+tenantID.String()[:8])
	if err != nil {
		t.Fatalf("insert tenant: %v", err)
	}

	_, err = pool.Exec(ctx,
		`INSERT INTO alert_rules (id, tenant_id, name, metric_name, condition, severity, enabled, sync_version)
		 VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8)`,
		ruleID, tenantID, name, "cpu.util", `{"op":">","threshold":90}`, "critical", true, syncVersion)
	if err != nil {
		t.Fatalf("insert alert_rule: %v", err)
	}

	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM alert_rules WHERE id = $1`, ruleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tenantID)
	})

	return tenantID, ruleID
}

// TestApplyAlertRuleStaleEnvelopeDoesNotOverwrite verifies the Bug #1 fix:
// an older-version envelope arriving after the row already exists at a
// higher sync_version must be a no-op — name/severity/enabled must remain
// at the newer values.
func TestApplyAlertRuleStaleEnvelopeDoesNotOverwrite(t *testing.T) {
	pool := newAlertRuleTestPool(t)
	defer pool.Close()

	const currentName = "current-high-version"
	const currentSev = "critical"
	const currentVersion = int64(100)

	tenantID, ruleID := seedAlertRule(t, pool, currentName, currentVersion)

	agent := NewAgent(pool, nil, &config.Config{EdgeNodeID: "edge-test"})

	// Stale envelope — older sync_version, different name + severity.
	stalePayload := map[string]interface{}{
		"id":          ruleID.String(),
		"tenant_id":   tenantID.String(),
		"name":        "stale-should-not-apply",
		"metric_name": "cpu.util",
		"condition":   map[string]interface{}{"op": ">", "threshold": 50},
		"severity":    "warning",
		"enabled":     false,
	}
	payloadBytes, err := json.Marshal(stalePayload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	staleEnv := SyncEnvelope{
		ID:         uuid.NewString(),
		Source:     "central",
		TenantID:   tenantID.String(),
		EntityType: "alert_rules",
		EntityID:   ruleID.String(),
		Action:     "update",
		Version:    currentVersion - 10, // older than existing row
		Diff:       payloadBytes,
	}

	if err := agent.applyAlertRule(context.Background(), staleEnv); err != nil {
		t.Fatalf("applyAlertRule: %v", err)
	}

	// Assert row is UNCHANGED.
	var gotName, gotSev string
	var gotEnabled bool
	var gotVersion int64
	err = pool.QueryRow(context.Background(),
		`SELECT name, severity, enabled, sync_version FROM alert_rules WHERE id = $1`,
		ruleID).Scan(&gotName, &gotSev, &gotEnabled, &gotVersion)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	if gotName != currentName {
		t.Errorf("name was overwritten by stale envelope: got %q want %q", gotName, currentName)
	}
	if gotSev != currentSev {
		t.Errorf("severity was overwritten by stale envelope: got %q want %q", gotSev, currentSev)
	}
	if !gotEnabled {
		t.Errorf("enabled flipped by stale envelope: got false want true")
	}
	if gotVersion != currentVersion {
		t.Errorf("sync_version regressed: got %d want %d", gotVersion, currentVersion)
	}
}

// TestApplyAlertRuleNewerEnvelopeApplies confirms the guard does not break
// the happy path: a newer-version envelope still overwrites the existing
// row.
func TestApplyAlertRuleNewerEnvelopeApplies(t *testing.T) {
	pool := newAlertRuleTestPool(t)
	defer pool.Close()

	tenantID, ruleID := seedAlertRule(t, pool, "initial", 10)
	agent := NewAgent(pool, nil, &config.Config{EdgeNodeID: "edge-test"})

	newPayload := map[string]interface{}{
		"id":          ruleID.String(),
		"tenant_id":   tenantID.String(),
		"name":        "updated-by-newer-env",
		"metric_name": "cpu.util",
		"condition":   map[string]interface{}{"op": ">", "threshold": 95},
		"severity":    "critical",
		"enabled":     true,
	}
	payloadBytes, err := json.Marshal(newPayload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	newEnv := SyncEnvelope{
		ID:         uuid.NewString(),
		Source:     "central",
		TenantID:   tenantID.String(),
		EntityType: "alert_rules",
		EntityID:   ruleID.String(),
		Action:     "update",
		Version:    50, // newer
		Diff:       payloadBytes,
	}

	if err := agent.applyAlertRule(context.Background(), newEnv); err != nil {
		t.Fatalf("applyAlertRule: %v", err)
	}

	var gotName string
	var gotVersion int64
	if err := pool.QueryRow(context.Background(),
		`SELECT name, sync_version FROM alert_rules WHERE id = $1`, ruleID,
	).Scan(&gotName, &gotVersion); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if gotName != "updated-by-newer-env" {
		t.Errorf("name not updated by newer envelope: got %q", gotName)
	}
	if gotVersion != 50 {
		t.Errorf("sync_version not bumped: got %d want 50", gotVersion)
	}
}

// TestApplyAlertRuleEqualVersionDoesNotOverwrite — Equal sync_version is
// treated as "already applied" and skipped. Documents the strict `<` guard.
func TestApplyAlertRuleEqualVersionDoesNotOverwrite(t *testing.T) {
	pool := newAlertRuleTestPool(t)
	defer pool.Close()

	tenantID, ruleID := seedAlertRule(t, pool, "initial-at-version-42", 42)
	agent := NewAgent(pool, nil, &config.Config{EdgeNodeID: "edge-test"})

	payload := map[string]interface{}{
		"id":          ruleID.String(),
		"tenant_id":   tenantID.String(),
		"name":        "equal-version-should-be-noop",
		"metric_name": "cpu.util",
		"condition":   map[string]interface{}{"op": ">", "threshold": 80},
		"severity":    "warning",
		"enabled":     true,
	}
	payloadBytes, _ := json.Marshal(payload)

	env := SyncEnvelope{
		ID:         uuid.NewString(),
		Source:     "central",
		TenantID:   tenantID.String(),
		EntityType: "alert_rules",
		EntityID:   ruleID.String(),
		Action:     "update",
		Version:    42,
		Diff:       payloadBytes,
	}

	if err := agent.applyAlertRule(context.Background(), env); err != nil {
		t.Fatalf("applyAlertRule: %v", err)
	}

	var gotName string
	if err := pool.QueryRow(context.Background(),
		`SELECT name FROM alert_rules WHERE id = $1`, ruleID,
	).Scan(&gotName); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if gotName != "initial-at-version-42" {
		t.Errorf("equal-version envelope overwrote row: got %q", gotName)
	}
}
