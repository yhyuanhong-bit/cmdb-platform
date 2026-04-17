package api

import (
	"context"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
)

// integrationService is the narrow interface the api package depends on for
// integration adapter and webhook CRUD + delivery history. It matches a subset
// of *integration.Service so handlers can be unit-tested with a mock.
type integrationService interface {
	ListAdapters(ctx context.Context, tenantID uuid.UUID) ([]dbgen.IntegrationAdapter, error)
	GetAdapterByID(ctx context.Context, id, tenantID uuid.UUID) (dbgen.IntegrationAdapter, error)
	CreateAdapter(ctx context.Context, params dbgen.CreateAdapterParams) (dbgen.IntegrationAdapter, error)
	UpdateAdapter(ctx context.Context, params dbgen.UpdateAdapterParams) (dbgen.IntegrationAdapter, error)
	DeleteAdapter(ctx context.Context, id, tenantID uuid.UUID) error

	ListWebhooks(ctx context.Context, tenantID uuid.UUID) ([]dbgen.WebhookSubscription, error)
	GetWebhookByID(ctx context.Context, id, tenantID uuid.UUID) (dbgen.WebhookSubscription, error)
	CreateWebhook(ctx context.Context, params dbgen.CreateWebhookParams) (dbgen.WebhookSubscription, error)
	UpdateWebhook(ctx context.Context, params dbgen.UpdateWebhookParams) (dbgen.WebhookSubscription, error)
	DeleteWebhook(ctx context.Context, id, tenantID uuid.UUID) error

	ListDeliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]dbgen.WebhookDelivery, error)
}
