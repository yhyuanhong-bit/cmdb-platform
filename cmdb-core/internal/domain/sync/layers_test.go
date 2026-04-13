// layers_test.go
package sync

import "testing"

func TestSyncLayersCompleteness(t *testing.T) {
	seen := make(map[string]bool)
	for _, layer := range SyncLayers {
		for _, entity := range layer {
			if seen[entity] {
				t.Errorf("duplicate entity %q in SyncLayers", entity)
			}
			seen[entity] = true
		}
	}
	required := []string{"locations", "assets", "racks", "work_orders", "alert_events", "inventory_tasks"}
	for _, r := range required {
		if !seen[r] {
			t.Errorf("required entity %q missing from SyncLayers", r)
		}
	}
}

func TestSyncLayerOrder(t *testing.T) {
	indexOf := func(entity string) int {
		for i, layer := range SyncLayers {
			for _, e := range layer {
				if e == entity {
					return i
				}
			}
		}
		return -1
	}
	// locations must be before racks
	if indexOf("locations") >= indexOf("racks") {
		t.Error("locations must be in an earlier layer than racks")
	}
	// assets must be before work_orders
	if indexOf("assets") >= indexOf("work_orders") {
		t.Error("assets must be in an earlier layer than work_orders")
	}
}
