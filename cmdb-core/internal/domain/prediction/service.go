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

// Service provides prediction and RCA operations.
type Service struct {
	queries  *dbgen.Queries
	registry *ai.Registry
}

// NewService creates a new prediction Service.
func NewService(queries *dbgen.Queries, registry *ai.Registry) *Service {
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

// ListByAsset returns prediction results for a given asset, up to the limit.
func (s *Service) ListByAsset(ctx context.Context, assetID uuid.UUID, limit int32) ([]dbgen.PredictionResult, error) {
	results, err := s.queries.ListPredictionsByAsset(ctx, dbgen.ListPredictionsByAssetParams{
		AssetID: assetID,
		Limit:   limit,
	})
	if err != nil {
		return nil, fmt.Errorf("list predictions by asset: %w", err)
	}
	return results, nil
}

// CreateRCA performs root-cause analysis via an AI provider and stores the result.
// If no provider is found, a placeholder record is created.
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

	// Call the AI provider for root-cause analysis.
	rcaResult, err := provider.AnalyzeRootCause(ctx, ai.RCARequest{
		IncidentID: req.IncidentID,
		TenantID:   tenantID,
		Context:    req.Context,
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
