package quality

import "testing"

// flagPenaltyFor is the per-asset accuracy deduction driven by the
// CountRecentFlagsByAsset row. The scanner reads flag counts once per
// tenant and applies this function to each asset's count — the cap
// exists so a flood of low-severity reports can't single-handedly
// zero the accuracy dimension.
func TestFlagPenaltyFor(t *testing.T) {
	tests := []struct {
		name  string
		count int64
		want  float64
	}{
		{"zero flags => zero penalty", 0, 0},
		{"negative guarded => zero", -3, 0},
		{"one flag => default weight", 1, flagPenaltyDefault},
		{"four flags => 4x default", 4, 4 * flagPenaltyDefault},
		{"eight flags => capped at total", 8, totalFlagPenaltyCap},
		{"much higher => stays capped", 999, totalFlagPenaltyCap},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := flagPenaltyFor(tc.count)
			if got != tc.want {
				t.Errorf("flagPenaltyFor(%d) = %f, want %f", tc.count, got, tc.want)
			}
		})
	}
}
