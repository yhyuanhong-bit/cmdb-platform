package dashboard

import (
	"testing"

	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
)

// TestInvalidationSubjects_CoversAllStatsFields pins the full list of
// subjects against the eight Stats fields. When a new field is added
// to Stats, this test forces the author to decide which events should
// invalidate it — rather than silently shipping a stat that serves
// stale data for 60s after every relevant change.
func TestInvalidationSubjects_CoversAllStatsFields(t *testing.T) {
	// Field → subjects that should invalidate its cached value.
	expected := map[string][]string{
		"assets (TotalAssets, EnergyCurrentKW, AvgQualityScore)": {
			eventbus.SubjectAssetCreated,
			eventbus.SubjectAssetUpdated,
			eventbus.SubjectAssetStatusChanged,
			eventbus.SubjectAssetDeleted,
		},
		"racks (TotalRacks, RackUtilizationPct)": {
			eventbus.SubjectRackCreated,
			eventbus.SubjectRackUpdated,
			eventbus.SubjectRackDeleted,
			eventbus.SubjectRackOccupancyChanged,
		},
		"alerts (CriticalAlerts)": {
			eventbus.SubjectAlertFired,
			eventbus.SubjectAlertResolved,
		},
		"work orders (ActiveOrders, PendingWorkOrders)": {
			eventbus.SubjectOrderCreated,
			eventbus.SubjectOrderUpdated,
			eventbus.SubjectOrderTransitioned,
		},
	}

	got := make(map[string]bool, len(invalidationSubjects))
	for _, s := range invalidationSubjects {
		got[s] = true
	}

	for group, subjects := range expected {
		for _, s := range subjects {
			if !got[s] {
				t.Errorf("missing subject %q (needed to invalidate %s)", s, group)
			}
		}
	}
	total := 0
	for _, subjects := range expected {
		total += len(subjects)
	}
	if len(invalidationSubjects) != total {
		t.Errorf("invalidationSubjects has %d entries, expected %d — drift between map above and the package var",
			len(invalidationSubjects), total)
	}
}
