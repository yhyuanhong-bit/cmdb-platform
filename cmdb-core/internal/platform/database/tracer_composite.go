package database

import (
	"context"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
)

// CompositeTracer combines the OpenTelemetry pgx tracer with the in-house
// SlowQueryTracer so both emit for every database round-trip. It embeds
// *otelpgx.Tracer to inherit BatchTracer / CopyFromTracer / PrepareTracer /
// ConnectTracer / pgxpool.AcquireTracer implementations, then overrides the
// QueryTracer methods to fan out to SlowQueryTracer as well.
//
// Ordering on Start: otel span is opened first so slow-query ctx keys never
// leak into the span attributes, and otel's ctx is the one SlowQueryTracer
// writes into — keeps both observable under one request span.
type CompositeTracer struct {
	*otelpgx.Tracer
	slow *SlowQueryTracer
}

// NewCompositeTracer returns a tracer that delegates to both otel and the
// slow-query path. slow may be nil (e.g. tests) in which case only otel is
// invoked.
func NewCompositeTracer(otel *otelpgx.Tracer, slow *SlowQueryTracer) *CompositeTracer {
	return &CompositeTracer{Tracer: otel, slow: slow}
}

// TraceQueryStart fans the Start call out to both tracers, returning the
// context that carries state for both.
func (c *CompositeTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if c.Tracer != nil {
		ctx = c.Tracer.TraceQueryStart(ctx, conn, data)
	}
	if c.slow != nil {
		ctx = c.slow.TraceQueryStart(ctx, conn, data)
	}
	return ctx
}

// TraceQueryEnd fans End to both. Each tracer reads its own ctx keys so
// they do not interfere.
func (c *CompositeTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	if c.Tracer != nil {
		c.Tracer.TraceQueryEnd(ctx, conn, data)
	}
	if c.slow != nil {
		c.slow.TraceQueryEnd(ctx, conn, data)
	}
}

var _ pgx.QueryTracer = (*CompositeTracer)(nil)
