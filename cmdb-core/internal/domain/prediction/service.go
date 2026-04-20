package prediction

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/cmdb-platform/cmdb-core/internal/ai"
	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// Queries is the subset of dbgen.Queries the prediction service relies on.
// Defining it here keeps the service decoupled from the full generated
// surface and lets tests substitute a fake implementation without bringing
// up a real database.
type Queries interface {
	ListAllModels(ctx context.Context) ([]dbgen.PredictionModel, error)
	CreateRCA(ctx context.Context, arg dbgen.CreateRCAParams) (dbgen.RcaAnalysis, error)
	VerifyRCA(ctx context.Context, arg dbgen.VerifyRCAParams) (dbgen.RcaAnalysis, error)
	ListAlertEventsByIncident(ctx context.Context, arg dbgen.ListAlertEventsByIncidentParams) ([]dbgen.ListAlertEventsByIncidentRow, error)
	ListAssetsForIncident(ctx context.Context, arg dbgen.ListAssetsForIncidentParams) ([]dbgen.Asset, error)
}

// Service provides prediction and RCA operations.
type Service struct {
	queries  Queries
	registry *ai.Registry
}

// NewService creates a new prediction Service.
func NewService(queries Queries, registry *ai.Registry) *Service {
	return &Service{queries: queries, registry: registry}
}

// ListModels returns all prediction models.
func (s *Service) ListModels(ctx context.Context) ([]dbgen.PredictionModel, error) {
	models, err := s.queries.ListAllModels(ctx)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	return models, nil
}

// CreateRCA performs root-cause analysis via an AI provider and stores the result.
// If no provider is found, a placeholder record is created.
//
// The tenantID argument MUST come from the caller's request context (e.g. the
// JWT / auth middleware). It is used as the authoritative tenant scope for all
// downstream queries, independent of any tenant_id that may be stored on the
// incident row. This is defence-in-depth against a forged or mis-linked
// incident_id bleeding data across tenants.
func (s *Service) CreateRCA(ctx context.Context, tenantID uuid.UUID, req CreateRCARequest) (*dbgen.RcaAnalysis, error) {
	modelName := req.ModelName
	if modelName == "" {
		modelName = "Default RCA"
	}

	provider, found := s.registry.Get(modelName)
	if !found {
		// No AI provider configured — store a placeholder.
		placeholder, _ := json.Marshal(map[string]string{"status": "no_ai_provider_configured"})
		rca, err := s.queries.CreateRCA(ctx, dbgen.CreateRCAParams{
			TenantID:          tenantID,
			IncidentID:        req.IncidentID,
			ModelID:           pgtype.UUID{Valid: false},
			Reasoning:         placeholder,
			ConclusionAssetID: pgtype.UUID{Valid: false},
			Confidence:        pgtype.Numeric{Valid: false},
		})
		if err != nil {
			return nil, fmt.Errorf("create placeholder rca: %w", err)
		}
		return &rca, nil
	}

	// Gather related alerts and affected assets for this incident, tenant-scoped.
	// Both calls enforce tenant_id independently so that even a mis-associated
	// incident cannot surface data from another tenant.
	alerts, err := s.queries.ListAlertEventsByIncident(ctx, dbgen.ListAlertEventsByIncidentParams{
		IncidentID: req.IncidentID,
		TenantID:   tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("list alerts for incident: %w", err)
	}
	assets, err := s.queries.ListAssetsForIncident(ctx, dbgen.ListAssetsForIncidentParams{
		IncidentID: req.IncidentID,
		TenantID:   tenantID,
	})
	if err != nil {
		return nil, fmt.Errorf("list assets for incident: %w", err)
	}

	// Call the AI provider for root-cause analysis with real context.
	rcaResult, err := provider.AnalyzeRootCause(ctx, ai.RCARequest{
		IncidentID:     req.IncidentID,
		TenantID:       tenantID,
		RelatedAlerts:  mapAlertsForRCA(alerts),
		AffectedAssets: mapAssetsForRCA(assets),
		Context:        req.Context,
	})
	if err != nil {
		return nil, fmt.Errorf("analyze root cause: %w", err)
	}

	// Build params from the AI result.
	params := dbgen.CreateRCAParams{
		TenantID:   tenantID,
		IncidentID: req.IncidentID,
		ModelID:    pgtype.UUID{Valid: false},
		Reasoning:  rcaResult.Reasoning,
		ConclusionAssetID: pgtype.UUID{Valid: false},
		Confidence: numericFromFloat(rcaResult.Confidence),
	}

	if rcaResult.ConclusionAssetID != nil {
		params.ConclusionAssetID = pgtype.UUID{Bytes: *rcaResult.ConclusionAssetID, Valid: true}
	}

	rca, err := s.queries.CreateRCA(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("create rca: %w", err)
	}
	return &rca, nil
}

// VerifyRCA marks an RCA as human-verified.
func (s *Service) VerifyRCA(ctx context.Context, id uuid.UUID, verifiedBy uuid.UUID) (*dbgen.RcaAnalysis, error) {
	rca, err := s.queries.VerifyRCA(ctx, dbgen.VerifyRCAParams{
		ID:         id,
		VerifiedBy: pgtype.UUID{Bytes: verifiedBy, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("verify rca: %w", err)
	}
	return &rca, nil
}

// mapAlertsForRCA converts sqlc rows into the lightweight ai.AlertBrief shape
// the RCA request expects. Alerts without an asset_id are still included —
// some alerting rules fire without a specific asset.
func mapAlertsForRCA(rows []dbgen.ListAlertEventsByIncidentRow) []ai.AlertBrief {
	briefs := make([]ai.AlertBrief, 0, len(rows))
	for _, r := range rows {
		b := ai.AlertBrief{
			ID:       r.ID,
			Severity: r.Severity,
			FiredAt:  r.FiredAt,
		}
		if r.AssetID.Valid {
			b.AssetID = r.AssetID.Bytes
		}
		if r.Message.Valid {
			b.Message = r.Message.String
		}
		briefs = append(briefs, b)
	}
	return briefs
}

// mapAssetsForRCA converts sqlc asset rows into the lightweight ai.AssetBrief
// shape the RCA request expects.
func mapAssetsForRCA(rows []dbgen.Asset) []ai.AssetBrief {
	briefs := make([]ai.AssetBrief, 0, len(rows))
	for _, r := range rows {
		briefs = append(briefs, ai.AssetBrief{
			ID:     r.ID,
			Name:   r.Name,
			Type:   r.Type,
			Status: r.Status,
		})
	}
	return briefs
}

// numericFromFloat converts a float64 to pgtype.Numeric.
func numericFromFloat(f float64) pgtype.Numeric {
	// Use big.Float to convert to Int with scaling for 4 decimal places.
	bf := new(big.Float).SetFloat64(f * 10000)
	bi, _ := bf.Int(nil)
	return pgtype.Numeric{
		Int:              bi,
		Exp:              -4,
		NaN:              false,
		InfinityModifier: pgtype.Finite,
		Valid:            true,
	}
}
