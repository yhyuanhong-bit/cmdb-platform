package telemetry

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer initialises an OpenTelemetry TracerProvider. When endpoint is
// empty tracing is disabled and a no-op shutdown function is returned.
func InitTracer(ctx context.Context, endpoint, serviceName, version string) (shutdown func(context.Context) error, err error) {
	noop := func(context.Context) error { return nil }

	if endpoint == "" {
		return noop, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return noop, fmt.Errorf("creating OTLP exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(version),
		),
	)
	if err != nil {
		return noop, fmt.Errorf("creating resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.1))),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

// TracingMiddleware returns a Gin middleware that creates OpenTelemetry spans
// for each incoming HTTP request.
//
// Sensitive-header handling: otelgin v0.67.0 does NOT record request or
// response headers as span attributes by default — only the standard
// semconv HTTP attributes (method, route, status_code, user_agent, etc.).
// This is verified by TestTracingMiddleware_ScrubsSensitiveRequestHeaders in
// tracing_test.go, which fails the build if a future contrib upgrade
// silently starts recording Authorization, Cookie, or similar headers.
//
// If you upgrade otelgin and the regression test starts to fail, either:
//   - pass otelgin.WithFilter to skip sensitive paths entirely, or
//   - migrate to a custom wrapper that strips SensitiveRequestHeaders from
//     the span attributes before export.
//
// See SensitiveRequestHeaders in scrubber.go for the canonical scrub list.
func TracingMiddleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
