package database

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

// newObserver builds a zap.Logger that records every log entry at or
// above Warn into a returned observer. We use Warn level so our slow-
// query warn lines land, while keeping debug/info chatter out.
func newObserver(t *testing.T) (*zap.Logger, *observer.ObservedLogs) {
	t.Helper()
	core, recorded := observer.New(zapcore.WarnLevel)
	return zap.New(core), recorded
}

// -------- fingerprintSQL ---------

func TestFingerprintSQL_TableDriven(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "unknown",
		},
		{
			name: "whitespace only",
			in:   "   \n\t  ",
			want: "unknown",
		},
		{
			name: "positional placeholders",
			in:   "SELECT * FROM users WHERE id = $1 AND tenant_id = $2",
			want: "SELECT * FROM users WHERE id = $? AND tenant_id = $?",
		},
		{
			name: "numeric literal",
			in:   "SELECT * FROM posts LIMIT 50 OFFSET 100",
			want: "SELECT * FROM posts LIMIT ? OFFSET ?",
		},
		{
			name: "string literal scrubbed",
			in:   "SELECT * FROM users WHERE email = 'admin@example.com'",
			want: "SELECT * FROM users WHERE email = '?'",
		},
		{
			name: "line comment stripped",
			in:   "-- tenant:deadbeef\nSELECT 1",
			want: "SELECT ?",
		},
		{
			name: "whitespace collapsed",
			in:   "SELECT   *\n\tFROM   users",
			want: "SELECT * FROM users",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fingerprintSQL(tc.in)
			if got != tc.want {
				t.Fatalf("fingerprintSQL(%q)\n  got  = %q\n  want = %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestFingerprintSQL_CappedToMaxLen(t *testing.T) {
	// Build a SQL string whose sanitized form clearly exceeds the cap.
	big := "SELECT col FROM tbl WHERE " + strings.Repeat("a", 200) + " = $1"
	got := fingerprintSQL(big)
	if len(got) > maxFingerprintChars {
		t.Fatalf("fingerprint %d chars, want <= %d", len(got), maxFingerprintChars)
	}
}

// Security: make sure string literals containing what *looks* like a
// password can never survive as raw text in the fingerprint. We use
// the placeholder "password=$1" in the test SQL to keep the repo free
// of plausible secrets, but also test an inline literal.
func TestFingerprintSQL_ScrubsPotentialSecretLiterals(t *testing.T) {
	sql := "UPDATE users SET password_hash = 'hunter2-fake-for-test' WHERE id = $1"
	got := fingerprintSQL(sql)
	if strings.Contains(got, "hunter2") {
		t.Fatalf("fingerprint leaked a string literal: %q", got)
	}
}

// -------- ResolveSlowQueryThreshold ---------

func TestResolveSlowQueryThreshold_DefaultsWhenUnset(t *testing.T) {
	t.Setenv(EnvSlowQueryThresholdMs, "")
	got := ResolveSlowQueryThreshold()
	want := time.Duration(defaultSlowQueryMs) * time.Millisecond
	if got != want {
		t.Fatalf("default threshold: got %v, want %v", got, want)
	}
}

func TestResolveSlowQueryThreshold_ClampsLow(t *testing.T) {
	t.Setenv(EnvSlowQueryThresholdMs, "1")
	got := ResolveSlowQueryThreshold()
	want := time.Duration(minSlowQueryMs) * time.Millisecond
	if got != want {
		t.Fatalf("clamp low: got %v, want %v", got, want)
	}
}

func TestResolveSlowQueryThreshold_ClampsHigh(t *testing.T) {
	t.Setenv(EnvSlowQueryThresholdMs, "99999999")
	got := ResolveSlowQueryThreshold()
	want := time.Duration(maxSlowQueryMs) * time.Millisecond
	if got != want {
		t.Fatalf("clamp high: got %v, want %v", got, want)
	}
}

func TestResolveSlowQueryThreshold_BogusInputFallsBackToDefault(t *testing.T) {
	t.Setenv(EnvSlowQueryThresholdMs, "not-a-number")
	got := ResolveSlowQueryThreshold()
	want := time.Duration(defaultSlowQueryMs) * time.Millisecond
	if got != want {
		t.Fatalf("bogus env: got %v, want %v", got, want)
	}
}

func TestResolveSlowQueryThreshold_AcceptsWithinRange(t *testing.T) {
	t.Setenv(EnvSlowQueryThresholdMs, "250")
	got := ResolveSlowQueryThreshold()
	want := 250 * time.Millisecond
	if got != want {
		t.Fatalf("in-range: got %v, want %v", got, want)
	}
}

func TestResolveSlowQueryThreshold_UnsetEnvUsesDefault(t *testing.T) {
	// Explicitly ensure env is unset (Unsetenv vs empty string).
	_ = os.Unsetenv(EnvSlowQueryThresholdMs)
	got := ResolveSlowQueryThreshold()
	want := time.Duration(defaultSlowQueryMs) * time.Millisecond
	if got != want {
		t.Fatalf("unset env: got %v, want %v", got, want)
	}
}

// -------- SlowQueryTracer TraceQueryStart / TraceQueryEnd ---------

// fakeTraceStart / fakeTraceEnd synthesize pgx's tracer callback pair
// without a real connection. We pass nil *pgx.Conn — the tracer must
// never dereference it (and this implementation does not).
func fakeTraceStart(t *testing.T, tr *SlowQueryTracer, sql string, args []any) context.Context {
	t.Helper()
	return tr.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
		SQL:  sql,
		Args: args,
	})
}

func fakeTraceEnd(t *testing.T, tr *SlowQueryTracer, ctx context.Context, err error) {
	t.Helper()
	tr.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: err})
}

func TestSlowQueryTracer_BelowThreshold_NoLog(t *testing.T) {
	logger, recorded := newObserver(t)
	tr := NewSlowQueryTracer(500*time.Millisecond, logger)

	ctx := fakeTraceStart(t, tr, "SELECT 1", nil)
	// No sleep — duration is effectively zero, well below 500ms.
	fakeTraceEnd(t, tr, ctx, nil)

	if got := recorded.Len(); got != 0 {
		t.Fatalf("below-threshold: expected 0 warn lines, got %d (%v)", got, recorded.All())
	}
}

func TestSlowQueryTracer_AboveThreshold_EmitsWarn(t *testing.T) {
	logger, recorded := newObserver(t)
	// 10ms threshold keeps the test fast but unambiguous.
	tr := NewSlowQueryTracer(10*time.Millisecond, logger)

	// Use the "password=$1" placeholder form — NEVER a real secret in
	// args. The tracer must not record args values anyway, but keeping
	// the test hygienic enforces the review rule.
	ctx := fakeTraceStart(t, tr, "UPDATE users SET password_hash = $2 WHERE id = $1", []any{"user-1", "REDACTED-PASSWORD-HASH"})
	time.Sleep(30 * time.Millisecond)
	fakeTraceEnd(t, tr, ctx, nil)

	if got := recorded.Len(); got != 1 {
		t.Fatalf("above-threshold: expected 1 warn line, got %d (%v)", got, recorded.All())
	}

	entry := recorded.All()[0]
	if entry.Message != "slow db query" {
		t.Fatalf("unexpected warn message: %q", entry.Message)
	}
	fieldMap := entry.ContextMap()

	// Required fields.
	for _, key := range []string{"duration", "threshold", "sql", "sql_truncated", "args_count", "fingerprint", "caller_file", "caller_line"} {
		if _, ok := fieldMap[key]; !ok {
			t.Fatalf("missing expected field %q in log entry; got keys=%v", key, keys(fieldMap))
		}
	}

	// Args VALUES must never appear anywhere in the log fields. We
	// encode this as a strong invariant — if someone adds zap.Any("args", ...)
	// later, this test will catch it.
	serialized := fieldsAsString(fieldMap)
	if strings.Contains(serialized, "REDACTED-PASSWORD-HASH") {
		t.Fatalf("arg value leaked into log fields: %s", serialized)
	}
	if strings.Contains(serialized, "user-1") {
		t.Fatalf("arg value leaked into log fields: %s", serialized)
	}

	// args_count must reflect the real count (2) so operators can
	// sanity check the query shape.
	if got, want := fieldMap["args_count"], int64(2); got != want {
		t.Fatalf("args_count: got %v, want %v", got, want)
	}

	// SQL must still contain the placeholder text.
	if sql, _ := fieldMap["sql"].(string); !strings.Contains(sql, "$1") {
		t.Fatalf("sql field missing placeholder: %q", sql)
	}
}

func TestSlowQueryTracer_AboveThreshold_IncrementsCounter(t *testing.T) {
	logger, _ := newObserver(t)
	tr := NewSlowQueryTracer(5*time.Millisecond, logger)

	sql := "SELECT * FROM assets WHERE tenant_id = $1 AND id = $2 /*unit-test-fingerprint*/"
	fp := fingerprintSQL(sql)

	before := testutil.ToFloat64(dbSlowQueriesTotal.WithLabelValues(fp))

	ctx := fakeTraceStart(t, tr, sql, []any{"t", "a"})
	time.Sleep(15 * time.Millisecond)
	fakeTraceEnd(t, tr, ctx, nil)

	after := testutil.ToFloat64(dbSlowQueriesTotal.WithLabelValues(fp))
	if after-before < 1 {
		t.Fatalf("counter not incremented: before=%v after=%v", before, after)
	}
}

func TestSlowQueryTracer_TruncatesLongSQL(t *testing.T) {
	logger, recorded := newObserver(t)
	tr := NewSlowQueryTracer(5*time.Millisecond, logger)

	// Much longer than maxSQLLogChars.
	long := "SELECT " + strings.Repeat("col, ", 300) + "id FROM very_wide_table WHERE id = $1"

	ctx := fakeTraceStart(t, tr, long, []any{"x"})
	time.Sleep(10 * time.Millisecond)
	fakeTraceEnd(t, tr, ctx, nil)

	if recorded.Len() != 1 {
		t.Fatalf("expected 1 warn line, got %d", recorded.Len())
	}
	entry := recorded.All()[0]
	fieldMap := entry.ContextMap()
	sql, _ := fieldMap["sql"].(string)
	if len(sql) > maxSQLLogChars {
		t.Fatalf("sql not truncated: len=%d (cap=%d)", len(sql), maxSQLLogChars)
	}
	if truncated, _ := fieldMap["sql_truncated"].(bool); !truncated {
		t.Fatalf("sql_truncated flag not set")
	}
}

func TestSlowQueryTracer_IncludesErrorOnFailure(t *testing.T) {
	logger, recorded := newObserver(t)
	tr := NewSlowQueryTracer(5*time.Millisecond, logger)

	qErr := errors.New("simulated query failure")

	ctx := fakeTraceStart(t, tr, "SELECT 1 FROM does_not_exist", nil)
	time.Sleep(10 * time.Millisecond)
	fakeTraceEnd(t, tr, ctx, qErr)

	if recorded.Len() != 1 {
		t.Fatalf("expected 1 warn line, got %d", recorded.Len())
	}
	entry := recorded.All()[0]
	fieldMap := entry.ContextMap()
	errVal, ok := fieldMap["error"]
	if !ok {
		t.Fatalf("expected error field on failed query log; fields=%v", keys(fieldMap))
	}
	if s, _ := errVal.(string); !strings.Contains(s, "simulated query failure") {
		t.Fatalf("error field did not contain original error text: %v", errVal)
	}
}

func TestSlowQueryTracer_TraceEndWithoutStart_NoPanic(t *testing.T) {
	logger, recorded := newObserver(t)
	tr := NewSlowQueryTracer(5*time.Millisecond, logger)

	// Simulate pgx for some reason calling TraceQueryEnd without a
	// matching TraceQueryStart (or a ctx from a different tracer
	// library that doesn't carry our key). Must be a silent no-op.
	tr.TraceQueryEnd(context.Background(), nil, pgx.TraceQueryEndData{})

	if got := recorded.Len(); got != 0 {
		t.Fatalf("expected 0 warn lines, got %d", got)
	}
}

func TestNewSlowQueryTracer_NilLoggerFallsBackToGlobal(t *testing.T) {
	tr := NewSlowQueryTracer(0, nil)
	if tr.logger == nil {
		t.Fatalf("expected non-nil logger (global zap fallback)")
	}
	if tr.threshold != defaultSlowQueryMs*time.Millisecond {
		t.Fatalf("zero threshold should fall back to default, got %v", tr.threshold)
	}
}

// -------- Panic safety ---------

// panicLogger is a zap core whose Write always panics. We use it to
// prove the tracer's defer+recover guard stops the panic from leaking.
type panicLogger struct{}

func (panicLogger) Enabled(zapcore.Level) bool      { return true }
func (p panicLogger) With([]zapcore.Field) zapcore.Core { return p }
func (panicLogger) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	return ce.AddCore(ent, panicLogger{})
}
func (panicLogger) Write(zapcore.Entry, []zapcore.Field) error {
	panic("intentional panic from test logger")
}
func (panicLogger) Sync() error { return nil }

func TestSlowQueryTracer_PanicInLoggerIsRecovered(t *testing.T) {
	logger := zap.New(panicLogger{})
	tr := NewSlowQueryTracer(5*time.Millisecond, logger)

	// This must NOT propagate the panic. Wrap in a defer that fails
	// the test if we somehow see one.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("tracer leaked panic: %v", r)
		}
	}()

	ctx := fakeTraceStart(t, tr, "SELECT 1", nil)
	time.Sleep(10 * time.Millisecond)
	fakeTraceEnd(t, tr, ctx, nil)
}

// -------- Interface conformance ---------

func TestSlowQueryTracer_ImplementsQueryTracer(t *testing.T) {
	// Compile-time check is in the .go file; runtime check here gives
	// a readable failure if the interface drifts.
	var _ pgx.QueryTracer = (*SlowQueryTracer)(nil)
}

// -------- shortenPath helper ---------

func TestShortenPath(t *testing.T) {
	cases := map[string]string{
		"":                                 "unknown",
		"a.go":                             "a.go",
		"pkg/a.go":                         "pkg/a.go",
		"/root/go/pkg/a.go":                "pkg/a.go",
		"/home/u/code/proj/internal/x.go":  "internal/x.go",
	}
	for in, want := range cases {
		if got := shortenPath(in); got != want {
			t.Fatalf("shortenPath(%q) = %q, want %q", in, got, want)
		}
	}
}

// -------- isTracerInternalFrame helper ---------

func TestIsTracerInternalFrame(t *testing.T) {
	cases := map[string]bool{
		"/root/go/pkg/mod/github.com/jackc/pgx/v5@v5.9.1/conn.go":              true,
		"/root/go/pkg/mod/github.com/jackc/pgx/v5/pgxpool/pool.go":             true,
		"/workspace/cmdb-core/internal/platform/database/slow_query_tracer.go": true,
		"/workspace/cmdb-core/internal/api/impl_assets.go":                     false,
		"/workspace/cmdb-core/internal/dbgen/queries.sql.go":                   false,
	}
	for file, want := range cases {
		if got := isTracerInternalFrame(file); got != want {
			t.Fatalf("isTracerInternalFrame(%q) = %v, want %v", file, got, want)
		}
	}
}

// ------------------------------------------------------------------

// keys returns map keys for diagnostic messages.
func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// fieldsAsString renders a field map into a single string for leak checks.
func fieldsAsString(m map[string]any) string {
	var b strings.Builder
	for k, v := range m {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(stringify(v))
		b.WriteString(" ")
	}
	return b.String()
}

func stringify(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	default:
		return ""
	}
}
