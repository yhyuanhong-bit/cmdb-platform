package workflows

import (
	"strings"

	"go.uber.org/zap"
	"golang.org/x/mod/semver"
)

// compareFirmwareVersion returns -1, 0, 1 like strings.Compare, but for
// firmware version strings. Accepts bare "1.2.3" or "v"-prefixed
// "v1.2.3" — both are normalized before comparison.
//
// When either operand is not valid semver (e.g. vendor build strings
// like "build-abc" or "BMC-3.7.1.2024-10-03"), we fall back to
// lexicographic comparison and emit a warn log so the fallback remains
// observable. Warn (not error) because many hardware vendors publish
// non-semver firmware tags and we don't want to spam.
func compareFirmwareVersion(a, b string) int {
	na := normalizeSemver(a)
	nb := normalizeSemver(b)
	if semver.IsValid(na) && semver.IsValid(nb) {
		return semver.Compare(na, nb)
	}
	zap.L().Warn("non-semver firmware version; falling back to lexicographic",
		zap.String("a", a), zap.String("b", b))
	return strings.Compare(a, b)
}

// normalizeSemver trims whitespace and prepends "v" so the string is in
// the form expected by golang.org/x/mod/semver. An empty string stays
// empty (IsValid will reject it and callers fall back to lex compare).
func normalizeSemver(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

// maxFirmwareVersion returns the greatest version among the inputs
// using semver ordering (with lex fallback per pair). Returns "" for
// an empty input. Ties keep the first occurrence.
func maxFirmwareVersion(versions []string) string {
	if len(versions) == 0 {
		return ""
	}
	best := versions[0]
	for _, v := range versions[1:] {
		if compareFirmwareVersion(v, best) > 0 {
			best = v
		}
	}
	return best
}
