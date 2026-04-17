package crypto

import (
	"fmt"
	"os"
	"sort"
	"strconv"
)

// versionedKeyEnvPrefix is the prefix for per-version key env vars.
// A key at version N is loaded from CMDB_SECRET_KEY_V{N} (hex-encoded 32
// bytes, same format as the legacy single-key CMDB_SECRET_KEY).
const versionedKeyEnvPrefix = "CMDB_SECRET_KEY_V"

// activeVersionEnv is the env var naming which version the KeyRing uses for
// Encrypt. If unset, the highest configured version is chosen (callers
// upgrading to a new v{N} who forget to flip this still benefit from the new
// key once the server is restarted).
const activeVersionEnv = "CMDB_SECRET_KEY_ACTIVE"

// legacyKeyEnv is the pre-rotation single-key env var. When neither
// CMDB_SECRET_KEY_V{N} nor CMDB_SECRET_KEY_ACTIVE is set, CMDB_SECRET_KEY is
// treated as if it were CMDB_SECRET_KEY_V1. This is the zero-config upgrade
// path: existing deployments don't break on pull.
const legacyKeyEnv = "CMDB_SECRET_KEY"

// maxScannedVersion caps how many CMDB_SECRET_KEY_V{N} slots KeyRingFromEnv
// will probe. The limit is generous (far beyond any realistic rotation
// cadence) and keeps startup O(1).
const maxScannedVersion = 32

// versionPrefixSep is the byte separating the ASCII version tag from the raw
// AEAD output. A ciphertext with the prefix looks like:
//
//	v{N}:{nonce||sealed_ciphertext_with_tag}
//
// Legacy ciphertext written before KeyRing shipped has no prefix and is
// routed to v1 on decrypt.
const versionPrefixSep = ':'

// KeyRing holds one or more Ciphers keyed by integer version. Encrypts using
// the active version; decrypts by reading the version prefix off the
// ciphertext and dispatching to that version's Cipher.
//
// KeyRing implements the Cipher interface, so it is a drop-in replacement at
// every call site that previously held a single Cipher.
//
// KeyRing is safe for concurrent use once constructed.
type KeyRing struct {
	ciphers       map[int]Cipher
	activeVersion int
}

// NewKeyRing builds a KeyRing from an already-constructed map of version
// to Cipher. The active version must exist in the map. Returns an error on
// empty input or an active version that isn't present.
func NewKeyRing(ciphers map[int]Cipher, active int) (*KeyRing, error) {
	if len(ciphers) == 0 {
		return nil, fmt.Errorf("crypto: KeyRing requires at least one key")
	}
	if _, ok := ciphers[active]; !ok {
		return nil, fmt.Errorf("crypto: active version %d has no configured key", active)
	}
	// Copy so external mutations can't race with in-flight Encrypt/Decrypt.
	copied := make(map[int]Cipher, len(ciphers))
	for k, v := range ciphers {
		if k <= 0 {
			return nil, fmt.Errorf("crypto: key version must be positive, got %d", k)
		}
		if v == nil {
			return nil, fmt.Errorf("crypto: key version %d has nil Cipher", k)
		}
		copied[k] = v
	}
	return &KeyRing{ciphers: copied, activeVersion: active}, nil
}

// ActiveVersion returns the integer version used by Encrypt.
func (k *KeyRing) ActiveVersion() int { return k.activeVersion }

// Versions returns the configured versions, sorted ascending. Handy for
// diagnostics and for rotation tooling deciding which legacy prefixes to
// scan for.
func (k *KeyRing) Versions() []int {
	out := make([]int, 0, len(k.ciphers))
	for v := range k.ciphers {
		out = append(out, v)
	}
	sort.Ints(out)
	return out
}

// HasVersion reports whether the KeyRing has a key for the given version.
func (k *KeyRing) HasVersion(v int) bool {
	_, ok := k.ciphers[v]
	return ok
}

// Encrypt seals plaintext under the active version's key and prepends
// "v{N}:" so Decrypt knows which key to use later. The prefix is ASCII to
// make on-disk rows greppable without needing the key material.
func (k *KeyRing) Encrypt(plaintext []byte) ([]byte, error) {
	c, ok := k.ciphers[k.activeVersion]
	if !ok {
		return nil, fmt.Errorf("crypto: active version %d missing from KeyRing", k.activeVersion)
	}
	sealed, err := c.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	prefix := versionPrefix(k.activeVersion)
	out := make([]byte, 0, len(prefix)+len(sealed))
	out = append(out, prefix...)
	out = append(out, sealed...)
	return out, nil
}

// Decrypt reads the "v{N}:" prefix to decide which key to use. Ciphertext
// without a recognizable prefix is assumed to be legacy v1 output (the
// on-wire format predating KeyRing). On an unknown version it returns an
// error rather than silently trying the active key — mismatched key IDs
// almost always mean misconfiguration and we want to fail loud.
func (k *KeyRing) Decrypt(ciphertext []byte) ([]byte, error) {
	version, body := parseVersionPrefix(ciphertext)
	c, ok := k.ciphers[version]
	if !ok {
		return nil, fmt.Errorf("crypto: no key configured for ciphertext version v%d", version)
	}
	return c.Decrypt(body)
}

// versionPrefix returns the ASCII "v{N}:" prefix for a given version.
func versionPrefix(v int) []byte {
	// Fast path for realistic version counts avoids Sprintf allocation.
	return []byte("v" + strconv.Itoa(v) + string(versionPrefixSep))
}

// parseVersionPrefix looks for a leading "v{N}:" prefix where N is a
// positive base-10 integer. If found, it returns (N, body). Otherwise it
// returns (1, input) so legacy unprefixed ciphertext routes to v1.
//
// The parser is conservative: it only accepts ASCII 'v', then 1+ digits,
// then ':'. Raw ciphertext starting with the byte 'v' but not followed by
// digits is left alone.
func parseVersionPrefix(ciphertext []byte) (int, []byte) {
	const minPrefix = 3 // v1:
	if len(ciphertext) < minPrefix || ciphertext[0] != 'v' {
		return 1, ciphertext
	}
	// Scan digits.
	i := 1
	for i < len(ciphertext) && ciphertext[i] >= '0' && ciphertext[i] <= '9' {
		i++
	}
	if i == 1 || i >= len(ciphertext) || ciphertext[i] != versionPrefixSep {
		return 1, ciphertext
	}
	v, err := strconv.Atoi(string(ciphertext[1:i]))
	if err != nil || v <= 0 {
		return 1, ciphertext
	}
	return v, ciphertext[i+1:]
}

// KeyRingFromEnv reads one or more CMDB_SECRET_KEY_V{N} env vars and an
// optional CMDB_SECRET_KEY_ACTIVE selector, returning a KeyRing. If no
// versioned vars are set it falls back to CMDB_SECRET_KEY (treated as v1),
// preserving the pre-rotation zero-config behaviour.
//
// Errors:
//   - no keys configured (neither versioned nor legacy)
//   - any configured key has an invalid hex value or wrong length
//   - CMDB_SECRET_KEY_ACTIVE is set but not a positive integer, or points
//     at a version that wasn't configured
func KeyRingFromEnv() (*KeyRing, error) {
	ciphers := map[int]Cipher{}

	// Scan versioned slots. A deployment in the middle of rotation might
	// have v1 and v2 both set; a fresh deployment adopting rotation has
	// just v1. The cap is checked at the end so an operator who *only*
	// sets V5 still works.
	for v := 1; v <= maxScannedVersion; v++ {
		envName := fmt.Sprintf("%s%d", versionedKeyEnvPrefix, v)
		raw, ok := os.LookupEnv(envName)
		if !ok || raw == "" {
			continue
		}
		c, err := CipherFromEnv(envName)
		if err != nil {
			return nil, err
		}
		ciphers[v] = c
	}

	// Legacy fallback: if no versioned slot was set, accept the old
	// single-key var. Treating it as v1 means any ciphertext that was
	// written under the legacy single-key regime still decrypts —
	// unprefixed bytes route to v1 in parseVersionPrefix above, and v1
	// now points at the same key.
	if len(ciphers) == 0 {
		if raw, ok := os.LookupEnv(legacyKeyEnv); ok && raw != "" {
			c, err := CipherFromEnv(legacyKeyEnv)
			if err != nil {
				return nil, err
			}
			ciphers[1] = c
		}
	}

	if len(ciphers) == 0 {
		return nil, fmt.Errorf("crypto: no encryption keys configured (set %s or %sN)", legacyKeyEnv, versionedKeyEnvPrefix)
	}

	active, err := resolveActiveVersion(ciphers)
	if err != nil {
		return nil, err
	}

	return NewKeyRing(ciphers, active)
}

// resolveActiveVersion picks which version Encrypt should use, respecting
// CMDB_SECRET_KEY_ACTIVE if set and otherwise defaulting to the highest
// configured version. Defaulting to the highest version is intentional: a
// deploy that adds CMDB_SECRET_KEY_V2 but forgets to flip ACTIVE still
// starts encrypting under v2, which is almost certainly what the operator
// wanted. Old ciphertext continues to decrypt under v1 via the prefix.
func resolveActiveVersion(ciphers map[int]Cipher) (int, error) {
	if raw, ok := os.LookupEnv(activeVersionEnv); ok && raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil || v <= 0 {
			return 0, fmt.Errorf("crypto: %s=%q is not a positive integer", activeVersionEnv, raw)
		}
		if _, ok := ciphers[v]; !ok {
			return 0, fmt.Errorf("crypto: %s=%d but %s%d is not set", activeVersionEnv, v, versionedKeyEnvPrefix, v)
		}
		return v, nil
	}
	highest := 0
	for v := range ciphers {
		if v > highest {
			highest = v
		}
	}
	return highest, nil
}
