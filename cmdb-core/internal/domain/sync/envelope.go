// envelope.go
package sync

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SyncEnvelope wraps a single entity change for sync transport.
//
// Two checksums coexist during the Phase 4.3 rollout window:
//
//   Checksum   (v1, legacy) — sha256(EntityID|Version|Diff). Does NOT cover
//              Source / TenantID / Action / Timestamp / EntityType, so a
//              forwarder can flip any of those four and keep the v1 sum
//              valid. Retained only so receivers still running old code
//              during a rolling deploy don't start rejecting legitimate
//              traffic mid-rollout.
//
//   ChecksumV2 — sha256(ID|Source|TenantID|EntityType|EntityID|Action|
//                       Version|Timestamp(RFC3339Nano UTC)|sha256(Diff)).
//              Covers every field on the envelope, including the four
//              previously-uncovered ones. VerifyChecksum prefers v2 when
//              it is populated.
//
// Grace window policy: NewEnvelope populates BOTH; VerifyChecksum accepts
// either. When v2 is present it is authoritative (tampering any covered
// field flips the sum). When v2 is absent we fall back to v1 so old-sender
// envelopes in flight during a rolling deploy still apply. Once the whole
// fleet has been rolled AND the JetStream MaxAge window has elapsed (see
// eventbus/nats.go CMDB_SYNC stream: 14 days), the v1-only fallback can
// be removed and computeChecksum + the Checksum field deleted.
type SyncEnvelope struct {
	ID         string          `json:"id"`
	Source     string          `json:"source"`
	TenantID   string          `json:"tenant_id"`
	EntityType string          `json:"entity_type"`
	EntityID   string          `json:"entity_id"`
	Action     string          `json:"action"`
	Version    int64           `json:"version"`
	Timestamp  time.Time       `json:"timestamp"`
	Diff       json.RawMessage `json:"diff"`
	Checksum   string          `json:"checksum"`
	// ChecksumV2 widens the integrity fingerprint to cover Source,
	// TenantID, Action, Timestamp, EntityType, and ID — fields the
	// legacy Checksum left uncovered. Omitted on-wire when empty so a
	// v1-only receiver parsing a v2 envelope sees a familiar shape.
	ChecksumV2 string `json:"checksum_v2,omitempty"`
}

// NewEnvelope creates a SyncEnvelope with both checksum fingerprints
// populated. Callers should not compute either hash by hand.
func NewEnvelope(source, tenantID, entityType, entityID, action string, version int64, diff json.RawMessage) SyncEnvelope {
	env := SyncEnvelope{
		ID:         uuid.New().String(),
		Source:     source,
		TenantID:   tenantID,
		EntityType: entityType,
		EntityID:   entityID,
		Action:     action,
		Version:    version,
		Timestamp:  time.Now(),
		Diff:       diff,
	}
	env.Checksum = env.computeChecksum()
	env.ChecksumV2 = env.computeChecksumV2()
	return env
}

// computeChecksum (v1) — legacy fingerprint kept only for the grace-window
// back-compat branch in VerifyChecksum. Do not extend it; use v2 instead.
func (e *SyncEnvelope) computeChecksum() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", e.EntityID, e.Version, string(e.Diff))))
	return fmt.Sprintf("%x", h)
}

// computeChecksumV2 — canonical integrity fingerprint covering every
// mutable field on the envelope.
//
// Canonical form: kid-less counterpart of the HMAC canonical string
// described in docs/reports/phase4/4.3-sync-envelope-hmac-signing.md §D3.
// Diff is hashed first (not embedded raw) so JSON re-serialization
// differences across JetStream hops cannot disturb the top-level hash.
// Timestamp is normalized to RFC3339Nano UTC for cross-timezone stability.
func (e *SyncEnvelope) computeChecksumV2() string {
	diffHash := sha256.Sum256(e.Diff)
	parts := []string{
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
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return fmt.Sprintf("%x", sum)
}

// VerifyChecksum returns true iff the envelope's recorded fingerprint is
// consistent with its current payload. When ChecksumV2 is present it is
// authoritative (any of the nine covered fields will trip tampering). When
// it is absent (old-sender envelope mid-rollout) we fall back to the
// legacy v1 fingerprint so in-flight traffic is not dropped.
//
// This dual-verify branch is a temporary back-compat window — see the
// package-level comment on SyncEnvelope for the removal plan.
func (e *SyncEnvelope) VerifyChecksum() bool {
	if e.ChecksumV2 != "" {
		return e.ChecksumV2 == e.computeChecksumV2()
	}
	return e.Checksum == e.computeChecksum()
}
