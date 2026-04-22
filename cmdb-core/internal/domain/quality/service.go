package quality

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Source label for telemetry.ErrorsSuppressedTotal on the quality
// rule engine's config parse path. A rule with malformed JSON in
// rule_config used to score the asset silently; now we see it.
const sourceQualityRuleConfig = "quality.rule_config_parse"

// Service provides data quality governance operations.
type Service struct {
	queries *dbgen.Queries
	pool    *pgxpool.Pool
}

// NewService creates a new quality Service.
func NewService(queries *dbgen.Queries, pool ...*pgxpool.Pool) *Service {
	s := &Service{queries: queries}
	if len(pool) > 0 {
		s.pool = pool[0]
	}
	return s
}

// ScanResult holds the evaluation outcome for a single asset.
type ScanResult struct {
	Completeness float64
	Accuracy     float64
	Timeliness   float64
	Consistency  float64
	Total        float64
	Issues       []map[string]string
}

// ListRules returns all quality rules for a tenant.
func (s *Service) ListRules(ctx context.Context, tenantID uuid.UUID) ([]dbgen.QualityRule, error) {
	return s.queries.ListQualityRules(ctx, tenantID)
}

// CreateRule inserts a new quality rule and returns it.
func (s *Service) CreateRule(ctx context.Context, params dbgen.CreateQualityRuleParams) (*dbgen.QualityRule, error) {
	rule, err := s.queries.CreateQualityRule(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create quality rule: %w", err)
	}
	return &rule, nil
}

// GetDashboard returns aggregate quality metrics for the last 24 hours.
func (s *Service) GetDashboard(ctx context.Context, tenantID uuid.UUID) (*dbgen.GetQualityDashboardRow, error) {
	row, err := s.queries.GetQualityDashboard(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("get quality dashboard: %w", err)
	}
	return &row, nil
}

// GetWorstAssets returns the 10 lowest-scoring assets from the last 24 hours.
func (s *Service) GetWorstAssets(ctx context.Context, tenantID uuid.UUID) ([]dbgen.GetWorstAssetsRow, error) {
	return s.queries.GetWorstAssets(ctx, tenantID)
}

// GetAssetHistory returns up to 30 recent quality scores for an asset.
func (s *Service) GetAssetHistory(ctx context.Context, assetID uuid.UUID) ([]dbgen.QualityScore, error) {
	return s.queries.GetAssetQualityHistory(ctx, assetID)
}

// FlagIssueParams is the input for FlagIssue. ReporterID is optional —
// external/downstream reporters may not have a CMDB user identity.
type FlagIssueParams struct {
	TenantID     uuid.UUID
	AssetID      uuid.UUID
	ReporterType string
	ReporterID   *uuid.UUID
	Severity     string
	Category     string
	Message      string
}

// FlagIssue records a consumer-side report that an asset's data is
// wrong. The scanner picks it up on its next pass via
// countRecentFlagsByAsset and applies an accuracy penalty until the
// flag is triaged (resolved/rejected).
func (s *Service) FlagIssue(ctx context.Context, p FlagIssueParams) (*dbgen.QualityFlag, error) {
	params := dbgen.CreateQualityFlagParams{
		TenantID:     p.TenantID,
		AssetID:      p.AssetID,
		ReporterType: p.ReporterType,
		Severity:     p.Severity,
		Category:     p.Category,
		Message:      p.Message,
	}
	if p.ReporterID != nil {
		params.ReporterID = pgtype.UUID{Bytes: *p.ReporterID, Valid: true}
	}
	flag, err := s.queries.CreateQualityFlag(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create quality flag: %w", err)
	}
	return &flag, nil
}

// ListOpenFlags returns up to 100 open flags for the tenant, sorted
// by severity (critical first) then newest first. Used by the
// operator triage view.
func (s *Service) ListOpenFlags(ctx context.Context, tenantID uuid.UUID) ([]dbgen.ListOpenQualityFlagsRow, error) {
	return s.queries.ListOpenQualityFlags(ctx, tenantID)
}

// ResolveFlag transitions an open/acknowledged flag to a terminal
// state. Only 'acknowledged', 'resolved', 'rejected' are accepted.
func (s *Service) ResolveFlag(ctx context.Context, tenantID, flagID uuid.UUID, newStatus string, resolvedBy *uuid.UUID, note string) (*dbgen.QualityFlag, error) {
	params := dbgen.ResolveQualityFlagParams{
		ID:       flagID,
		TenantID: tenantID,
		Status:   newStatus,
	}
	if resolvedBy != nil {
		params.ResolvedBy = pgtype.UUID{Bytes: *resolvedBy, Valid: true}
	}
	if note != "" {
		params.ResolutionNote = pgtype.Text{String: note, Valid: true}
	}
	flag, err := s.queries.ResolveQualityFlag(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("resolve quality flag: %w", err)
	}
	return &flag, nil
}

// flagPenalty maps how much accuracy a recent open flag shaves off.
// Capped per-call at totalFlagPenaltyCap so a storm of low-sev
// reports can't drive the score negative.
const (
	flagPenaltyLow        = 3.0
	flagPenaltyMedium     = 7.0
	flagPenaltyHigh       = 15.0
	flagPenaltyCritical   = 25.0
	totalFlagPenaltyCap   = 40.0
	flagPenaltyDefault    = 5.0
)

// flagPenaltyFor returns the accuracy deduction for a given count
// and severity. Severity is read from the aggregate row's implicit
// default; the caller supplies a weighted count from the raw rows.
func flagPenaltyFor(count int64) float64 {
	if count <= 0 {
		return 0
	}
	p := float64(count) * flagPenaltyDefault
	if p > totalFlagPenaltyCap {
		return totalFlagPenaltyCap
	}
	return p
}

// ScanTenant runs the full per-tenant quality scan and is the entry
// point used by the scheduled scanner (Phase 2.11). It is a thin wrapper
// around ScanAllAssets that discards the scanned-count (the scheduler
// only cares about success/failure so the Prometheus outcome label is
// well-defined) and returns any error unchanged so the caller can log
// it alongside the tenant ID.
func (s *Service) ScanTenant(ctx context.Context, tenantID uuid.UUID) error {
	_, err := s.ScanAllAssets(ctx, tenantID)
	return err
}

// ScanAllAssets evaluates every asset for the tenant and persists quality scores.
func (s *Service) ScanAllAssets(ctx context.Context, tenantID uuid.UUID) (int, error) {
	assets, err := s.queries.ListAssets(ctx, dbgen.ListAssetsParams{
		TenantID: tenantID,
		Limit:    10000,
		Offset:   0,
	})
	if err != nil {
		return 0, fmt.Errorf("list assets for scan: %w", err)
	}

	rules, err := s.queries.ListQualityRules(ctx, tenantID)
	if err != nil {
		return 0, fmt.Errorf("list quality rules: %w", err)
	}

	// D9-P0: fetch open+acknowledged flag counts per asset in one shot so
	// the inner loop stays O(assets) rather than O(assets × query).
	recentFlagCounts := map[uuid.UUID]int64{}
	if flagRows, flagErr := s.queries.CountRecentFlagsByAsset(ctx, tenantID); flagErr == nil {
		for _, r := range flagRows {
			recentFlagCounts[r.AssetID] = r.FlagCount
		}
	} else {
		zap.L().Debug("quality: count recent flags failed", zap.Error(flagErr))
	}

	scanned := 0
	for _, asset := range assets {
		result := evaluateAsset(asset, rules)

		// Apply consumer-feedback penalty on the accuracy dimension.
		// Accuracy is the right dimension because a recent flag means
		// "somebody consuming this CI found its data wrong" — which
		// is the definition of accuracy, not completeness or timeliness.
		if count := recentFlagCounts[asset.ID]; count > 0 {
			penalty := flagPenaltyFor(count)
			result.Accuracy -= penalty
			if result.Accuracy < 0 {
				result.Accuracy = 0
			}
			result.Issues = append(result.Issues, map[string]string{
				"field":     "accuracy",
				"dimension": "accuracy",
				"error":     fmt.Sprintf("%d open quality flag(s) in the last 24h", count),
			})
			result.Total = result.Completeness*0.4 + result.Accuracy*0.3 + result.Timeliness*0.1 + result.Consistency*0.2
		}

		// Location consistency bonus check via MAC cache
		if s.pool != nil && asset.RackID.Valid {
			var detectedRackID *uuid.UUID
			sc := database.Scope(s.pool, tenantID)
			if err := sc.QueryRow(ctx,
				"SELECT detected_rack_id FROM mac_address_cache WHERE asset_id = $2 AND tenant_id = $1",
				asset.ID).Scan(&detectedRackID); err != nil {
				zap.L().Debug("quality: mac cache lookup failed", zap.String("asset", asset.ID.String()), zap.Error(err))
			}

			if detectedRackID != nil {
				assetRack := uuid.UUID(asset.RackID.Bytes)
				if *detectedRackID != assetRack {
					result.Consistency -= 50
					result.Issues = append(result.Issues, map[string]string{
						"field": "rack_id", "dimension": "consistency",
						"error": "CMDB location differs from network-detected location",
					})
					if result.Consistency < 0 {
						result.Consistency = 0
					}
					// Recalculate total
					result.Total = result.Completeness*0.4 + result.Accuracy*0.3 + result.Timeliness*0.1 + result.Consistency*0.2
				}
			}
		}

		issueJSON, _ := json.Marshal(result.Issues)

		_ = s.queries.CreateQualityScore(ctx, dbgen.CreateQualityScoreParams{
			TenantID:     tenantID,
			AssetID:      asset.ID,
			Completeness: numericFromFloat(result.Completeness),
			Accuracy:     numericFromFloat(result.Accuracy),
			Timeliness:   numericFromFloat(result.Timeliness),
			Consistency:  numericFromFloat(result.Consistency),
			TotalScore:   numericFromFloat(result.Total),
			IssueDetails: issueJSON,
			AccessWeight: numericFromFloat(accessWeightFor(asset.AccessCount24h)),
		})
		scanned++
	}
	return scanned, nil
}

// accessWeightFor maps a 24h read count to the per-asset dashboard
// weight stored alongside the score. The log damps the tail so one
// super-hot asset can't dominate the weighted average — a cold asset
// still counts, just less.
//
// Values (for reference):
//
//	count=0    → 1.0
//	count=1    → 1.693
//	count=10   → 3.398
//	count=100  → 5.615
//	count=1000 → 7.908
//	count=1e4  → 10.0 (capped)
//
// Matches the formula stated in migration 000059's comment.
func accessWeightFor(count int32) float64 {
	if count <= 0 {
		return 1.0
	}
	w := 1.0 + math.Log(1.0+float64(count))
	if w > accessWeightCap {
		return accessWeightCap
	}
	return w
}

const accessWeightCap = 10.0

// ValidateForCreation evaluates whether an asset meets minimum quality standards
// before creation. Returns the score and any issues found.
// A nil error means the asset passes the quality gate.
func (s *Service) ValidateForCreation(ctx context.Context, tenantID uuid.UUID, assetType, name, status string, rackID *uuid.UUID, vendor, model, serialNumber string) (*ScanResult, error) {
	rules, err := s.queries.ListQualityRules(ctx, tenantID)
	if err != nil || len(rules) == 0 {
		return nil, nil // No rules = no gate
	}

	// Build a temporary asset for evaluation.
	tmpAsset := dbgen.Asset{
		TenantID:     tenantID,
		Type:         assetType,
		Name:         name,
		Status:       status,
		Vendor:       pgtype.Text{String: vendor, Valid: vendor != ""},
		Model:        pgtype.Text{String: model, Valid: model != ""},
		SerialNumber: pgtype.Text{String: serialNumber, Valid: serialNumber != ""},
		UpdatedAt:    time.Now(), // New asset won't have timeliness penalty.
	}
	if rackID != nil {
		tmpAsset.RackID = pgtype.UUID{Bytes: *rackID, Valid: true}
	}

	result := evaluateAsset(tmpAsset, rules)

	if result.Total < 40 {
		return &result, fmt.Errorf("asset quality score %.0f is below minimum threshold (40). Issues: %v", result.Total, result.Issues)
	}

	return &result, nil
}

func evaluateAsset(asset dbgen.Asset, rules []dbgen.QualityRule) ScanResult {
	scores := map[string]float64{
		"completeness": 100, "accuracy": 100, "timeliness": 100, "consistency": 100,
	}
	var issues []map[string]string

	for _, rule := range rules {
		// Check if rule applies to this asset type.
		if rule.CiType.Valid && rule.CiType.String != asset.Type {
			continue
		}
		if rule.Enabled.Valid && !rule.Enabled.Bool {
			continue
		}

		weight := int32(10)
		if rule.Weight.Valid {
			weight = rule.Weight.Int32
		}

		value := getAssetField(asset, rule.FieldName)

		switch rule.Dimension {
		case "completeness":
			if rule.RuleType == "required" && value == "" {
				scores["completeness"] -= float64(weight)
				issues = append(issues, map[string]string{
					"field": rule.FieldName, "dimension": "completeness", "error": "missing required field",
				})
			}
		case "accuracy":
			if rule.RuleType == "regex" && value != "" {
				var config map[string]string
				// A malformed rule_config JSON used to be a silent
				// no-op: the asset would score full points on the
				// accuracy dimension despite the rule being broken.
				// Warn + counter so a bad rule is visible, then
				// skip the regex check for this field.
				if cfgErr := json.Unmarshal(rule.RuleConfig, &config); cfgErr != nil {
					zap.L().Warn("quality: rule_config parse failed",
						zap.String("rule_id", rule.ID.String()),
						zap.String("field", rule.FieldName),
						zap.Error(cfgErr))
					telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceQualityRuleConfig, telemetry.ReasonJSONUnmarshal).Inc()
					continue
				}
				if pattern, ok := config["regex"]; ok {
					if matched, _ := regexp.MatchString(pattern, value); !matched {
						scores["accuracy"] -= float64(weight)
						issues = append(issues, map[string]string{
							"field": rule.FieldName, "dimension": "accuracy", "error": fmt.Sprintf("format mismatch: %s", value),
						})
					}
				}
			}
		case "consistency":
			if rule.RuleType == "required" && value == "" {
				scores["consistency"] -= float64(weight)
				issues = append(issues, map[string]string{
					"field": rule.FieldName, "dimension": "consistency", "error": "missing required field for consistency",
				})
			}
		}
	}

	// Timeliness: >90 days without update penalises score.
	if time.Since(asset.UpdatedAt) > 90*24*time.Hour {
		scores["timeliness"] = 60
		issues = append(issues, map[string]string{
			"field": "updated_at", "dimension": "timeliness", "error": "not updated in 90+ days",
		})
	}

	// Consistency: servers should have rack_id.
	if asset.Type == "server" && !asset.RackID.Valid {
		scores["consistency"] -= 50
		issues = append(issues, map[string]string{
			"field": "rack_id", "dimension": "consistency", "error": "server not assigned to rack",
		})
	}

	// Clamp scores to 0.
	for k, v := range scores {
		if v < 0 {
			scores[k] = 0
		}
	}

	total := scores["completeness"]*0.4 + scores["accuracy"]*0.3 + scores["timeliness"]*0.1 + scores["consistency"]*0.2

	return ScanResult{
		Completeness: scores["completeness"],
		Accuracy:     scores["accuracy"],
		Timeliness:   scores["timeliness"],
		Consistency:  scores["consistency"],
		Total:        total,
		Issues:       issues,
	}
}

func getAssetField(asset dbgen.Asset, fieldName string) string {
	switch fieldName {
	case "name":
		return asset.Name
	case "serial_number":
		if asset.SerialNumber.Valid {
			return asset.SerialNumber.String
		}
	case "vendor":
		if asset.Vendor.Valid {
			return asset.Vendor.String
		}
	case "model":
		if asset.Model.Valid {
			return asset.Model.String
		}
	case "rack_id":
		if asset.RackID.Valid {
			return fmt.Sprintf("%x-%x-%x-%x-%x", asset.RackID.Bytes[0:4], asset.RackID.Bytes[4:6], asset.RackID.Bytes[6:8], asset.RackID.Bytes[8:10], asset.RackID.Bytes[10:16])
		}
	case "location_id":
		if asset.LocationID.Valid {
			return fmt.Sprintf("%x-%x-%x-%x-%x", asset.LocationID.Bytes[0:4], asset.LocationID.Bytes[4:6], asset.LocationID.Bytes[6:8], asset.LocationID.Bytes[8:10], asset.LocationID.Bytes[10:16])
		}
	case "asset_tag":
		return asset.AssetTag
	}
	return ""
}

func numericFromFloat(f float64) pgtype.Numeric {
	var n pgtype.Numeric
	_ = n.Scan(fmt.Sprintf("%.2f", f))
	return n
}
