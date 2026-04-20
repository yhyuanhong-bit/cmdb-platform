package telemetry

import "strings"

// SensitiveRequestHeaders is the canonical list of HTTP headers that must
// never land in OpenTelemetry span attributes. Any new header carrying
// credentials, session state, or CSRF tokens belongs here.
//
// Keep the list in lower-case — HTTP header names are case-insensitive and
// the matching helpers below lower-case their inputs before comparing.
//
// Consumers:
//   - tracing_test.go asserts that no span produced by TracingMiddleware
//     records an attribute whose key or value matches any entry here.
//   - Phase 4.6 Step 7 (outbound otelhttp.NewTransport wiring) will pass
//     this list to otelhttp.WithSpanAttributesFn / otelhttp.WithFilter so
//     the client-side span does not capture credentials on outbound calls.
var SensitiveRequestHeaders = []string{
	"authorization",
	"cookie",
	"set-cookie",
	"x-api-key",
	"x-csrf-token",
	"x-auth-token",
	"proxy-authorization",
}

// IsSensitiveHeader reports whether the given HTTP header name matches one
// of the entries in SensitiveRequestHeaders. The comparison is case- and
// trim-insensitive to survive arbitrary client capitalisation.
func IsSensitiveHeader(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, h := range SensitiveRequestHeaders {
		if n == h {
			return true
		}
	}
	return false
}

// isSensitiveHeaderAttribute reports whether an OpenTelemetry span attribute
// key names one of the sensitive headers. OTel semconv encodes headers as
// `http.request.header.<name>` or `http.response.header.<name>`, so we match
// any attribute key ending in `.<sensitive-header>` as well as a bare
// header name. Used by the regression test to guard against a future
// otel-contrib upgrade silently starting to record request headers.
func isSensitiveHeaderAttribute(key string) bool {
	k := strings.ToLower(key)
	for _, h := range SensitiveRequestHeaders {
		if strings.HasSuffix(k, "."+h) || k == h {
			return true
		}
	}
	return false
}
