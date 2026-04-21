package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// BackgroundTracerName is the package identifier used for background-
// loop spans so Jaeger / Tempo filters can target "workflow.tick.*" and
// "sync.tick.*" names without tracer-name ambiguity.
const BackgroundTracerName = "github.com/cmdb-platform/cmdb-core/background"

// StartTickSpan opens a root span for a single iteration of a background
// ticker loop. It is intended to replace the bare `e.doWork(ctx)` call
// inside `case <-ticker.C:` branches so every iteration gets its own
// trace id. The returned context carries the span — downstream pgx /
// Redis / NATS work is stitched under it via the normal propagation.
//
// The returned cleanup function should be deferred by the caller.
//
// Sampling is unchanged from the global TracerProvider configuration
// (ParentBased(TraceIDRatioBased(0.1)) by default). Because tick spans
// are roots with no parent, they take the ratio directly — ~10% of
// ticks are sampled end-to-end, same as before.
func StartTickSpan(ctx context.Context, name string) (context.Context, func()) {
	tracer := otel.Tracer(BackgroundTracerName)
	ctx, span := tracer.Start(ctx, name, trace.WithSpanKind(trace.SpanKindInternal))
	return ctx, func() { span.End() }
}
