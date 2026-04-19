// Package monitoring — Alert Rule Evaluator.
//
// Phase 2.1 of the remediation roadmap: alert_rules had schema + UI but no
// evaluator, so the table silently never fired. This file is the in-process
// evaluator that scans enabled rules every interval, aggregates TimescaleDB
// metrics, and emits alert_events rows on threshold breach.
//
// Design notes (see docs/reports/audit-2026-04-19/REMEDIATION-ROADMAP.md §2.1):
//
//   - Scheme A (internal evaluator) — NOT Alertmanager webhook. The service
//     runs as a goroutine off main's server context; SIGTERM cancels it.
//   - Tenant safety — every metric aggregation query is scoped by the rule's
//     own tenant_id. A bug here = cross-tenant leak, so the query is a
//     single raw SQL statement with explicit tenant_id binding and no code
//     path bypasses it.
//   - Flapping guard — consecutive_triggers counter lives in-memory keyed by
//     (rule_id, asset_id). We only emit after N consecutive breaches. This
//     resets on process restart, which is an acceptable trade-off — a flap
//     that happens to straddle a restart loses one cycle of accumulation
//     and pays one extra tick before firing. Documented in the commit body.
//   - Dedup — alert_events.dedup_key = "<rule>:<asset>:<UTC-hour>" enforced
//     by a unique index. The INSERT is ON CONFLICT DO UPDATE, so repeated
//     ticks inside the same hour refresh trigger_value + updated_at rather
//     than spamming new rows. Fired_at stays pinned to the first firing.
//   - Panic safety — every tick body is wrapped in a deferred recover so a
//     single malformed rule cannot kill the background loop.
package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

// DefaultEvaluatorInterval is the default tick cadence for the evaluator.
// 60s was chosen to match the roadmap spec and the workflows.SLAChecker.
const DefaultEvaluatorInterval = 60 * time.Second

// Supported operators and aggregations. Centralised so unknown values are
// caught during parse instead of at SQL time.
var (
	validOperators = map[string]struct{}{
		">": {}, "<": {}, ">=": {}, "<=": {}, "==": {}, "!=": {},
	}
	validAggregations = map[string]struct{}{
		"avg": {}, "max": {}, "min": {}, "p95": {}, "p99": {},
	}
)

// RuleCondition is the parsed shape of alert_rules.condition JSONB.
//
// Spec (see REMEDIATION-ROADMAP.md §2.1 task 1):
//
//	{
//	  "operator": ">" | "<" | ">=" | "<=" | "==" | "!=",
//	  "threshold": 85,
//	  "window_seconds": 300,
//	  "aggregation": "avg" | "max" | "min" | "p95" | "p99",
//	  "consecutive_triggers": 2
//	}
//
// Malformed condition JSON is logged and skipped for that rule. We do NOT
// fail the whole tick, because one bad row should not block every other
// tenant's alerts.
type RuleCondition struct {
	Operator            string  `json:"operator"`
	Threshold           float64 `json:"threshold"`
	WindowSeconds       int     `json:"window_seconds"`
	Aggregation         string  `json:"aggregation"`
	ConsecutiveTriggers int     `json:"consecutive_triggers"`
}

func (c RuleCondition) validate() error {
	if _, ok := validOperators[c.Operator]; !ok {
		return fmt.Errorf("invalid operator %q", c.Operator)
	}
	if _, ok := validAggregations[c.Aggregation]; !ok {
		return fmt.Errorf("invalid aggregation %q", c.Aggregation)
	}
	if c.WindowSeconds <= 0 {
		return fmt.Errorf("window_seconds must be > 0, got %d", c.WindowSeconds)
	}
	if c.ConsecutiveTriggers < 1 {
		// 0 or negative is nonsense. Default-fill to 1 at parse time so
		// a rule without the field still behaves sensibly.
		return fmt.Errorf("consecutive_triggers must be >= 1, got %d", c.ConsecutiveTriggers)
	}
	if math.IsNaN(c.Threshold) || math.IsInf(c.Threshold, 0) {
		return fmt.Errorf("threshold is not a finite number")
	}
	return nil
}

// aggregateQuerier is the narrow interface the evaluator uses to pull data
// out of the metrics hypertable and write alert_events rows. Matches the
// read+write subset of pgxpool.Pool we need so production code passes the
// real pool, while tests pass a fake. See evaluator_test.go.
type aggregateQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ruleLister is the narrow interface the evaluator uses to read enabled
// rules. Satisfied by dbgen.Queries in production and a fake in tests.
type ruleLister interface {
	ListEnabledAlertRules(ctx context.Context) ([]dbgen.AlertRule, error)
}

// stateKey uniquely identifies (rule_id, asset_id) for the in-memory
// consecutive-trigger counter. We use a string key because asset_id can
// be the zero UUID when a metric arrived with a NULL asset_id (rare but
// permitted by the schema).
type stateKey struct {
	RuleID  uuid.UUID
	AssetID uuid.UUID
}

// ruleState holds per-(rule,asset) flap-guard + firing state. Completely
// in-memory — documented trade-off in the package header.
type ruleState struct {
	consecutive int  // consecutive breach ticks since the last non-breach
	firing      bool // true once we've emitted a firing event for this pair
}

// Evaluator is the in-process alert rule evaluator.
type Evaluator struct {
	queries  ruleLister
	pool     aggregateQuerier
	bus      eventbus.Bus
	interval time.Duration
	logger   *zap.Logger

	// now is overridable for deterministic tests. Production passes time.Now.
	now func() time.Time

	mu    sync.Mutex
	state map[stateKey]*ruleState
}

// Option is a functional option for NewEvaluator.
type Option func(*Evaluator)

// WithInterval overrides the default 60s tick cadence. Used by tests so the
// loop can run in milliseconds.
func WithInterval(d time.Duration) Option {
	return func(e *Evaluator) { e.interval = d }
}

// WithLogger injects a zap logger. Defaults to zap.L() so wiring in main.go
// can stay a one-liner.
func WithLogger(l *zap.Logger) Option {
	return func(e *Evaluator) { e.logger = l }
}

// WithClock injects a clock for deterministic tests. Production always uses
// time.Now.
func WithClock(now func() time.Time) Option {
	return func(e *Evaluator) { e.now = now }
}

// NewEvaluator constructs an Evaluator. The pool MUST have the alert_events
// dedup_key unique index (migration 000046) applied — the evaluator's insert
// depends on it.
func NewEvaluator(queries ruleLister, pool aggregateQuerier, bus eventbus.Bus, opts ...Option) *Evaluator {
	e := &Evaluator{
		queries:  queries,
		pool:     pool,
		bus:      bus,
		interval: DefaultEvaluatorInterval,
		logger:   zap.L(),
		now:      time.Now,
		state:    make(map[stateKey]*ruleState),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Start runs the evaluator loop until ctx is cancelled. Blocking call —
// typical usage from main is `go evaluator.Start(ctx)`. On ctx.Done the
// loop exits within one interval.
func (e *Evaluator) Start(ctx context.Context) {
	e.logger.Info("alert evaluator started", zap.Duration("interval", e.interval))

	// Run one tick immediately on startup so rules that were breached during
	// a restart fire on the next reconciliation tick rather than after a
	// full interval.
	e.runTick(ctx)

	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			e.logger.Info("alert evaluator stopped")
			return
		case <-ticker.C:
			e.runTick(ctx)
		}
	}
}

// runTick is one full scan of enabled rules. A panic inside this function
// must NOT kill the goroutine — deferred recover + telemetry counter bump.
func (e *Evaluator) runTick(ctx context.Context) {
	outcome := "ok"
	defer func() {
		if r := recover(); r != nil {
			outcome = "error"
			e.logger.Error("alert evaluator panic recovered",
				zap.Any("panic", r))
		}
		telemetry.MonitoringEvaluatorRunsTotal.WithLabelValues(outcome).Inc()
	}()

	tracer := otel.Tracer("cmdb-core/monitoring")
	tickCtx, span := tracer.Start(ctx, "alert_evaluator.tick")
	defer span.End()

	rules, err := e.queries.ListEnabledAlertRules(tickCtx)
	if err != nil {
		outcome = "error"
		e.logger.Error("alert evaluator: list rules failed", zap.Error(err))
		return
	}
	span.SetAttributes(attribute.Int("rules.count", len(rules)))

	for _, rule := range rules {
		e.evaluateRule(tickCtx, rule)
	}
}

// evaluateRule runs one rule against its tenant's metrics. Any error in this
// method is logged and swallowed — one bad rule must not block the loop.
func (e *Evaluator) evaluateRule(ctx context.Context, rule dbgen.AlertRule) {
	start := e.now()
	cond, err := parseCondition(rule.Condition)
	if err != nil {
		e.logger.Warn("alert evaluator: skipping rule with malformed condition",
			zap.String("rule_id", rule.ID.String()),
			zap.String("tenant_id", rule.TenantID.String()),
			zap.Error(err))
		// Bucket malformed rules as errors so the ok-counter reflects healthy
		// evaluation runs only.
		telemetry.MonitoringEvaluatorRunsTotal.WithLabelValues("error").Inc()
		return
	}

	defer func() {
		telemetry.MonitoringRuleEvaluationDuration.
			WithLabelValues(cond.Operator, cond.Aggregation).
			Observe(e.now().Sub(start).Seconds())
	}()

	windowStart := e.now().Add(-time.Duration(cond.WindowSeconds) * time.Second)
	samples, err := e.aggregate(ctx, rule.TenantID, rule.MetricName, cond.Aggregation, windowStart)
	if err != nil {
		e.logger.Warn("alert evaluator: aggregation failed",
			zap.String("rule_id", rule.ID.String()),
			zap.String("tenant_id", rule.TenantID.String()),
			zap.Error(err))
		return
	}

	for _, sample := range samples {
		e.judgeSample(ctx, rule, cond, sample)
	}
}

// aggregatedSample is one row returned by the AggregateMetricPerAsset query.
// Value is nullable — when the aggregation is unknown OR no samples fall
// inside the window the DB returns NULL and we skip the sample.
type aggregatedSample struct {
	AssetID uuid.UUID // uuid.Nil when the metric row had no asset_id
	Value   float64
	HasData bool
}

// aggregate runs the tenant-scoped aggregation query. TENANT SAFETY: the
// tenant_id parameter comes straight from rule.TenantID and is bound as
// the first positional arg. Never splice tenant_id into the SQL string.
func (e *Evaluator) aggregate(ctx context.Context, tenantID uuid.UUID, metricName, aggregation string, windowStart time.Time) ([]aggregatedSample, error) {
	const sql = `
SELECT
    asset_id,
    CASE $3::text
        WHEN 'avg' THEN avg(value)
        WHEN 'max' THEN max(value)
        WHEN 'min' THEN min(value)
        WHEN 'p95' THEN percentile_cont(0.95) WITHIN GROUP (ORDER BY value)
        WHEN 'p99' THEN percentile_cont(0.99) WITHIN GROUP (ORDER BY value)
        ELSE NULL
    END AS aggregated_value
FROM metrics
WHERE tenant_id = $1
  AND name = $2
  AND time > $4
GROUP BY asset_id`

	rows, err := e.pool.Query(ctx, sql, tenantID, metricName, aggregation, windowStart)
	if err != nil {
		return nil, fmt.Errorf("query metrics: %w", err)
	}
	defer rows.Close()

	var out []aggregatedSample
	for rows.Next() {
		var (
			assetID *uuid.UUID
			value   *float64
		)
		if err := rows.Scan(&assetID, &value); err != nil {
			return nil, fmt.Errorf("scan metric row: %w", err)
		}
		sample := aggregatedSample{}
		if assetID != nil {
			sample.AssetID = *assetID
		}
		if value != nil {
			sample.Value = *value
			sample.HasData = true
		}
		out = append(out, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate metric rows: %w", err)
	}
	return out, nil
}

// judgeSample compares one (asset, value) against the rule's threshold,
// advances/resets the consecutive-trigger counter, and emits fired/resolved
// events at the right transition points.
func (e *Evaluator) judgeSample(ctx context.Context, rule dbgen.AlertRule, cond RuleCondition, sample aggregatedSample) {
	if !sample.HasData {
		return
	}

	key := stateKey{RuleID: rule.ID, AssetID: sample.AssetID}
	breached := compareValue(sample.Value, cond.Operator, cond.Threshold)

	e.mu.Lock()
	st, ok := e.state[key]
	if !ok {
		st = &ruleState{}
		e.state[key] = st
	}

	if breached {
		st.consecutive++
		shouldFire := st.consecutive >= cond.ConsecutiveTriggers && !st.firing
		st.firing = st.consecutive >= cond.ConsecutiveTriggers || st.firing
		e.mu.Unlock()

		if shouldFire {
			e.emit(ctx, rule, cond, sample, "firing")
		} else if st.firing {
			// Already firing this hour — still refresh the dedup row so
			// trigger_value reflects the latest tick.
			e.emit(ctx, rule, cond, sample, "firing")
		}
		return
	}

	// Not breached. Reset the consecutive counter. If we were firing, emit a
	// resolved event exactly once.
	wasFiring := st.firing
	st.consecutive = 0
	st.firing = false
	e.mu.Unlock()

	if wasFiring {
		e.emit(ctx, rule, cond, sample, "resolved")
	}
}

// compareValue evaluates `value <op> threshold`. Unknown operators return
// false so an unparseable rule is a silent no-op rather than a firing storm.
func compareValue(value float64, operator string, threshold float64) bool {
	switch operator {
	case ">":
		return value > threshold
	case "<":
		return value < threshold
	case ">=":
		return value >= threshold
	case "<=":
		return value <= threshold
	case "==":
		return value == threshold
	case "!=":
		return value != threshold
	default:
		return false
	}
}

// emit writes the alert_events row (upsert by dedup_key) and publishes an
// event on the bus. Bus is best-effort: if it's nil or errors we still keep
// the DB row.
func (e *Evaluator) emit(ctx context.Context, rule dbgen.AlertRule, cond RuleCondition, sample aggregatedSample, status string) {
	dedupKey := buildDedupKey(rule.ID, sample.AssetID, e.now())
	message := buildMessage(rule, cond, sample, status)

	var assetIDArg interface{}
	if sample.AssetID != uuid.Nil {
		assetIDArg = sample.AssetID
	}

	const sql = `
INSERT INTO alert_events (
    tenant_id, rule_id, asset_id, status, severity, message, trigger_value, dedup_key, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, now())
ON CONFLICT (dedup_key) DO UPDATE SET
    status        = EXCLUDED.status,
    trigger_value = EXCLUDED.trigger_value,
    message       = EXCLUDED.message,
    updated_at    = now(),
    resolved_at   = CASE WHEN EXCLUDED.status = 'resolved' THEN now() ELSE alert_events.resolved_at END
RETURNING id, (xmax = 0) AS inserted`

	var (
		alertID  uuid.UUID
		inserted bool
	)
	err := e.pool.QueryRow(ctx, sql,
		rule.TenantID,
		rule.ID,
		assetIDArg,
		status,
		rule.Severity,
		message,
		sample.Value,
		dedupKey,
	).Scan(&alertID, &inserted)
	if err != nil {
		e.logger.Error("alert evaluator: emit failed",
			zap.String("rule_id", rule.ID.String()),
			zap.String("tenant_id", rule.TenantID.String()),
			zap.String("dedup_key", dedupKey),
			zap.String("status", status),
			zap.Error(err))
		return
	}

	action := "updated"
	if inserted {
		action = "inserted"
	}
	telemetry.MonitoringAlertsEmittedTotal.
		WithLabelValues(status, rule.Severity, action).
		Inc()

	if e.bus == nil {
		return
	}
	subject := eventbus.SubjectAlertFired
	if status == "resolved" {
		subject = eventbus.SubjectAlertResolved
	}
	payload, _ := json.Marshal(map[string]any{
		"alert_id":  alertID.String(),
		"rule_id":   rule.ID.String(),
		"asset_id":  sample.AssetID.String(),
		"severity":  rule.Severity,
		"message":   message,
		"value":     sample.Value,
		"threshold": cond.Threshold,
	})
	if err := e.bus.Publish(ctx, eventbus.Event{
		Subject:  subject,
		TenantID: rule.TenantID.String(),
		Payload:  payload,
	}); err != nil {
		e.logger.Warn("alert evaluator: bus publish failed",
			zap.String("subject", subject),
			zap.Error(err))
	}
}

// buildDedupKey returns "<rule_id>:<asset_id>:<UTC-hour>". Hour granularity
// means repeated firings within the same hour collapse into one row. The
// format string matches the backfill UPDATE in migration 000046 exactly.
func buildDedupKey(ruleID, assetID uuid.UUID, now time.Time) string {
	assetPart := "none"
	if assetID != uuid.Nil {
		assetPart = assetID.String()
	}
	return fmt.Sprintf("%s:%s:%s",
		ruleID.String(),
		assetPart,
		now.UTC().Format("2006-01-02T15"))
}

// buildMessage produces a terse operator-readable message. Kept deterministic
// so tests can assert against it.
func buildMessage(rule dbgen.AlertRule, cond RuleCondition, sample aggregatedSample, status string) string {
	if status == "resolved" {
		return fmt.Sprintf("rule %q resolved: %s(%s) back within threshold %s %g (observed %g)",
			rule.Name, cond.Aggregation, rule.MetricName, cond.Operator, cond.Threshold, sample.Value)
	}
	return fmt.Sprintf("rule %q firing: %s(%s) = %g %s %g",
		rule.Name, cond.Aggregation, rule.MetricName, sample.Value, cond.Operator, cond.Threshold)
}

// parseCondition unmarshals + validates the rule's condition JSONB.
func parseCondition(raw json.RawMessage) (RuleCondition, error) {
	var cond RuleCondition
	if len(raw) == 0 {
		return cond, errors.New("condition is empty")
	}
	if err := json.Unmarshal(raw, &cond); err != nil {
		return cond, fmt.Errorf("unmarshal condition: %w", err)
	}
	// Default consecutive_triggers to 1 if the field was omitted entirely.
	// Zero-valued field means "fire on the first breach" which is the
	// minimum sensible behaviour.
	if cond.ConsecutiveTriggers == 0 {
		cond.ConsecutiveTriggers = 1
	}
	if err := cond.validate(); err != nil {
		return cond, err
	}
	return cond, nil
}

// poolAdapter wraps a *pgxpool.Pool to satisfy the aggregateQuerier interface
// that tests mock. The method signatures match pgxpool.Pool exactly, so the
// adapter is a zero-cost wrapper at runtime.
type poolAdapter struct {
	pool *pgxpool.Pool
}

// NewPoolAdapter wraps a *pgxpool.Pool so it satisfies aggregateQuerier.
// Exported so main.go can pass the real pool without importing the interface
// directly.
func NewPoolAdapter(pool *pgxpool.Pool) aggregateQuerier {
	return &poolAdapter{pool: pool}
}

func (p *poolAdapter) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}

func (p *poolAdapter) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}
