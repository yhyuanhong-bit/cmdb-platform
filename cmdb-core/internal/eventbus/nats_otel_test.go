package eventbus

import (
	"context"
	"testing"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// TestNATSHeaderCarrierRoundTrip asserts that a trace id injected into a
// nats.Header via the carrier can be extracted back out with byte-identical
// trace + span id. This is the single most important invariant for the
// HTTP → NATS → subscriber trace stitching, so the guarantee is locked in
// at unit-test granularity instead of relying on a running JetStream.
func TestNATSHeaderCarrierRoundTrip(t *testing.T) {
	// Stand up an always-sampled tracer provider so the span we create
	// below is guaranteed non-zero — the default global provider uses
	// ParentBased(TraceIDRatioBased(0.1)) and would silently drop ~90%
	// of injection attempts, making this test flake.
	prev := otel.GetTracerProvider()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)

	// Same propagator contract as main.go wires at startup — TraceContext
	// only (no baggage) to match the production carrier surface.
	otel.SetTextMapPropagator(propagation.TraceContext{})

	ctx, span := tp.Tracer("test").Start(context.Background(), "producer")
	defer span.End()

	src := span.SpanContext()

	h := nats.Header{}
	otel.GetTextMapPropagator().Inject(ctx, natsHeaderCarrier(h))

	if got := h.Get("traceparent"); got == "" {
		t.Fatalf("expected traceparent header after Inject, got empty header: %+v", h)
	}

	extracted := otel.GetTextMapPropagator().Extract(context.Background(), natsHeaderCarrier(h))
	dst := trace.SpanContextFromContext(extracted)

	if !dst.IsValid() {
		t.Fatalf("extracted span context is invalid: %+v", dst)
	}
	if dst.TraceID() != src.TraceID() {
		t.Fatalf("trace id drift: src=%s dst=%s", src.TraceID(), dst.TraceID())
	}
	if dst.SpanID() != src.SpanID() {
		t.Fatalf("span id drift: src=%s dst=%s", src.SpanID(), dst.SpanID())
	}
}

// TestNATSHeaderCarrierEmpty verifies that Get on an empty header does not
// panic and returns "" — important because subscribe-side Extract runs on
// every received message including legacy messages published before the
// propagator was wired.
func TestNATSHeaderCarrierEmpty(t *testing.T) {
	var c natsHeaderCarrier
	if got := c.Get("traceparent"); got != "" {
		t.Fatalf("expected empty string from nil carrier, got %q", got)
	}

	h := nats.Header{}
	c2 := natsHeaderCarrier(h)
	if got := c2.Get("traceparent"); got != "" {
		t.Fatalf("expected empty string from empty header, got %q", got)
	}
	if keys := c2.Keys(); len(keys) != 0 {
		t.Fatalf("expected no keys from empty header, got %v", keys)
	}
}
