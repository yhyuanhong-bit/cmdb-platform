package sync

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/config"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// TestHandleIncomingEnvelopeRejectsTenantMismatch covers Bug #2: an
// envelope claiming tenant=B arriving on a subject `sync.<A>.*` MUST NOT
// be applied. The NATS subject segment is the authoritative tenant scope
// (it is the routing key the publisher chose); a divergence between the
// routing scope and the in-body TenantID is a cross-tenant replay attempt
// or a publisher bug. Either way: drop + metric + log, never apply.
//
// We use pool=nil to prove no DB path runs: if the tenant guard regresses
// and dispatches to apply*, the test would nil-panic on pool access.
func TestHandleIncomingEnvelopeRejectsTenantMismatch(t *testing.T) {
	agent := NewAgent(nil, nil, &config.Config{EdgeNodeID: "edge-test"})

	envTenant := uuid.NewString()   // the body claims this tenant
	routedTenant := uuid.NewString() // the subject was routed for this tenant

	env := SyncEnvelope{
		ID:         uuid.NewString(),
		Source:     "central",
		TenantID:   envTenant,
		EntityType: "alert_rules",
		EntityID:   uuid.NewString(),
		Action:     "update",
		Version:    1,
		Diff:       json.RawMessage(`{}`),
	}
	env.Checksum = env.computeChecksum()

	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	subject := "sync." + routedTenant + ".alert_rules.update"

	before := testutil.ToFloat64(
		telemetry.SyncEnvelopeRejected.WithLabelValues("alert_rules", "tenant_mismatch"),
	)

	// If the guard is missing, dispatch would reach applyAlertRule and
	// nil-panic on pool.Exec. The guard must stop execution before that.
	if err := agent.handleIncomingEnvelope(context.Background(), eventbus.Event{
		Subject: subject,
		Payload: payload,
	}); err != nil {
		t.Fatalf("handleIncomingEnvelope: unexpected error: %v", err)
	}

	after := testutil.ToFloat64(
		telemetry.SyncEnvelopeRejected.WithLabelValues("alert_rules", "tenant_mismatch"),
	)

	if after-before != 1 {
		t.Errorf("SyncEnvelopeRejected{tenant_mismatch} delta = %v, want 1", after-before)
	}
}

// TestHandleIncomingEnvelopeAcceptsMatchingTenantRouting confirms the guard
// does not fire when the routed tenant and the envelope tenant agree. We
// deliberately use an entity_type that LayerOf rejects so the function
// returns cleanly without reaching any DB path.
func TestHandleIncomingEnvelopeAcceptsMatchingTenantRouting(t *testing.T) {
	agent := NewAgent(nil, nil, &config.Config{EdgeNodeID: "edge-test"})

	tenantID := uuid.NewString()

	env := SyncEnvelope{
		ID:         uuid.NewString(),
		Source:     "central",
		TenantID:   tenantID,
		EntityType: "__nonexistent_entity__", // LayerOf returns -1 → early return
		EntityID:   uuid.NewString(),
		Action:     "update",
		Version:    1,
		Diff:       json.RawMessage(`{}`),
	}
	env.Checksum = env.computeChecksum()

	payload, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	subject := "sync." + tenantID + ".__nonexistent_entity__.update"

	before := testutil.ToFloat64(
		telemetry.SyncEnvelopeRejected.WithLabelValues("__nonexistent_entity__", "tenant_mismatch"),
	)

	if err := agent.handleIncomingEnvelope(context.Background(), eventbus.Event{
		Subject: subject,
		Payload: payload,
	}); err != nil {
		t.Fatalf("handleIncomingEnvelope: %v", err)
	}

	after := testutil.ToFloat64(
		telemetry.SyncEnvelopeRejected.WithLabelValues("__nonexistent_entity__", "tenant_mismatch"),
	)

	if after != before {
		t.Errorf("tenant_mismatch counter should not have moved; delta = %v", after-before)
	}
}

// TestHandleIncomingEnvelopeAllowsSubjectWithoutTenantSegment — some
// subjects (e.g. "sync.resync_hint" published by the reconciler) do not
// embed a tenant UUID. The guard should only engage when the subject
// actually carries a tenant segment; otherwise it would reject legitimate
// operational traffic.
func TestHandleIncomingEnvelopeAllowsSubjectWithoutTenantSegment(t *testing.T) {
	agent := NewAgent(nil, nil, &config.Config{EdgeNodeID: "edge-test"})

	tenantID := uuid.NewString()
	env := SyncEnvelope{
		ID:         uuid.NewString(),
		Source:     "central",
		TenantID:   tenantID,
		EntityType: "__nonexistent_entity__",
		EntityID:   uuid.NewString(),
		Action:     "update",
		Version:    1,
		Diff:       json.RawMessage(`{}`),
	}
	env.Checksum = env.computeChecksum()
	payload, _ := json.Marshal(env)

	// Subject has only 2 segments — not `sync.<tenant>.<entity>.<action>`.
	subject := "sync.resync_hint"

	before := testutil.ToFloat64(
		telemetry.SyncEnvelopeRejected.WithLabelValues("__nonexistent_entity__", "tenant_mismatch"),
	)

	if err := agent.handleIncomingEnvelope(context.Background(), eventbus.Event{
		Subject: subject,
		Payload: payload,
	}); err != nil {
		t.Fatalf("handleIncomingEnvelope: %v", err)
	}

	after := testutil.ToFloat64(
		telemetry.SyncEnvelopeRejected.WithLabelValues("__nonexistent_entity__", "tenant_mismatch"),
	)
	if after != before {
		t.Errorf("guard fired on a non-tenant-scoped subject; delta = %v", after-before)
	}
}
