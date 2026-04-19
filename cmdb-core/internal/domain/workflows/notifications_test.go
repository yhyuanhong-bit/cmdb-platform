//go:build integration

package workflows

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests verify Phase 2.14: critical-alert → emergency-WO dedup
// narrows from "any open WO on this asset" to "any open *emergency* WO
// on this asset in this tenant". The pre-fix predicate let a stale
// routine maintenance WO silently swallow every downstream critical
// alert for the same asset.
//
// Run with:
//   go test -tags integration -race ./internal/domain/workflows/...

// seedAsset inserts a minimal asset row so work_orders.asset_id has a
// target. asset_tag is unique across tenants so we derive it from the
// tenant UUID.
func seedAsset(t *testing.T, pool *pgxpool.Pool, tenantID uuid.UUID, tag string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	assetID := uuid.New()
	fullTag := fmt.Sprintf("%s-%s", tag, tenantID.String()[:8])
	_, err := pool.Exec(ctx,
		`INSERT INTO assets (id, tenant_id, asset_tag, name, type, status)
		 VALUES ($1, $2, $3, $4, 'server', 'deployed')`,
		assetID, tenantID, fullTag, "alert-dedup-"+fullTag)
	if err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	return assetID
}

// seedExistingWO inserts a work_order directly (bypassing service layer)
// at the given type+status. We set requestor_id to uuid.Nil so it
// matches the harness user, and deleted_at stays NULL. We use a
// timestamp-derived code so parallel tests don't collide on the UNIQUE
// constraint.
func seedExistingWO(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID, woType, status string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	code := fmt.Sprintf("WO-TEST-%s", id.String()[:12])
	_, err := pool.Exec(ctx,
		`INSERT INTO work_orders
		   (id, tenant_id, code, title, type, status, priority, asset_id, requestor_id)
		 VALUES ($1, $2, $3, $4, $5, $6, 'high', $7, $8)`,
		id, tenantID, code, "seed "+woType+" "+status, woType, status, assetID, uuid.Nil)
	if err != nil {
		t.Fatalf("seed WO (%s/%s): %v", woType, status, err)
	}
	return id
}

func countEmergencyWOs(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID) int {
	t.Helper()
	var n int
	err := pool.QueryRow(context.Background(),
		`SELECT count(*) FROM work_orders
		  WHERE tenant_id = $1 AND asset_id = $2 AND type = 'emergency' AND deleted_at IS NULL`,
		tenantID, assetID).Scan(&n)
	if err != nil {
		t.Fatalf("count emergency WOs: %v", err)
	}
	return n
}

// fireCriticalAlert builds the alert.fired event payload the subscriber
// expects and dispatches it through the handler synchronously so we can
// inspect post-state deterministically. Note the subscriber is wired
// with a nil bus, which is fine because onAlertFired does not re-publish.
func fireCriticalAlert(t *testing.T, w *WorkflowSubscriber, tenantID, assetID uuid.UUID, msg string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"alert_id": uuid.NewString(),
		"severity": "critical",
		"asset_id": assetID.String(),
		"message":  msg,
	})
	if err != nil {
		t.Fatalf("marshal alert payload: %v", err)
	}
	if err := w.onAlertFired(context.Background(), eventbus.Event{
		Subject:  "alert.fired",
		TenantID: tenantID.String(),
		Payload:  payload,
	}); err != nil {
		t.Fatalf("onAlertFired: %v", err)
	}
}

// TestAlertDedup_EmergencyTypeOnly is the core Phase 2.14 contract:
// a stale NON-emergency open WO on an asset must NOT suppress creating
// a new emergency WO when a critical alert fires for that same asset.
func TestAlertDedup_EmergencyTypeOnly(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()
	w := newTestSubscriber(t, pool)

	t.Run("open non-emergency WO does NOT block emergency auto-creation", func(t *testing.T) {
		fix := setupTenant(t, pool, "alert-nonemerg")
		assetID := seedAsset(t, pool, fix.tenantID, "asset-nonemerg")

		// Pre-existing routine maintenance WO in 'in_progress' — before
		// the fix this muted every downstream critical alert.
		seedExistingWO(t, pool, fix.tenantID, assetID, "maintenance", "in_progress")

		fireCriticalAlert(t, w, fix.tenantID, assetID, "disk failure imminent")

		// We expect exactly 1 emergency WO to have been created by the
		// subscriber (the pre-seed was type=maintenance, not emergency).
		if got := countEmergencyWOs(t, pool, fix.tenantID, assetID); got != 1 {
			t.Fatalf("emergency WO count = %d, want 1 (pre-fix would be 0)", got)
		}
	})

	t.Run("open emergency WO DOES block duplicate emergency creation", func(t *testing.T) {
		fix := setupTenant(t, pool, "alert-emerg-block")
		assetID := seedAsset(t, pool, fix.tenantID, "asset-emerg")

		// Pre-existing emergency WO in 'approved' — same type + open
		// status → dedup must suppress the second auto-create.
		seedExistingWO(t, pool, fix.tenantID, assetID, "emergency", "approved")

		fireCriticalAlert(t, w, fix.tenantID, assetID, "temperature critical")

		if got := countEmergencyWOs(t, pool, fix.tenantID, assetID); got != 1 {
			t.Fatalf("emergency WO count = %d, want 1 (dedup must hold for same-type open WO)", got)
		}
	})

	t.Run("no open WOs → critical alert generates emergency WO", func(t *testing.T) {
		fix := setupTenant(t, pool, "alert-clean")
		assetID := seedAsset(t, pool, fix.tenantID, "asset-clean")

		fireCriticalAlert(t, w, fix.tenantID, assetID, "cpu thermal runaway")

		if got := countEmergencyWOs(t, pool, fix.tenantID, assetID); got != 1 {
			t.Fatalf("emergency WO count = %d, want 1 (no prior WOs → unconditional create)", got)
		}
	})

	t.Run("cross-tenant isolation: tenant A open emergency does NOT block tenant B", func(t *testing.T) {
		fixA := setupTenant(t, pool, "alert-iso-a")
		fixB := setupTenant(t, pool, "alert-iso-b")

		// Both tenants get an asset row; we reuse the same asset_tag
		// prefix but setupTenant UUIDs are unique so no collision.
		assetA := seedAsset(t, pool, fixA.tenantID, "asset-iso")
		assetB := seedAsset(t, pool, fixB.tenantID, "asset-iso")

		// Tenant A has an open emergency WO — this must NOT reach into
		// tenant B and suppress B's alert-driven WO creation.
		seedExistingWO(t, pool, fixA.tenantID, assetA, "emergency", "approved")

		fireCriticalAlert(t, w, fixB.tenantID, assetB, "tenant-B power loss")

		if got := countEmergencyWOs(t, pool, fixB.tenantID, assetB); got != 1 {
			t.Fatalf("tenant B emergency WO count = %d, want 1 (tenant isolation broken)", got)
		}
		// And tenant A still has exactly its one pre-seeded WO — we
		// never fired an alert for it.
		if got := countEmergencyWOs(t, pool, fixA.tenantID, assetA); got != 1 {
			t.Fatalf("tenant A emergency WO count = %d, want 1 (no leakage from B)", got)
		}
	})
}
