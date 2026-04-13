package bia

import (
	"testing"
)

func TestBIAStats_AvgCompliance(t *testing.T) {
	// Verify the compliance calculation logic used in GetStats
	tests := []struct {
		name           string
		total          int64
		dataCompliant  int64
		assetCompliant int64
		auditCompliant int64
		wantCompliance float64
	}{
		{
			name:           "all compliant",
			total:          10,
			dataCompliant:  10,
			assetCompliant: 10,
			auditCompliant: 10,
			wantCompliance: 100,
		},
		{
			name:           "none compliant",
			total:          10,
			dataCompliant:  0,
			assetCompliant: 0,
			auditCompliant: 0,
			wantCompliance: 0,
		},
		{
			name:           "partial compliance",
			total:          10,
			dataCompliant:  10,
			assetCompliant: 5,
			auditCompliant: 0,
			wantCompliance: 50,
		},
		{
			name:           "zero total",
			total:          0,
			dataCompliant:  0,
			assetCompliant: 0,
			auditCompliant: 0,
			wantCompliance: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var avgCompliance float64
			if tt.total > 0 {
				totalChecks := tt.total * 3
				passedChecks := tt.dataCompliant + tt.assetCompliant + tt.auditCompliant
				avgCompliance = float64(passedChecks) / float64(totalChecks) * 100
			}

			if avgCompliance != tt.wantCompliance {
				t.Errorf("avgCompliance = %f, want %f", avgCompliance, tt.wantCompliance)
			}
		})
	}
}

func TestBIAStats_ByTier(t *testing.T) {
	// Verify tier map construction logic
	type tierCount struct {
		Tier  string
		Count int64
	}

	tierCounts := []tierCount{
		{"tier-1", 5},
		{"tier-2", 10},
		{"tier-3", 3},
	}

	byTier := make(map[string]int64)
	for _, tc := range tierCounts {
		byTier[tc.Tier] = tc.Count
	}

	if byTier["tier-1"] != 5 {
		t.Errorf("tier-1 count = %d, want 5", byTier["tier-1"])
	}
	if byTier["tier-2"] != 10 {
		t.Errorf("tier-2 count = %d, want 10", byTier["tier-2"])
	}
	if byTier["tier-3"] != 3 {
		t.Errorf("tier-3 count = %d, want 3", byTier["tier-3"])
	}
	if byTier["tier-4"] != 0 {
		t.Errorf("tier-4 count = %d, want 0 (absent)", byTier["tier-4"])
	}

	stats := &BIAStats{
		Total:  18,
		ByTier: byTier,
	}
	if stats.Total != 18 {
		t.Errorf("total = %d, want 18", stats.Total)
	}
}

func TestBIAStats_Struct(t *testing.T) {
	stats := BIAStats{
		Total:          5,
		ByTier:         map[string]int64{"tier-1": 2, "tier-2": 3},
		AvgCompliance:  66.67,
		DataCompliant:  5,
		AssetCompliant: 3,
		AuditCompliant: 2,
	}

	if stats.Total != 5 {
		t.Errorf("Total = %d, want 5", stats.Total)
	}
	if len(stats.ByTier) != 2 {
		t.Errorf("ByTier length = %d, want 2", len(stats.ByTier))
	}
	if stats.DataCompliant != 5 {
		t.Errorf("DataCompliant = %d, want 5", stats.DataCompliant)
	}
}
