//go:build integration

package workflows

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/maintenance"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// These tests verify Phase 2.10: the emergency work-order approve+start
// transition is a single atomic UPDATE, not a two-step dance. Before the
// fix, a crash or retry between UpdateGovernanceStatus('approved') and
// UpdateExecutionStatus('working') could strand the WO half-approved
// (approved_at stamped, sla_deadline set, but execution_status still
// 'pending'), which then tripped the SLA scan for a row that was never
// in progress.
//
// Run with:
//   go test -tags integration -race ./internal/domain/workflows/...

// newTestMaintenanceService returns a maintenance.Service wired to the
// real DB pool but with nil bus — TransitionEmergencyAtomic does not
// publish events; the caller in onAlertFired does.
func newTestMaintenanceService(t *testing.T, pool *pgxpool.Pool) *maintenance.Service {
	t.Helper()
	return maintenance.NewService(dbgen.New(pool), nil, pool)
}

// seedEmergencyWOSubmitted inserts a fresh emergency WO in the exact
// pre-transition state: governance='submitted', execution='pending',
// type='emergency'. We go through the INSERT rather than the service's
// Create() because Create() forces status='submitted' but we want to
// control the full initial state for each test.
func seedEmergencyWOSubmitted(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	code := fmt.Sprintf("WO-EMG-%s", id.String()[:12])
	_, err := pool.Exec(ctx,
		`INSERT INTO work_orders
		   (id, tenant_id, code, title, type, status, priority, asset_id,
		    requestor_id, governance_status, execution_status)
		 VALUES ($1, $2, $3, 'emergency under test', 'emergency',
		         'submitted', 'critical', $4, $5, 'submitted', 'pending')`,
		id, tenantID, code, assetID, uuid.Nil)
	if err != nil {
		t.Fatalf("seed emergency WO: %v", err)
	}
	return id
}

// seedMaintenanceWOSubmitted is the type='maintenance' analogue. Used
// by TestNonEmergency to prove the type='emergency' guard in the
// UPDATE WHERE clause actually protects routine WOs.
func seedMaintenanceWOSubmitted(t *testing.T, pool *pgxpool.Pool, tenantID, assetID uuid.UUID) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	code := fmt.Sprintf("WO-MNT-%s", id.String()[:12])
	_, err := pool.Exec(ctx,
		`INSERT INTO work_orders
		   (id, tenant_id, code, title, type, status, priority, asset_id,
		    requestor_id, governance_status, execution_status)
		 VALUES ($1, $2, $3, 'maintenance under test', 'maintenance',
		         'submitted', 'medium', $4, $5, 'submitted', 'pending')`,
		id, tenantID, code, assetID, uuid.Nil)
	if err != nil {
		t.Fatalf("seed maintenance WO: %v", err)
	}
	return id
}

type woStatuses struct {
	status        string
	governance    string
	execution     string
	approvedAtSet bool
	slaDeadlineOK bool
}

func readWOStatuses(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) woStatuses {
	t.Helper()
	var s woStatuses
	err := pool.QueryRow(context.Background(),
		`SELECT status, governance_status, execution_status,
		        approved_at IS NOT NULL, sla_deadline IS NOT NULL
		 FROM work_orders WHERE id = $1`,
		id).Scan(&s.status, &s.governance, &s.execution, &s.approvedAtSet, &s.slaDeadlineOK)
	if err != nil {
		t.Fatalf("read WO statuses: %v", err)
	}
	return s
}

// TestTransitionEmergencyAtomic_FreshOrder: happy path.
// A submitted+pending emergency WO should be flipped to
// approved+working+in_progress in one SQL round-trip, with approved_at
// and sla_deadline stamped.
func TestTransitionEmergencyAtomic_FreshOrder(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	fix := setupTenant(t, pool, "emg-fresh")
	assetID := seedAsset(t, pool, fix.tenantID, "emg-fresh")
	orderID := seedEmergencyWOSubmitted(t, pool, fix.tenantID, assetID)

	svc := newTestMaintenanceService(t, pool)
	updated, err := svc.TransitionEmergencyAtomic(context.Background(), fix.tenantID, orderID, uuid.Nil)
	if err != nil {
		t.Fatalf("TransitionEmergencyAtomic: %v", err)
	}
	if updated == nil {
		t.Fatalf("expected updated row, got nil")
	}

	got := readWOStatuses(t, pool, orderID)
	if got.status != "in_progress" {
		t.Errorf("status = %q, want in_progress", got.status)
	}
	if got.governance != "approved" {
		t.Errorf("governance_status = %q, want approved", got.governance)
	}
	if got.execution != "working" {
		t.Errorf("execution_status = %q, want working", got.execution)
	}
	if !got.approvedAtSet {
		t.Errorf("approved_at should be set")
	}
	if !got.slaDeadlineOK {
		t.Errorf("sla_deadline should be set")
	}
}

// TestTransitionEmergencyAtomic_NonEmergency: type guard.
// A type='maintenance' WO must NOT be auto-approved by this call even
// if it is submitted+pending. The service returns (nil, nil) and the
// row stays pristine.
func TestTransitionEmergencyAtomic_NonEmergency(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	fix := setupTenant(t, pool, "emg-nonemg")
	assetID := seedAsset(t, pool, fix.tenantID, "emg-nonemg")
	orderID := seedMaintenanceWOSubmitted(t, pool, fix.tenantID, assetID)

	svc := newTestMaintenanceService(t, pool)
	updated, err := svc.TransitionEmergencyAtomic(context.Background(), fix.tenantID, orderID, uuid.Nil)
	if err != nil {
		t.Fatalf("TransitionEmergencyAtomic: %v", err)
	}
	if updated != nil {
		t.Fatalf("expected (nil, nil) for non-emergency WO, got %+v", updated)
	}

	got := readWOStatuses(t, pool, orderID)
	if got.status != "submitted" || got.governance != "submitted" || got.execution != "pending" {
		t.Errorf("row mutated: %+v", got)
	}
	if got.approvedAtSet {
		t.Errorf("approved_at should not be set for untouched WO")
	}
}

// TestTransitionEmergencyAtomic_AlreadyApproved: idempotency.
// Calling twice must not re-flip the row or re-stamp approved_at. The
// second call returns (nil, nil).
func TestTransitionEmergencyAtomic_AlreadyApproved(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	fix := setupTenant(t, pool, "emg-idem")
	assetID := seedAsset(t, pool, fix.tenantID, "emg-idem")
	orderID := seedEmergencyWOSubmitted(t, pool, fix.tenantID, assetID)

	svc := newTestMaintenanceService(t, pool)
	ctx := context.Background()

	first, err := svc.TransitionEmergencyAtomic(ctx, fix.tenantID, orderID, uuid.Nil)
	if err != nil || first == nil {
		t.Fatalf("first call: err=%v updated=%v", err, first)
	}

	// Read approved_at after first call so we can verify the second
	// call doesn't touch it.
	var approvedAt1 any
	if err := pool.QueryRow(ctx, `SELECT approved_at FROM work_orders WHERE id = $1`, orderID).Scan(&approvedAt1); err != nil {
		t.Fatalf("read approved_at: %v", err)
	}

	second, err := svc.TransitionEmergencyAtomic(ctx, fix.tenantID, orderID, uuid.Nil)
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if second != nil {
		t.Fatalf("second call should return (nil, nil) [idempotent], got %+v", second)
	}

	var approvedAt2 any
	if err := pool.QueryRow(ctx, `SELECT approved_at FROM work_orders WHERE id = $1`, orderID).Scan(&approvedAt2); err != nil {
		t.Fatalf("read approved_at: %v", err)
	}
	if fmt.Sprintf("%v", approvedAt1) != fmt.Sprintf("%v", approvedAt2) {
		t.Errorf("approved_at changed between calls: before=%v after=%v", approvedAt1, approvedAt2)
	}
}

// TestTransitionEmergencyAtomic_CrossTenant: tenant isolation.
// A caller with the wrong tenant_id must get 0 rows back (treated as
// idempotent nil) and the victim WO must stay pristine.
func TestTransitionEmergencyAtomic_CrossTenant(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	victim := setupTenant(t, pool, "emg-victim")
	attacker := setupTenant(t, pool, "emg-attacker")
	assetID := seedAsset(t, pool, victim.tenantID, "emg-victim")
	orderID := seedEmergencyWOSubmitted(t, pool, victim.tenantID, assetID)

	svc := newTestMaintenanceService(t, pool)
	updated, err := svc.TransitionEmergencyAtomic(context.Background(), attacker.tenantID, orderID, uuid.Nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated != nil {
		t.Fatalf("cross-tenant call should get (nil, nil), got %+v", updated)
	}

	got := readWOStatuses(t, pool, orderID)
	if got.status != "submitted" || got.governance != "submitted" || got.execution != "pending" {
		t.Errorf("victim row mutated by cross-tenant call: %+v", got)
	}
	if got.approvedAtSet {
		t.Errorf("victim approved_at should not be set")
	}
}

// TestTransitionEmergencyAtomic_Concurrent: race safety.
// N goroutines call TransitionEmergencyAtomic on the same WO
// simultaneously. Exactly one must see the non-nil updated row; the
// rest must see (nil, nil). The row itself must land in the fully
// approved+in_progress state with no intermediate half-state visible.
func TestTransitionEmergencyAtomic_Concurrent(t *testing.T) {
	pool := newTestPool(t)
	defer pool.Close()

	fix := setupTenant(t, pool, "emg-race")
	assetID := seedAsset(t, pool, fix.tenantID, "emg-race")
	orderID := seedEmergencyWOSubmitted(t, pool, fix.tenantID, assetID)

	svc := newTestMaintenanceService(t, pool)

	const workers = 8
	var winners atomic.Int64
	var wg sync.WaitGroup
	wg.Add(workers)

	// Gate all goroutines behind a shared channel to maximise contention.
	start := make(chan struct{})
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			<-start
			updated, err := svc.TransitionEmergencyAtomic(context.Background(), fix.tenantID, orderID, uuid.Nil)
			if err != nil {
				errCh <- err
				return
			}
			if updated != nil {
				winners.Add(1)
			}
		}()
	}

	close(start)
	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent call error: %v", err)
	}

	if got := winners.Load(); got != 1 {
		t.Errorf("expected exactly 1 winner, got %d", got)
	}

	got := readWOStatuses(t, pool, orderID)
	if got.status != "in_progress" || got.governance != "approved" || got.execution != "working" {
		t.Errorf("row did not settle correctly: %+v", got)
	}
	if !got.approvedAtSet || !got.slaDeadlineOK {
		t.Errorf("stamps missing: %+v", got)
	}
}
