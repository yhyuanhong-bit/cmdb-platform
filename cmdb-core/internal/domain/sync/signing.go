// signing.go — Phase 4.3 HMAC-SHA256 signing for SyncEnvelope.
//
// The v2 checksum (see envelope.go) detects corruption but does not detect
// forgery: any node with JetStream publish capability can recompute the
// fingerprint after tampering. This file adds a keyed HMAC over the same
// canonical string, so only nodes holding the shared secret can produce an
// envelope receivers will accept.
//
// Key management
//
//   Primary key — set via CMDB_SYNC_HMAC_KEY. Used for SIGNING every new
//                 envelope on the publish path, AND for VERIFY on the
//                 receive path. Its kid is the blake-free SHA-256 prefix
//                 of the raw key material, hex-encoded and truncated to 8
//                 chars; stored on every envelope as env.SigKID so the
//                 receiver can pick the right key without guessing.
//
//   Previous keys — set via CMDB_SYNC_HMAC_KEY_PREVIOUS, comma-separated.
//                   Accepted for VERIFY only; never used for signing.
//                   Lets operators rotate the primary key without a fleet
//                   restart storm: deploy new primary + move old to
//                   previous, drain, remove.
//
// Fail-closed policy
//
//   When a keyring is configured, envelopes MUST carry a valid signature.
//   Unsigned or bad-signature traffic is dropped with a counter bump. When
//   NO keyring is configured (CMDB_SYNC_HMAC_KEY unset), signing is a no-op
//   and verification is skipped — a rollout affordance so the grace window
//   before every edge node has the key does not black-hole sync traffic.
//   A startup WARN log surfaces the unsigned state so operators notice.
//
//   Production deployments MUST set CMDB_SYNC_HMAC_KEY. The unsigned path
//   is a rollout escape hatch, not a supported long-term configuration.
package sync

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// HMACKey wraps raw key material + a stable identifier. The identifier is
// emitted on every signed envelope as env.SigKID so receivers can pick the
// right key during rotation windows without having to try every candidate.
type HMACKey struct {
	// KID is a short, stable identifier for the key — first 8 hex chars
	// of SHA-256(raw). Safe to log (non-reversible) and stable across
	// restarts.
	KID string

	// raw is the secret bytes. Never logged, never serialized, never
	// exposed via getter. Consumed only inside this file by hmacSum.
	raw []byte
}

// newHMACKey constructs an HMACKey with the canonical kid derived from
// SHA-256(raw)[0:8]. Accepts raw bytes directly so callers can choose
// whatever encoding they like (env: UTF-8 text; future: base64 from secret
// store).
func newHMACKey(raw []byte) HMACKey {
	sum := sha256.Sum256(raw)
	return HMACKey{
		KID: hex.EncodeToString(sum[:])[:8],
		raw: raw,
	}
}

// KeyRing holds one primary (signing) key and zero or more previous keys
// (verify-only). The ring is immutable after construction so the hot path
// does not need locking — rotation publishes a new *KeyRing atomically via
// the package-level pointer.
type KeyRing struct {
	primary  HMACKey
	previous []HMACKey
}

// activeKeyRing is the package-level keyring. Set exactly once during
// startup via ConfigureKeyRing. Nil means "no keyring configured" —
// signing/verification become no-ops, see the fail-closed policy at the
// top of the file.
//
// Stored as atomic.Pointer so a test that calls ConfigureKeyRing on each
// setup does not race with another test's envelope verification.
var activeKeyRing atomic.Pointer[KeyRing]

// ConfigureKeyRing installs the keyring used for signing and verifying
// envelopes. Passing nil clears the ring (test affordance, not a
// production path). Safe to call concurrently — replaces the pointer
// atomically.
func ConfigureKeyRing(kr *KeyRing) {
	activeKeyRing.Store(kr)
}

// ActiveKeyRing returns the currently-installed keyring, or nil when none
// is configured. Exported for tests only; production code should go
// through Sign / Verify.
func ActiveKeyRing() *KeyRing {
	return activeKeyRing.Load()
}

// LoadKeyRingFromEnv builds a KeyRing from CMDB_SYNC_HMAC_KEY (required for
// signing) and CMDB_SYNC_HMAC_KEY_PREVIOUS (optional, comma-separated —
// verify-only). Returns (nil, nil) when CMDB_SYNC_HMAC_KEY is unset so
// callers can distinguish "not configured" from "misconfigured".
//
// Keys are required to be at least 32 bytes — a SHA-256 block. Shorter
// keys are accepted by HMAC spec but weaken the security margin below what
// operators reasonably expect when reading "HMAC-SHA256" on a config
// dashboard. Reject at load time so the gap never goes to prod.
func LoadKeyRingFromEnv() (*KeyRing, error) {
	primaryRaw := strings.TrimSpace(os.Getenv(envSyncHMACKey))
	if primaryRaw == "" {
		return nil, nil
	}
	if len(primaryRaw) < hmacMinKeyBytes {
		return nil, fmt.Errorf("%s must be at least %d bytes, got %d",
			envSyncHMACKey, hmacMinKeyBytes, len(primaryRaw))
	}
	kr := &KeyRing{primary: newHMACKey([]byte(primaryRaw))}

	if prev := strings.TrimSpace(os.Getenv(envSyncHMACKeyPrevious)); prev != "" {
		for _, p := range strings.Split(prev, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			if len(p) < hmacMinKeyBytes {
				return nil, fmt.Errorf("%s entry must be at least %d bytes, got %d",
					envSyncHMACKeyPrevious, hmacMinKeyBytes, len(p))
			}
			kr.previous = append(kr.previous, newHMACKey([]byte(p)))
		}
	}
	return kr, nil
}

// PrimaryKID returns the kid of the signing key. Used at startup so the
// operator can cross-check deployed fleet config against their secret
// store without shipping the raw key.
func (kr *KeyRing) PrimaryKID() string {
	if kr == nil {
		return ""
	}
	return kr.primary.KID
}

// PreviousKIDs returns kids of every verify-only key, in config order. Same
// non-sensitive, operator-facing purpose as PrimaryKID.
func (kr *KeyRing) PreviousKIDs() []string {
	if kr == nil {
		return nil
	}
	out := make([]string, 0, len(kr.previous))
	for _, k := range kr.previous {
		out = append(out, k.KID)
	}
	return out
}

// signingInputString returns the canonical byte-stable string that both
// Sign and Verify hash. Identical structure to computeChecksumV2 (kept in
// envelope.go) except the sig string ADDS the kid under the hash so a
// key-id swap cannot be spoofed. Diff is pre-hashed so JetStream hop
// re-serialization cannot disturb the outer hash.
func (e *SyncEnvelope) signingInputString(kid string) string {
	diffHash := sha256.Sum256(e.Diff)
	parts := []string{
		kid,
		e.ID,
		e.Source,
		e.TenantID,
		e.EntityType,
		e.EntityID,
		e.Action,
		fmt.Sprintf("%d", e.Version),
		e.Timestamp.UTC().Format(time.RFC3339Nano),
		fmt.Sprintf("%x", diffHash),
	}
	return strings.Join(parts, "\n")
}

// hmacSum computes HMAC-SHA256 over input with key. Kept as a small helper
// so both Sign and Verify produce identical tag bytes from the same
// signing-input string — the comparison must be exact for verification to
// succeed, so any canonicalization drift would silently reject every
// message.
func hmacSum(key []byte, input string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(input))
	return m.Sum(nil)
}

// Sign populates env.Signature + env.SigKID with an HMAC-SHA256 tag
// computed with the primary key. Called by the publish path right after
// NewEnvelope. If no keyring is configured this is a no-op — see the
// fail-closed policy at the top of the file.
func (kr *KeyRing) Sign(env *SyncEnvelope) {
	if kr == nil || env == nil {
		return
	}
	input := env.signingInputString(kr.primary.KID)
	env.SigKID = kr.primary.KID
	env.Signature = hex.EncodeToString(hmacSum(kr.primary.raw, input))
}

// VerifyResult is the verdict from Verify, distinct-enough to drive the
// telemetry.SyncEnvelopeRejected reason label without stringifying errors.
type VerifyResult int

const (
	// VerifyOK means the signature matched one of the configured keys.
	VerifyOK VerifyResult = iota

	// VerifySigMissing means env.Signature or env.SigKID was empty. The
	// receiver treats this as a drop in fail-closed mode. Corresponds to
	// the telemetry reason label "sig_missing".
	VerifySigMissing

	// VerifyUnknownKID means env.SigKID matched neither primary nor any
	// previous key. Counter label "sig_unknown_kid".
	VerifyUnknownKID

	// VerifyBadSignature means the kid matched a known key but the HMAC
	// tag did not. This is the "someone tried to forge" outcome.
	// Counter label "bad_signature".
	VerifyBadSignature
)

// String maps VerifyResult onto the telemetry reason label. Single source
// of truth so the counter + log + metric stay consistent.
func (r VerifyResult) String() string {
	switch r {
	case VerifyOK:
		return "ok"
	case VerifySigMissing:
		return "sig_missing"
	case VerifyUnknownKID:
		return "sig_unknown_kid"
	case VerifyBadSignature:
		return "bad_signature"
	default:
		return "unknown"
	}
}

// Verify checks the envelope's signature against the keyring. Returns
// VerifyOK when the tag validates against either the primary or any
// previous key. Constant-time comparison prevents timing-channel leaks
// about which byte of the tag diverged.
//
// When kr is nil (no keyring configured), Verify returns VerifyOK — see
// the fail-closed policy at the top of the file for why this is an
// explicit rollout affordance rather than an accidental bypass.
func (kr *KeyRing) Verify(env *SyncEnvelope) VerifyResult {
	if kr == nil || env == nil {
		return VerifyOK
	}
	if env.Signature == "" || env.SigKID == "" {
		return VerifySigMissing
	}

	// Find the candidate key by kid. Primary first — the hot path — then
	// each previous key in configuration order.
	var key []byte
	switch {
	case env.SigKID == kr.primary.KID:
		key = kr.primary.raw
	default:
		for _, p := range kr.previous {
			if p.KID == env.SigKID {
				key = p.raw
				break
			}
		}
	}
	if key == nil {
		return VerifyUnknownKID
	}

	gotTag, err := hex.DecodeString(env.Signature)
	if err != nil {
		return VerifyBadSignature
	}
	wantTag := hmacSum(key, env.signingInputString(env.SigKID))
	if !hmac.Equal(gotTag, wantTag) {
		return VerifyBadSignature
	}
	return VerifyOK
}

// Tunable constants kept in one place so a future secret-store migration
// can cover them in one sweep.
const (
	envSyncHMACKey         = "CMDB_SYNC_HMAC_KEY"
	envSyncHMACKeyPrevious = "CMDB_SYNC_HMAC_KEY_PREVIOUS"

	// hmacMinKeyBytes is the minimum acceptable key length. SHA-256's
	// block size is 64 bytes; picking 32 is a middle ground that refuses
	// trivially-short keys without forcing operators into full-block
	// keys they'd need a CSPRNG to generate. Operators MUST source the
	// key from a CSPRNG (secret store, `openssl rand -base64 48`, etc.).
	hmacMinKeyBytes = 32
)
