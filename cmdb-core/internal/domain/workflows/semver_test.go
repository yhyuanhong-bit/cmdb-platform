package workflows

import "testing"

// TestCompareFirmwareVersion is the regression guard for the
// lexicographic firmware bug: under strings.Compare, "1.10.0" sorts
// BEFORE "1.9.0" because '1' < '9' byte-wise. The semver-aware
// comparator must correctly report 1.10.0 > 1.9.0.
func TestCompareFirmwareVersion(t *testing.T) {
	cases := []struct {
		name, a, b string
		want       int
	}{
		{"semver equal", "1.2.3", "1.2.3", 0},
		{"semver minor", "1.10.0", "1.9.0", 1}, // regression case
		{"semver major", "2.0.0", "1.99.99", 1},
		{"semver prerelease", "1.2.3-rc1", "1.2.3", -1},
		{"v-prefix normalizes", "v1.2.3", "1.2.3", 0},
		{"non-semver falls back lex", "build-abc", "build-abd", -1},
		{"empty vs semver is non-semver", "", "1.0.0", -1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compareFirmwareVersion(tc.a, tc.b)
			if got != tc.want {
				t.Fatalf("compareFirmwareVersion(%q, %q) = %d, want %d",
					tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestCompareFirmwareVersion_Symmetry(t *testing.T) {
	// If a > b then b < a, and compare(x, x) == 0 — invariants we
	// rely on in maxFirmwareVersion.
	pairs := [][2]string{
		{"1.10.0", "1.9.0"},
		{"2.0.0", "1.99.99"},
		{"v1.2.3", "1.2.2"},
	}
	for _, p := range pairs {
		ab := compareFirmwareVersion(p[0], p[1])
		ba := compareFirmwareVersion(p[1], p[0])
		if ab != -ba {
			t.Fatalf("asymmetric: cmp(%q,%q)=%d cmp(%q,%q)=%d",
				p[0], p[1], ab, p[1], p[0], ba)
		}
		if compareFirmwareVersion(p[0], p[0]) != 0 {
			t.Fatalf("cmp(x,x) != 0 for %q", p[0])
		}
	}
}

func TestNormalizeSemver(t *testing.T) {
	cases := []struct {
		in, out string
	}{
		{"1.2.3", "v1.2.3"},
		{"v1.2.3", "v1.2.3"},
		{"  1.2.3  ", "v1.2.3"},
		{"", ""},
		{"v", "v"},
	}
	for _, tc := range cases {
		if got := normalizeSemver(tc.in); got != tc.out {
			t.Fatalf("normalizeSemver(%q) = %q, want %q", tc.in, got, tc.out)
		}
	}
}

// TestMaxFirmwareVersion is the behavior the SQL MAX() cannot express.
// 1.10.0 must win over 1.9.0 when we compute the "latest" per bmc_type.
func TestMaxFirmwareVersion(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want string
	}{
		{"empty", nil, ""},
		{"single", []string{"1.2.3"}, "1.2.3"},
		{"semver double-digit minor beats single-digit", []string{"1.9.0", "1.10.0", "1.2.0"}, "1.10.0"},
		{"major wins", []string{"1.99.99", "2.0.0", "1.10.0"}, "2.0.0"},
		{"prerelease loses to release", []string{"1.2.3-rc1", "1.2.3"}, "1.2.3"},
		{"mixed v-prefix", []string{"v1.2.3", "1.10.0"}, "1.10.0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := maxFirmwareVersion(tc.in)
			if got != tc.want {
				t.Fatalf("maxFirmwareVersion(%v) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
