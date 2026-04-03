package asset

import (
	"encoding/json"

	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
)

// AssetEvent represents a domain event related to an asset.
type AssetEvent struct {
	AssetID  uuid.UUID `json:"asset_id"`
	TenantID uuid.UUID `json:"tenant_id"`
	Action   string    `json:"action"`
}

// NewAssetEvent creates a new event bus event for an asset action.
func NewAssetEvent(assetID, tenantID uuid.UUID, subject string) eventbus.Event {
	payload, _ := json.Marshal(AssetEvent{AssetID: assetID, TenantID: tenantID, Action: subject})
	return eventbus.Event{Subject: subject, TenantID: tenantID.String(), Payload: payload}
}
