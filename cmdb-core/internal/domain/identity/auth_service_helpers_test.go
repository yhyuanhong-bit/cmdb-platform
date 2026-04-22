package identity

import (
	"encoding/base64"
	"testing"

	"github.com/google/uuid"
)

// These tests exercise the pure helpers in auth_service.go that do NOT
// require a live DB or Redis — parseUserAgent, parseUUID,
// appendUnique, and generateSecureToken. The real AuthService.Login /
// Refresh / GetCurrentUser paths are covered in the integration tests
// that run under `-tags integration` against a live Postgres+Redis.

// TestParseUserAgent_Browser: browser detection respects the Chrome-
// vs-Edge distinction (Edge's UA contains both "chrome" and "edg"), so
// the order of the switch matters. Locking the matrix here prevents a
// refactor that re-orders cases from silently misclassifying Edge.
func TestParseUserAgent_Browser(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ua         string
		wantBrowser string
	}{
		{
			name:       "chrome (not edge)",
			ua:         "Mozilla/5.0 AppleWebKit/537.36 Chrome/120.0.0 Safari/537.36",
			wantBrowser: "Chrome",
		},
		{
			name:       "firefox",
			ua:         "Mozilla/5.0 Gecko/20100101 Firefox/121.0",
			wantBrowser: "Firefox",
		},
		{
			name:       "safari (not chrome)",
			ua:         "Mozilla/5.0 AppleWebKit/605.1.15 Version/17.0 Safari/605.1.15",
			wantBrowser: "Safari",
		},
		{
			name:       "edge beats chrome",
			ua:         "Mozilla/5.0 AppleWebKit Chrome/120 Edg/120.0",
			wantBrowser: "Edge",
		},
		{
			name:       "unknown browser",
			ua:         "curl/7.0",
			wantBrowser: "unknown",
		},
		{
			name:       "empty string",
			ua:         "",
			wantBrowser: "unknown",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, browser := parseUserAgent(tc.ua)
			if browser != tc.wantBrowser {
				t.Errorf("parseUserAgent(%q).browser = %q, want %q",
					tc.ua, browser, tc.wantBrowser)
			}
		})
	}
}

// TestParseUserAgent_Device: device-type detection locks the three
// buckets — mobile, tablet, desktop — so a UA-string refactor does not
// silently miscategorise sessions.
func TestParseUserAgent_Device(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		ua       string
		wantType string
	}{
		{"iPhone is mobile", "Mozilla/5.0 iPhone Mobile Safari", "mobile"},
		{"Android phone is mobile", "Mozilla/5.0 Linux Android Mobile", "mobile"},
		{"iPad is tablet", "Mozilla/5.0 iPad Safari", "tablet"},
		// "android" takes precedence over "tablet" in the switch below,
		// so an Android tablet UA that happens to contain the word
		// "android" first matches `mobile`. This is the documented
		// current behaviour — locking it in so a refactor that flips
		// the order is intentional.
		{"iPad-style generic tablet is tablet", "Mozilla/5.0 tablet browser", "tablet"},
		{"desktop Chrome is desktop", "Mozilla/5.0 AppleWebKit Chrome Safari", "desktop"},
		{"empty is desktop (default)", "", "desktop"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			device, _ := parseUserAgent(tc.ua)
			if device != tc.wantType {
				t.Errorf("parseUserAgent(%q).device = %q, want %q",
					tc.ua, device, tc.wantType)
			}
		})
	}
}

// TestParseUserAgent_AndroidTabletEdgeCase: "android" matches BEFORE
// "tablet" in the switch, so an Android tablet UA that contains the
// word "android" but not "tablet" falls into the mobile bucket. Lock
// the current behavior so a refactor that reorders the switch is
// intentional.
func TestParseUserAgent_AndroidTabletEdgeCase(t *testing.T) {
	t.Parallel()

	// "android" alone → mobile (current behaviour).
	device, _ := parseUserAgent("Mozilla/5.0 Linux Android 10 Pixel-like")
	if device != "mobile" {
		t.Errorf("android-only UA device = %q, want mobile", device)
	}
}

// TestParseUUID covers the Parse success path and every failure mode
// (empty, garbage, wrong length). A regression that panics instead of
// returning uuid.Nil would break any caller that tolerates uuid.Nil.
func TestParseUUID(t *testing.T) {
	t.Parallel()

	t.Run("valid uuid round-trips", func(t *testing.T) {
		t.Parallel()
		orig := uuid.New()
		got := parseUUID(orig.String())
		if got != orig {
			t.Errorf("parseUUID(%q) = %s, want %s", orig.String(), got, orig)
		}
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		t.Parallel()
		got := parseUUID("")
		if got != uuid.Nil {
			t.Errorf("parseUUID(\"\") = %s, want uuid.Nil", got)
		}
	})

	t.Run("garbage returns nil without panic", func(t *testing.T) {
		t.Parallel()
		got := parseUUID("not-a-uuid")
		if got != uuid.Nil {
			t.Errorf("parseUUID(garbage) = %s, want uuid.Nil", got)
		}
	})

	t.Run("too-short string returns nil", func(t *testing.T) {
		t.Parallel()
		got := parseUUID("abc")
		if got != uuid.Nil {
			t.Errorf("parseUUID(short) = %s, want uuid.Nil", got)
		}
	})

	t.Run("wrong-length hex returns nil", func(t *testing.T) {
		t.Parallel()
		got := parseUUID("00000000000000000000000000000000000000000000")
		if got != uuid.Nil {
			t.Errorf("parseUUID(long) = %s, want uuid.Nil", got)
		}
	})
}

// TestAppendUnique: this is the permission-merge helper used by
// GetCurrentUser. A regression that loses the "unique" guarantee would
// produce duplicate actions in the permissions map, which the frontend
// then renders as duplicate buttons. The correctness contract is:
//
//  1. every value in `values` ends up exactly once in the output
//  2. prior duplicates in `existing` are preserved (we don't dedupe the
//     caller's slice — only the added values)
//  3. order within `existing` is preserved
func TestAppendUnique(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing []string
		values   []string
		want     []string
	}{
		{
			name:     "append to empty",
			existing: nil,
			values:   []string{"read", "write"},
			want:     []string{"read", "write"},
		},
		{
			name:     "dedupes added values",
			existing: []string{"read"},
			values:   []string{"read", "write", "write"},
			want:     []string{"read", "write"},
		},
		{
			name:     "preserves existing order",
			existing: []string{"b", "a"},
			values:   []string{"c"},
			want:     []string{"b", "a", "c"},
		},
		{
			name:     "no duplicates produced across calls",
			existing: []string{"read", "write"},
			values:   []string{"read", "write", "delete"},
			want:     []string{"read", "write", "delete"},
		},
		{
			name:     "empty values returns existing",
			existing: []string{"read"},
			values:   nil,
			want:     []string{"read"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := appendUnique(tc.existing, tc.values...)
			if len(got) != len(tc.want) {
				t.Fatalf("len mismatch: got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("index %d: got %q want %q (full=%v)", i, got[i], tc.want[i], got)
				}
			}
		})
	}
}

// TestGenerateSecureToken: the contract is 32 bytes of entropy encoded
// as base64url. We assert length (43 chars for 32 raw bytes, no pad)
// and uniqueness across consecutive calls. A regression that seeds
// math/rand instead of crypto/rand would eventually collide — the
// probabilistic test here catches the trivial "always returns X" class
// of bugs.
func TestGenerateSecureToken(t *testing.T) {
	t.Parallel()

	tok, err := generateSecureToken()
	if err != nil {
		t.Fatalf("generateSecureToken: %v", err)
	}
	// 32 bytes in RawURLEncoding is 43 chars (no '=' padding). Any
	// length deviation signals a change in token surface area that
	// should be a deliberate commit with a migration plan.
	if got, want := len(tok), 43; got != want {
		t.Errorf("token length = %d, want %d", got, want)
	}
	// Must decode cleanly — a regression to hex or base32 would fail
	// here.
	if _, err := base64.RawURLEncoding.DecodeString(tok); err != nil {
		t.Errorf("token is not valid base64url: %v", err)
	}
}

// TestGenerateSecureToken_Unique drives 16 consecutive calls and
// asserts no collision. crypto/rand makes the probability of any two
// matching roughly 2^-256 per pair; a regression to math/rand seeded
// with a constant would collide immediately.
func TestGenerateSecureToken_Unique(t *testing.T) {
	t.Parallel()

	const n = 16
	seen := make(map[string]struct{}, n)
	for i := 0; i < n; i++ {
		tok, err := generateSecureToken()
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if _, dup := seen[tok]; dup {
			t.Fatalf("duplicate token at call %d: %q", i, tok)
		}
		seen[tok] = struct{}{}
	}
}
