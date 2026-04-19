package monitoring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
)

// ---------- Fakes ----------

// fakeRuleLister returns a canned list of rules. Mirrors the ruleLister
// interface in the evaluator.
type fakeRuleLister struct {
	rules []dbgen.AlertRule
	err   error
}

func (f *fakeRuleLister) ListEnabledAlertRules(ctx context.Context) ([]dbgen.AlertRule, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.rules, nil
}

// fakePool is a minimal aggregateQuerier implementation. It does two things:
//
//   - Query: intercepts the metric aggregation SQL and returns canned rows
//     from `metricSamples` for the (tenant, name) pair. If the query is
//     called with a tenant_id not in the allow-list it returns an error —
//     this lets the tenant-isolation test verify the evaluator never asks
//     for cross-tenant metrics.
//   - QueryRow: intercepts the alert_events upsert and records the attempt.
//     The returned fakeRow yields a generated id + `inserted` bool based
//     on whether the dedup_key has been seen before.
type fakePool struct {
	mu sync.Mutex

	// metricSamples is keyed by (tenant_id, metric_name); the slice of
	// (assetID, value) is returned from Query.
	metricSamples map[string][]fakeSample

	// allowedTenants restricts which tenant_ids Query is allowed to be
	// called with. Empty means "allow all". Used by the tenant-isolation
	// test.
	allowedTenants map[uuid.UUID]struct{}

	// Emissions recorded from QueryRow. One entry per upsert call.
	emits []fakeEmit

	// dedupSeen maps dedup_key to the UUID we've already issued so repeat
	// calls yield `inserted = false`.
	dedupSeen map[string]uuid.UUID

	// Assertion knob — if set, a Query that fails the tenant check returns
	// this error instead of panicking.
	tenantMismatchErr error
}

type fakeSample struct {
	AssetID uuid.UUID
	Value   float64
}

type fakeEmit struct {
	TenantID     uuid.UUID
	RuleID       uuid.UUID
	AssetID      *uuid.UUID
	Status       string
	Severity     string
	Message      string
	TriggerValue float64
	DedupKey     string
	Inserted     bool
}

func newFakePool() *fakePool {
	return &fakePool{
		metricSamples:  make(map[string][]fakeSample),
		allowedTenants: make(map[uuid.UUID]struct{}),
		dedupSeen:      make(map[string]uuid.UUID),
	}
}

// setSamples configures the rows returned when the evaluator asks for
// `metric_name` under `tenant_id`.
func (p *fakePool) setSamples(tenantID uuid.UUID, metricName string, samples ...fakeSample) {
	key := tenantID.String() + "|" + metricName
	p.metricSamples[key] = samples
}

// clearSamples wipes all configured samples so a later tick sees an empty
// aggregation result (simulating threshold clearing).
func (p *fakePool) clearSamples() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.metricSamples = make(map[string][]fakeSample)
}

// allowTenant restricts the fake to only respond for the given tenant. Any
// Query for a different tenant returns tenantMismatchErr. Calling with no
// ids disables the check.
func (p *fakePool) allowTenant(ids ...uuid.UUID) {
	for _, id := range ids {
		p.allowedTenants[id] = struct{}{}
	}
}

func (p *fakePool) getEmits() []fakeEmit {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]fakeEmit, len(p.emits))
	copy(out, p.emits)
	return out
}

func (p *fakePool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	// The evaluator uses exactly one Query call (aggregation). Positional
	// args: tenant_id, metric_name, aggregation, window_start.
	if len(args) < 4 {
		return nil, fmt.Errorf("fake pool: aggregation query expects 4 args, got %d", len(args))
	}
	tenantID, ok := args[0].(uuid.UUID)
	if !ok {
		return nil, fmt.Errorf("fake pool: arg 0 is not uuid.UUID: %T", args[0])
	}
	metricName, ok := args[1].(string)
	if !ok {
		return nil, fmt.Errorf("fake pool: arg 1 is not string: %T", args[1])
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.allowedTenants) > 0 {
		if _, ok := p.allowedTenants[tenantID]; !ok {
			err := p.tenantMismatchErr
			if err == nil {
				err = fmt.Errorf("fake pool: cross-tenant query attempted for %s", tenantID)
			}
			return nil, err
		}
	}

	key := tenantID.String() + "|" + metricName
	samples := p.metricSamples[key]
	return &fakeRows{samples: samples}, nil
}

func (p *fakePool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	// The evaluator uses QueryRow only for the alert_events upsert. Args
	// (in order): tenant_id, rule_id, asset_id, status, severity, message,
	// trigger_value, dedup_key.
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(args) < 8 {
		return &fakeRow{err: fmt.Errorf("fake pool: emit query expects 8 args, got %d", len(args))}
	}

	emit := fakeEmit{}
	if v, ok := args[0].(uuid.UUID); ok {
		emit.TenantID = v
	}
	if v, ok := args[1].(uuid.UUID); ok {
		emit.RuleID = v
	}
	if args[2] != nil {
		if v, ok := args[2].(uuid.UUID); ok {
			a := v
			emit.AssetID = &a
		}
	}
	if v, ok := args[3].(string); ok {
		emit.Status = v
	}
	if v, ok := args[4].(string); ok {
		emit.Severity = v
	}
	if v, ok := args[5].(string); ok {
		emit.Message = v
	}
	if v, ok := args[6].(float64); ok {
		emit.TriggerValue = v
	}
	if v, ok := args[7].(string); ok {
		emit.DedupKey = v
	}

	existingID, seen := p.dedupSeen[emit.DedupKey]
	var alertID uuid.UUID
	if seen {
		alertID = existingID
	} else {
		alertID = uuid.New()
		p.dedupSeen[emit.DedupKey] = alertID
	}
	emit.Inserted = !seen

	p.emits = append(p.emits, emit)
	return &fakeRow{id: alertID, inserted: !seen}
}

// fakeRow implements pgx.Row for the upsert RETURNING id, inserted path.
type fakeRow struct {
	id       uuid.UUID
	inserted bool
	err      error
}

func (r *fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != 2 {
		return fmt.Errorf("fake row: expected 2 dest fields, got %d", len(dest))
	}
	if idPtr, ok := dest[0].(*uuid.UUID); ok {
		*idPtr = r.id
	} else {
		return fmt.Errorf("fake row: dest[0] not *uuid.UUID: %T", dest[0])
	}
	if boolPtr, ok := dest[1].(*bool); ok {
		*boolPtr = r.inserted
	} else {
		return fmt.Errorf("fake row: dest[1] not *bool: %T", dest[1])
	}
	return nil
}

// fakeRows implements pgx.Rows for the aggregation query. Only Scan + Next +
// Close + Err are used by the evaluator; the other methods panic so a future
// caller that relies on them is caught in a test rather than silently
// returning zero values.
type fakeRows struct {
	samples []fakeSample
	idx     int
	closed  bool
}

func (r *fakeRows) Close()                                     { r.closed = true }
func (r *fakeRows) Err() error                                 { return nil }
func (r *fakeRows) CommandTag() pgconn.CommandTag              { return pgconn.CommandTag{} }
func (r *fakeRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (r *fakeRows) Values() ([]any, error)                     { panic("unused") }
func (r *fakeRows) RawValues() [][]byte                        { panic("unused") }
func (r *fakeRows) Conn() *pgx.Conn                            { return nil }

func (r *fakeRows) Next() bool {
	if r.idx >= len(r.samples) {
		return false
	}
	return true
}

func (r *fakeRows) Scan(dest ...any) error {
	s := r.samples[r.idx]
	r.idx++
	if len(dest) != 2 {
		return fmt.Errorf("fake rows: expected 2 dest fields, got %d", len(dest))
	}
	// dest[0] **uuid.UUID — the evaluator scans into *uuid.UUID but because
	// the column is nullable we scan through a **uuid.UUID pointer. Mirror
	// that.
	if ptr, ok := dest[0].(**uuid.UUID); ok {
		if s.AssetID == uuid.Nil {
			*ptr = nil
		} else {
			id := s.AssetID
			*ptr = &id
		}
	} else {
		return fmt.Errorf("fake rows: dest[0] not **uuid.UUID: %T", dest[0])
	}
	if ptr, ok := dest[1].(**float64); ok {
		v := s.Value
		*ptr = &v
	} else {
		return fmt.Errorf("fake rows: dest[1] not **float64: %T", dest[1])
	}
	return nil
}

// fakeBus is a no-op bus that records published events.
type fakeBus struct {
	mu     sync.Mutex
	events []eventbus.Event
	err    error
}

func (b *fakeBus) Publish(ctx context.Context, event eventbus.Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.err != nil {
		return b.err
	}
	b.events = append(b.events, event)
	return nil
}

func (b *fakeBus) Subscribe(subject string, handler eventbus.Handler) error { return nil }
func (b *fakeBus) Close() error                                             { return nil }

func (b *fakeBus) recorded() []eventbus.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]eventbus.Event, len(b.events))
	copy(out, b.events)
	return out
}

// ---------- Helpers ----------

func buildRule(tenantID uuid.UUID, metric string, cond RuleCondition, severity string) dbgen.AlertRule {
	raw, _ := json.Marshal(cond)
	return dbgen.AlertRule{
		ID:         uuid.New(),
		TenantID:   tenantID,
		Name:       "test-rule",
		MetricName: metric,
		Condition:  raw,
		Severity:   severity,
		Enabled:    true,
		CreatedAt:  time.Now(),
	}
}

// ---------- Tests ----------

func TestParseCondition(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{"valid", `{"operator":">","threshold":85,"window_seconds":300,"aggregation":"avg","consecutive_triggers":2}`, false},
		{"missing op", `{"threshold":85,"window_seconds":300,"aggregation":"avg","consecutive_triggers":1}`, true},
		{"unknown op", `{"operator":"~","threshold":1,"window_seconds":60,"aggregation":"avg","consecutive_triggers":1}`, true},
		{"bad agg", `{"operator":">","threshold":1,"window_seconds":60,"aggregation":"mode","consecutive_triggers":1}`, true},
		{"zero window", `{"operator":">","threshold":1,"window_seconds":0,"aggregation":"avg","consecutive_triggers":1}`, true},
		{"empty", ``, true},
		{"garbage", `{not json}`, true},
		{"default consecutive", `{"operator":">","threshold":1,"window_seconds":60,"aggregation":"avg"}`, false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := parseCondition(json.RawMessage(tc.raw))
			if tc.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCompareValue(t *testing.T) {
	tests := []struct {
		op   string
		a, b float64
		want bool
	}{
		{">", 10, 5, true},
		{">", 5, 10, false},
		{">=", 5, 5, true},
		{"<", 3, 5, true},
		{"<=", 5, 5, true},
		{"==", 5, 5, true},
		{"!=", 5, 6, true},
		{"??", 1, 1, false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%g%s%g", tc.a, tc.op, tc.b), func(t *testing.T) {
			t.Parallel()
			if got := compareValue(tc.a, tc.op, tc.b); got != tc.want {
				t.Fatalf("compareValue(%g,%q,%g) = %v, want %v", tc.a, tc.op, tc.b, got, tc.want)
			}
		})
	}
}

func TestBuildDedupKey(t *testing.T) {
	ruleID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	assetID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	fixed := time.Date(2026, 4, 19, 14, 37, 11, 0, time.UTC)

	got := buildDedupKey(ruleID, assetID, fixed)
	want := "11111111-1111-1111-1111-111111111111:22222222-2222-2222-2222-222222222222:2026-04-19T14"
	if got != want {
		t.Fatalf("buildDedupKey: got %q, want %q", got, want)
	}

	// Different minute but same hour collapses.
	sameHour := fixed.Add(20 * time.Minute)
	if buildDedupKey(ruleID, assetID, sameHour) != want {
		t.Fatalf("dedup key changed within the same hour — defeats hour-granularity dedup")
	}

	// Hour boundary produces a different key.
	nextHour := fixed.Add(time.Hour)
	if buildDedupKey(ruleID, assetID, nextHour) == want {
		t.Fatalf("dedup key did not change across hour boundary")
	}

	// Nil asset uses "none" placeholder.
	nilAsset := buildDedupKey(ruleID, uuid.Nil, fixed)
	wantNil := "11111111-1111-1111-1111-111111111111:none:2026-04-19T14"
	if nilAsset != wantNil {
		t.Fatalf("nil-asset dedup key: got %q, want %q", nilAsset, wantNil)
	}
}

// TestEvaluator_ConsecutiveBelowThreshold: one breach with consecutive=2
// required must NOT emit. This is the primary flapping guard.
func TestEvaluator_ConsecutiveBelowThreshold(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	cond := RuleCondition{Operator: ">", Threshold: 80, WindowSeconds: 60, Aggregation: "avg", ConsecutiveTriggers: 2}
	rule := buildRule(tenantID, "cpu.usage", cond, "warning")

	lister := &fakeRuleLister{rules: []dbgen.AlertRule{rule}}
	pool := newFakePool()
	pool.setSamples(tenantID, "cpu.usage", fakeSample{AssetID: assetID, Value: 95})

	bus := &fakeBus{}
	ev := NewEvaluator(lister, pool, bus, WithLogger(zap.NewNop()))
	ev.runTick(context.Background())

	if emits := pool.getEmits(); len(emits) != 0 {
		t.Fatalf("expected 0 emits after first breach (consecutive=2 required), got %d", len(emits))
	}
	if events := bus.recorded(); len(events) != 0 {
		t.Fatalf("expected 0 bus events, got %d", len(events))
	}
}

// TestEvaluator_ConsecutiveCrossesThreshold: second breach in a row fires.
func TestEvaluator_ConsecutiveCrossesThreshold(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	cond := RuleCondition{Operator: ">", Threshold: 80, WindowSeconds: 60, Aggregation: "avg", ConsecutiveTriggers: 2}
	rule := buildRule(tenantID, "cpu.usage", cond, "warning")

	lister := &fakeRuleLister{rules: []dbgen.AlertRule{rule}}
	pool := newFakePool()
	pool.setSamples(tenantID, "cpu.usage", fakeSample{AssetID: assetID, Value: 95})

	bus := &fakeBus{}
	ev := NewEvaluator(lister, pool, bus, WithLogger(zap.NewNop()))

	ev.runTick(context.Background())
	ev.runTick(context.Background())

	emits := pool.getEmits()
	if len(emits) != 1 {
		t.Fatalf("expected exactly 1 emit after two consecutive breaches, got %d", len(emits))
	}
	if emits[0].Status != "firing" {
		t.Fatalf("expected status=firing, got %q", emits[0].Status)
	}
	if !emits[0].Inserted {
		t.Fatalf("expected inserted=true on first emit")
	}
	events := bus.recorded()
	if len(events) != 1 {
		t.Fatalf("expected 1 bus event, got %d", len(events))
	}
	if events[0].Subject != eventbus.SubjectAlertFired {
		t.Fatalf("expected subject %q, got %q", eventbus.SubjectAlertFired, events[0].Subject)
	}
}

// TestEvaluator_DedupWithinHour: once firing, subsequent breaches UPDATE the
// same row (inserted=false) instead of spamming new rows.
func TestEvaluator_DedupWithinHour(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	cond := RuleCondition{Operator: ">", Threshold: 80, WindowSeconds: 60, Aggregation: "avg", ConsecutiveTriggers: 1}
	rule := buildRule(tenantID, "cpu.usage", cond, "warning")

	lister := &fakeRuleLister{rules: []dbgen.AlertRule{rule}}
	pool := newFakePool()
	pool.setSamples(tenantID, "cpu.usage", fakeSample{AssetID: assetID, Value: 95})

	bus := &fakeBus{}

	// Pin the clock so all three ticks land in the same hour bucket.
	fixed := time.Date(2026, 4, 19, 12, 0, 0, 0, time.UTC)
	ev := NewEvaluator(lister, pool, bus, WithLogger(zap.NewNop()), WithClock(func() time.Time {
		return fixed
	}))

	ev.runTick(context.Background())
	ev.runTick(context.Background())
	ev.runTick(context.Background())

	emits := pool.getEmits()
	if len(emits) != 3 {
		t.Fatalf("expected 3 emit calls (all dedup to same row), got %d", len(emits))
	}
	if !emits[0].Inserted {
		t.Fatalf("first emit should insert")
	}
	if emits[1].Inserted || emits[2].Inserted {
		t.Fatalf("subsequent emits within the same hour must be UPDATEs, got inserted=%v,%v", emits[1].Inserted, emits[2].Inserted)
	}
	for _, e := range emits {
		if e.Status != "firing" {
			t.Fatalf("expected status=firing, got %q", e.Status)
		}
	}
}

// TestEvaluator_ResolvesWhenBelow: value back below threshold emits resolved.
func TestEvaluator_ResolvesWhenBelow(t *testing.T) {
	tenantID := uuid.New()
	assetID := uuid.New()
	cond := RuleCondition{Operator: ">", Threshold: 80, WindowSeconds: 60, Aggregation: "avg", ConsecutiveTriggers: 1}
	rule := buildRule(tenantID, "cpu.usage", cond, "warning")

	lister := &fakeRuleLister{rules: []dbgen.AlertRule{rule}}
	pool := newFakePool()
	pool.setSamples(tenantID, "cpu.usage", fakeSample{AssetID: assetID, Value: 95})

	bus := &fakeBus{}
	ev := NewEvaluator(lister, pool, bus, WithLogger(zap.NewNop()))

	// Fire once (consecutive=1 so first tick fires).
	ev.runTick(context.Background())

	// Now value drops below threshold.
	pool.clearSamples()
	pool.setSamples(tenantID, "cpu.usage", fakeSample{AssetID: assetID, Value: 10})

	ev.runTick(context.Background())

	emits := pool.getEmits()
	if len(emits) != 2 {
		t.Fatalf("expected 2 emits (firing + resolved), got %d", len(emits))
	}
	if emits[0].Status != "firing" {
		t.Fatalf("first emit should be firing, got %q", emits[0].Status)
	}
	if emits[1].Status != "resolved" {
		t.Fatalf("second emit should be resolved, got %q", emits[1].Status)
	}

	// Bus should see one alert.fired and one alert.resolved.
	events := bus.recorded()
	if len(events) != 2 || events[0].Subject != eventbus.SubjectAlertFired || events[1].Subject != eventbus.SubjectAlertResolved {
		t.Fatalf("unexpected bus events: %+v", events)
	}
}

// TestEvaluator_MalformedConditionSkipped: a rule with bogus condition is
// logged + skipped, evaluation continues for the rest.
func TestEvaluator_MalformedConditionSkipped(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	assetB := uuid.New()

	bogus := dbgen.AlertRule{
		ID:         uuid.New(),
		TenantID:   tenantA,
		Name:       "bogus",
		MetricName: "cpu.usage",
		Condition:  json.RawMessage(`{"operator":"~","threshold":0}`),
		Severity:   "warning",
		Enabled:    true,
		CreatedAt:  time.Now(),
	}
	good := buildRule(tenantB, "cpu.usage",
		RuleCondition{Operator: ">", Threshold: 80, WindowSeconds: 60, Aggregation: "avg", ConsecutiveTriggers: 1},
		"warning")

	lister := &fakeRuleLister{rules: []dbgen.AlertRule{bogus, good}}
	pool := newFakePool()
	pool.setSamples(tenantB, "cpu.usage", fakeSample{AssetID: assetB, Value: 99})

	ev := NewEvaluator(lister, pool, &fakeBus{}, WithLogger(zap.NewNop()))
	ev.runTick(context.Background())

	emits := pool.getEmits()
	if len(emits) != 1 {
		t.Fatalf("expected exactly 1 emit (good rule only), got %d", len(emits))
	}
	if emits[0].TenantID != tenantB {
		t.Fatalf("emit tenant mismatch: got %s, want %s", emits[0].TenantID, tenantB)
	}
}

// TestEvaluator_ContextCancelExitsLoop: Start returns promptly on ctx.Done.
func TestEvaluator_ContextCancelExitsLoop(t *testing.T) {
	lister := &fakeRuleLister{rules: nil}
	pool := newFakePool()
	ev := NewEvaluator(lister, pool, nil,
		WithLogger(zap.NewNop()),
		WithInterval(10*time.Millisecond))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ev.Start(ctx)
		close(done)
	}()

	// Let the loop run at least one tick.
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("evaluator did not exit within 500ms of ctx cancel")
	}
}

// TestEvaluator_TenantIsolation: a tenant-A rule must never query tenant-B
// metrics. The fake pool rejects any unapproved tenant call — if the
// evaluator ever fans a tenant-A rule into a tenant-B query, this returns
// an error and emits are suppressed.
func TestEvaluator_TenantIsolation(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	assetA := uuid.New()

	ruleA := buildRule(tenantA, "cpu.usage",
		RuleCondition{Operator: ">", Threshold: 50, WindowSeconds: 60, Aggregation: "avg", ConsecutiveTriggers: 1},
		"warning")
	ruleB := buildRule(tenantB, "cpu.usage",
		RuleCondition{Operator: ">", Threshold: 50, WindowSeconds: 60, Aggregation: "avg", ConsecutiveTriggers: 1},
		"warning")

	lister := &fakeRuleLister{rules: []dbgen.AlertRule{ruleA, ruleB}}
	pool := newFakePool()
	pool.allowTenant(tenantA, tenantB) // both allowed, but strictly isolated
	pool.tenantMismatchErr = errors.New("cross-tenant query — tenant isolation broken")

	// Only tenant A has data. Tenant B's query will return no rows (empty
	// allowed result), NOT a tenant-mismatch error, because B is in the
	// allow-list but has no samples.
	pool.setSamples(tenantA, "cpu.usage", fakeSample{AssetID: assetA, Value: 90})

	ev := NewEvaluator(lister, pool, &fakeBus{}, WithLogger(zap.NewNop()))
	ev.runTick(context.Background())

	emits := pool.getEmits()
	if len(emits) != 1 {
		t.Fatalf("expected 1 emit (tenant A only), got %d", len(emits))
	}
	if emits[0].TenantID != tenantA {
		t.Fatalf("emit tenant = %s, want %s — evaluator mixed tenants", emits[0].TenantID, tenantA)
	}
}

// TestEvaluator_TenantIsolationStrict: explicitly rejects any call that
// escapes the correct tenant scope. Invented by building a second fake that
// only allows tenant A and confirms tenant B's rule does NOT query pool (the
// fake would error and we'd see zero emits for B — which is the whole
// point).
func TestEvaluator_TenantIsolationStrict(t *testing.T) {
	tenantA := uuid.New()
	tenantB := uuid.New()
	assetA := uuid.New()
	assetB := uuid.New()

	ruleA := buildRule(tenantA, "cpu.usage",
		RuleCondition{Operator: ">", Threshold: 50, WindowSeconds: 60, Aggregation: "avg", ConsecutiveTriggers: 1},
		"warning")

	// Build a pool that ONLY allows tenant A. If the evaluator ever leaks
	// tenant B into a query, the Query call errors and we never emit.
	pool := newFakePool()
	pool.allowTenant(tenantA)
	pool.setSamples(tenantA, "cpu.usage", fakeSample{AssetID: assetA, Value: 90})
	// Even though we registered a sample for tenant B, the allow-list will
	// block the query. So a bug where rule A somehow asks for tenant B's
	// data would be caught.
	pool.setSamples(tenantB, "cpu.usage", fakeSample{AssetID: assetB, Value: 99})

	lister := &fakeRuleLister{rules: []dbgen.AlertRule{ruleA}}
	ev := NewEvaluator(lister, pool, &fakeBus{}, WithLogger(zap.NewNop()))
	ev.runTick(context.Background())

	emits := pool.getEmits()
	if len(emits) != 1 {
		t.Fatalf("expected 1 emit, got %d", len(emits))
	}
	if emits[0].TenantID != tenantA || emits[0].TriggerValue != 90 {
		t.Fatalf("wrong emit: %+v", emits[0])
	}
}

// TestEvaluator_PanicRecovery: evaluator survives a panic in the lister.
// We exercise this via a lister that panics on its first call; the tick
// must not propagate the panic to Start's goroutine.
func TestEvaluator_PanicRecovery(t *testing.T) {
	pool := newFakePool()
	ev := NewEvaluator(&panicLister{}, pool, &fakeBus{}, WithLogger(zap.NewNop()))

	// runTick must not panic.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runTick propagated panic: %v", r)
		}
	}()
	ev.runTick(context.Background())
}

type panicLister struct{}

func (p *panicLister) ListEnabledAlertRules(ctx context.Context) ([]dbgen.AlertRule, error) {
	panic("synthetic panic to verify recovery")
}

// TestEvaluator_ListError: a DB error from ListEnabledAlertRules is logged,
// tick is counted as error, loop continues.
func TestEvaluator_ListError(t *testing.T) {
	lister := &fakeRuleLister{err: errors.New("connection reset")}
	pool := newFakePool()
	ev := NewEvaluator(lister, pool, &fakeBus{}, WithLogger(zap.NewNop()))

	ev.runTick(context.Background())

	if len(pool.getEmits()) != 0 {
		t.Fatalf("expected no emits on list error")
	}
}
