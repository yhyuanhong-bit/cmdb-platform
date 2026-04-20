package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// sensitiveHeaderValues lists the placeholder values used in fixtures. If any
// of these appear in span attribute values the scrubber has failed.
var sensitiveHeaderValues = []string{
	"Bearer test-secret-do-not-use",
	"session=test-cookie-do-not-use",
	"test-apikey-do-not-use",
	"test-csrf-do-not-use",
	"test-auth-token-do-not-use",
	"Basic proxy-test-do-not-use",
}

// sensitiveHeaders are the request headers our middleware must scrub from
// any OpenTelemetry span attributes it produces. Mirrors the scrub list in
// the middleware itself.
var sensitiveHeaders = []string{
	"Authorization",
	"Cookie",
	"Set-Cookie",
	"X-Api-Key",
	"X-Csrf-Token",
	"X-Auth-Token",
	"Proxy-Authorization",
}

// setupRecorderTracer installs an in-memory span recorder as the global
// TracerProvider and returns the recorder plus a cleanup function that
// restores the previous provider. Any spans ended during the test are
// captured synchronously.
func setupRecorderTracer(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	sr := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		otel.SetTracerProvider(prev)
	})
	return sr
}

// assertionT is the minimal subset of *testing.T the scrub assertion
// helper uses. Defining an interface lets the meta-test
// (TestAssertNoSensitiveValues_FailsOnLeak) stub the failure channel so
// the helper itself can be unit-tested for correctness.
type assertionT interface {
	Helper()
	Errorf(format string, args ...any)
	Fatal(args ...any)
}

// assertNoSensitiveValues walks every ended span and fails the test if any
// attribute value contains one of the known-sensitive placeholder strings.
// This catches leaks even if the attribute key name changes between otelgin
// versions (e.g. http.request.header.authorization vs http.header.auth).
func assertNoSensitiveValues(t assertionT, sr *tracetest.SpanRecorder) {
	t.Helper()
	spans := sr.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one ended span, got zero — middleware did not produce a span")
	}
	for _, s := range spans {
		for _, kv := range s.Attributes() {
			key := string(kv.Key)
			val := kv.Value.Emit()
			// Check value never contains a sensitive placeholder.
			for _, bad := range sensitiveHeaderValues {
				if strings.Contains(val, bad) {
					t.Errorf("span %q attribute %q leaked sensitive value %q (full value=%q)",
						s.Name(), key, bad, val)
				}
			}
			// Attribute keys that name sensitive headers should also not be present,
			// even if redacted — cleaner to drop entirely so nothing downstream
			// surfaces the header name.
			lowerKey := strings.ToLower(key)
			for _, h := range sensitiveHeaders {
				needle := strings.ToLower(h)
				if strings.Contains(lowerKey, needle) {
					t.Errorf("span %q retained sensitive header key %q (value=%q) — should be dropped",
						s.Name(), key, val)
				}
			}
		}
	}
}

// TestTracingMiddleware_ScrubsSensitiveRequestHeaders fires a request through
// the middleware with every header on the scrub list populated with obvious
// placeholder values. The assertion is that no span attribute contains the
// placeholder value and no attribute key names a sensitive header.
func TestTracingMiddleware_ScrubsSensitiveRequestHeaders(t *testing.T) {
	sr := setupRecorderTracer(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TracingMiddleware("cmdb-core-test"))
	router.GET("/api/v1/assets", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/assets", nil)
	req.Header.Set("Authorization", "Bearer test-secret-do-not-use")
	req.Header.Set("Cookie", "session=test-cookie-do-not-use")
	req.Header.Set("X-Api-Key", "test-apikey-do-not-use")
	req.Header.Set("X-Csrf-Token", "test-csrf-do-not-use")
	req.Header.Set("X-Auth-Token", "test-auth-token-do-not-use")
	req.Header.Set("Proxy-Authorization", "Basic proxy-test-do-not-use")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	assertNoSensitiveValues(t, sr)
}

// TestTracingMiddleware_ScrubsSensitiveResponseHeaders ensures Set-Cookie in
// the response does not leak into span attributes either.
func TestTracingMiddleware_ScrubsSensitiveResponseHeaders(t *testing.T) {
	sr := setupRecorderTracer(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TracingMiddleware("cmdb-core-test"))
	router.GET("/login", func(c *gin.Context) {
		c.Header("Set-Cookie", "session=test-cookie-do-not-use; Path=/")
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assertNoSensitiveValues(t, sr)
}

// TestTracingMiddleware_PreservesBenignHeaders confirms the scrub list is not
// overbroad: a non-sensitive header like User-Agent must still be allowed to
// appear in span attributes if the underlying otel library chooses to record
// it. We only assert the request succeeded and a span was produced — we do
// not require User-Agent to be present, just that scrubbing did not break
// the happy path.
func TestTracingMiddleware_PreservesBenignHeaders(t *testing.T) {
	sr := setupRecorderTracer(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TracingMiddleware("cmdb-core-test"))
	router.GET("/health", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("User-Agent", "cmdb-test/1.0")
	req.Header.Set("Accept", "text/plain")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if len(sr.Ended()) == 0 {
		t.Fatal("expected at least one span from middleware")
	}
}

// TestAssertNoSensitiveValues_FailsOnLeak is a meta-test: it confirms the
// assertion helper actually fails when a span carries a sensitive attribute
// key or value. Without this, a silently-passing test suite could mean
// either "no leak" or "assertion is broken". We construct a span by hand,
// set a sensitive attribute, and run the helper against a stub testing.T.
func TestAssertNoSensitiveValues_FailsOnLeak(t *testing.T) {
	sr := setupRecorderTracer(t)

	// Produce a span with a sensitive header attribute directly, as if
	// a hypothetical future contrib upgrade had started recording it.
	tracer := otel.Tracer("cmdb-core-leak-meta-test")
	_, span := tracer.Start(context.Background(), "leaky-span")
	span.SetAttributes(
		attribute.String("http.request.header.authorization", "Bearer test-secret-do-not-use"),
	)
	span.End()

	stub := &fakeTestingT{}
	assertNoSensitiveValues(stub, sr)
	if !stub.failed {
		t.Fatal("assertNoSensitiveValues did not flag a known leak — assertion is broken")
	}
}

// fakeTestingT satisfies the assertionT interface. It captures whether
// any Errorf / Fatal call fired without failing the outer real test.
type fakeTestingT struct {
	failed bool
}

func (f *fakeTestingT) Errorf(format string, args ...any) { f.failed = true }
func (f *fakeTestingT) Fatal(args ...any)                 { f.failed = true }
func (f *fakeTestingT) Helper()                           {}

// TestTracingMiddleware_DocumentsRecordedAttributes is a snapshot-style test:
// it lists every attribute the default otelgin span records so a future
// contrib upgrade that starts recording headers cannot ship unnoticed. It
// does NOT assert a fixed set — it only asserts no key matches the known
// sensitive-header patterns from scrubSensitiveHeaderAttribute. Acts as a
// regression guard paired with the scrub helper.
func TestTracingMiddleware_DocumentsRecordedAttributes(t *testing.T) {
	sr := setupRecorderTracer(t)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(TracingMiddleware("cmdb-core-test"))
	router.GET("/api/v1/audit", func(c *gin.Context) { c.String(http.StatusOK, "ok") })

	req := httptest.NewRequest(http.MethodGet, "/api/v1/audit", nil)
	req.Header.Set("Authorization", "Bearer test-secret-do-not-use")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	spans := sr.Ended()
	if len(spans) == 0 {
		t.Fatal("expected at least one span")
	}
	for _, s := range spans {
		for _, kv := range s.Attributes() {
			if isSensitiveHeaderAttribute(string(kv.Key)) {
				t.Errorf("span %q retained sensitive attribute key %q after scrub",
					s.Name(), kv.Key)
			}
		}
	}
}
