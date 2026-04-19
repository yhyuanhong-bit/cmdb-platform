package netguard

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func mustGuard(t *testing.T, allow []string) *Guard {
	t.Helper()
	g, err := New(nil, allow)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return g
}

func TestNew_InvalidCIDR(t *testing.T) {
	_, err := New([]string{"not-a-cidr"}, nil)
	if err == nil {
		t.Fatal("expected error for invalid CIDR, got nil")
	}
}

func TestValidateURL_RejectsMetadataIP(t *testing.T) {
	g := mustGuard(t, nil)
	err := g.ValidateURL("http://169.254.169.254/latest/meta-data/")
	if err == nil || !strings.Contains(err.Error(), "blocked network") {
		t.Fatalf("expected blocked network error, got %v", err)
	}
}

func TestValidateURL_RejectsRFC1918(t *testing.T) {
	g := mustGuard(t, nil)
	cases := []string{
		"http://10.0.0.1/",
		"http://192.168.1.1/",
		"http://172.17.0.1/",
	}
	for _, u := range cases {
		t.Run(u, func(t *testing.T) {
			err := g.ValidateURL(u)
			if err == nil || !strings.Contains(err.Error(), "blocked") {
				t.Fatalf("expected blocked error for %s, got %v", u, err)
			}
		})
	}
}

func TestValidateURL_RejectsLoopback(t *testing.T) {
	g := mustGuard(t, nil)
	// IP literal — no DNS needed.
	if err := g.ValidateURL("http://127.0.0.1/"); err == nil {
		t.Fatal("expected loopback IP to be blocked")
	}
	// Hostname "localhost" typically resolves to 127.0.0.1 or ::1 — both are
	// in the blocklist, so this must error regardless of resolver ordering.
	if err := g.ValidateURL("http://localhost/"); err == nil {
		t.Fatal("expected localhost to be blocked")
	}
}

func TestValidateURL_RejectsIPv6Loopback(t *testing.T) {
	g := mustGuard(t, nil)
	if err := g.ValidateURL("http://[::1]/"); err == nil {
		t.Fatal("expected IPv6 loopback to be blocked")
	}
}

func TestValidateURL_RejectsNonHTTP(t *testing.T) {
	g := mustGuard(t, nil)
	cases := []string{
		"file:///etc/passwd",
		"ftp://example.com/",
		"gopher://evil.example.com/",
		"javascript:alert(1)",
	}
	for _, u := range cases {
		t.Run(u, func(t *testing.T) {
			err := g.ValidateURL(u)
			if err == nil || !strings.Contains(err.Error(), "scheme") {
				t.Fatalf("expected scheme error for %s, got %v", u, err)
			}
		})
	}
}

func TestValidateURL_RejectsEmptyHost(t *testing.T) {
	g := mustGuard(t, nil)
	err := g.ValidateURL("http:///foo")
	if err == nil || !strings.Contains(err.Error(), "hostname") {
		t.Fatalf("expected no-hostname error, got %v", err)
	}
}

func TestValidateURL_AllowsPublicIP(t *testing.T) {
	// Use a public IP literal to avoid DNS flakiness. 1.1.1.1 is not in any
	// blocked CIDR.
	g := mustGuard(t, nil)
	if err := g.ValidateURL("https://1.1.1.1/"); err != nil {
		t.Fatalf("expected public IP to pass, got %v", err)
	}
}

func TestValidateURL_AllowHostOverride(t *testing.T) {
	g := mustGuard(t, []string{"localhost"})
	if err := g.ValidateURL("http://localhost/"); err != nil {
		t.Fatalf("allowlisted host should pass, got %v", err)
	}
}

func TestValidateURL_AllowHostCaseInsensitive(t *testing.T) {
	g := mustGuard(t, []string{"LocalHost"})
	if err := g.ValidateURL("http://LOCALHOST/"); err != nil {
		t.Fatalf("allowlist must be case-insensitive, got %v", err)
	}
}

func TestCheckHost_IPLiteral(t *testing.T) {
	g := mustGuard(t, nil)
	if err := g.CheckHost("169.254.169.254"); err == nil {
		t.Fatal("expected metadata IP to be blocked at CheckHost")
	}
	if err := g.CheckHost("10.0.0.5"); err == nil {
		t.Fatal("expected RFC1918 IP to be blocked at CheckHost")
	}
	// Public IP literal
	if err := g.CheckHost("1.1.1.1"); err != nil {
		t.Fatalf("public IP should pass CheckHost, got %v", err)
	}
}

func TestCheckHost_AllowOverride(t *testing.T) {
	g := mustGuard(t, []string{"127.0.0.1"})
	if err := g.CheckHost("127.0.0.1"); err != nil {
		t.Fatalf("allowlisted IP-as-host should pass, got %v", err)
	}
}

func TestSafeTransport_BlocksOnDial(t *testing.T) {
	g := mustGuard(t, nil)
	client := &http.Client{
		Transport: g.SafeTransport(nil),
		Timeout:   2 * time.Second,
	}
	// Port 9 (discard) — we expect the dial to be rejected *before* it ever
	// reaches the kernel, so the error comes from the guard, not from a
	// connection-refused.
	resp, err := client.Get("http://127.0.0.1:9/")
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected SafeTransport to block dial to loopback")
	}
	if !strings.Contains(err.Error(), "blocked network") {
		t.Fatalf("expected blocked network error, got %v", err)
	}
}

func TestSafeTransport_BlocksMetadataIPDial(t *testing.T) {
	g := mustGuard(t, nil)
	client := &http.Client{
		Transport: g.SafeTransport(nil),
		Timeout:   2 * time.Second,
	}
	resp, err := client.Get("http://169.254.169.254/latest/meta-data/")
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected SafeTransport to block AWS metadata IP")
	}
	if !strings.Contains(err.Error(), "blocked network") {
		t.Fatalf("expected blocked network error, got %v", err)
	}
}

func TestPermissive_DoesNotBlock(t *testing.T) {
	// Permissive guard allows anything — used only in tests.
	g := Permissive()
	if err := g.ValidateURL("http://127.0.0.1/"); err != nil {
		t.Fatalf("Permissive guard must not block, got %v", err)
	}
	if err := g.CheckHost("10.0.0.1"); err != nil {
		t.Fatalf("Permissive CheckHost must not block, got %v", err)
	}
}

func TestValidateURL_RejectsUnparseable(t *testing.T) {
	g := mustGuard(t, nil)
	// An input containing a control character fails url.Parse.
	err := g.ValidateURL("http://exa\x00mple.com/")
	if err == nil {
		t.Fatal("expected parse error for malformed URL")
	}
}
