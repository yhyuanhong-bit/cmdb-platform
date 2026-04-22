package workflows

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
)

// These tests exercise the "early return" paths of every event handler.
// Every handler must:
//
//  1. return nil when the payload JSON is malformed (the subscriber is
//     background-safe; a bad payload must not stop the bus), and
//  2. return nil when the payload is semantically invalid (missing or
//     zero-valued required IDs).
//
// Each handler also has a positive "do work" path gated on a DB query;
// those paths are covered in the //go:build integration tests. The
// tests below are the guard rails around the bail-out arms so a
// regression that starts *panicking* on bad input is caught in unit
// test time rather than at 3am via a poison-pill event.
//
// All handlers take a *WorkflowSubscriber; we construct one with every
// dependency nil because none of the early-return arms dereferences
// anything. A regression that adds a pre-parse pool.Exec would crash
// here — which is exactly what we want.

func blankSubscriber() *WorkflowSubscriber {
	// All nil dependencies — the early return paths must not touch them.
	return &WorkflowSubscriber{}
}

// TestOnOrderTransitioned_BadJSONReturnsNil: the subscriber is
// background-safe; a malformed payload logs a warning and returns nil
// so the bus keeps flowing.
func TestOnOrderTransitioned_BadJSONReturnsNil(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()
	err := w.onOrderTransitioned(context.Background(), eventbus.Event{
		Subject:  "order.transitioned",
		TenantID: uuid.NewString(),
		Payload:  []byte(`{not-json`),
	})
	if err != nil {
		t.Errorf("bad JSON should return nil, got %v", err)
	}
}

// TestOnOrderTransitioned_NonCompletedIgnored: only status=="completed"
// triggers cross-module actions. Other statuses return immediately
// without touching the DB.
func TestOnOrderTransitioned_NonCompletedIgnored(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()

	for _, status := range []string{"draft", "submitted", "approved", "in_progress", "rejected", "verified"} {
		status := status
		t.Run(status, func(t *testing.T) {
			t.Parallel()
			payload, err := json.Marshal(orderTransitionPayload{
				OrderID:  uuid.NewString(),
				Status:   status,
				TenantID: uuid.NewString(),
			})
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if err := w.onOrderTransitioned(context.Background(), eventbus.Event{
				Subject: "order.transitioned",
				Payload: payload,
			}); err != nil {
				t.Errorf("non-completed should return nil (no DB touched), got %v", err)
			}
		})
	}
}

// TestOnOrderTransitioned_BadOrderIDReturnsNil: a malformed order_id
// UUID returns nil without touching the DB (the next step would be a
// DB query that can't be attempted with uuid.Nil).
func TestOnOrderTransitioned_BadOrderIDReturnsNil(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()
	payload, err := json.Marshal(orderTransitionPayload{
		OrderID:  "not-a-uuid",
		Status:   "completed",
		TenantID: uuid.NewString(),
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := w.onOrderTransitioned(context.Background(), eventbus.Event{Payload: payload}); err != nil {
		t.Errorf("bad order_id UUID should return nil, got %v", err)
	}
}

// TestOnAlertFired_BadJSONReturnsNil: malformed payload is a soft skip.
func TestOnAlertFired_BadJSONReturnsNil(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()
	err := w.onAlertFired(context.Background(), eventbus.Event{
		Subject:  "alert.fired",
		TenantID: uuid.NewString(),
		Payload:  []byte(`garbage`),
	})
	if err != nil {
		t.Errorf("bad alert payload should return nil, got %v", err)
	}
}

// TestOnAlertFired_NonCriticalWithoutTenantIsNoOp: non-critical alerts
// only notify ops-admins (which requires a tenant+DB); with tenant
// uuid.Nil the helper skips notifications, and non-critical severity
// bypasses the emergency-WO path entirely. Both arms short-circuit
// without touching the nil pool.
func TestOnAlertFired_NonCriticalWithoutTenantIsNoOp(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()

	payload, err := json.Marshal(map[string]string{
		"alert_id": uuid.NewString(),
		"severity": "warning",
		"asset_id": uuid.NewString(),
		"message":  "cpu high",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// tenant_id empty → parses to uuid.Nil → notification fan-out skipped;
	// severity=warning → emergency-WO path also skipped.
	if err := w.onAlertFired(context.Background(), eventbus.Event{
		Subject:  "alert.fired",
		TenantID: "",
		Payload:  payload,
	}); err != nil {
		t.Errorf("non-critical + nil-tenant should return nil, got %v", err)
	}
}

// TestOnAlertFired_BadAssetIDReturnsNil: a malformed asset_id on a
// critical alert still returns nil (logged + dropped). The handler
// must never propagate a parse error back to the bus.
func TestOnAlertFired_BadAssetIDReturnsNil(t *testing.T) {
	t.Parallel()
	// This test is tricky: the handler attempts opsAdminUserIDs BEFORE
	// asset-id parsing, which would touch the nil pool. Supply an
	// *empty* tenant_id so that branch is skipped, then keep severity
	// at "warning" so we short-circuit before asset-id parse — this
	// was already covered above.
	//
	// The genuine "bad asset_id on critical" branch needs a live pool
	// for the opsAdmins lookup; that's an integration-test concern.
	// Skip with a documented reason so the gap is visible.
	t.Skip("requires live pool for opsAdminUserIDs (integration test in notifications_test.go)")
}

// TestOnAssetCreatedNotify_BadJSONReturnsNil soft-skips on malformed
// payload.
func TestOnAssetCreatedNotify_BadJSONReturnsNil(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()
	err := w.onAssetCreatedNotify(context.Background(), eventbus.Event{
		Subject: "asset.created",
		Payload: []byte(`{{{`),
	})
	if err != nil {
		t.Errorf("bad JSON should return nil, got %v", err)
	}
}

// TestOnAssetCreatedNotify_NilTenantReturnsNil: an empty or invalid
// tenant_id parses to uuid.Nil; the handler short-circuits so the nil
// pool is never touched. This locks in the ordering of the tenant
// validation step.
func TestOnAssetCreatedNotify_NilTenantReturnsNil(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()
	payload, err := json.Marshal(map[string]string{
		"asset_id": uuid.NewString(),
		"name":     "host-1",
		"type":     "server",
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := w.onAssetCreatedNotify(context.Background(), eventbus.Event{
		TenantID: "",
		Payload:  payload,
	}); err != nil {
		t.Errorf("nil tenant should return nil, got %v", err)
	}
}

// TestOnInventoryCompletedNotify_BadJSON / nil-IDs
func TestOnInventoryCompletedNotify_SoftSkipPaths(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()

	t.Run("bad json", func(t *testing.T) {
		t.Parallel()
		err := w.onInventoryCompletedNotify(context.Background(), eventbus.Event{
			Payload: []byte(`NOT JSON`),
		})
		if err != nil {
			t.Errorf("bad JSON should return nil, got %v", err)
		}
	})

	t.Run("nil tenant", func(t *testing.T) {
		t.Parallel()
		payload, _ := json.Marshal(map[string]string{
			"task_id":   uuid.NewString(),
			"tenant_id": uuid.NewString(),
		})
		if err := w.onInventoryCompletedNotify(context.Background(), eventbus.Event{
			TenantID: "",
			Payload:  payload,
		}); err != nil {
			t.Errorf("nil tenant should return nil, got %v", err)
		}
	})

	t.Run("nil task", func(t *testing.T) {
		t.Parallel()
		payload, _ := json.Marshal(map[string]string{
			"task_id": "",
		})
		if err := w.onInventoryCompletedNotify(context.Background(), eventbus.Event{
			TenantID: uuid.NewString(),
			Payload:  payload,
		}); err != nil {
			t.Errorf("nil task should return nil, got %v", err)
		}
	})
}

// TestOnImportCompletedNotify_SoftSkipPaths covers bad JSON and a
// missing tenant — both must return nil without touching the pool.
func TestOnImportCompletedNotify_SoftSkipPaths(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()

	t.Run("bad json", func(t *testing.T) {
		t.Parallel()
		err := w.onImportCompletedNotify(context.Background(), eventbus.Event{
			Payload: []byte(`not json`),
		})
		if err != nil {
			t.Errorf("bad JSON should return nil, got %v", err)
		}
	})

	t.Run("empty tenant", func(t *testing.T) {
		t.Parallel()
		payload, _ := json.Marshal(map[string]int{
			"created": 1, "updated": 2, "errors": 0,
		})
		if err := w.onImportCompletedNotify(context.Background(), eventbus.Event{
			TenantID: "",
			Payload:  payload,
		}); err != nil {
			t.Errorf("empty tenant should return nil, got %v", err)
		}
	})
}

// TestOnScanDifferencesDetected_SoftSkipPaths: governance scanner
// variant of the same contract — bad JSON, bad asset_id, and
// uuid.Nil tenant all must return nil.
func TestOnScanDifferencesDetected_SoftSkipPaths(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()

	t.Run("bad json", func(t *testing.T) {
		t.Parallel()
		err := w.onScanDifferencesDetected(context.Background(), eventbus.Event{
			Payload: []byte(`{{bad`),
		})
		if err != nil {
			t.Errorf("bad JSON should return nil, got %v", err)
		}
	})

	t.Run("empty tenant", func(t *testing.T) {
		t.Parallel()
		payload, _ := json.Marshal(scanDifferencesPayload{
			AssetID:   uuid.NewString(),
			AssetTag:  "SRV-1",
			AssetName: "srv-1",
			Diffs:     map[string]interface{}{},
		})
		if err := w.onScanDifferencesDetected(context.Background(), eventbus.Event{
			TenantID: "",
			Payload:  payload,
		}); err != nil {
			t.Errorf("empty tenant should return nil, got %v", err)
		}
	})

	t.Run("bad asset id", func(t *testing.T) {
		t.Parallel()
		payload, _ := json.Marshal(scanDifferencesPayload{
			AssetID: "not-a-uuid",
		})
		if err := w.onScanDifferencesDetected(context.Background(), eventbus.Event{
			TenantID: uuid.NewString(),
			Payload:  payload,
		}); err != nil {
			t.Errorf("bad asset id should return nil, got %v", err)
		}
	})
}

// TestCheckScanDifferences_EmptyDiffsIsNoOp: the pure no-op guard on
// the diffs slice. No DB touched, safe with nil pool.
func TestCheckScanDifferences_EmptyDiffsIsNoOp(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()
	// An empty diff map must early-return before any DB probe. A nil
	// pool would crash the probe, so the fact that this does not
	// panic proves the empty-map guard came first.
	w.checkScanDifferences(context.Background(), uuid.New(), uuid.New(), "SRV-1", "host", nil)
	w.checkScanDifferences(context.Background(), uuid.New(), uuid.New(), "SRV-1", "host", map[string]interface{}{})
}

// TestOnBMCDefaultPassword_SoftSkipPaths: the BMC security event
// handler has the same three early-return arms as scan differences.
func TestOnBMCDefaultPassword_SoftSkipPaths(t *testing.T) {
	t.Parallel()
	w := blankSubscriber()

	t.Run("bad json", func(t *testing.T) {
		t.Parallel()
		err := w.onBMCDefaultPassword(context.Background(), eventbus.Event{
			Payload: []byte(`not json`),
		})
		if err != nil {
			t.Errorf("bad JSON should return nil, got %v", err)
		}
	})

	t.Run("bad asset id", func(t *testing.T) {
		t.Parallel()
		payload, _ := json.Marshal(map[string]string{
			"asset_id": "not-uuid",
		})
		if err := w.onBMCDefaultPassword(context.Background(), eventbus.Event{
			TenantID: uuid.NewString(),
			Payload:  payload,
		}); err != nil {
			t.Errorf("bad asset id should return nil, got %v", err)
		}
	})

	t.Run("nil tenant", func(t *testing.T) {
		t.Parallel()
		payload, _ := json.Marshal(map[string]string{
			"asset_id": uuid.NewString(),
		})
		if err := w.onBMCDefaultPassword(context.Background(), eventbus.Event{
			TenantID: "",
			Payload:  payload,
		}); err != nil {
			t.Errorf("nil tenant should return nil, got %v", err)
		}
	})
}

// TestRegister_NilBusIsNoOp: Register() must short-circuit when the
// subscriber was constructed without a bus (edge deployments, tests).
// A regression that calls bus.Subscribe on a nil bus would panic.
func TestRegister_NilBusIsNoOp(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{} // no bus
	// Must not panic.
	w.Register()
}

// TestResolveSystemUser_NilResolverFailsClosed: when the systemUsers
// resolver is nil, resolveSystemUser must return (uuid.Nil, false) —
// never (some ambient UUID, true). The caller in every auto-WO path
// interprets false as "skip" and continues; returning true with Nil
// would reinstate the FK-violation that migration 000052 was written
// to prevent.
func TestResolveSystemUser_NilResolverFailsClosed(t *testing.T) {
	t.Parallel()
	w := &WorkflowSubscriber{systemUsers: nil}
	id, ok := w.resolveSystemUser(context.Background(), uuid.New(), "test.source")
	if ok {
		t.Errorf("expected ok=false with nil resolver, got true")
	}
	if id != uuid.Nil {
		t.Errorf("expected uuid.Nil with nil resolver, got %s", id)
	}
}
