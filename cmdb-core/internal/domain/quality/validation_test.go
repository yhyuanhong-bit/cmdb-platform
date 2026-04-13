package quality

import (
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestEvaluateAsset_NoRules(t *testing.T) {
	asset := dbgen.Asset{
		Type:      "server",
		Name:      "test-server",
		Status:    "operational",
		UpdatedAt: time.Now(),
	}
	result := evaluateAsset(asset, nil)
	// With no rules, only the hardcoded consistency check for server+rack applies.
	// Server without rack_id gets -50 consistency. Total = 100*0.4 + 100*0.3 + 100*0.1 + 50*0.2 = 90
	if result.Total != 90 {
		t.Errorf("expected 90 for server without rack and no rules, got %.0f", result.Total)
	}
}

func TestEvaluateAsset_NonServerNoRules(t *testing.T) {
	asset := dbgen.Asset{
		Type:      "switch",
		Name:      "test-switch",
		Status:    "operational",
		UpdatedAt: time.Now(),
	}
	result := evaluateAsset(asset, nil)
	if result.Total != 100 {
		t.Errorf("expected 100 for non-server with no rules, got %.0f", result.Total)
	}
}

func TestEvaluateAsset_RequiredFieldMissing(t *testing.T) {
	asset := dbgen.Asset{
		Type:      "switch",
		Name:      "test-switch",
		Status:    "operational",
		UpdatedAt: time.Now(),
	}
	rules := []dbgen.QualityRule{
		{
			Dimension: "completeness",
			FieldName: "serial_number",
			RuleType:  "required",
			Weight:    pgtype.Int4{Int32: 50, Valid: true},
			Enabled:   pgtype.Bool{Bool: true, Valid: true},
		},
		{
			Dimension: "completeness",
			FieldName: "vendor",
			RuleType:  "required",
			Weight:    pgtype.Int4{Int32: 50, Valid: true},
			Enabled:   pgtype.Bool{Bool: true, Valid: true},
		},
	}
	result := evaluateAsset(asset, rules)
	// Completeness should be 0 (100 - 50 - 50), total = 0*0.4 + 100*0.3 + 100*0.1 + 100*0.2 = 60
	if result.Completeness != 0 {
		t.Errorf("expected completeness 0, got %.0f", result.Completeness)
	}
	if result.Total != 60 {
		t.Errorf("expected total 60, got %.0f", result.Total)
	}
	if len(result.Issues) != 2 {
		t.Errorf("expected 2 issues, got %d", len(result.Issues))
	}
}

func TestEvaluateAsset_BelowQualityGateThreshold(t *testing.T) {
	// An asset that would score below 40 should fail the quality gate.
	asset := dbgen.Asset{
		Type:      "server",
		Name:      "s",
		Status:    "operational",
		UpdatedAt: time.Now(),
		// No rack_id, no vendor, no model, no serial_number
	}
	rules := []dbgen.QualityRule{
		{
			Dimension: "completeness",
			FieldName: "serial_number",
			RuleType:  "required",
			Weight:    pgtype.Int4{Int32: 50, Valid: true},
			Enabled:   pgtype.Bool{Bool: true, Valid: true},
		},
		{
			Dimension: "completeness",
			FieldName: "vendor",
			RuleType:  "required",
			Weight:    pgtype.Int4{Int32: 50, Valid: true},
			Enabled:   pgtype.Bool{Bool: true, Valid: true},
		},
	}
	result := evaluateAsset(asset, rules)
	// Completeness=0, Accuracy=100, Timeliness=100, Consistency=50 (server no rack)
	// Total = 0*0.4 + 100*0.3 + 100*0.1 + 50*0.2 = 50
	// Wait — that's 50, which is above 40. Let's add more rules to drop it below.
	// Actually 50 > 40, so let's also penalize accuracy.
	rules = append(rules, dbgen.QualityRule{
		Dimension: "accuracy",
		FieldName: "name",
		RuleType:  "regex",
		RuleConfig: []byte(`{"regex":"^[a-z]+-[a-z]+-\\d{3}$"}`),
		Weight:    pgtype.Int4{Int32: 100, Valid: true},
		Enabled:   pgtype.Bool{Bool: true, Valid: true},
	})
	result = evaluateAsset(asset, rules)
	// Completeness=0, Accuracy=0, Timeliness=100, Consistency=50
	// Total = 0*0.4 + 0*0.3 + 100*0.1 + 50*0.2 = 20
	if result.Total != 20 {
		t.Errorf("expected total 20, got %.0f", result.Total)
	}
	if result.Total >= 40 {
		t.Errorf("expected total below 40, got %.0f", result.Total)
	}
}

func TestEvaluateAsset_TimelinessPenalty(t *testing.T) {
	asset := dbgen.Asset{
		Type:      "switch",
		Name:      "test-switch",
		Status:    "operational",
		UpdatedAt: time.Now().Add(-100 * 24 * time.Hour), // 100 days old
	}
	result := evaluateAsset(asset, nil)
	if result.Timeliness != 60 {
		t.Errorf("expected timeliness 60 for stale asset, got %.0f", result.Timeliness)
	}
}
