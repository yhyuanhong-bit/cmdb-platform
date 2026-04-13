package quality

import (
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestEvaluateAsset_PerfectScore(t *testing.T) {
	asset := dbgen.Asset{
		Name:     "web-server-01",
		Type:     "server",
		AssetTag: "ASSET-001",
		Vendor:   pgtype.Text{String: "Dell", Valid: true},
		Model:    pgtype.Text{String: "R740", Valid: true},
		RackID:   pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		UpdatedAt: time.Now(),
	}

	result := evaluateAsset(asset, nil)

	if result.Completeness != 100 {
		t.Errorf("expected completeness 100, got %f", result.Completeness)
	}
	if result.Accuracy != 100 {
		t.Errorf("expected accuracy 100, got %f", result.Accuracy)
	}
	if result.Timeliness != 100 {
		t.Errorf("expected timeliness 100, got %f", result.Timeliness)
	}
	if result.Consistency != 100 {
		t.Errorf("expected consistency 100, got %f", result.Consistency)
	}
	if len(result.Issues) != 0 {
		t.Errorf("expected no issues, got %d", len(result.Issues))
	}
}

func TestEvaluateAsset_ServerWithoutRack(t *testing.T) {
	asset := dbgen.Asset{
		Name:      "web-server-02",
		Type:      "server",
		AssetTag:  "ASSET-002",
		UpdatedAt: time.Now(),
	}

	result := evaluateAsset(asset, nil)

	if result.Consistency != 50 {
		t.Errorf("expected consistency 50 for server without rack, got %f", result.Consistency)
	}

	found := false
	for _, issue := range result.Issues {
		if issue["field"] == "rack_id" && issue["dimension"] == "consistency" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected rack_id consistency issue for server without rack")
	}
}

func TestEvaluateAsset_StaleUpdate(t *testing.T) {
	asset := dbgen.Asset{
		Name:      "old-switch",
		Type:      "switch",
		AssetTag:  "ASSET-003",
		UpdatedAt: time.Now().Add(-100 * 24 * time.Hour), // 100 days ago
	}

	result := evaluateAsset(asset, nil)

	if result.Timeliness != 60 {
		t.Errorf("expected timeliness 60 for stale asset, got %f", result.Timeliness)
	}
}

func TestEvaluateAsset_CompletenessRule(t *testing.T) {
	asset := dbgen.Asset{
		Name:      "incomplete-server",
		Type:      "server",
		AssetTag:  "ASSET-004",
		RackID:    pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		UpdatedAt: time.Now(),
	}

	rules := []dbgen.QualityRule{
		{
			CiType:    pgtype.Text{String: "server", Valid: true},
			Dimension: "completeness",
			FieldName: "serial_number",
			RuleType:  "required",
			Weight:    pgtype.Int4{Int32: 20, Valid: true},
			Enabled:   pgtype.Bool{Bool: true, Valid: true},
		},
	}

	result := evaluateAsset(asset, rules)

	if result.Completeness != 80 {
		t.Errorf("expected completeness 80 (100 - 20 weight), got %f", result.Completeness)
	}
}

func TestEvaluateAsset_DisabledRuleIgnored(t *testing.T) {
	asset := dbgen.Asset{
		Name:      "test-server",
		Type:      "server",
		AssetTag:  "ASSET-005",
		RackID:    pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		UpdatedAt: time.Now(),
	}

	rules := []dbgen.QualityRule{
		{
			CiType:    pgtype.Text{String: "server", Valid: true},
			Dimension: "completeness",
			FieldName: "serial_number",
			RuleType:  "required",
			Weight:    pgtype.Int4{Int32: 20, Valid: true},
			Enabled:   pgtype.Bool{Bool: false, Valid: true},
		},
	}

	result := evaluateAsset(asset, rules)

	if result.Completeness != 100 {
		t.Errorf("disabled rule should not affect score, expected 100, got %f", result.Completeness)
	}
}

func TestEvaluateAsset_WrongCiTypeIgnored(t *testing.T) {
	asset := dbgen.Asset{
		Name:      "test-switch",
		Type:      "switch",
		AssetTag:  "ASSET-006",
		UpdatedAt: time.Now(),
	}

	rules := []dbgen.QualityRule{
		{
			CiType:    pgtype.Text{String: "server", Valid: true},
			Dimension: "completeness",
			FieldName: "serial_number",
			RuleType:  "required",
			Weight:    pgtype.Int4{Int32: 20, Valid: true},
			Enabled:   pgtype.Bool{Bool: true, Valid: true},
		},
	}

	result := evaluateAsset(asset, rules)

	if result.Completeness != 100 {
		t.Errorf("rule for different ci_type should not affect score, expected 100, got %f", result.Completeness)
	}
}

func TestEvaluateAsset_TotalWeightedScore(t *testing.T) {
	// Total = completeness*0.4 + accuracy*0.3 + timeliness*0.1 + consistency*0.2
	asset := dbgen.Asset{
		Name:      "perfect-switch",
		Type:      "switch",
		AssetTag:  "ASSET-007",
		UpdatedAt: time.Now(),
	}

	result := evaluateAsset(asset, nil)

	expected := 100*0.4 + 100*0.3 + 100*0.1 + 100*0.2
	if result.Total != expected {
		t.Errorf("expected total %f, got %f", expected, result.Total)
	}
}

func TestEvaluateAsset_ScoresClampToZero(t *testing.T) {
	asset := dbgen.Asset{
		Name:      "bad-server",
		Type:      "server",
		AssetTag:  "ASSET-008",
		RackID:    pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
		UpdatedAt: time.Now(),
	}

	// Apply many heavy completeness rules to drive score below zero
	rules := []dbgen.QualityRule{
		{
			CiType:    pgtype.Text{String: "server", Valid: true},
			Dimension: "completeness",
			FieldName: "serial_number",
			RuleType:  "required",
			Weight:    pgtype.Int4{Int32: 60, Valid: true},
			Enabled:   pgtype.Bool{Bool: true, Valid: true},
		},
		{
			CiType:    pgtype.Text{String: "server", Valid: true},
			Dimension: "completeness",
			FieldName: "vendor",
			RuleType:  "required",
			Weight:    pgtype.Int4{Int32: 60, Valid: true},
			Enabled:   pgtype.Bool{Bool: true, Valid: true},
		},
	}

	result := evaluateAsset(asset, rules)

	if result.Completeness < 0 {
		t.Errorf("completeness should be clamped to 0, got %f", result.Completeness)
	}
}
