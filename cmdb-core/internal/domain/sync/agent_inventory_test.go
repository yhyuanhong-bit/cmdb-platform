package sync

import (
	"encoding/json"
	"testing"
)

func TestApplyInventoryTaskPayloadParse(t *testing.T) {
	payload := `{"id":"00000000-0000-0000-0000-000000000001","tenant_id":"00000000-0000-0000-0000-000000000002","name":"Q1 Inventory","status":"in_progress","sync_version":5}`
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["name"] != "Q1 Inventory" {
		t.Errorf("name = %v, want Q1 Inventory", m["name"])
	}
	if m["status"] != "in_progress" {
		t.Errorf("status = %v, want in_progress", m["status"])
	}
}

func TestApplyAuditEventPayloadParse(t *testing.T) {
	payload := `{"id":"00000000-0000-0000-0000-000000000001","tenant_id":"00000000-0000-0000-0000-000000000002","action":"asset.created","module":"assets","target_type":"asset","target_id":"00000000-0000-0000-0000-000000000003","source":"edge-taipei","created_at":"2026-04-13T10:00:00Z"}`
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m["action"] != "asset.created" {
		t.Errorf("action = %v, want asset.created", m["action"])
	}
	if m["source"] != "edge-taipei" {
		t.Errorf("source = %v, want edge-taipei", m["source"])
	}
}
