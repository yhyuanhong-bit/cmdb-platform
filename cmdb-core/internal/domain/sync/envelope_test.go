package sync

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewEnvelope(t *testing.T) {
	diff := json.RawMessage(`{"name":"test"}`)
	env := NewEnvelope("central", "tenant-1", "assets", "asset-1", "create", 1, diff)

	if env.Source != "central" {
		t.Errorf("expected source 'central', got %q", env.Source)
	}
	if env.EntityType != "assets" {
		t.Errorf("expected entity_type 'assets', got %q", env.EntityType)
	}
	if env.Checksum == "" {
		t.Error("expected non-empty checksum")
	}
	if env.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestEnvelopeVerifyChecksum(t *testing.T) {
	diff := json.RawMessage(`{"status":"active"}`)
	env := NewEnvelope("edge-1", "t1", "assets", "a1", "update", 5, diff)

	if !env.VerifyChecksum() {
		t.Error("checksum should verify for unmodified envelope")
	}

	// Tamper with the payload
	env.Diff = json.RawMessage(`{"status":"hacked"}`)
	if env.VerifyChecksum() {
		t.Error("checksum should fail for tampered envelope")
	}
}

func TestEnvelopeJSON(t *testing.T) {
	diff := json.RawMessage(`{"x":1}`)
	env := NewEnvelope("central", "t1", "racks", "r1", "delete", 10, diff)

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var decoded SyncEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if decoded.EntityID != env.EntityID {
		t.Errorf("entity_id mismatch: %q vs %q", decoded.EntityID, env.EntityID)
	}
	if decoded.Checksum != env.Checksum {
		t.Errorf("checksum mismatch")
	}
}

// TestComputeChecksumV2CoversSource verifies Bug #3: two envelopes
// differing ONLY in Source field MUST produce different v2 checksums.
// Before the fix, computeChecksum hashed only EntityID|Version|Diff, so
// an attacker could substitute Source="central" on an edge-originated
// envelope without breaking the fingerprint.
func TestComputeChecksumV2CoversSource(t *testing.T) {
	diff := json.RawMessage(`{"k":"v"}`)
	base := SyncEnvelope{
		ID:         "00000000-0000-0000-0000-000000000001",
		Source:     "edge-01",
		TenantID:   "tenant-A",
		EntityType: "alert_rules",
		EntityID:   "rule-1",
		Action:     "update",
		Version:    7,
		Timestamp:  time.Unix(1713600000, 0).UTC(),
		Diff:       diff,
	}
	tampered := base
	tampered.Source = "central"

	if base.computeChecksumV2() == tampered.computeChecksumV2() {
		t.Error("v2 checksum did not detect Source tampering — Bug #3 regression")
	}
}

// TestComputeChecksumV2CoversAllCriticalFields runs the full tamper matrix
// against v2. Source / TenantID / Action / Timestamp are the four fields
// listed in Phase 4.3 open issue #2 as uncovered by the legacy checksum.
func TestComputeChecksumV2CoversAllCriticalFields(t *testing.T) {
	base := SyncEnvelope{
		ID:         "00000000-0000-0000-0000-000000000001",
		Source:     "edge-01",
		TenantID:   "tenant-A",
		EntityType: "alert_rules",
		EntityID:   "rule-1",
		Action:     "update",
		Version:    7,
		Timestamp:  time.Unix(1713600000, 0).UTC(),
		Diff:       json.RawMessage(`{"k":"v"}`),
	}
	baseSum := base.computeChecksumV2()

	cases := []struct {
		name   string
		mutate func(*SyncEnvelope)
	}{
		{"source", func(e *SyncEnvelope) { e.Source = "central" }},
		{"tenant_id", func(e *SyncEnvelope) { e.TenantID = "tenant-B" }},
		{"action", func(e *SyncEnvelope) { e.Action = "delete" }},
		{"timestamp", func(e *SyncEnvelope) { e.Timestamp = time.Unix(0, 0).UTC() }},
		{"entity_type", func(e *SyncEnvelope) { e.EntityType = "user_roles" }},
		{"entity_id", func(e *SyncEnvelope) { e.EntityID = "rule-2" }},
		{"version", func(e *SyncEnvelope) { e.Version = 99 }},
		{"diff", func(e *SyncEnvelope) { e.Diff = json.RawMessage(`{"k":"pwn"}`) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := base
			tc.mutate(&mutated)
			if mutated.computeChecksumV2() == baseSum {
				t.Errorf("v2 checksum did not detect %s tampering", tc.name)
			}
		})
	}
}

// TestVerifyChecksumAcceptsV2 — the happy path: NewEnvelope populates both
// v1 and v2; VerifyChecksum must accept the unmodified envelope.
func TestVerifyChecksumAcceptsV2(t *testing.T) {
	env := NewEnvelope("central", "t1", "alert_rules", "r1", "update", 5,
		json.RawMessage(`{"severity":"critical"}`))
	if env.ChecksumV2 == "" {
		t.Fatal("NewEnvelope did not populate ChecksumV2")
	}
	if !env.VerifyChecksum() {
		t.Error("VerifyChecksum rejected a freshly-made envelope")
	}
}

// TestVerifyChecksumRejectsV2Tampering — when v2 is present and a covered
// field is tampered, verification MUST fail. This is the whole point of
// Bug #3: Source/TenantID/Action/Timestamp tampering was previously
// undetectable.
func TestVerifyChecksumRejectsV2Tampering(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*SyncEnvelope)
	}{
		{"source", func(e *SyncEnvelope) { e.Source = "edge-evil" }},
		{"tenant_id", func(e *SyncEnvelope) { e.TenantID = "tenant-other" }},
		{"action", func(e *SyncEnvelope) { e.Action = "delete" }},
		{"timestamp", func(e *SyncEnvelope) { e.Timestamp = time.Unix(0, 0).UTC() }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := NewEnvelope("central", "t1", "alert_rules", "r1", "update", 5,
				json.RawMessage(`{"severity":"critical"}`))
			tc.mutate(&env)
			if env.VerifyChecksum() {
				t.Errorf("VerifyChecksum accepted tampered %s", tc.name)
			}
		})
	}
}

// TestVerifyChecksumBackCompatV1Only — during the rolling-deploy grace
// window, in-flight envelopes produced by old senders carry only v1. The
// receiver MUST still accept them (otherwise we'd drop legitimate traffic
// mid-rollout); the fallback path is exercised here by wiping ChecksumV2.
//
// This back-compat path is a short-lived grace window. Once the full fleet
// has been rolled to the new sender (post-rollout + 14 days of JetStream
// MaxAge to drain durable consumers), the v1-only branch can be removed.
func TestVerifyChecksumBackCompatV1Only(t *testing.T) {
	env := NewEnvelope("central", "t1", "alert_rules", "r1", "update", 5,
		json.RawMessage(`{"severity":"critical"}`))
	// Simulate an old-sender envelope: strip v2.
	env.ChecksumV2 = ""
	if !env.VerifyChecksum() {
		t.Error("v1-only envelope rejected — back-compat grace window broken")
	}
}

// TestVerifyChecksumJSONRoundTrip ensures the new field survives the
// marshal/unmarshal cycle exactly like the legacy Checksum field does.
func TestVerifyChecksumJSONRoundTrip(t *testing.T) {
	env := NewEnvelope("central", "t1", "alert_rules", "r1", "update", 5,
		json.RawMessage(`{"severity":"critical"}`))

	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded SyncEnvelope
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.ChecksumV2 != env.ChecksumV2 {
		t.Errorf("ChecksumV2 lost in round-trip: %q vs %q", decoded.ChecksumV2, env.ChecksumV2)
	}
	if !decoded.VerifyChecksum() {
		t.Error("decoded envelope failed VerifyChecksum")
	}
}

func TestLayerOf(t *testing.T) {
	tests := []struct {
		entity   string
		expected int
	}{
		{"locations", 0},
		{"assets", 1},
		{"racks", 1},
		{"rack_slots", 2},
		{"work_orders", 3},
		{"audit_events", 4},
		{"unknown_table", -1},
	}
	for _, tt := range tests {
		t.Run(tt.entity, func(t *testing.T) {
			got := LayerOf(tt.entity)
			if got != tt.expected {
				t.Errorf("LayerOf(%q) = %d, want %d", tt.entity, got, tt.expected)
			}
		})
	}
}
