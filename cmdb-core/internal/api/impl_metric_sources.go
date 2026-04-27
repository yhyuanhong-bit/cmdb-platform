package api

import (
	"errors"
	"strings"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/domain/metricsource"
	"github.com/cmdb-platform/cmdb-core/internal/platform/response"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// ListMetricSources — GET /metrics/sources
func (s *APIServer) ListMetricSources(c *gin.Context, params ListMetricSourcesParams) {
	tenantID := tenantIDFromContext(c)
	var statusPtr, kindPtr *string
	if params.Status != nil {
		v := string(*params.Status)
		statusPtr = &v
	}
	if params.Kind != nil {
		v := *params.Kind
		kindPtr = &v
	}
	rows, err := s.metricSourceSvc.List(c.Request.Context(), tenantID, statusPtr, kindPtr)
	if err != nil {
		response.InternalError(c, "failed to list metric sources")
		return
	}
	out := make([]MetricSource, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIMetricSource(r))
	}
	response.OK(c, out)
}

// GetMetricSource — GET /metrics/sources/{id}
func (s *APIServer) GetMetricSource(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	row, err := s.metricSourceSvc.Get(c.Request.Context(), tenantID, uuid.UUID(id))
	if err != nil {
		if errors.Is(err, metricsource.ErrNotFound) {
			response.NotFound(c, "metric source not found")
			return
		}
		response.InternalError(c, "failed to load metric source")
		return
	}
	response.OK(c, toAPIMetricSource(*row))
}

// CreateMetricSource — POST /metrics/sources
func (s *APIServer) CreateMetricSource(c *gin.Context) {
	var body CreateMetricSourceJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tenantID := tenantIDFromContext(c)
	p := metricsource.CreateParams{
		TenantID:                tenantID,
		Name:                    body.Name,
		Kind:                    string(body.Kind),
		ExpectedIntervalSeconds: int32(body.ExpectedIntervalSeconds),
	}
	if body.Status != nil {
		p.Status = string(*body.Status)
	}
	if body.Notes != nil {
		p.Notes = *body.Notes
	}
	row, err := s.metricSourceSvc.Create(c.Request.Context(), p)
	if err != nil {
		s.writeMetricSourceErr(c, err)
		return
	}
	s.recordAudit(c, "metric_source.created", "metrics", "source", row.ID, map[string]any{
		"name": row.Name, "kind": row.Kind,
	})
	response.Created(c, toAPIMetricSource(*row))
}

// UpdateMetricSource — PUT /metrics/sources/{id}
func (s *APIServer) UpdateMetricSource(c *gin.Context, id IdPath) {
	var body UpdateMetricSourceJSONRequestBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	tenantID := tenantIDFromContext(c)
	p := metricsource.UpdateParams{TenantID: tenantID, ID: uuid.UUID(id)}
	if body.Name != nil {
		p.Name = body.Name
	}
	if body.Kind != nil {
		k := string(*body.Kind)
		p.Kind = &k
	}
	if body.ExpectedIntervalSeconds != nil {
		v := int32(*body.ExpectedIntervalSeconds)
		p.ExpectedIntervalSeconds = &v
	}
	if body.Status != nil {
		v := string(*body.Status)
		p.Status = &v
	}
	if body.Notes != nil {
		p.Notes = body.Notes
	}
	row, err := s.metricSourceSvc.Update(c.Request.Context(), p)
	if err != nil {
		s.writeMetricSourceErr(c, err)
		return
	}
	s.recordAudit(c, "metric_source.updated", "metrics", "source", row.ID, nil)
	response.OK(c, toAPIMetricSource(*row))
}

// DeleteMetricSource — DELETE /metrics/sources/{id}
func (s *APIServer) DeleteMetricSource(c *gin.Context, id IdPath) {
	tenantID := tenantIDFromContext(c)
	if err := s.metricSourceSvc.Delete(c.Request.Context(), tenantID, uuid.UUID(id)); err != nil {
		response.InternalError(c, "failed to delete metric source")
		return
	}
	s.recordAudit(c, "metric_source.deleted", "metrics", "source", uuid.UUID(id), nil)
	c.Status(204)
}

// HeartbeatMetricSource — POST /metrics/sources/{id}/heartbeat
func (s *APIServer) HeartbeatMetricSource(c *gin.Context, id IdPath) {
	var body struct {
		SampleDelta *int64 `json:"sample_delta"`
	}
	_ = c.ShouldBindJSON(&body)
	tenantID := tenantIDFromContext(c)
	delta := int64(0)
	if body.SampleDelta != nil {
		delta = *body.SampleDelta
		if delta < 0 {
			response.BadRequest(c, "sample_delta must be >= 0")
			return
		}
	}
	row, err := s.metricSourceSvc.Heartbeat(c.Request.Context(), tenantID, uuid.UUID(id), delta)
	if err != nil {
		if errors.Is(err, metricsource.ErrNotFound) {
			response.NotFound(c, "metric source not found")
			return
		}
		response.InternalError(c, "failed to heartbeat")
		return
	}
	response.OK(c, toAPIMetricSource(*row))
}

// ListStaleMetricSources — GET /metrics/sources/freshness
func (s *APIServer) ListStaleMetricSources(c *gin.Context) {
	tenantID := tenantIDFromContext(c)
	rows, err := s.metricSourceSvc.ListStale(c.Request.Context(), tenantID)
	if err != nil {
		response.InternalError(c, "failed to compute freshness")
		return
	}
	out := make([]MetricSourceFreshness, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIMetricSourceFreshness(r))
	}
	response.OK(c, out)
}

// writeMetricSourceErr maps domain errors to HTTP status codes.
func (s *APIServer) writeMetricSourceErr(c *gin.Context, err error) {
	switch {
	case errors.Is(err, metricsource.ErrNotFound):
		response.NotFound(c, "metric source not found")
	case errors.Is(err, metricsource.ErrDuplicateName):
		response.Err(c, 409, "METRIC_SOURCE_DUPLICATE",
			"a metric source with this name already exists in the tenant")
	case errors.Is(err, metricsource.ErrValidation):
		response.BadRequest(c, strings.TrimPrefix(err.Error(), "metric source: validation failed: "))
	default:
		response.InternalError(c, "failed to apply metric source change")
	}
}

// ---------------------------------------------------------------------------
// Converters.
// ---------------------------------------------------------------------------

func toAPIMetricSource(db dbgen.MetricSource) MetricSource {
	out := MetricSource{
		Id:                      db.ID,
		Name:                    db.Name,
		Kind:                    MetricSourceKind(db.Kind),
		ExpectedIntervalSeconds: int(db.ExpectedIntervalSeconds),
		Status:                  MetricSourceStatus(db.Status),
		LastSampleCount:         db.LastSampleCount,
		CreatedAt:               db.CreatedAt,
	}
	if db.LastHeartbeatAt.Valid {
		t := db.LastHeartbeatAt.Time
		out.LastHeartbeatAt = &t
	}
	if db.Notes.Valid {
		s := db.Notes.String
		out.Notes = &s
	}
	if !db.UpdatedAt.IsZero() {
		t := db.UpdatedAt
		out.UpdatedAt = &t
	}
	return out
}

func toAPIMetricSourceFreshness(db dbgen.ListStaleMetricSourcesRow) MetricSourceFreshness {
	out := MetricSourceFreshness{
		Id:                      db.ID,
		Name:                    db.Name,
		Kind:                    db.Kind,
		ExpectedIntervalSeconds: int(db.ExpectedIntervalSeconds),
		Status:                  db.Status,
	}
	if db.LastHeartbeatAt.Valid {
		t := db.LastHeartbeatAt.Time
		out.LastHeartbeatAt = &t
	}
	// SecondsSinceHeartbeat lands as interface{} because sqlc can't
	// infer the type through COALESCE. The query returns int4 (or -1
	// for never-heartbeated rows); convert via type assertion. -1 is
	// the sentinel from the SQL — map it back to nil in the API
	// shape so a never-heartbeated source surfaces as null rather
	// than -1.
	if v, ok := db.SecondsSinceHeartbeat.(int32); ok && v >= 0 {
		secs := int(v)
		out.SecondsSinceHeartbeat = &secs
	} else if v, ok := db.SecondsSinceHeartbeat.(int64); ok && v >= 0 {
		secs := int(v)
		out.SecondsSinceHeartbeat = &secs
	}
	return out
}
