package service

import "testing"

// Pure-function tests for the service domain. These exercise the validation
// surface that handlers depend on without needing a live database — the
// integration_test.go file already covers the SQL paths under the
// `integration` build tag.

func TestIsValidCode(t *testing.T) {
	cases := []struct {
		name string
		code string
		want bool
	}{
		// Q1 sign-off requires uppercase letter prefix, then 1-63 of [A-Z0-9_-].
		{"min_length_two", "AB", true},
		{"underscore", "ORDER_API", true},
		{"hyphen", "ORDER-API", true},
		{"digits_after_letter", "API2", true},
		{"max_64_chars", "A" + repeat("X", 63), true},

		// Rejections — anything that would let a typo into the BIA / k8s
		// pipeline. Must match the DB CHECK so we can give callers a
		// well-typed error before the round-trip.
		{"empty", "", false},
		{"single_char", "A", false},
		{"lowercase_prefix", "order-api", false},
		{"digit_prefix", "1ORDER", false},
		{"hyphen_prefix", "-API", false},
		{"underscore_prefix", "_API", false},
		{"space", "ORDER API", false},
		{"unicode", "OŔDER", false},
		{"too_long_65", "A" + repeat("X", 64), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsValidCode(tc.code); got != tc.want {
				t.Errorf("IsValidCode(%q) = %v, want %v", tc.code, got, tc.want)
			}
		})
	}
}

func TestValidTiersMap(t *testing.T) {
	// Tier constants must round-trip through validTiers — a typo in either
	// the constant or the map would silently break Create/Update validation.
	for _, tier := range []string{TierCritical, TierImportant, TierNormal, TierLow, TierMinor} {
		if !validTiers[tier] {
			t.Errorf("tier %q expected valid", tier)
		}
	}
	for _, bad := range []string{"", "Critical", "high", "tier1"} {
		if validTiers[bad] {
			t.Errorf("tier %q expected invalid", bad)
		}
	}
}

func TestValidStatusesMap(t *testing.T) {
	for _, status := range []string{StatusActive, StatusDeprecated, StatusDecommissioned} {
		if !validStatuses[status] {
			t.Errorf("status %q expected valid", status)
		}
	}
	for _, bad := range []string{"", "ACTIVE", "retired", "draft"} {
		if validStatuses[bad] {
			t.Errorf("status %q expected invalid", bad)
		}
	}
}

func TestValidRolesMap(t *testing.T) {
	// Q3 sign-off locks the 7-value role enum. New roles require a spec
	// revision; this test fails first if someone sneaks one in.
	want := []string{RolePrimary, RoleReplica, RoleCache, RoleProxy, RoleStorage, RoleDependency, RoleComponent}
	for _, r := range want {
		if !validRoles[r] {
			t.Errorf("role %q expected valid", r)
		}
	}
	if len(validRoles) != len(want) {
		t.Errorf("validRoles size drifted: got %d, want %d", len(validRoles), len(want))
	}
	for _, bad := range []string{"", "MASTER", "slave", "controller"} {
		if validRoles[bad] {
			t.Errorf("role %q expected invalid", bad)
		}
	}
}

func TestHelpers(t *testing.T) {
	t.Run("textOrNull_empty", func(t *testing.T) {
		if got := textOrNull(""); got.Valid {
			t.Errorf("expected invalid pgtype.Text for empty string")
		}
	})
	t.Run("textOrNull_value", func(t *testing.T) {
		got := textOrNull("hello")
		if !got.Valid || got.String != "hello" {
			t.Errorf("expected valid pgtype.Text{hello}, got %+v", got)
		}
	})
	t.Run("ensureTags_nil_returns_empty_slice", func(t *testing.T) {
		// Postgres TEXT[] NOT NULL requires a non-nil slice; nil would
		// fail the insert with a NULL violation.
		got := ensureTags(nil)
		if got == nil || len(got) != 0 {
			t.Errorf("expected empty non-nil slice, got %#v", got)
		}
	})
	t.Run("strOrEmpty", func(t *testing.T) {
		if strOrEmpty(nil) != "" {
			t.Errorf("nil should return empty string")
		}
		s := "active"
		if strOrEmpty(&s) != "active" {
			t.Errorf("pointer deref failed")
		}
	})
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(&fakeErr{"ERROR: duplicate key value (SQLSTATE 23505)"}) {
		t.Error("expected SQLSTATE 23505 to be detected as unique violation")
	}
	if !isUniqueViolation(&fakeErr{"duplicate key value violates unique constraint"}) {
		t.Error("expected 'duplicate key value' message to be detected")
	}
	if isUniqueViolation(&fakeErr{"connection refused"}) {
		t.Error("unrelated error should not be a unique violation")
	}
	if isUniqueViolation(nil) {
		t.Error("nil error should not be a unique violation")
	}
}

// repeat avoids importing strings just for one test helper.
func repeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

// fakeErr is a minimal error type that lets us simulate driver error
// messages without taking on a pgx test-double dependency.
type fakeErr struct{ s string }

func (e *fakeErr) Error() string { return e.s }
