// layers.go
package sync

// SyncLayers defines entity processing order for dependency-safe sync.
// Each layer depends only on layers with a lower index.
var SyncLayers = [][]string{
	{"locations", "users", "roles", "alert_rules"},                                      // Layer 0: no dependencies
	{"racks", "assets"},                                                                  // Layer 1: depends on locations
	{"rack_slots", "asset_dependencies", "alert_events"},                                 // Layer 2: depends on racks, assets
	{"work_orders", "inventory_tasks"},                                                   // Layer 3: depends on assets
	{"work_order_logs", "inventory_items", "inventory_scan_history", "audit_events"},      // Layer 4: depends on work_orders, inventory_tasks
}

// LayerOf returns the layer index for an entity type, or -1 if unknown.
func LayerOf(entityType string) int {
	for i, layer := range SyncLayers {
		for _, e := range layer {
			if e == entityType {
				return i
			}
		}
	}
	return -1
}
