// Package database — slow query tracer.
//
// SlowQueryTracer implements pgx.QueryTracer. It captures a monotonic start
// timestamp in TraceQueryStart (stashed in the returned context), and in
// TraceQueryEnd computes time.Since, emits a structured zap.Warn when the
// duration exceeds a configurable threshold, and increments a Prometheus
// counter labelled by a low-cardinality query fingerprint.
//
// Safety invariants (critical — this runs in every DB round trip):
//
//   - No tracer method may panic. All emission work is wrapped in
//     defer+recover so a misbehaving logger or metric registry cannot
//     take down a live query path.
//   - Query argument VALUES are never logged. pgx passes them to us via
//     TraceQueryStartData.Args, but args frequently contain passwords
//     (login, password reset), API tokens, and PII. We log only the
//     arg COUNT (useful for query-shape sanity checks) and the SQL
//     string with placeholders intact ($1, $2, ...).
//   - SQL text is truncated to 500 chars to keep log lines bounded on
//     bulk inserts / generated IN-list queries.
//   - Fingerprint label is capped at 80 chars and has literals scrubbed
//     so Prometheus label cardinality stays manageable — we want one
//     series per query shape, not one per concrete invocation.
//
// This tracer is independent of Phase 4.6 (otelpgx). It can ship standalone
// and be composed with an OTel tracer later.
package database

import (
	"context"
	"os"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
)

// Threshold bounds and env-var name. Defaults chosen to be noisy-but-
// useful on slow paths without drowning staging in chatter.
const (
	defaultSlowQueryMs = 500
	minSlowQueryMs     = 50
	maxSlowQueryMs     = 10_000

	// EnvSlowQueryThresholdMs is the env var override read by
	// ResolveSlowQueryThreshold. Value is in milliseconds, clamped to
	// [50, 10000] so operators cannot accidentally disable the tracer
	// (0) or turn it into a firehose (very small value).
	EnvSlowQueryThresholdMs = "CMDB_SLOW_QUERY_THRESHOLD_MS"

	// maxSQLLogChars caps the SQL string inside the zap log field.
	// Bulk INSERTs and generated IN-list queries can exceed 10KB and
	// drown the log pipeline; 500 chars is the spec.
	maxSQLLogChars = 500

	// maxFingerprintChars caps the Prometheus label for cardinality
	// control. Real-world SQL shapes collapse well below this with
	// literals scrubbed.
	maxFingerprintChars = 80
)

// Prometheus metrics. Registered via promauto so they appear on the
// default registry and are scraped by the existing /metrics handler
// without any additional wiring.
var (
	dbSlowQueriesTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "cmdb_slow_queries_total",
		Help: "Count of DB queries whose duration exceeded the configured slow-query threshold. Labelled by a sanitized SQL fingerprint (literals stripped).",
	}, []string{"query_fingerprint"})
)

// SlowQueryCounter exposes the underlying CounterVec for tests. Package-
// private users (tests in this package) access dbSlowQueriesTotal directly.
func SlowQueryCounter() *prometheus.CounterVec { return dbSlowQueriesTotal }

// tracerCtxKey is a zero-sized, unexported type so our context value
// cannot collide with anything else (including otelpgx if later composed).
type tracerCtxKey struct{}

// queryStartData is what we stash in ctx at TraceQueryStart and read at
// TraceQueryEnd. startedAt is a monotonic clock reading (time.Now() on
// Go returns monotonic-capable times; time.Since uses the monotonic
// component when present).
type queryStartData struct {
	startedAt time.Time
	sql       string
	argsCount int
}

// SlowQueryTracer is a zero-dep pgx.QueryTracer that emits a warn log and
// bumps a Prometheus counter when a query runs longer than threshold.
type SlowQueryTracer struct {
	threshold time.Duration
	logger    *zap.Logger
}

// NewSlowQueryTracer returns a tracer with threshold clamp applied. A nil
// logger falls back to zap.L() (the global replaced in main()).
func NewSlowQueryTracer(threshold time.Duration, logger *zap.Logger) *SlowQueryTracer {
	if threshold <= 0 {
		threshold = defaultSlowQueryMs * time.Millisecond
	}
	if logger == nil {
		logger = zap.L()
	}
	return &SlowQueryTracer{threshold: threshold, logger: logger}
}

// ResolveSlowQueryThreshold reads EnvSlowQueryThresholdMs and clamps to
// [minSlowQueryMs, maxSlowQueryMs]. Missing / malformed env → default.
// Exported so NewPool and cmd-line tools share one resolver.
func ResolveSlowQueryThreshold() time.Duration {
	raw := os.Getenv(EnvSlowQueryThresholdMs)
	if raw == "" {
		return defaultSlowQueryMs * time.Millisecond
	}
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return defaultSlowQueryMs * time.Millisecond
	}
	if v < minSlowQueryMs {
		v = minSlowQueryMs
	}
	if v > maxSlowQueryMs {
		v = maxSlowQueryMs
	}
	return time.Duration(v) * time.Millisecond
}

// TraceQueryStart stashes a start timestamp + query shape into the ctx
// and returns the new ctx, which pgx threads into TraceQueryEnd.
//
// We deliberately do NOT access data.Args — pgx hands us the concrete
// argument values, but those values may be passwords, tokens, or PII.
// We record the length only.
func (t *SlowQueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, tracerCtxKey{}, queryStartData{
		startedAt: time.Now(),
		sql:       data.SQL,
		argsCount: len(data.Args),
	})
}

// TraceQueryEnd computes duration, and if above threshold emits a warn
// log and increments the Prometheus counter.
//
// Everything after the initial ctx read is wrapped in a defer/recover so
// a logger panic, metric registry race, or runtime.Caller wobble can
// never propagate into the caller's query result.
func (t *SlowQueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	start, ok := ctx.Value(tracerCtxKey{}).(queryStartData)
	if !ok {
		return
	}
	dur := time.Since(start.startedAt)
	if dur < t.threshold {
		return
	}

	defer func() {
		// Absolute-last line of defence. We never want a tracer bug
		// to kill a user-facing request. Recover swallows the panic
		// and drops the observation.
		_ = recover()
	}()

	fingerprint := fingerprintSQL(start.sql)
	dbSlowQueriesTotal.WithLabelValues(fingerprint).Inc()

	sql := start.sql
	truncated := false
	if len(sql) > maxSQLLogChars {
		sql = sql[:maxSQLLogChars]
		truncated = true
	}

	file, line := callerOutsidePgx()

	fields := []zap.Field{
		zap.Duration("duration", dur),
		zap.Duration("threshold", t.threshold),
		zap.String("sql", sql),
		zap.Bool("sql_truncated", truncated),
		zap.Int("args_count", start.argsCount),
		zap.String("fingerprint", fingerprint),
		zap.String("caller_file", file),
		zap.Int("caller_line", line),
	}
	if data.Err != nil {
		fields = append(fields, zap.Error(data.Err))
	}
	t.logger.Warn("slow db query", fields...)
}

// Compile-time assertion that we satisfy the pgx interface.
var _ pgx.QueryTracer = (*SlowQueryTracer)(nil)

// ---------- fingerprint + caller helpers ----------

// These regexes are module-level so they compile once.
//
//  1. Strip single-quoted strings first (handles 'it''s' via the lazy
//     match repeated; good-enough for fingerprinting — we are not
//     parsing SQL, we are making a stable low-cardinality label).
//  2. Collapse numeric literals (standalone integers / floats).
//  3. Collapse bind placeholders $1 $23 ... to $?.
//  4. Collapse runs of whitespace to single space for stable labels.
var (
	reStringLit    = regexp.MustCompile(`'(?:[^']|'')*'`)
	reNumericLit   = regexp.MustCompile(`\b\d+(?:\.\d+)?\b`)
	rePlaceholder  = regexp.MustCompile(`\$\d+`)
	reWhitespace   = regexp.MustCompile(`\s+`)
	reLeadingWS    = regexp.MustCompile(`^\s+`)
	reTrailingWS   = regexp.MustCompile(`\s+$`)
	reSQLCommentLn = regexp.MustCompile(`--[^\n]*`)
)

// fingerprintSQL returns a sanitized, length-capped SQL prefix suitable
// as a Prometheus label value. Rules:
//   - Strip SQL line comments (often contain tenant info in ORM output).
//   - Replace single-quoted string literals with '?'.
//   - Replace $1 / $N placeholders with $?.
//   - Replace numeric literals with ?.
//   - Collapse whitespace; trim; cap to maxFingerprintChars.
//
// Empty / all-whitespace input → "unknown". This guarantees the label
// value is always a non-empty stable string.
func fingerprintSQL(sql string) string {
	if sql == "" {
		return "unknown"
	}
	// Comments first — they may contain literals that would otherwise
	// get partially scrubbed and produce noisy fingerprints.
	s := reSQLCommentLn.ReplaceAllString(sql, "")
	s = reStringLit.ReplaceAllString(s, "'?'")
	s = rePlaceholder.ReplaceAllString(s, "$?")
	s = reNumericLit.ReplaceAllString(s, "?")
	s = reWhitespace.ReplaceAllString(s, " ")
	s = reLeadingWS.ReplaceAllString(s, "")
	s = reTrailingWS.ReplaceAllString(s, "")
	if s == "" {
		return "unknown"
	}
	if len(s) > maxFingerprintChars {
		s = s[:maxFingerprintChars]
	}
	return s
}

// callerOutsidePgx walks a bounded window of stack frames to find the
// first caller that lives outside the pgx driver + this package. The
// intent is to point a human at the *application* line that issued the
// slow query (e.g. the sqlc-generated file, or a handler), not at a
// pgx internal frame which is useless for debugging.
//
// Bound (16 frames) keeps the runtime cost tiny and avoids pathological
// walking on deeply instrumented stacks.
func callerOutsidePgx() (string, int) {
	const maxDepth = 16
	pcs := make([]uintptr, maxDepth)
	n := runtime.Callers(2, pcs) // skip runtime.Callers + this helper
	if n == 0 {
		return "unknown", 0
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if frame.File != "" && !isTracerInternalFrame(frame.File) {
			return shortenPath(frame.File), frame.Line
		}
		if !more {
			break
		}
	}
	return "unknown", 0
}

// isTracerInternalFrame returns true for frames we want to walk past:
// this tracer file itself, and anything inside the pgx module (driver
// and pgxpool). We do NOT filter out dbgen / sqlc output — those are
// application code the developer wants to land in.
func isTracerInternalFrame(file string) bool {
	// Walk past *our own* tracer frames.
	if strings.HasSuffix(file, "slow_query_tracer.go") {
		return true
	}
	// Walk past pgx driver frames. Module path is
	// github.com/jackc/pgx/v{N}/...; also catch the pgxpool subpkg.
	if strings.Contains(file, "/jackc/pgx/") {
		return true
	}
	return false
}

// shortenPath returns the last two path segments, which is plenty to
// locate a file in a code search without bloating log volume with
// GOPATH absolute paths.
func shortenPath(p string) string {
	// We want "internal/api/impl_assets.go" style, not a full
	// /home/runner/... absolute.
	if p == "" {
		return "unknown"
	}
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	prev := strings.LastIndex(p[:idx], "/")
	if prev < 0 {
		return p[idx+1:]
	}
	return p[prev+1:]
}
