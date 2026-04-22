package quality

import (
	"math"
	"testing"
)

// accessWeightFor maps an asset's 24h read count into the weight used
// by the tenant dashboard's weighted average. The log damps the tail
// so the heaviest-read asset can't drown out cold ones, but cold
// assets still count (weight = 1.0, not 0).
func TestAccessWeightFor(t *testing.T) {
	const eps = 1e-6
	tests := []struct {
		name  string
		count int32
		want  float64
	}{
		{"cold asset", 0, 1.0},
		{"negative guarded to cold", -5, 1.0},
		{"one read", 1, 1.0 + math.Log(2.0)},
		{"ten reads", 10, 1.0 + math.Log(11.0)},
		{"hundred reads", 100, 1.0 + math.Log(101.0)},
		{"thousand reads", 1000, 1.0 + math.Log(1001.0)},
		{"at the cap threshold", 10000, accessWeightCap},
		{"way above cap stays at cap", 1_000_000, accessWeightCap},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := accessWeightFor(tc.count)
			if math.Abs(got-tc.want) > eps {
				t.Errorf("accessWeightFor(%d) = %v, want %v", tc.count, got, tc.want)
			}
		})
	}
}

// TestAccessWeightFor_Monotonic pins down the shape of the curve:
// more reads never decreases weight. A regression that accidentally
// flips a sign or flips min/max would be caught here.
func TestAccessWeightFor_Monotonic(t *testing.T) {
	prev := accessWeightFor(0)
	for c := int32(1); c <= 20000; c *= 2 {
		got := accessWeightFor(c)
		if got < prev {
			t.Errorf("accessWeightFor(%d) = %v dropped below prev %v — weight curve must be non-decreasing", c, got, prev)
		}
		prev = got
	}
}

// TestAccessWeightFor_CapRespected guards the NUMERIC(6,3) CHECK
// constraint from migration 000059. If the function ever exceeds 10,
// inserts will fail at write time.
func TestAccessWeightFor_CapRespected(t *testing.T) {
	for _, count := range []int32{0, 1, 100, 10000, 100000, math.MaxInt32} {
		got := accessWeightFor(count)
		if got < 0 || got > accessWeightCap {
			t.Errorf("accessWeightFor(%d) = %v outside [0, %v]", count, got, accessWeightCap)
		}
	}
}
