package sync

import (
	"encoding/json"
	"testing"
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
