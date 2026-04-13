package ai

import (
	"context"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
)

// QueriesAdapter wraps dbgen.Queries to satisfy the ModelLister interface.
type QueriesAdapter struct {
	Q *dbgen.Queries
}

// ListEnabledModels implements ModelLister by delegating to dbgen and converting
// PredictionModel rows into ModelRecord values.
func (a *QueriesAdapter) ListEnabledModels(ctx context.Context) ([]ModelRecord, error) {
	rows, err := a.Q.ListEnabledModels(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ModelRecord, len(rows))
	for i, r := range rows {
		out[i] = ModelRecord{
			ID:       r.ID,
			Name:     r.Name,
			Provider: r.Provider,
			Endpoint: "", // prediction_models table has no endpoint column yet
			Config:   r.Config,
		}
	}
	return out, nil
}
