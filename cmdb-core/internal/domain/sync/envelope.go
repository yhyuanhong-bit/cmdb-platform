// envelope.go
package sync

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SyncEnvelope wraps a single entity change for sync transport.
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
}

// NewEnvelope creates a SyncEnvelope with computed checksum.
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
	return env
}

func (e *SyncEnvelope) computeChecksum() string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%d:%s", e.EntityID, e.Version, string(e.Diff))))
	return fmt.Sprintf("%x", h)
}

// VerifyChecksum returns true if the checksum matches.
func (e *SyncEnvelope) VerifyChecksum() bool {
	return e.Checksum == e.computeChecksum()
}
