package telemetry

import "testing"

func TestIsSensitiveHeader(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"exact lowercase authorization", "authorization", true},
		{"canonical Authorization", "Authorization", true},
		{"mixed case aUtHoRiZaTiOn", "aUtHoRiZaTiOn", true},
		{"leading/trailing whitespace", "  Authorization  ", true},
		{"cookie", "Cookie", true},
		{"set-cookie", "Set-Cookie", true},
		{"x-api-key", "X-Api-Key", true},
		{"x-csrf-token", "X-Csrf-Token", true},
		{"x-auth-token", "X-Auth-Token", true},
		{"proxy-authorization", "Proxy-Authorization", true},
		{"benign content-type", "Content-Type", false},
		{"benign user-agent", "User-Agent", false},
		{"benign accept", "Accept", false},
		{"empty", "", false},
		{"partial match authorization-extra", "authorization-extra", false},
		{"unrelated x-request-id", "X-Request-Id", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsSensitiveHeader(tc.in); got != tc.want {
				t.Errorf("IsSensitiveHeader(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestIsSensitiveHeaderAttribute(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"bare authorization", "authorization", true},
		{"semconv request header form", "http.request.header.authorization", true},
		{"semconv response header form", "http.response.header.set-cookie", true},
		{"mixed case attribute", "HTTP.Request.Header.Authorization", true},
		{"cookie bare", "cookie", true},
		{"x-api-key semconv", "http.request.header.x-api-key", true},
		{"benign http.method", "http.method", false},
		{"benign http.route", "http.route", false},
		{"benign user-agent attribute", "http.user_agent", false},
		{"empty", "", false},
		// Edge: ensure a non-header attribute that happens to contain
		// "authorization" as a substring without the required `.`
		// prefix is NOT matched, to avoid false positives.
		{"substring without dot prefix", "myauthorization", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSensitiveHeaderAttribute(tc.in); got != tc.want {
				t.Errorf("isSensitiveHeaderAttribute(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestSensitiveRequestHeadersContainsExpectedEntries(t *testing.T) {
	// The scrub list is a security contract — any accidental removal
	// would silently allow credentials into span attributes. Pin the
	// expected entries with an explicit set check rather than a length
	// count so diffs stay readable.
	expected := map[string]bool{
		"authorization":       false,
		"cookie":              false,
		"set-cookie":          false,
		"x-api-key":           false,
		"x-csrf-token":        false,
		"x-auth-token":        false,
		"proxy-authorization": false,
	}
	for _, h := range SensitiveRequestHeaders {
		if _, ok := expected[h]; !ok {
			t.Errorf("unexpected header %q in SensitiveRequestHeaders — update the expected set if intentional", h)
			continue
		}
		expected[h] = true
	}
	for h, seen := range expected {
		if !seen {
			t.Errorf("required sensitive header %q missing from SensitiveRequestHeaders", h)
		}
	}
}
